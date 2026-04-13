package domain

import (
	"time"

	"github.com/google/uuid"
)

// Category represents a grouping of lead sources (e.g. "Бизнес-клубы", "Парсинг").
type Category struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Name      string
	SortOrder int
	CreatedAt time.Time
}

// Source represents a concrete lead source within a category (e.g. "2GIS", "CSV файл").
type Source struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	CategoryID uuid.UUID
	Name       string
	SortOrder  int
	CreatedAt  time.Time
}

// CategoryWithSources groups a category with its nested sources.
type CategoryWithSources struct {
	Category
	Sources []Source
}
