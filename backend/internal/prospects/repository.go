package prospects

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/prospects/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that Repository implements domain.Repository.
var _ domain.Repository = (*Repository)(nil)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// q returns the Querier bound to the current context: a pgx.Tx when the
// caller wrapped the call in db.TxManager.WithTx, otherwise the pool.
func (r *Repository) q(ctx context.Context) db.Querier {
	return db.ConnFromCtx(ctx, r.pool)
}

// prospectColumns is the column list shared by every single-row prospect
// SELECT, in the exact order scanProspect expects. Centralised so the SELECT
// and Scan lists cannot drift apart and silently drop the consent state — a
// legal risk for a compliance field.
const prospectColumns = `id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
	        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, consent_status, consent_source, consent_at, created_at, updated_at`

// scanProspect scans prospectColumns from row into p, mapping the nullable
// consent_at into the Consent VO (NULL → zero timestamp, the 'none' case).
func scanProspect(row pgx.Row, p *domain.Prospect) error {
	var consentAt *time.Time
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
		&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.Consent.Status, &p.Consent.Source, &consentAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return err
	}
	if consentAt != nil {
		p.Consent.Timestamp = *consentAt
	}
	return nil
}

func (r *Repository) ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT p.id, p.user_id, p.name, p.company, p.title, p.email, p.phone, p.whatsapp, p.telegram_username, p.industry, p.company_size, p.context,
		        p.source, p.source_id, COALESCE(ls.name, ''), p.status, p.verify_status, p.verify_score, p.verify_details, p.verified_at, p.converted_lead_id, p.consent_status, p.consent_source, p.consent_at, p.created_at, p.updated_at
		 FROM prospects p
		 LEFT JOIN lead_sources ls ON ls.id = p.source_id
		 WHERE p.user_id = $1 ORDER BY p.created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list prospects: %w", err)
	}
	defer rows.Close()

	var prospects []domain.ProspectWithSource
	for rows.Next() {
		var item domain.ProspectWithSource
		var consentAt *time.Time
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Company, &item.Title, &item.Email, &item.Phone, &item.WhatsApp, &item.TelegramUsername, &item.Industry, &item.CompanySize, &item.Context,
			&item.Source, &item.SourceID, &item.SourceName, &item.Status, &item.VerifyStatus, &item.VerifyScore, &item.VerifyDetails, &item.VerifiedAt, &item.ConvertedLeadID, &item.Consent.Status, &item.Consent.Source, &consentAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prospect: %w", err)
		}
		if consentAt != nil {
			item.Consent.Timestamp = *consentAt
		}
		prospects = append(prospects, item)
	}
	return prospects, rows.Err()
}

func (r *Repository) GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error) {
	var p domain.Prospect
	err := scanProspect(r.q(ctx).QueryRow(ctx,
		`SELECT `+prospectColumns+`
		 FROM prospects WHERE id = $1`, id), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prospect: %w", err)
	}
	return &p, nil
}

// GetProspectForUser returns the prospect iff it exists AND belongs to userID.
// Returns (nil, nil) on missing or foreign — callers map this to
// leads/domain.ErrProspectNotFound to avoid leaking cross-tenant existence.
func (r *Repository) GetProspectForUser(ctx context.Context, userID, prospectID uuid.UUID) (*domain.Prospect, error) {
	var p domain.Prospect
	err := scanProspect(r.q(ctx).QueryRow(ctx,
		`SELECT `+prospectColumns+`
		 FROM prospects WHERE id = $1 AND user_id = $2`, prospectID, userID), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prospect for user: %w", err)
	}
	return &p, nil
}

func (r *Repository) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*domain.Prospect, error) {
	var p domain.Prospect
	err := scanProspect(r.q(ctx).QueryRow(ctx,
		`SELECT `+prospectColumns+`
		 FROM prospects WHERE user_id = $1 AND LOWER(email) = LOWER($2) LIMIT 1`, userID, email), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prospect by email: %w", err)
	}
	return &p, nil
}

func (r *Repository) FindByTelegramUsername(ctx context.Context, userID uuid.UUID, username string) (*domain.Prospect, error) {
	var p domain.Prospect
	err := scanProspect(r.q(ctx).QueryRow(ctx,
		`SELECT `+prospectColumns+`
		 FROM prospects WHERE user_id = $1 AND LOWER(telegram_username) = LOWER($2) LIMIT 1`, userID, username), &p)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prospect by telegram username: %w", err)
	}
	return &p, nil
}

