package prospects

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// SuggestionRow is a read-model returned by FindSuggestionsForLead.
// Stays local to the prospects package — the composition root adapts it
// to the leads-facing DTO defined in leads/domain.
type SuggestionRow struct {
	ProspectID       uuid.UUID
	Name             string
	Company          string
	Email            string
	TelegramUsername string
	SourceName       string
	Status           string
	Confidence       string // "high" | "medium" | "low"
}

// FindSuggestionsForLead returns prospects whose normalized name matches leadName,
// excluding converted prospects and any (lead, prospect) pair recorded in
// prospect_suggestion_dismissals. Confidence is classified per row:
//   - high:   also matches on normalized company (both non-empty)
//   - medium: also matches on email domain (both sides have email)
//   - low:    name match only
//
// Rows are ordered high → medium → low, then newest first within each tier.
// Returns an empty slice if leadName is empty.
func (r *Repository) FindSuggestionsForLead(
	ctx context.Context,
	userID, leadID uuid.UUID,
	leadName, leadCompany, leadEmail string,
) ([]SuggestionRow, error) {
	const q = `
SELECT p.id, p.name, p.company, p.email, p.telegram_username,
       p.status, COALESCE(ls.name, '') AS source_name,
       CASE
         WHEN LOWER(TRIM(p.company)) <> ''
              AND LOWER(TRIM($3)) <> ''
              AND LOWER(TRIM(p.company)) = LOWER(TRIM($3))
         THEN 'high'
         WHEN p.email <> ''
              AND POSITION('@' IN $4) > 0
              AND POSITION('@' IN p.email) > 0
              AND LOWER(SPLIT_PART(p.email, '@', 2)) = LOWER(SPLIT_PART($4, '@', 2))
         THEN 'medium'
         ELSE 'low'
       END AS confidence
FROM prospects p
LEFT JOIN lead_sources ls ON ls.id = p.source_id
WHERE p.user_id = $1
  AND p.status <> 'converted'
  AND TRIM($2) <> ''
  AND LOWER(TRIM(p.name)) = LOWER(TRIM($2))
  AND NOT EXISTS (
    SELECT 1 FROM prospect_suggestion_dismissals d
    WHERE d.lead_id = $5 AND d.prospect_id = p.id
  )
ORDER BY
  CASE
    WHEN LOWER(TRIM(p.company)) <> ''
         AND LOWER(TRIM($3)) <> ''
         AND LOWER(TRIM(p.company)) = LOWER(TRIM($3))
    THEN 1
    WHEN p.email <> ''
         AND POSITION('@' IN $4) > 0
         AND POSITION('@' IN p.email) > 0
         AND LOWER(SPLIT_PART(p.email, '@', 2)) = LOWER(SPLIT_PART($4, '@', 2))
    THEN 2
    ELSE 3
  END,
  p.created_at DESC`

	rows, err := r.pool.Query(ctx, q, userID, leadName, leadCompany, leadEmail, leadID)
	if err != nil {
		return nil, fmt.Errorf("find prospect suggestions: %w", err)
	}
	defer rows.Close()

	var out []SuggestionRow
	for rows.Next() {
		var s SuggestionRow
		if err := rows.Scan(&s.ProspectID, &s.Name, &s.Company, &s.Email, &s.TelegramUsername,
			&s.Status, &s.SourceName, &s.Confidence); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CountSuggestionsForUser returns a map of lead_id → number of non-dismissed
// prospect suggestions (name match only — UI just needs to know "is there
// something to look at"). Only leads with count > 0 are included.
func (r *Repository) CountSuggestionsForUser(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int, error) {
	const q = `
SELECT l.id, COUNT(p.id)
FROM leads l
JOIN prospects p ON
     p.user_id = l.user_id
 AND p.status <> 'converted'
 AND TRIM(l.contact_name) <> ''
 AND LOWER(TRIM(p.name)) = LOWER(TRIM(l.contact_name))
 AND NOT EXISTS (
     SELECT 1 FROM prospect_suggestion_dismissals d
     WHERE d.lead_id = l.id AND d.prospect_id = p.id
 )
WHERE l.user_id = $1
GROUP BY l.id
HAVING COUNT(p.id) > 0`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("count prospect suggestions: %w", err)
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var leadID uuid.UUID
		var n int
		if err := rows.Scan(&leadID, &n); err != nil {
			return nil, fmt.Errorf("scan suggestion count: %w", err)
		}
		counts[leadID] = n
	}
	return counts, rows.Err()
}

// DismissSuggestion records that the user rejected a given prospect as a match
// for a given lead. Idempotent — re-dismissing is a no-op.
func (r *Repository) DismissSuggestion(ctx context.Context, leadID, prospectID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO prospect_suggestion_dismissals (lead_id, prospect_id)
		 VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`,
		leadID, prospectID)
	if err != nil {
		return fmt.Errorf("dismiss prospect suggestion: %w", err)
	}
	return nil
}
