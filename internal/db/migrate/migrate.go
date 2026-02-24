package migrate

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// RunMigrations applies all pending migrations to the database.
// Blocks until a database-level advisory lock is acquired (safe for concurrent
// multi-instance startup). Returns an error if any migration fails.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)

	driver, err := pgxv5.WithInstance(sqlDB, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("migrate: create driver: %w", err)
	}

	src, err := iofs.New(db.SchemaFiles, "schema")
	if err != nil {
		return fmt.Errorf("migrate: create source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("migrate: init: %w", err)
	}
	m.Log = &migrateLogger{}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate: up: %w", err)
	}

	return nil
}

// migrateLogger bridges golang-migrate logging to standard log.
type migrateLogger struct{}

func (l *migrateLogger) Printf(format string, v ...interface{}) {
	log.Printf("[migrate] "+format, v...)
}

func (l *migrateLogger) Verbose() bool { return false }
