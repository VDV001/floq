package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock pgx helpers ---

type fakeRow struct {
	scanFn func(dest ...any) error
}

func (r *fakeRow) Scan(dest ...any) error { return r.scanFn(dest...) }

type fakeQuerier struct {
	execErr    error
	queryRowFn func() pgx.Row
}

func (q *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 0 1"), q.execErr
}
func (q *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.queryRowFn != nil {
		return q.queryRowFn()
	}
	return &fakeRow{scanFn: func(dest ...any) error { return nil }}
}
func (q *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

var _ db.Querier = (*fakeQuerier)(nil)

// --- Tests ---

func TestRepository_NewRepository(t *testing.T) {
	r := NewRepository(nil)
	require.NotNil(t, r)
}

func TestRepository_NewRepositoryFromQuerier(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	require.NotNil(t, r)
}

func TestRepository_CreateUser(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)

	err := r.CreateUser(context.Background(), uuid.New(), "test@example.com", "hash", "Test User", time.Now())
	assert.NoError(t, err)
}

func TestRepository_CreateUser_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("duplicate key")}
	r := NewRepositoryFromQuerier(q)

	err := r.CreateUser(context.Background(), uuid.New(), "test@example.com", "hash", "Test User", time.Now())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate key")
}

func TestRepository_FindUserByEmail(t *testing.T) {
	expectedID := uuid.New()
	q := &fakeQuerier{
		queryRowFn: func() pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*uuid.UUID); ok {
					*p = expectedID
				}
				if p, ok := dest[1].(*string); ok {
					*p = "hashed-password"
				}
				return nil
			}}
		},
	}
	r := NewRepositoryFromQuerier(q)

	id, hash, err := r.FindUserByEmail(context.Background(), "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, expectedID, id)
	assert.Equal(t, "hashed-password", hash)
}

func TestRepository_FindUserByEmail_NotFound(t *testing.T) {
	q := &fakeQuerier{
		queryRowFn: func() pgx.Row {
			return &fakeRow{scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			}}
		},
	}
	r := NewRepositoryFromQuerier(q)

	_, _, err := r.FindUserByEmail(context.Background(), "nope@example.com")
	assert.Error(t, err)
}
