package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
