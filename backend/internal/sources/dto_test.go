package sources

import (
	"testing"
	"time"

	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestSourceToResponse(t *testing.T) {
	id := uuid.New()
	catID := uuid.New()
	now := time.Now().UTC()
	src := &domain.Source{
		ID:         id,
		CategoryID: catID,
		Name:       "CSV файл",
		SortOrder:  2,
		CreatedAt:  now,
	}

	resp := SourceToResponse(src)

	assert.Equal(t, id, resp.ID)
	assert.Equal(t, catID, resp.CategoryID)
	assert.Equal(t, "CSV файл", resp.Name)
	assert.Equal(t, 2, resp.SortOrder)
	assert.Equal(t, now, resp.CreatedAt)
}

func TestCategoryWithSourcesToResponse(t *testing.T) {
	catID := uuid.New()
	srcID := uuid.New()
	now := time.Now().UTC()

	cws := &domain.CategoryWithSources{
		Category: domain.Category{
			ID:        catID,
			Name:      "Парсинг",
			SortOrder: 1,
			CreatedAt: now,
		},
		Sources: []domain.Source{
			{
				ID:         srcID,
				CategoryID: catID,
				Name:       "2GIS",
				SortOrder:  0,
				CreatedAt:  now,
			},
		},
	}

	resp := CategoryWithSourcesToResponse(cws)

	assert.Equal(t, catID, resp.ID)
	assert.Equal(t, "Парсинг", resp.Name)
	assert.Equal(t, 1, resp.SortOrder)
	assert.Equal(t, now, resp.CreatedAt)
	assert.Len(t, resp.Sources, 1)
	assert.Equal(t, srcID, resp.Sources[0].ID)
	assert.Equal(t, "2GIS", resp.Sources[0].Name)
}

func TestCategoryWithSourcesToResponse_EmptySources(t *testing.T) {
	cws := &domain.CategoryWithSources{
		Category: domain.Category{
			ID:   uuid.New(),
			Name: "Пустая",
		},
		Sources: []domain.Source{},
	}

	resp := CategoryWithSourcesToResponse(cws)
	assert.Empty(t, resp.Sources)
}

func TestCategoriesToResponse(t *testing.T) {
	cats := []domain.CategoryWithSources{
		{
			Category: domain.Category{ID: uuid.New(), Name: "A"},
			Sources:  []domain.Source{},
		},
		{
			Category: domain.Category{ID: uuid.New(), Name: "B"},
			Sources:  []domain.Source{{ID: uuid.New(), Name: "src"}},
		},
	}

	resp := CategoriesToResponse(cats)
	assert.Len(t, resp, 2)
	assert.Equal(t, "A", resp[0].Name)
	assert.Equal(t, "B", resp[1].Name)
	assert.Empty(t, resp[0].Sources)
	assert.Len(t, resp[1].Sources, 1)
}

func TestCategoriesToResponse_Empty(t *testing.T) {
	resp := CategoriesToResponse(nil)
	assert.Empty(t, resp)
}

func TestStatsToResponse(t *testing.T) {
	srcID := uuid.New()
	stats := []domain.SourceStat{
		{
			SourceID:       srcID,
			SourceName:     "2GIS",
			CategoryName:   "Парсинг",
			ProspectCount:  10,
			LeadCount:      5,
			ConvertedCount: 2,
		},
	}

	resp := StatsToResponse(stats)
	assert.Len(t, resp, 1)
	assert.Equal(t, srcID, resp[0].SourceID)
	assert.Equal(t, "2GIS", resp[0].SourceName)
	assert.Equal(t, "Парсинг", resp[0].CategoryName)
	assert.Equal(t, 10, resp[0].ProspectCount)
	assert.Equal(t, 5, resp[0].LeadCount)
	assert.Equal(t, 2, resp[0].ConvertedCount)
}

func TestStatsToResponse_Empty(t *testing.T) {
	resp := StatsToResponse(nil)
	assert.Empty(t, resp)
}

func TestStatsToResponse_Multiple(t *testing.T) {
	stats := []domain.SourceStat{
		{SourceID: uuid.New(), SourceName: "A"},
		{SourceID: uuid.New(), SourceName: "B"},
		{SourceID: uuid.New(), SourceName: "C"},
	}

	resp := StatsToResponse(stats)
	assert.Len(t, resp, 3)
	assert.Equal(t, "A", resp[0].SourceName)
	assert.Equal(t, "B", resp[1].SourceName)
	assert.Equal(t, "C", resp[2].SourceName)
}
