package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCategory(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		userID := uuid.New()
		before := time.Now().UTC()

		cat, err := NewCategory(userID, "Бизнес-клубы")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, cat.ID)
		assert.Equal(t, userID, cat.UserID)
		assert.Equal(t, "Бизнес-клубы", cat.Name)
		assert.Equal(t, 0, cat.SortOrder)
		assert.False(t, cat.CreatedAt.IsZero())
		assert.True(t, !cat.CreatedAt.Before(before), "CreatedAt should be >= before")
	})

	t.Run("empty name returns error", func(t *testing.T) {
		cat, err := NewCategory(uuid.New(), "")

		require.Error(t, err)
		assert.Nil(t, cat)
		assert.Contains(t, err.Error(), "category name is required")
	})
}

func TestNewSource(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		userID := uuid.New()
		categoryID := uuid.New()
		before := time.Now().UTC()

		src, err := NewSource(userID, categoryID, "2GIS")

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, src.ID)
		assert.Equal(t, userID, src.UserID)
		assert.Equal(t, categoryID, src.CategoryID)
		assert.Equal(t, "2GIS", src.Name)
		assert.Equal(t, 0, src.SortOrder)
		assert.False(t, src.CreatedAt.IsZero())
		assert.True(t, !src.CreatedAt.Before(before), "CreatedAt should be >= before")
	})

	t.Run("empty name returns error", func(t *testing.T) {
		src, err := NewSource(uuid.New(), uuid.New(), "")

		require.Error(t, err)
		assert.Nil(t, src)
		assert.Contains(t, err.Error(), "source name is required")
	})
}

func TestCategory_Rename(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		cat, err := NewCategory(uuid.New(), "Old")
		require.NoError(t, err)

		err = cat.Rename("New")

		require.NoError(t, err)
		assert.Equal(t, "New", cat.Name)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		cat, err := NewCategory(uuid.New(), "Original")
		require.NoError(t, err)

		err = cat.Rename("")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "category name is required")
		assert.Equal(t, "Original", cat.Name, "name should not change on error")
	})
}

func TestSource_Rename(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		src, err := NewSource(uuid.New(), uuid.New(), "Old")
		require.NoError(t, err)

		err = src.Rename("New")

		require.NoError(t, err)
		assert.Equal(t, "New", src.Name)
	})

	t.Run("empty name returns error", func(t *testing.T) {
		src, err := NewSource(uuid.New(), uuid.New(), "Original")
		require.NoError(t, err)

		err = src.Rename("")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "source name is required")
		assert.Equal(t, "Original", src.Name, "name should not change on error")
	})
}
