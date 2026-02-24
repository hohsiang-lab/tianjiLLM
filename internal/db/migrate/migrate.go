package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

// RunMigrations applies all pending migrations to the database using the
// production embed.FS (internal/db/schema/*.up.sql).
//
// pool must not be nil; call this only when DATABASE_URL is configured.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	return RunMigrationsFromFS(ctx, pool, db.SchemaFiles, "schema")
}

// RunMigrationsFromFS applies all pending migrations from the given fs.FS.
// The dir argument is the subdirectory inside fsys that contains *.up.sql files.
//
// Exposed for testing — production code should call RunMigrations instead.
func RunMigrationsFromFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, dir string) error {
	if pool == nil {
		return fmt.Errorf("migrate: RunMigrations called with nil pool — ensure DATABASE_URL is configured before calling RunMigrations")
	}

	sqlDB := stdlib.OpenDBFromPool(pool)

	driver, err := pgxv5.WithInstance(sqlDB, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("migrate: create driver: %w", err)
	}

	src, err := iofs.New(fsys, dir)
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
