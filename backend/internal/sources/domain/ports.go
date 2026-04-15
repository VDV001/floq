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

// SourceStat is a read model for source analytics.
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
