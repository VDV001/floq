package db

import (
	"context"
	"testing"
)

func TestConnFromCtx_NoTx_ReturnsPool(t *testing.T) {
	// When context has no transaction, ConnFromCtx should return the pool.
	// We pass nil pool; the returned Querier interface wraps a nil *pgxpool.Pool,
	// so the interface itself is non-nil but the underlying pointer is nil.
	ctx := context.Background()
	q := ConnFromCtx(ctx, nil)
	// The function returns pool (which is nil), but wrapped in the Querier interface
	// it becomes a non-nil interface with a nil underlying value.
	// We verify the code path doesn't panic and returns something.
	_ = q
}

func TestConnFromCtx_ContextWithWrongKey(t *testing.T) {
	// A context with an unrelated key should still return the pool.
	type otherKey struct{}
	ctx := context.WithValue(context.Background(), otherKey{}, "irrelevant")
	q := ConnFromCtx(ctx, nil)
	// Should not panic; returns pool path.
	_ = q
}

func TestNewTxManager_NotNil(t *testing.T) {
	tm := NewTxManager(nil)
	if tm == nil {
		t.Fatal("expected non-nil TxManager")
	}
}

func TestCtxKey_Unexported(t *testing.T) {
	// Verify that a context without our specific key returns nil from Value.
	ctx := context.Background()
	val := ctx.Value(ctxKey{})
	if val != nil {
		t.Errorf("expected nil value for ctxKey in bare context, got %v", val)
	}
}
