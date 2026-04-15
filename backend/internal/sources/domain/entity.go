package domain

import (
	"fmt"
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

func NewCategory(userID uuid.UUID, name string) (*Category, error) {
	if name == "" {
		return nil, fmt.Errorf("category name is required")
	}
	return &Category{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		SortOrder: 0,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (c *Category) Rename(name string) error {
	if name == "" {
		return fmt.Errorf("category name is required")
	}
	c.Name = name
	return nil
}

func NewSource(userID, categoryID uuid.UUID, name string) (*Source, error) {
	if name == "" {
		return nil, fmt.Errorf("source name is required")
	}
	return &Source{
		ID:         uuid.New(),
		UserID:     userID,
		CategoryID: categoryID,
		Name:       name,
		SortOrder:  0,
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (s *Source) Rename(name string) error {
	if name == "" {
		return fmt.Errorf("source name is required")
	}
	s.Name = name
	return nil
}

// CategoryWithSources groups a category with its nested sources.
type CategoryWithSources struct {
	Category
	Sources []Source
}
