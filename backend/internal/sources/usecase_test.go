package sources

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/sources/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	categories []domain.CategoryWithSources
	sources    map[uuid.UUID]*domain.Source
	defaults   bool
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		sources: make(map[uuid.UUID]*domain.Source),
	}
}

func (m *mockRepo) ListCategories(_ context.Context, _ uuid.UUID) ([]domain.CategoryWithSources, error) {
	return m.categories, nil
}

func (m *mockRepo) CreateCategory(_ context.Context, cat *domain.Category) error {
	m.categories = append(m.categories, domain.CategoryWithSources{
		Category: *cat,
		Sources:  []domain.Source{},
	})
	return nil
}

func (m *mockRepo) UpdateCategory(_ context.Context, id uuid.UUID, name string) error {
	for i := range m.categories {
		if m.categories[i].ID == id {
			m.categories[i].Name = name
		}
	}
	return nil
}

func (m *mockRepo) DeleteCategory(_ context.Context, id uuid.UUID) error {
	for i := range m.categories {
		if m.categories[i].ID == id {
			m.categories = append(m.categories[:i], m.categories[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockRepo) CreateSource(_ context.Context, src *domain.Source) error {
	m.sources[src.ID] = src
	for i := range m.categories {
		if m.categories[i].ID == src.CategoryID {
			m.categories[i].Sources = append(m.categories[i].Sources, *src)
		}
	}
	return nil
}

func (m *mockRepo) UpdateSource(_ context.Context, id uuid.UUID, name string) error {
	if s, ok := m.sources[id]; ok {
		s.Name = name
	}
	return nil
}

func (m *mockRepo) DeleteSource(_ context.Context, id uuid.UUID) error {
	delete(m.sources, id)
	return nil
}

func (m *mockRepo) GetSource(_ context.Context, id uuid.UUID) (*domain.Source, error) {
	s, ok := m.sources[id]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockRepo) EnsureDefaults(_ context.Context, _ uuid.UUID) error {
	m.defaults = true
	return nil
}

func TestCreateCategory(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	cat, err := uc.CreateCategory(context.Background(), uuid.New(), "Парсинг")
	require.NoError(t, err)
	require.NotNil(t, cat)
	assert.Equal(t, "Парсинг", cat.Name)
	assert.NotEqual(t, uuid.Nil, cat.ID)
}

func TestCreateCategory_EmptyName(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	cat, err := uc.CreateCategory(context.Background(), uuid.New(), "")
	assert.Error(t, err)
	assert.Nil(t, cat)
}

func TestCreateSource(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	userID := uuid.New()
	cat, err := uc.CreateCategory(context.Background(), userID, "Импорт")
	require.NoError(t, err)

	src, err := uc.CreateSource(context.Background(), userID, cat.ID, "CSV файл")
	require.NoError(t, err)
	require.NotNil(t, src)
	assert.Equal(t, "CSV файл", src.Name)
	assert.Equal(t, cat.ID, src.CategoryID)
}

func TestCreateSource_EmptyName(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	src, err := uc.CreateSource(context.Background(), uuid.New(), uuid.New(), "")
	assert.Error(t, err)
	assert.Nil(t, src)
}

func TestListCategories_EnsuresDefaults(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	_, err := uc.ListCategories(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.True(t, repo.defaults)
}

func TestUpdateCategory(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	cat, _ := uc.CreateCategory(context.Background(), uuid.New(), "Old")
	err := uc.UpdateCategory(context.Background(), cat.ID, "New")
	require.NoError(t, err)
	assert.Equal(t, "New", repo.categories[0].Name)
}

func TestDeleteCategory(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	cat, _ := uc.CreateCategory(context.Background(), uuid.New(), "ToDelete")
	err := uc.DeleteCategory(context.Background(), cat.ID)
	require.NoError(t, err)
	assert.Empty(t, repo.categories)
}

func TestDeleteSource(t *testing.T) {
	repo := newMockRepo()
	uc := NewUseCase(repo)

	userID := uuid.New()
	cat, _ := uc.CreateCategory(context.Background(), userID, "Cat")
	src, _ := uc.CreateSource(context.Background(), userID, cat.ID, "Src")

	err := uc.DeleteSource(context.Background(), src.ID)
	require.NoError(t, err)
	assert.Empty(t, repo.sources)
}
