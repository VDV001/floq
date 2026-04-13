package sources

import (
	"context"
	"fmt"

	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
)

type UseCase struct {
	repo domain.Repository
}

func NewUseCase(repo domain.Repository) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) ListCategories(ctx context.Context, userID uuid.UUID) ([]domain.CategoryWithSources, error) {
	if err := uc.repo.EnsureDefaults(ctx, userID); err != nil {
		return nil, fmt.Errorf("ensure defaults: %w", err)
	}
	return uc.repo.ListCategories(ctx, userID)
}

func (uc *UseCase) CreateCategory(ctx context.Context, userID uuid.UUID, name string) (*domain.Category, error) {
	if name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	cat := domain.NewCategory(userID, name)
	if err := uc.repo.CreateCategory(ctx, cat); err != nil {
		return nil, err
	}
	return cat, nil
}

func (uc *UseCase) UpdateCategory(ctx context.Context, id uuid.UUID, name string) error {
	if name == "" {
		return fmt.Errorf("category name is required")
	}
	return uc.repo.UpdateCategory(ctx, id, name)
}

func (uc *UseCase) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteCategory(ctx, id)
}

func (uc *UseCase) CreateSource(ctx context.Context, userID, categoryID uuid.UUID, name string) (*domain.Source, error) {
	if name == "" {
		return nil, fmt.Errorf("source name is required")
	}
	src := domain.NewSource(userID, categoryID, name)
	if err := uc.repo.CreateSource(ctx, src); err != nil {
		return nil, err
	}
	return src, nil
}

func (uc *UseCase) UpdateSource(ctx context.Context, id uuid.UUID, name string) error {
	if name == "" {
		return fmt.Errorf("source name is required")
	}
	return uc.repo.UpdateSource(ctx, id, name)
}

func (uc *UseCase) DeleteSource(ctx context.Context, id uuid.UUID) error {
	return uc.repo.DeleteSource(ctx, id)
}