func (r *Repository) CreateProspect(ctx context.Context, p *domain.Prospect) error {
	// consent_at is NULL for 'none' (no basis recorded); a zero time.Time would
	// otherwise persist as '0001-01-01' and read back non-NULL.
	var consentAt *time.Time
	if !p.Consent.Timestamp.IsZero() {
		consentAt = &p.Consent.Timestamp
	}
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		                        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at,
		                        consent_status, consent_source, consent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`,
		p.ID, p.UserID, p.Name, p.Company, p.Title, p.Email, p.Phone, p.WhatsApp, p.TelegramUsername, p.Industry, p.CompanySize, p.Context,
		p.Source, p.SourceID, p.Status, p.VerifyStatus, p.VerifyScore, p.VerifyDetails, p.VerifiedAt, p.ConvertedLeadID, p.CreatedAt, p.UpdatedAt,
		p.Consent.Status, p.Consent.Source, consentAt)
	if err != nil {
		return fmt.Errorf("create prospect: %w", err)
	}
	return nil
}

func (r *Repository) CreateProspectsBatch(ctx context.Context, prospects []domain.Prospect) error {
	for i := range prospects {
		if err := r.CreateProspect(ctx, &prospects[i]); err != nil {
			return fmt.Errorf("create prospects batch [%d]: %w", i, err)
		}
	}
	return nil
}

func (r *Repository) DeleteProspect(ctx context.Context, id uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`DELETE FROM prospects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete prospect: %w", err)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ProspectStatus) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE prospects SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update prospect status: %w", err)
	}
	return nil
}

func (r *Repository) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE prospects SET status = $2, converted_lead_id = $3, updated_at = $4 WHERE id = $1`,
		prospectID, domain.ProspectStatusConverted, leadID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("convert prospect to lead: %w", err)
	}
	return nil
}

// UpdateConsent persists a consent transition (grant/withdraw) on an existing
// prospect. The nullable consent_at maps a zero timestamp to NULL, mirroring
// CreateProspect.
func (r *Repository) UpdateConsent(ctx context.Context, prospectID uuid.UUID, c domain.Consent) error {
	var consentAt *time.Time
	if !c.Timestamp.IsZero() {
		consentAt = &c.Timestamp
	}
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE prospects SET consent_status = $2, consent_source = $3, consent_at = $4, updated_at = $5 WHERE id = $1`,
		prospectID, c.Status, c.Source, consentAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update consent: %w", err)
	}
	return nil
}

// AddSuppression records that an address must never be contacted again on its
// channel. Idempotent: a repeated unsubscribe collapses to a no-op rather than
// a unique-constraint error.
func (r *Repository) AddSuppression(ctx context.Context, s *domain.Suppression) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO suppressions (id, user_id, channel, address, reason, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, channel, address) DO NOTHING`,
		s.ID, s.UserID, s.Channel, s.Address, s.Reason, s.CreatedAt)
	if err != nil {
		return fmt.Errorf("add suppression: %w", err)
	}
	return nil
}

// IsSuppressed reports whether address is on the suppression list for userID on
// the given channel. The address is normalized the same way it was stored so
// the lookup is case-insensitive.
func (r *Repository) IsSuppressed(ctx context.Context, userID uuid.UUID, channel domain.SuppressionChannel, address string) (bool, error) {
	addr := domain.NormalizeSuppressionAddress(channel, address)
	var exists bool
	err := r.q(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM suppressions WHERE user_id = $1 AND channel = $2 AND address = $3)`,
		userID, channel, addr).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is suppressed: %w", err)
	}
	return exists, nil
}

func (r *Repository) UpdateVerification(ctx context.Context, id uuid.UUID, verifyStatus domain.VerifyStatus, verifyScore int, verifyDetails string, verifiedAt time.Time) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE prospects SET verify_status = $2, verify_score = $3, verify_details = $4, verified_at = $5, updated_at = $6 WHERE id = $1`,
		id, verifyStatus, verifyScore, verifyDetails, verifiedAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update prospect verification: %w", err)
	}
	return nil
}
