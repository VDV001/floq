package sources

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
)

// SourceStat is a read model for source analytics (not a domain entity).
type SourceStat struct {
	SourceID       uuid.UUID
	SourceName     string
	CategoryName   string
	ProspectCount  int
	LeadCount      int
	ConvertedCount int
}

// StatsReader provides source statistics from the database.
type StatsReader interface {
	SourceStats(ctx context.Context, userID uuid.UUID) ([]SourceStat, error)
}

type UseCase struct {
	repo        domain.Repository
	statsReader StatsReader
}

func NewUseCase(repo domain.Repository, opts ...func(*UseCase)) *UseCase {
	uc := &UseCase{repo: repo}
	for _, opt := range opts {
		opt(uc)
	}
	return uc
}

func WithStatsReader(sr StatsReader) func(*UseCase) {
	return func(uc *UseCase) { uc.statsReader = sr }
}

func (uc *UseCase) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.CategoryWithSources, error) {
	if err := uc.repo.EnsureDefaults(ctx, userID); err != nil {
		return nil, fmt.Errorf("ensure defaults: %w", err)
	}
	return uc.repo.ListCategories(ctx, userID)
}

func (uc *UseCase) CreateCategory(ctx context.Context, userID uuid.UUID, name string) (*domain.Category, error) {
	cat, err := domain.NewCategory(userID, name)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.CreateCategory(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (uc *UseCase) UpdateCategory(ctx context.Context, id uuid.UUID, name string) error {
	cat, err := uc.repo.GetCategory(ctx, id)
	if err != nil {
		return err
	}
	if cat == nil {
		return fmt.Errorf("category not found")
	}
	if err := cat.Rename(name); err != nil {
		return err
	}
	return uc.repo.UpdateCategory(ctx, id, cat.Name)
}

func (uc *UseCase) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteCategory(ctx, id)
}

func (uc *UseCase) CreateSource(ctx context.Context, userID, categoryID uuid.UUID, name string) (*domain.Source, error) {
	src, err := domain.NewSource(userID, categoryID, name)
	if err != nil {
		return nil, err
	}
	if err := uc.repo.CreateSource(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (uc *UseCase) UpdateSource(ctx context.Context, id uuid.UUID, name string) error {
	src, err := uc.repo.GetSource(ctx, id)
	if err != nil {
		return err
	}
	if src == nil {
		return fmt.Errorf("source not found")
	}
	if err := src.Rename(name); err != nil {
		return err
	}
	return uc.repo.UpdateSource(ctx, id, src.Name)
}

func (uc *UseCase) DeleteSource(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteSource(ctx, id)
}

func (uc *UseCase) Stats(ctx context.Context, userID uuid.UUID) ([]SourceStat, error) {
	if uc.statsReader == nil {
		return nil, fmt.Errorf("stats reader not configured")
	}
	return uc.statsReader.SourceStats(ctx, userID)
}
