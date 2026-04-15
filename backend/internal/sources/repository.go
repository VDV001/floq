package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile-time check that Repository implements domain.Repository.
var _ domain.Repository = (*Repository)(nil)

type Repository struct {
	q db.Querier
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{q: pool}
}

// NewRepositoryFromQuerier creates a Repository from any db.Querier (useful for testing).
func NewRepositoryFromQuerier(q db.Querier) *Repository {
	return &Repository{q: q}
}

func (r *Repository) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.CategoryWithSources, error) {
	rows, err := r.q.Query(ctx, `
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
	_, err := r.q.Exec(ctx,
		`INSERT INTO source_categories (id, user_id, name, sort_order, created_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (user_id, name) DO NOTHING`,
		cat.ID, cat.UserID, cat.Name, cat.SortOrder, cat.CreatedAt)
	if err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

func (r *Repository) GetCategory(ctx context.Context, id uuid.UUID) (*domain.Category, error) {
	var c domain.Category
	err := r.q.QueryRow(ctx,
		`SELECT id, user_id, name, sort_order, created_at FROM source_categories WHERE id = $1`, id).
		Scan(&c.ID, &c.UserID, &c.Name, &c.SortOrder, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get category: %w", err)
	}
	return &c, nil
}

func (r *Repository) UpdateCategory(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE source_categories SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return fmt.Errorf("update category: %w", err)
	}
	return nil
}

func (r *Repository) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	_, err := r.q.Exec(ctx,
		`DELETE FROM source_categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	return nil
}

func (r *Repository) CreateSource(ctx context.Context, src *domain.Source) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO lead_sources (id, user_id, category_id, name, sort_order, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (category_id, name) DO NOTHING`,
		src.ID, src.UserID, src.CategoryID, src.Name, src.SortOrder, src.CreatedAt)
	if err != nil {
		return fmt.Errorf("create source: %w", err)
	}
	return nil
}

func (r *Repository) UpdateSource(ctx context.Context, id uuid.UUID, name string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE lead_sources SET name = $2 WHERE id = $1`, id, name)
	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSource(ctx context.Context, id uuid.UUID) error {
	_, err := r.q.Exec(ctx,
		`DELETE FROM lead_sources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}
	return nil
}

func (r *Repository) GetSource(ctx context.Context, id uuid.UUID) (*domain.Source, error) {
	var s domain.Source
	err := r.q.QueryRow(ctx,
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
func (r *Repository) SourceStats(ctx context.Context, userID uuid.UUID) ([]domain.SourceStat, error) {
	rows, err := r.q.Query(ctx, `
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

	var stats []domain.SourceStat
	for rows.Next() {
		var s domain.SourceStat
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
	err := r.q.QueryRow(ctx,
		`SELECT COUNT(*) FROM source_categories WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return fmt.Errorf("count categories: %w", err)
	}
	if count > 0 {
		// Even if defaults exist, migrate orphan prospects (source_id IS NULL)
		r.migrateOrphanProspects(ctx, userID)
		return nil
	}

	now := time.Now().UTC()

	// Track created source IDs for prospect migration
	sourceNameToID := make(map[string]uuid.UUID)

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
			sourceNameToID[srcName] = src.ID
		}
	}

	// Migrate existing prospects with old text source to new source_id
	for oldSource, newName := range map[string]string{"csv": "CSV файл", "manual": "Вручную", "2gis": "2GIS"} {
		if srcID, ok := sourceNameToID[newName]; ok {
			_, _ = r.q.Exec(ctx,
				`UPDATE prospects SET source_id = $1 WHERE user_id = $2 AND source = $3 AND source_id IS NULL`,
				srcID, userID, oldSource)
		}
	}

	return nil
}

// migrateOrphanProspects links prospects with source_id=NULL to existing sources by text name.
func (r *Repository) migrateOrphanProspects(ctx context.Context, userID uuid.UUID) {
	migrations := map[string]string{"csv": "CSV файл", "manual": "Вручную", "2gis": "2GIS"}
	for oldSource, newName := range migrations {
		_, _ = r.q.Exec(ctx,
			`UPDATE prospects SET source_id = (SELECT id FROM lead_sources WHERE user_id = $1 AND name = $2 LIMIT 1)
			 WHERE user_id = $1 AND source = $3 AND source_id IS NULL`,
			userID, newName, oldSource)
	}
}
