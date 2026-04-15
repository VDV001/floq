package chat

import (
	"context"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock pgx.Row ---

type mockRow struct {
	scanFn func(dest ...any) error
}

func (r *mockRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

// --- mock pgx.Rows ---

type mockRows struct {
	data    [][]any
	idx     int
	scanErr error
	closed  bool
}

func (r *mockRows) Close()                                         { r.closed = true }
func (r *mockRows) Err() error                                     { return nil }
func (r *mockRows) CommandTag() pgconn.CommandTag                  { return pgconn.NewCommandTag("") }
func (r *mockRows) FieldDescriptions() []pgconn.FieldDescription   { return nil }
func (r *mockRows) RawValues() [][]byte                            { return nil }
func (r *mockRows) Conn() *pgx.Conn                               { return nil }
func (r *mockRows) Values() ([]any, error)                         { return nil, nil }

func (r *mockRows) Next() bool {
	if r.idx >= len(r.data) {
		return false
	}
	r.idx++
	return true
}

func (r *mockRows) Scan(dest ...any) error {
	if r.scanErr != nil {
		return r.scanErr
	}
	row := r.data[r.idx-1]
	for i, d := range dest {
		if i < len(row) {
			switch v := d.(type) {
			case *int:
				if val, ok := row[i].(int); ok {
					*v = val
				}
			case *string:
				if val, ok := row[i].(string); ok {
					*v = val
				}
			}
		}
	}
	return nil
}

// --- mock db.Querier ---

type mockQuerier struct {
	queryRowResults []*mockRow
	queryRowIdx     int
	queryResults    []*mockRows
	queryIdx        int
	queryErr        error
}

func (q *mockQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (q *mockQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.queryRowIdx >= len(q.queryRowResults) {
		return &mockRow{scanFn: func(dest ...any) error { return nil }}
	}
	r := q.queryRowResults[q.queryRowIdx]
	q.queryRowIdx++
	return r
}

func (q *mockQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	if q.queryErr != nil {
		return nil, q.queryErr
	}
	if q.queryIdx >= len(q.queryResults) {
		return &mockRows{}, nil
	}
	r := q.queryResults[q.queryIdx]
	q.queryIdx++
	return r, nil
}

var _ db.Querier = (*mockQuerier)(nil)

// --- Tests ---

func TestNewRepository(t *testing.T) {
	r := NewRepository(nil)
	require.NotNil(t, r)
}

func TestNewRepositoryFromQuerier(t *testing.T) {
	q := &mockQuerier{}
	r := NewRepositoryFromQuerier(q)
	require.NotNil(t, r)
	assert.Equal(t, db.Querier(q), r.q)
}

func TestFetchStats_HappyPath(t *testing.T) {
	userID := uuid.New()

	// FetchStats does 7 queries in order:
	// 1. QueryRow: total leads count
	// 2. QueryRow: month leads count
	// 3. Query: status counts (rows)
	// 4. QueryRow: prospect count
	// 5. QueryRow: sequence count
	// 6. QueryRow: queued msgs count
	// 7. Query: recent leads (rows)

	q := &mockQuerier{
		queryRowResults: []*mockRow{
			// 1. total leads
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok {
					*p = 42
				}
				return nil
			}},
			// 2. month leads
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok {
					*p = 10
				}
				return nil
			}},
			// 4. prospect count
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok {
					*p = 15
				}
				return nil
			}},
			// 5. sequence count
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok {
					*p = 4
				}
				return nil
			}},
			// 6. queued msgs
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok {
					*p = 7
				}
				return nil
			}},
		},
		queryResults: []*mockRows{
			// 3. status counts
			{data: [][]any{
				{"new", 5},
				{"qualified", 3},
			}},
			// 7. recent leads (empty for simplicity)
			{data: [][]any{}},
		},
	}

	repo := NewRepositoryFromQuerier(q)
	stats, err := repo.FetchStats(context.Background(), userID)
	require.NoError(t, err)
	require.NotNil(t, stats)

	assert.Equal(t, 42, stats.TotalLeads)
	assert.Equal(t, 10, stats.MonthLeads)
	assert.Equal(t, 5, stats.StatusCounts["new"])
	assert.Equal(t, 3, stats.StatusCounts["qualified"])
	assert.Equal(t, 15, stats.ProspectCount)
	assert.Equal(t, 4, stats.SequenceCount)
	assert.Equal(t, 7, stats.QueuedMsgs)
}

