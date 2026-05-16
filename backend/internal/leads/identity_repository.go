package leads

import (
	"context"
	"errors"
	"fmt"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/leads/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that IdentityRepository satisfies the domain port.
var _ domain.IdentityRepository = (*IdentityRepository)(nil)

// IdentityRepository persists domain.Identity rows in the `identities`
// table and looks them up by each canonical identifier. Lookups are
// scoped to a user_id so identical handles owned by different users
// stay isolated.
type IdentityRepository struct {
	pool *pgxpool.Pool
}

// NewIdentityRepository wires the SQL-backed implementation.
func NewIdentityRepository(pool *pgxpool.Pool) *IdentityRepository {
	return &IdentityRepository{pool: pool}
}

// q returns the Querier bound to the current context (a pgx.Tx when
// the caller wrapped the call in db.TxManager.WithTx, otherwise the
// pool) — mirrors the pattern used by the existing Repository.
func (r *IdentityRepository) q(ctx context.Context) db.Querier {
	return db.ConnFromCtx(ctx, r.pool)
}

func (r *IdentityRepository) Save(ctx context.Context, id *domain.Identity) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO identities (id, user_id, email, phone, telegram_username, created_at)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6)`,
		id.ID, id.UserID, id.Email, id.Phone, id.TelegramUsername, id.CreatedAt)
	if err != nil {
		return fmt.Errorf("save identity: %w", err)
	}
	return nil
}

const findIdentitySelect = `SELECT id, user_id, COALESCE(email, ''), COALESCE(phone, ''), COALESCE(telegram_username, ''), created_at
 FROM identities WHERE user_id = $1 AND `

func (r *IdentityRepository) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*domain.Identity, error) {
	if email == "" {
		return nil, nil
	}
	return r.scanIdentity(ctx, findIdentitySelect+`email = $2`, userID, email, "email")
}

func (r *IdentityRepository) FindByPhone(ctx context.Context, userID uuid.UUID, phone string) (*domain.Identity, error) {
	if phone == "" {
		return nil, nil
	}
	return r.scanIdentity(ctx, findIdentitySelect+`phone = $2`, userID, phone, "phone")
}

func (r *IdentityRepository) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, tg string) (*domain.Identity, error) {
	if tg == "" {
		return nil, nil
	}
	return r.scanIdentity(ctx, findIdentitySelect+`telegram_username = $2`, userID, tg, "telegram_username")
}

func (r *IdentityRepository) LinkLead(ctx context.Context, leadID, identityID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO lead_identities (lead_id, identity_id) VALUES ($1, $2)
		 ON CONFLICT (lead_id, identity_id) DO NOTHING`,
		leadID, identityID)
	if err != nil {
		return fmt.Errorf("link lead identity: %w", err)
	}
	return nil
}

func (r *IdentityRepository) LinkProspect(ctx context.Context, prospectID, identityID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO prospect_identities (prospect_id, identity_id) VALUES ($1, $2)
		 ON CONFLICT (prospect_id, identity_id) DO NOTHING`,
		prospectID, identityID)
	if err != nil {
		return fmt.Errorf("link prospect identity: %w", err)
	}
	return nil
}

// GetByLeadID returns the Identity linked to the given lead via
// lead_identities, or (nil, nil) if the lead has no link row yet.
// One row per lead is the contract — the link table's PK guarantees
// uniqueness, but if multiple links sneak in (operator-driven manual
// merge in a future phase) the LIMIT 1 keeps the call deterministic.
func (r *IdentityRepository) GetByLeadID(ctx context.Context, leadID uuid.UUID) (*domain.Identity, error) {
	var id domain.Identity
	err := r.q(ctx).QueryRow(ctx,
		`SELECT i.id, i.user_id, COALESCE(i.email, ''), COALESCE(i.phone, ''), COALESCE(i.telegram_username, ''), i.created_at
		 FROM identities i
		 JOIN lead_identities li ON li.identity_id = i.id
		 WHERE li.lead_id = $1
		 ORDER BY li.linked_at ASC
		 LIMIT 1`, leadID).
		Scan(&id.ID, &id.UserID, &id.Email, &id.Phone, &id.TelegramUsername, &id.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("identity by lead_id: %w", err)
	}
	return &id, nil
}

// LinkedLeadIDs returns every lead currently associated with the
// identity. Order is by linked_at ASC so the original/triggering lead
// surfaces first — useful for "first contact" semantics in the UI.
func (r *IdentityRepository) LinkedLeadIDs(ctx context.Context, identityID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT lead_id FROM lead_identities WHERE identity_id = $1 ORDER BY linked_at ASC`,
		identityID)
	if err != nil {
		return nil, fmt.Errorf("linked lead_ids: %w", err)
	}
	defer rows.Close()
	out := make([]uuid.UUID, 0)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan lead_id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// scanIdentity runs a single-row identity SELECT and unifies the
// (nil, nil) on-not-found contract plus the error-wrapping label. The
// SQL argument is built from compile-time literals only (no runtime
// interpolation reaches the query string).
func (r *IdentityRepository) scanIdentity(ctx context.Context, sql string, userID uuid.UUID, value, label string) (*domain.Identity, error) {
	var id domain.Identity
	err := r.q(ctx).QueryRow(ctx, sql, userID, value).
		Scan(&id.ID, &id.UserID, &id.Email, &id.Phone, &id.TelegramUsername, &id.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find identity by %s: %w", label, err)
	}
	return &id, nil
}
