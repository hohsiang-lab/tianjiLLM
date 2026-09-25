package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BeginTx preserves the underlying pool's transaction capability without
// expanding Store. A sqlc query wrapper alone is not a writable device fence.
func (q *Queries) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	if q != nil && q.db != nil {
		if pool, ok := q.db.(*pgxpool.Pool); ok && pool == nil {
			return nil, errors.New("database transactions unavailable")
		}
		if beginner, ok := q.db.(interface {
			BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error)
		}); ok {
			return beginner.BeginTx(ctx, opts)
		}
	}
	return nil, errors.New("database transactions unavailable")
}

// Pool returns the underlying pgxpool.Pool if the Queries was created with one.
func (q *Queries) Pool() *pgxpool.Pool {
	if p, ok := q.db.(*pgxpool.Pool); ok {
		return p
	}
	return nil
}

// Ping pings the database.
func (q *Queries) Ping(ctx context.Context) error {
	if p := q.Pool(); p != nil {
		return p.Ping(ctx)
	}
	return nil
}
