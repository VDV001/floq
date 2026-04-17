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

func (r *Repository) ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.ProspectWithSource, error) {
	rows, err := r.q(ctx).Query(ctx,
		`SELECT p.id, p.user_id, p.name, p.company, p.title, p.email, p.phone, p.whatsapp, p.telegram_username, p.industry, p.company_size, p.context,
		        p.source, p.source_id, COALESCE(ls.name, ''), p.status, p.verify_status, p.verify_score, p.verify_details, p.verified_at, p.converted_lead_id, p.created_at, p.updated_at
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
		if err := rows.Scan(&item.ID, &item.UserID, &item.Name, &item.Company, &item.Title, &item.Email, &item.Phone, &item.WhatsApp, &item.TelegramUsername, &item.Industry, &item.CompanySize, &item.Context,
			&item.Source, &item.SourceID, &item.SourceName, &item.Status, &item.VerifyStatus, &item.VerifyScore, &item.VerifyDetails, &item.VerifiedAt, &item.ConvertedLeadID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prospect: %w", err)
		}
		prospects = append(prospects, item)
	}
	return prospects, rows.Err()
}

func (r *Repository) GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error) {
	var p domain.Prospect
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
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
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE id = $1 AND user_id = $2`, prospectID, userID).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
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
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE user_id = $1 AND LOWER(email) = LOWER($2) LIMIT 1`, userID, email).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
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
	err := r.q(ctx).QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE user_id = $1 AND LOWER(telegram_username) = LOWER($2) LIMIT 1`, userID, username).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prospect by telegram username: %w", err)
	}
	return &p, nil
}

func (r *Repository) CreateProspect(ctx context.Context, p *domain.Prospect) error {
	_, err := r.q(ctx).Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		                        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)`,
		p.ID, p.UserID, p.Name, p.Company, p.Title, p.Email, p.Phone, p.WhatsApp, p.TelegramUsername, p.Industry, p.CompanySize, p.Context,
		p.Source, p.SourceID, p.Status, p.VerifyStatus, p.VerifyScore, p.VerifyDetails, p.VerifiedAt, p.ConvertedLeadID, p.CreatedAt, p.UpdatedAt)
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

func (r *Repository) UpdateVerification(ctx context.Context, id uuid.UUID, verifyStatus domain.VerifyStatus, verifyScore int, verifyDetails string, verifiedAt time.Time) error {
	_, err := r.q(ctx).Exec(ctx,
		`UPDATE prospects SET verify_status = $2, verify_score = $3, verify_details = $4, verified_at = $5, updated_at = $6 WHERE id = $1`,
		id, verifyStatus, verifyScore, verifyDetails, verifiedAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update prospect verification: %w", err)
	}
	return nil
}
