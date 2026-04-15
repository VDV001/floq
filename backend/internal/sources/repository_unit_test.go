package sources

import (
	"context"
	"fmt"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/sources/domain"
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

type fakeRows struct {
	data   [][]any
	idx    int
	errVal error
}

func (r *fakeRows) Close()                                       {}
func (r *fakeRows) Err() error                                   { return r.errVal }
func (r *fakeRows) CommandTag() pgconn.CommandTag                { return pgconn.NewCommandTag("") }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) RawValues() [][]byte                          { return nil }
func (r *fakeRows) Conn() *pgx.Conn                              { return nil }
func (r *fakeRows) Values() ([]any, error)                       { return nil, nil }
func (r *fakeRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}
func (r *fakeRows) Scan(dest ...any) error { return nil }

type fakeQuerier struct {
	execErr      error
	queryRowFns  []func() pgx.Row
	queryRowIdx  int
	queryResults []queryResult
	queryIdx     int
}

type queryResult struct {
	rows pgx.Rows
	err  error
}

func (q *fakeQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), q.execErr
}
func (q *fakeQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.queryRowIdx < len(q.queryRowFns) {
		fn := q.queryRowFns[q.queryRowIdx]
		q.queryRowIdx++
		return fn()
	}
	return &fakeRow{scanFn: func(dest ...any) error { return nil }}
}
func (q *fakeQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if q.queryIdx < len(q.queryResults) {
		r := q.queryResults[q.queryIdx]
		q.queryIdx++
		return r.rows, r.err
	}
	return &fakeRows{}, nil
}

var _ db.Querier = (*fakeQuerier)(nil)

// --- Tests ---

func TestRepo_NewRepository(t *testing.T) {
	r := NewRepository(nil)
	require.NotNil(t, r)
}

func TestRepo_NewRepositoryFromQuerier(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	require.NotNil(t, r)
}

func TestRepo_CreateCategory(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	cat, _ := domain.NewCategory(uuid.New(), "Test")
	assert.NoError(t, r.CreateCategory(context.Background(), cat))
}

func TestRepo_CreateCategory_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	cat, _ := domain.NewCategory(uuid.New(), "Test")
	err := r.CreateCategory(context.Background(), cat)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create category")
}

func TestRepo_UpdateCategory(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.UpdateCategory(context.Background(), uuid.New(), "Name"))
}

func TestRepo_UpdateCategory_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	assert.Error(t, r.UpdateCategory(context.Background(), uuid.New(), "Name"))
}

func TestRepo_DeleteCategory(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.DeleteCategory(context.Background(), uuid.New()))
}

func TestRepo_DeleteCategory_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	assert.Error(t, r.DeleteCategory(context.Background(), uuid.New()))
}

func TestRepo_CreateSource(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	src, _ := domain.NewSource(uuid.New(), uuid.New(), "Src")
	assert.NoError(t, r.CreateSource(context.Background(), src))
}

func TestRepo_CreateSource_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	src, _ := domain.NewSource(uuid.New(), uuid.New(), "Src")
	err := r.CreateSource(context.Background(), src)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create source")
}

func TestRepo_UpdateSource(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.UpdateSource(context.Background(), uuid.New(), "Name"))
}

func TestRepo_UpdateSource_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	assert.Error(t, r.UpdateSource(context.Background(), uuid.New(), "Name"))
}

func TestRepo_DeleteSource(t *testing.T) {
	q := &fakeQuerier{}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.DeleteSource(context.Background(), uuid.New()))
}

func TestRepo_DeleteSource_Error(t *testing.T) {
	q := &fakeQuerier{execErr: fmt.Errorf("db error")}
	r := NewRepositoryFromQuerier(q)
	assert.Error(t, r.DeleteSource(context.Background(), uuid.New()))
}

func TestRepo_GetSource_NotFound(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	src, err := r.GetSource(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Nil(t, src)
}

func TestRepo_GetSource_Error(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error { return fmt.Errorf("db error") }}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	_, err := r.GetSource(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "get source")
}

func TestRepo_GetSource_HappyPath(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error { return nil }}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	src, err := r.GetSource(context.Background(), uuid.New())
	require.NoError(t, err)
	require.NotNil(t, src)
}

func TestRepo_ListCategories_QueryError(t *testing.T) {
	q := &fakeQuerier{
		queryResults: []queryResult{{rows: nil, err: fmt.Errorf("db error")}},
	}
	r := NewRepositoryFromQuerier(q)
	_, err := r.ListCategories(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "list categories")
}

func TestRepo_ListCategories_Empty(t *testing.T) {
	q := &fakeQuerier{
		queryResults: []queryResult{{rows: &fakeRows{}, err: nil}},
	}
	r := NewRepositoryFromQuerier(q)
	cats, err := r.ListCategories(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, cats)
}

func TestRepo_SourceStats_QueryError(t *testing.T) {
	q := &fakeQuerier{
		queryResults: []queryResult{{rows: nil, err: fmt.Errorf("db error")}},
	}
	r := NewRepositoryFromQuerier(q)
	_, err := r.SourceStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source stats")
}

func TestRepo_SourceStats_Empty(t *testing.T) {
	q := &fakeQuerier{
		queryResults: []queryResult{{rows: &fakeRows{}, err: nil}},
	}
	r := NewRepositoryFromQuerier(q)
	stats, err := r.SourceStats(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, stats)
}

func TestRepo_EnsureDefaults_AlreadyExists(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*int); ok { *p = 5 }
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.EnsureDefaults(context.Background(), uuid.New()))
}

func TestRepo_EnsureDefaults_CountError(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error { return fmt.Errorf("db error") }}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	err := r.EnsureDefaults(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count categories")
}

func TestRepo_EnsureDefaults_CreatesDefaults(t *testing.T) {
	q := &fakeQuerier{
		queryRowFns: []func() pgx.Row{
			func() pgx.Row {
				return &fakeRow{scanFn: func(dest ...any) error {
					if p, ok := dest[0].(*int); ok { *p = 0 }
					return nil
				}}
			},
		},
	}
	r := NewRepositoryFromQuerier(q)
	assert.NoError(t, r.EnsureDefaults(context.Background(), uuid.New()))
}
