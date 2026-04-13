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

func NewCategory(userID uuid.UUID, name string) *Category {
	return &Category{
		ID:        uuid.New(),
		UserID:    userID,
		Name:      name,
		SortOrder: 0,
		CreatedAt: time.Now().UTC(),
	}
}

func NewSource(userID, categoryID uuid.UUID, name string) *Source {
	return &Source{
		ID:         uuid.New(),
		UserID:     userID,
		CategoryID: categoryID,
		Name:       name,
		SortOrder:  0,
		CreatedAt:  time.Now().UTC(),
	}
}

type DefaultSeed struct {
	CategoryName string
	SourceNames  []string
}

func DefaultSeeds() []DefaultSeed {
	return []DefaultSeed{
		{"Импорт", []string{"CSV файл"}},
		{"Ручное добавление", []string{"Вручную"}},
		{"Парсинг", []string{"2GIS"}},
		{"Входящие", []string{"Telegram", "Email"}},
	}
}

// CategoryWithSources groups a category with its nested sources.
type CategoryWithSources struct {
	Category
	Sources []Source
}
