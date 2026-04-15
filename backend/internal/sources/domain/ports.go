package domain

import (
	"context"

	"github.com/google/uuid"
)

// Repository defines the persistence interface for lead sources.
type Repository interface {
	ListCategories(ctx context.Context, userID uuid.UUID) ([]CategoryWithSources, error)
	CreateCategory(ctx context.Context, cat *Category) error
	GetCategory(ctx context.Context, id uuid.UUID) (*Category, error)
	UpdateCategory(ctx context.Context, id uuid.UUID, name string) error
	DeleteCategory(ctx context.Context, id uuid.UUID) error
	CreateSource(ctx context.Context, src *Source) error
	UpdateSource(ctx context.Context, id uuid.UUID, name string) error
	DeleteSource(ctx context.Context, id uuid.UUID) error
	GetSource(ctx context.Context, id uuid.UUID) (*Source, error)
	EnsureDefaults(ctx context.Context, userID uuid.UUID) error
}
