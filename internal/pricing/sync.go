package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

const minModelCount = 50

// upsertModelPricingSQL is the SQL used by pgx.Batch for batch upsert.
// Mirrors the sqlc-generated query in model_pricing.sql.go.
const upsertModelPricingSQL = `INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url, synced_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9, NOW()
)
ON CONFLICT (model_name) DO UPDATE SET
    input_cost_per_token  = EXCLUDED.input_cost_per_token,
    output_cost_per_token = EXCLUDED.output_cost_per_token,
    max_input_tokens      = EXCLUDED.max_input_tokens,
    max_output_tokens     = EXCLUDED.max_output_tokens,
    max_tokens            = EXCLUDED.max_tokens,
    mode                  = EXCLUDED.mode,
    provider              = EXCLUDED.provider,
    source_url            = EXCLUDED.source_url,
    synced_at             = NOW(),
    updated_at            = NOW()`

// upstreamModelEntry holds the fields we care about from the upstream JSON.
type upstreamModelEntry struct {
	InputCostPerToken  float64 `json:"input_cost_per_token"`
	OutputCostPerToken float64 `json:"output_cost_per_token"`
	MaxInputTokens     int     `json:"max_input_tokens"`
	MaxOutputTokens    int     `json:"max_output_tokens"`
	MaxTokens          int     `json:"max_tokens"`
	Mode               string  `json:"mode"`
	LiteLLMProvider    string  `json:"litellm_provider"`
}

var syncHTTPClient = &http.Client{Timeout: 30 * time.Second}

// SyncFromUpstream fetches model pricing from the upstream URL, validates it,
// batch-upserts into the DB, and reloads the in-memory calculator.
// Returns the number of models synced.
func SyncFromUpstream(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, calc *Calculator, upstreamURL string) (int, error) {
	// Step 1: HTTP fetch
	raw, err := fetchUpstream(ctx, upstreamURL)
	if err != nil {
		return 0, err
	}

	// Step 2: validate model count
	if valErr := validateUpstreamData(raw); valErr != nil {
		return 0, valErr
	}

	// Step 3: parse individual entries (skip sample_spec, skip bad entries)
	type parsedEntry struct {
		name string
		info upstreamModelEntry
	}
	entries := make([]parsedEntry, 0, len(raw))
	for name, data := range raw {
		if name == "sample_spec" {
			continue
		}
		var info upstreamModelEntry
		if parseErr := json.Unmarshal(data, &info); parseErr != nil {
			log.Printf("pricing sync: skipping %q: %v", name, parseErr)
			continue
		}
		entries = append(entries, parsedEntry{name: name, info: info})
	}

	// Step 4: batch upsert inside a transaction (all-or-nothing)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after successful commit

	batch := &pgx.Batch{}
	for _, e := range entries {
		batch.Queue(upsertModelPricingSQL,
			e.name,
			e.info.InputCostPerToken,
			e.info.OutputCostPerToken,
			int32(e.info.MaxInputTokens),
			int32(e.info.MaxOutputTokens),
			int32(e.info.MaxTokens),
			e.info.Mode,
			e.info.LiteLLMProvider,
			upstreamURL,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, execErr := br.Exec(); execErr != nil {
			_ = br.Close()
			return 0, fmt.Errorf("batch upsert item %d: %w", i, execErr)
		}
	}
	if closeErr := br.Close(); closeErr != nil {
		return 0, fmt.Errorf("close batch: %w", closeErr)
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	// Step 5: reload in-memory calculator
	dbEntries, err := queries.ListModelPricing(ctx)
	if err != nil {
		return 0, fmt.Errorf("list model pricing after sync: %w", err)
	}
	calc.ReloadFromDB(dbEntries)

	return len(entries), nil
}

// fetchUpstream performs the HTTP GET and returns the raw JSON map.
func fetchUpstream(ctx context.Context, url string) (map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := syncHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var raw map[string]json.RawMessage
	if parseErr := json.Unmarshal(body, &raw); parseErr != nil {
		return nil, fmt.Errorf("parse upstream JSON: %w", parseErr)
	}
	return raw, nil
}

// validateUpstreamData checks that the upstream response has enough models.
func validateUpstreamData(raw map[string]json.RawMessage) error {
	count := len(raw)
	if _, ok := raw["sample_spec"]; ok {
		count--
	}
	if count < minModelCount {
		return fmt.Errorf("upstream returned only %d models, expected at least %d (possible corruption)", count, minModelCount)
	}
	return nil
}
