package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/sources/domain"
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

func (r *Repository) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.CategoryWithSources, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.user_id, c.name, c.sort_order, c.created_at,
		       s.id, s.user_id, s.category_id, s.name, s.sort_order, s.created_at
		FROM source_categories c
		LEFT JOIN lead_sources s ON s.category_id = c.id
		WHERE c.user_id = $1
		ORDER BY c.sort_order, c.name, s.sort_order, s.name`, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	catMap := make(map[uuid.UUID]*domain.CategoryWithSources)
	var catOrder []uuid.UUID

	for rows.Next() {
		var c domain.Category
		var sID, sUserID, sCatID *uuid.UUID
		var sName *string
		var sSortOrder *int
		var sCreatedAt *time.Time

		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Name, &c.SortOrder, &c.CreatedAt,
			&sID, &sUserID, &sCatID, &sName, &sSortOrder, &sCreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category row: %w", err)
		}

		cws, exists := catMap[c.ID]
		if !exists {
			cws = &domain.CategoryWithSources{Category: c}
			catMap[c.ID] = cws
			catOrder = append(catOrder, c.ID)
		}

		if sID != nil {
			cws.Sources = append(cws.Sources, domain.Source{
				ID:         *sID,
				UserID:     *sUserID,
				CategoryID: *sCatID,
				Name:       *sName,
				SortOrder:  *sSortOrder,
				CreatedAt:  *sCreatedAt,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list categories rows: %w", err)
	}

	result := make([]domain.CategoryWithSources, 0, len(catOrder))
	for _, id := range catOrder {
		cws := catMap[id]
		if cws.Sources == nil {
			cws.Sources = []domain.Source{}
		}
		result = append(result, *cws)
	}
	return result, nil
}

func (r *Repository) CreateCategory(ctx context.Context, cat *domain.Category) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO source_categories (id, user_id, name, sort_order, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		cat.ID, cat.UserID, cat.Name, cat.SortOrder, cat.CreatedAt)
	if err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

func (r *Repository) UpdateCategory(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE source_categories SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

func (r *Repository) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM source_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

func (r *Repository) CreateSource(ctx context.Context, src *domain.Source) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO lead_sources (id, user_id, category_id, name, sort_order, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		src.ID, src.UserID, src.CategoryID, src.Name, src.SortOrder, src.CreatedAt)
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	return nil
}

func (r *Repository) UpdateSource(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE lead_sources SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSource(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM lead_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

func (r *Repository) GetSource(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	var s domain.Source
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, category_id, name, sort_order, created_at
		 FROM lead_sources WHERE id = $1`, id).
		Scan(&s.ID, &s.UserID, &s.CategoryID, &s.Name, &s.SortOrder, &s.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	return &s, nil
}

// SourceStats implements StatsReader interface.
func (r *Repository) SourceStats(ctx context.Context, userID uuid.UUID) ([]SourceStat, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ls.id, ls.name, COALESCE(sc.name, ''),
		       COALESCE((SELECT COUNT(*) FROM prospects p WHERE p.source_id = ls.id), 0),
		       COALESCE((SELECT COUNT(*) FROM leads l WHERE l.source_id = ls.id), 0),
		       COALESCE((SELECT COUNT(*) FROM prospects p WHERE p.source_id = ls.id AND p.status = 'converted'), 0)
		FROM lead_sources ls
		LEFT JOIN source_categories sc ON sc.id = ls.category_id
		WHERE ls.user_id = $1
		ORDER BY sc.sort_order, ls.sort_order`, userID)
	if err != nil {
		return nil, fmt.Errorf("source stats: %w", err)
	}
	defer rows.Close()

	var stats []SourceStat
	for rows.Next() {
		var s SourceStat
		if err := rows.Scan(&s.SourceID, &s.SourceName, &s.CategoryName, &s.ProspectCount, &s.LeadCount, &s.ConvertedCount); err != nil {
			return nil, fmt.Errorf("scan source stat: %w", err)
		}
		stats = append(stats, s)
	}
	return stats, rows.Err()
}

type defaultSeed struct {
	categoryName string
	sourceNames  []string
}

var defaultSeeds = []defaultSeed{
	{"Импорт", []string{"CSV файл"}},
	{"Ручное добавление", []string{"Вручную"}},
	{"Парсинг", []string{"2GIS"}},
	{"Входящие", []string{"Telegram", "Email"}},
}

func (r *Repository) EnsureDefaults(ctx context.Context, userID uuid.UUID) error {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM source_categories WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if count > 0 {
		return nil
	}

	now := time.Now().UTC()

	for i, seed := range defaultSeeds {
		cat, _ := domain.NewCategory(userID, seed.categoryName) // seed names are always valid
		cat.SortOrder = i
		cat.CreatedAt = now
		if err := r.CreateCategory(ctx, cat); err != nil {
			return fmt.Errorf("insert default category %q: %w", seed.categoryName, err)
		}
		for j, srcName := range seed.sourceNames {
			src, _ := domain.NewSource(userID, cat.ID, srcName) // seed names are always valid
			src.SortOrder = j
			src.CreatedAt = now
			if err := r.CreateSource(ctx, src); err != nil {
				return fmt.Errorf("insert default source %q: %w", srcName, err)
			}
		}
	}

	return nil
}
