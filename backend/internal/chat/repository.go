package chat

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements StatsReader using PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FetchStats(ctx context.Context, userID uuid.UUID) (*userStats, error) {
	s := &userStats{StatusCounts: make(map[string]int)}

	// Total leads
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE user_id = $1`, userID).Scan(&s.TotalLeads)
	if err != nil {
		return nil, fmt.Errorf("total leads: %w", err)
	}

	// Leads this month
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM leads WHERE user_id = $1 AND created_at >= date_trunc('month', CURRENT_DATE)`,
		userID).Scan(&s.MonthLeads)
	if err != nil {
		return nil, fmt.Errorf("month leads: %w", err)
	}

	// Leads by status
	rows, err := r.pool.Query(ctx,
		`SELECT status::text, COUNT(*) FROM leads WHERE user_id = $1 GROUP BY status`, userID)
	if err != nil {
		return nil, fmt.Errorf("status counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("scan status: %w", err)
		}
		s.StatusCounts[status] = count
	}

	// Prospect count
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prospects WHERE user_id = $1`, userID).Scan(&s.ProspectCount)
	if err != nil {
		return nil, fmt.Errorf("prospects: %w", err)
	}

	// Sequence count
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sequences WHERE user_id = $1`, userID).Scan(&s.SequenceCount)
	if err != nil {
		return nil, fmt.Errorf("sequences: %w", err)
	}

	// Queued outbound messages (draft = awaiting approval)
	err = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM outbound_messages WHERE status = 'draft' AND sequence_id IN (SELECT id FROM sequences WHERE user_id = $1)`,
		userID).Scan(&s.QueuedMsgs)
	if err != nil {
		s.QueuedMsgs = 0
	}

	// Recent leads (last 10)
	recentRows, err := r.pool.Query(ctx,
		`SELECT contact_name, COALESCE(company, ''), status::text, created_at FROM leads WHERE user_id = $1 ORDER BY created_at DESC LIMIT 10`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("recent leads: %w", err)
	}
	defer recentRows.Close()
	for recentRows.Next() {
		var l recentLead
		if err := recentRows.Scan(&l.Name, &l.Company, &l.Status, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent lead: %w", err)
		}
		s.RecentLeads = append(s.RecentLeads, l)
	}

	return s, nil
}