func TestFetchStats_TotalLeadsError(t *testing.T) {
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			}},
		},
	}

	repo := NewRepositoryFromQuerier(q)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "total leads")
}

func TestFetchStats_MonthLeadsError(t *testing.T) {
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			// total leads OK
			{scanFn: func(dest ...any) error {
				if p, ok := dest[0].(*int); ok { *p = 1 }
				return nil
			}},
			// month leads fails
			{scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			}},
		},
	}

	repo := NewRepositoryFromQuerier(q)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "month leads")
}

func TestFetchStats_StatusCountsQueryError(t *testing.T) {
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
		},
		queryErr: pgx.ErrNoRows,
	}

	repo := NewRepositoryFromQuerier(q)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "status counts")
}

func TestFetchStats_ProspectCountError(t *testing.T) {
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			// prospect count fails
			{scanFn: func(dest ...any) error { return pgx.ErrNoRows }},
		},
		queryResults: []*mockRows{
			{data: [][]any{}}, // status counts OK
		},
	}

	repo := NewRepositoryFromQuerier(q)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "prospects")
}

func TestFetchStats_SequenceCountError(t *testing.T) {
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }}, // prospect OK
			// sequence count fails
			{scanFn: func(dest ...any) error { return pgx.ErrNoRows }},
		},
		queryResults: []*mockRows{
			{data: [][]any{}},
		},
	}

	repo := NewRepositoryFromQuerier(q)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sequences")
}

func TestFetchStats_QueuedMsgsError_IsSilent(t *testing.T) {
	// QueuedMsgs error is silently swallowed (s.QueuedMsgs = 0)
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 5 }; return nil }}, // prospect
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 3 }; return nil }}, // sequence
			// queued msgs fails
			{scanFn: func(dest ...any) error { return pgx.ErrNoRows }},
		},
		queryResults: []*mockRows{
			{data: [][]any{}}, // status counts
			{data: [][]any{}}, // recent leads
		},
	}

	repo := NewRepositoryFromQuerier(q)
	stats, err := repo.FetchStats(context.Background(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, 0, stats.QueuedMsgs) // silently set to 0
}

func TestFetchStats_RecentLeadsError(t *testing.T) {
	callCount := 0
	q := &mockQuerier{
		queryRowResults: []*mockRow{
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 1 }; return nil }},
			{scanFn: func(dest ...any) error { if p, ok := dest[0].(*int); ok { *p = 0 }; return nil }},
		},
	}
	// Override Query to fail on second call (recent leads)
	q2 := &queryCountQuerier{
		queryRowResults: q.queryRowResults,
		queryFn: func() (pgx.Rows, error) {
			callCount++
			if callCount == 1 {
				return &mockRows{data: [][]any{}}, nil // status counts OK
			}
			return nil, pgx.ErrNoRows // recent leads fails
		},
	}

	repo := NewRepositoryFromQuerier(q2)
	_, err := repo.FetchStats(context.Background(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "recent leads")
}

// queryCountQuerier allows different behavior per Query call
type queryCountQuerier struct {
	queryRowResults []*mockRow
	queryRowIdx     int
	queryFn         func() (pgx.Rows, error)
}

func (q *queryCountQuerier) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func (q *queryCountQuerier) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	if q.queryRowIdx >= len(q.queryRowResults) {
		return &mockRow{scanFn: func(dest ...any) error { return nil }}
	}
	r := q.queryRowResults[q.queryRowIdx]
	q.queryRowIdx++
	return r
}

func (q *queryCountQuerier) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return q.queryFn()
}
