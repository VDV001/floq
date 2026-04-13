package sources

import (
	"time"

	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
)

// --- Response DTOs ---

type SourceResponse struct {
	ID         uuid.UUID `json:"id"`
	CategoryID uuid.UUID `json:"category_id"`
	Name       string    `json:"name"`
	SortOrder  int       `json:"sort_order"`
	CreatedAt  time.Time `json:"created_at"`
}

type CategoryResponse struct {
	ID        uuid.UUID        `json:"id"`
	Name      string           `json:"name"`
	SortOrder int              `json:"sort_order"`
	Sources   []SourceResponse `json:"sources"`
	CreatedAt time.Time        `json:"created_at"`
}

// --- Mapping functions ---

func SourceToResponse(s *domain.Source) SourceResponse {
	return SourceResponse{
		ID:         s.ID,
		CategoryID: s.CategoryID,
		Name:       s.Name,
		SortOrder:  s.SortOrder,
		CreatedAt:  s.CreatedAt,
	}
}

func CategoryWithSourcesToResponse(cws *domain.CategoryWithSources) CategoryResponse {
	sources := make([]SourceResponse, len(cws.Sources))
	for i := range cws.Sources {
		sources[i] = SourceToResponse(&cws.Sources[i])
	}
	return CategoryResponse{
		ID:        cws.ID,
		Name:      cws.Name,
		SortOrder: cws.SortOrder,
		Sources:   sources,
		CreatedAt: cws.CreatedAt,
	}
}

func CategoriesToResponse(cats []domain.CategoryWithSources) []CategoryResponse {
	resp := make([]CategoryResponse, len(cats))
	for i := range cats {
		resp[i] = CategoryWithSourcesToResponse(&cats[i])
	}
	return resp
}
