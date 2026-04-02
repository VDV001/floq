package prospects

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Prospect struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	Name            string     `json:"name"`
	Company         string     `json:"company"`
	Title           string     `json:"title"`
	Email           string     `json:"email"`
	Source          string     `json:"source"`
	Status          string     `json:"status"`
	ConvertedLeadID *uuid.UUID `json:"converted_lead_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListProspects(ctx context.Context, userID uuid.UUID) ([]Prospect, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, user_id, name, company, title, email, source, status, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE user_id = $1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list prospects: %w", err)
	}
	defer rows.Close()

	var prospects []Prospect
	for rows.Next() {
		var p Prospect
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Source, &p.Status, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prospect: %w", err)
		}
		prospects = append(prospects, p)
	}
	return prospects, rows.Err()
}

func (r *Repository) GetProspect(ctx context.Context, id uuid.UUID) (*Prospect, error) {
	var p Prospect
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, name, company, title, email, source, status, converted_lead_id, created_at, updated_at
		 FROM prospects WHERE id = $1`, id).
		Scan(&p.ID, &p.UserID, &p.Name, &p.Company, &p.Title, &p.Email, &p.Source, &p.Status, &p.ConvertedLeadID, &p.CreatedAt, &p.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get prospect: %w", err)
	}
	return &p, nil
}

func (r *Repository) CreateProspect(ctx context.Context, p *Prospect) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO prospects (id, user_id, name, company, title, email, source, status, converted_lead_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		p.ID, p.UserID, p.Name, p.Company, p.Title, p.Email, p.Source, p.Status, p.ConvertedLeadID, p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create prospect: %w", err)
	}
	return nil
}

func (r *Repository) CreateProspectsBatch(ctx context.Context, prospects []Prospect) error {
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

func (r *Repository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
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
