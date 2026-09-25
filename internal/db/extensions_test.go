package db

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
)

func TestQueriesBeginTxUsesUnderlyingBackend(t *testing.T) {
	pool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer pool.Close()
	q := New(pool)
	beginner, ok := any(q).(interface {
		BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
	})
	require.True(t, ok, "Queries must expose native transaction capability")
	pool.ExpectBeginTx(pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	pool.ExpectRollback()
	tx, err := beginner.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(context.Background()))
	require.NoError(t, pool.ExpectationsWereMet())
}
func TestQueriesBeginTxFailsClosed(t *testing.T) {
	var pool *pgxpool.Pool
	for _, q := range []*Queries{nil, New(nil), New(pool), New(struct{ DBTX }{})} {
		beginner, ok := any(q).(interface {
			BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
		})
		require.True(t, ok)
		tx, err := beginner.BeginTx(context.Background(), pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.Error(t, err)
		require.Nil(t, tx)
	}
}
