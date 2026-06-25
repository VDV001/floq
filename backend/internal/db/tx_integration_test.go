//go:build integration

package db_test

import (
	"context"
	"errors"
	"testing"

	"github.com/daniil/floq/internal/db"
	"github.com/daniil/floq/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These exercise WithTx against a real pool — the commit and rollback paths
// that the unit tests (bogus pool → Begin always fails) can't reach. A
// throwaway scratch table is created per run so writes are observable and the
// shared test DB is left clean.

func TestWithTx_CommitsOnSuccess(t *testing.T) {
	pool := testutil.TestDB(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS tx_it_scratch (id int primary key)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `TRUNCATE tx_it_scratch`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS tx_it_scratch`) })

	tm := db.NewTxManager(pool)
	err = tm.WithTx(ctx, func(txCtx context.Context) error {
		// The repository pattern: write through ConnFromCtx, which must resolve
		// to the in-flight tx (not the pool) inside WithTx.
		_, e := db.ConnFromCtx(txCtx, pool).Exec(txCtx, `INSERT INTO tx_it_scratch (id) VALUES (1)`)
		return e
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM tx_it_scratch WHERE id = 1`).Scan(&count))
	assert.Equal(t, 1, count, "a successful WithTx must commit the write")
}

func TestWithTx_RollsBackOnError(t *testing.T) {
	pool := testutil.TestDB(t)
	ctx := context.Background()

	_, err := pool.Exec(ctx, `CREATE TABLE IF NOT EXISTS tx_it_scratch (id int primary key)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `TRUNCATE tx_it_scratch`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP TABLE IF EXISTS tx_it_scratch`) })

	wantErr := errors.New("boom")
	tm := db.NewTxManager(pool)
	err = tm.WithTx(ctx, func(txCtx context.Context) error {
		if _, e := db.ConnFromCtx(txCtx, pool).Exec(txCtx, `INSERT INTO tx_it_scratch (id) VALUES (2)`); e != nil {
			return e
		}
		// fn returns an error after writing → WithTx must roll the write back.
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM tx_it_scratch WHERE id = 2`).Scan(&count))
	assert.Equal(t, 0, count, "a WithTx whose fn errors must roll back the write")
}
