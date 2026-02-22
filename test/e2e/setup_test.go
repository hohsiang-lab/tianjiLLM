//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/playwright-community/playwright-go"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ui"
)

const masterKey = "sk-e2e-master"

var (
	testServer  *httptest.Server
	testDB      *db.Queries
	testPool    *pgxpool.Pool
	pw          *playwright.Playwright
	testBrowser playwright.Browser
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	// 1. Connect to PostgreSQL
	dbURL := os.Getenv("E2E_DATABASE_URL")
	if dbURL == "" {
		fmt.Fprintln(os.Stderr, "E2E_DATABASE_URL is required")
		os.Exit(1)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to DB: %v", err)
	}
	testPool = pool

	// 2. Drop all tables and reload schema (idempotent fresh start)
	if err := dropAllTables(ctx, pool); err != nil {
		log.Fatalf("drop tables: %v", err)
	}
	schemaDir := findSchemaDir()
	if err := loadSchema(ctx, pool, schemaDir); err != nil {
		log.Fatalf("load schema: %v", err)
	}
	testDB = db.New(pool)

	// 3. Build httptest.Server
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{ModelName: "gpt-4o", TianjiParams: config.TianjiParams{Model: "openai/gpt-4o"}},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: masterKey,
		},
	}

	uiHandler := &ui.UIHandler{
		DB:        testDB,
		Config:    cfg,
		MasterKey: masterKey,
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: masterKey,
		UIHandler: uiHandler,
	})

	testServer = httptest.NewServer(srv)

	// 4. Launch Playwright browser
	pw, err = playwright.Run()
	if err != nil {
		log.Fatalf("start playwright: %v", err)
	}

	headless := os.Getenv("E2E_HEADLESS") != "false"
	testBrowser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		log.Fatalf("launch chromium: %v", err)
	}

	// 5. Run tests
	code := m.Run()

	// 6. Cleanup
	testBrowser.Close()
	pw.Stop()
	testServer.Close()
	pool.Close()

	os.Exit(code)
}

func findSchemaDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(thisFile), "..", "..")
	return filepath.Join(projectRoot, "internal", "db", "schema")
}

func dropAllTables(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		DO $$ DECLARE r RECORD;
		BEGIN
			FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname = 'public') LOOP
				EXECUTE 'DROP TABLE IF EXISTS "' || r.tablename || '" CASCADE';
			END LOOP;
		END $$;
	`)
	return err
}

func loadSchema(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read schema dir %s: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".sql" {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files) // 001_, 002_, ... lexicographic order

	for _, f := range files {
		sql, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
	}
	return nil
}
