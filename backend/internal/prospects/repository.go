package prospects

import (
	"context"
	"fmt"
	"time"

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

func (r *Repository) ListProspects(ctx context.Context, userID uuid.UUID) ([]domain.Prospect, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list prospects: %w", err)
	}
	defer rows.Close()

	var prospects []domain.Prospect
	for rows.Next() {
		var p domain.Prospect
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prospect: %w", err)
		}
		prospects = append(prospects, p)
	}
	return prospects, rows.Err()
}

func (r *Repository) GetProspect(ctx context.Context, id uuid.UUID) (*domain.Prospect, error) {
	var p domain.Prospect
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prospect: %w", err)
	}
	return &p, nil
}

func (r *Repository) FindByEmail(ctx context.Context, userID uuid.UUID, email string) (*domain.Prospect, error) {
	var p domain.Prospect
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, phone, whatsapp, telegram_username, industry, company_size, context,
		        source, source_id, status, verify_status, verify_score, verify_details, verified_at, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE user_id = $1 AND LOWER(email) = LOWER($2) LIMIT 1`, userID, email).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Phone, &p.WhatsApp, &p.TelegramUsername, &p.Industry, &p.CompanySize, &p.Context,
			&p.Source, &p.SourceID, &p.Status, &p.VerifyStatus, &p.VerifyScore, &p.VerifyDetails, &p.VerifiedAt, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find prospect by email: %w", err)
	}
	return &p, nil
}

func (r *Repository) CreateProspect(ctx context.Context, p *domain.Prospect) error {
	_, err := r.pool.Exec(ctx,
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
	_, err := r.pool.Exec(ctx,
		`DELETE FROM prospects WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete prospect: %w", err)
	}
	return nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.ProspectStatus) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE prospects SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update prospect status: %w", err)
	}
	return nil
}

func (r *Repository) ConvertToLead(ctx context.Context, prospectID, leadID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE prospects SET status = 'converted', converted_lead_id = $2, updated_at = $3 WHERE id = $1`,
		prospectID, leadID, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("convert prospect to lead: %w", err)
	}
	return nil
}

func (r *Repository) UpdateVerification(ctx context.Context, id uuid.UUID, verifyStatus domain.VerifyStatus, verifyScore int, verifyDetails string, verifiedAt time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE prospects SET verify_status = $2, verify_score = $3, verify_details = $4, verified_at = $5, updated_at = $6 WHERE id = $1`,
		id, verifyStatus, verifyScore, verifyDetails, verifiedAt, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("update prospect verification: %w", err)
	}
	return nil
}
