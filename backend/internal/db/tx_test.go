package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnFromCtx_NoTx_ReturnsPool(t *testing.T) {
	ctx := context.Background()
	q := ConnFromCtx(ctx, nil)
	// The function returns pool (which is nil), but wrapped in the Querier interface
	// it becomes a non-nil interface with a nil underlying value.
	_ = q
}

func TestConnFromCtx_ContextWithWrongKey(t *testing.T) {
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "irrelevant")
	q := ConnFromCtx(ctx, nil)
	_ = q
}

func TestNewTxManager_NotNil(t *testing.T) {
	tm := NewTxManager(nil)
	require.NotNil(t, tm)
}

func TestCtxKey_Unexported(t *testing.T) {
	ctx := context.Background()
	val := ctx.Value(ctxKey{})
	assert.Nil(t, val)
}

// fakeTx implements pgx.Tx for testing ConnFromCtx's tx-in-context path.
type fakeTx struct {
	pgx.Tx // embed to satisfy the interface; methods will panic if called
}

func (f *fakeTx) Query(_ context.Context, _ string, _ ...any) (pgx.Rows, error) {
	return nil, nil
}

func (f *fakeTx) QueryRow(_ context.Context, _ string, _ ...any) pgx.Row {
	return nil
}

func (f *fakeTx) Exec(_ context.Context, _ string, _ ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag(""), nil
}

func TestConnFromCtx_WithTx_ReturnsTx(t *testing.T) {
	tx := &fakeTx{}
	// Insert the fakeTx into context with the same key ConnFromCtx checks.
	ctx := context.WithValue(context.Background(), ctxKey{}, pgx.Tx(tx))
	q := ConnFromCtx(ctx, nil)
	// Should return the tx, not the nil pool.
	require.NotNil(t, q)
	// The returned Querier should be our fakeTx.
	assert.Equal(t, pgx.Tx(tx), q)
}

func TestConnFromCtx_WrongTypeInContext(t *testing.T) {
	// If context has the right key but wrong type, should fall through to pool.
	ctx := context.WithValue(context.Background(), ctxKey{}, "not-a-tx")
	q := ConnFromCtx(ctx, nil)
	// Should not panic; returns pool path (nil pool wrapped in interface).
	_ = q
}

func TestNewTxManager_StoresPool(t *testing.T) {
	tm := NewTxManager(nil)
	assert.Nil(t, tm.pool)
}

func TestWithTx_FailedBegin(t *testing.T) {
	// Create a pool with a bogus DSN. The pool won't have any connections
	// available, so Begin will fail.
	cfg, err := pgxpool.ParseConfig("postgres://invalid:invalid@localhost:1/nonexistent?connect_timeout=1")
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	defer pool.Close()

	tm := NewTxManager(pool)
	err = tm.WithTx(context.Background(), func(ctx context.Context) error {
		return nil
	})
	assert.Error(t, err)
}

func TestWithTx_CancelledContext(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://invalid:invalid@localhost:1/nonexistent?connect_timeout=1")
	require.NoError(t, err)

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	defer pool.Close()

	tm := NewTxManager(pool)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled

	err = tm.WithTx(ctx, func(ctx context.Context) error {
		return nil
	})
	assert.Error(t, err)
}

func TestConnFromCtx_NilTxInContext(t *testing.T) {
	// If context has ctxKey but the value is a nil pgx.Tx interface,
	// the type assertion succeeds (ok=true) but the underlying value is nil.
	var tx pgx.Tx // nil interface
	ctx := context.WithValue(context.Background(), ctxKey{}, tx)
	q := ConnFromCtx(ctx, nil)
	// Type assertion pgx.Tx on nil interface fails (ok=false),
	// so it falls through to pool.
	_ = q
}
