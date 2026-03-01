package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

const minModelCount = 50

// openRouterResponse is the top-level response from the OpenRouter models API.
type openRouterResponse struct {
	Data []openRouterModel `json:"data"`
}

// openRouterModel is a single model entry from the OpenRouter models API.
type openRouterModel struct {
	ID      string            `json:"id"`
	Pricing openRouterPricing `json:"pricing"`
}

// openRouterPricing holds the per-token pricing strings from OpenRouter.
type openRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

// upsertModelPricingSQL is the SQL used by pgx.Batch for batch upsert.
// Mirrors the sqlc-generated query in model_pricing.sql.go.
const upsertModelPricingSQL = `INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url,
    cache_read_input_token_cost, cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k, cache_creation_input_token_cost_above_200k,
    synced_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9,
    $10, $11,
    $12, $13,
    NOW()
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
    cache_read_input_token_cost             = EXCLUDED.cache_read_input_token_cost,
    cache_creation_input_token_cost         = EXCLUDED.cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k  = EXCLUDED.cache_read_input_token_cost_above_200k,
    cache_creation_input_token_cost_above_200k = EXCLUDED.cache_creation_input_token_cost_above_200k,
    synced_at             = NOW(),
    updated_at            = NOW()`

// upstreamModelEntry holds the fields we care about from the upstream JSON.
type upstreamModelEntry struct {
	InputCostPerToken                    float64 `json:"input_cost_per_token"`
	OutputCostPerToken                   float64 `json:"output_cost_per_token"`
	MaxInputTokens                       int     `json:"max_input_tokens"`
	MaxOutputTokens                      int     `json:"max_output_tokens"`
	MaxTokens                            int     `json:"max_tokens"`
	Mode                                 string  `json:"mode"`
	LiteLLMProvider                      string  `json:"litellm_provider"`
	CacheReadInputTokenCost              float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost          float64 `json:"cache_creation_input_token_cost"`
	CacheReadInputTokenCostAbove200k     float64 `json:"cache_read_input_token_cost_above_200k_tokens"`
	CacheCreationInputTokenCostAbove200k float64 `json:"cache_creation_input_token_cost_above_200k_tokens"`
}

var syncHTTPClient = &http.Client{Timeout: 30 * time.Second}

// SyncFromUpstream fetches model pricing from the LiteLLM upstream URL, validates it,
// supplements with OpenRouter data for models not in LiteLLM, batch-upserts into
// the DB, and reloads the in-memory calculator. Returns the number of models synced.
func SyncFromUpstream(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, calc *Calculator, upstreamURL string, openRouterURL string) (int, error) {
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
	litellmNames := make(map[string]struct{}, len(raw))
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
		litellmNames[name] = struct{}{}
	}

	// Step 3b: supplement with OpenRouter models not covered by LiteLLM
	if openRouterURL != "" {
		orModels, orErr := fetchOpenRouter(ctx, openRouterURL)
		if orErr != nil {
			log.Printf("pricing sync: warning: OpenRouter fetch failed (%v), continuing with LiteLLM data only", orErr)
		} else {
			for _, m := range orModels {
				promptCost, promptErr := strconv.ParseFloat(m.Pricing.Prompt, 64)
				completionCost, completionErr := strconv.ParseFloat(m.Pricing.Completion, 64)
				if (promptErr != nil && completionErr != nil) || (promptCost == 0 && completionCost == 0) {
					continue
				}

				// Extract provider and bare name from "provider/model" format
				var provider, bareName string
				if idx := strings.IndexByte(m.ID, '/'); idx >= 0 {
					provider = m.ID[:idx]
					bareName = m.ID[idx+1:]
				} else {
					bareName = m.ID
				}

				info := upstreamModelEntry{
					InputCostPerToken:  promptCost,
					OutputCostPerToken: completionCost,
					LiteLLMProvider:    provider,
					Mode:               "chat",
				}

				// Add full id (provider/model) if not already in LiteLLM
				if _, exists := litellmNames[m.ID]; !exists {
					entries = append(entries, parsedEntry{name: m.ID, info: info})
				}

				// Add bare name if different from full id and not in LiteLLM
				if bareName != m.ID {
					if _, exists := litellmNames[bareName]; !exists {
						entries = append(entries, parsedEntry{name: bareName, info: info})
					}
				}
			}
		}
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
			e.info.CacheReadInputTokenCost,
			e.info.CacheCreationInputTokenCost,
			e.info.CacheReadInputTokenCostAbove200k,
			e.info.CacheCreationInputTokenCostAbove200k,
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

	// Step 6: insert embedded fallback models (insert-only)
	existingDB := make(map[string]struct{}, len(dbEntries))
	for _, e := range dbEntries {
		existingDB[e.ModelName] = struct{}{}
	}
	if embErr := syncEmbeddedFallback(ctx, pool, calc, existingDB); embErr != nil {
		log.Printf("pricing sync: warning: embedded fallback failed: %v", embErr)
	}

	return len(entries), nil
}

// fetchOpenRouter fetches the OpenRouter models API and returns the data array.
func fetchOpenRouter(ctx context.Context, url string) ([]openRouterModel, error) {
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
		return nil, fmt.Errorf("openrouter returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	var result openRouterResponse
	if parseErr := json.Unmarshal(body, &result); parseErr != nil {
		return nil, fmt.Errorf("parse openrouter JSON: %w", parseErr)
	}
	return result.Data, nil
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

// insertEmbeddedFallback inserts embedded models that are not yet in the DB.
// Uses INSERT ... ON CONFLICT DO NOTHING (insert-only, never overwrites admin data).
const insertEmbeddedFallbackSQL = `INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url,
    cache_read_input_token_cost, cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k, cache_creation_input_token_cost_above_200k,
    synced_at
) VALUES (
    $1, $2, $3,
    $4, $5, $6,
    $7, $8, $9,
    $10, $11,
    $12, $13,
    NOW()
)
ON CONFLICT (model_name) DO NOTHING`

// syncEmbeddedFallback inserts embedded-only models into the DB (insert-only).
// It queries current DB model names, finds embedded models not present, and inserts them.
func syncEmbeddedFallback(ctx context.Context, pool *pgxpool.Pool, calc *Calculator, existingDB map[string]struct{}) error {
	toInsert := selectEmbeddedToInsert(calc.embedded, existingDB)
	if len(toInsert) == 0 {
		return nil
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("syncEmbeddedFallback: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	batch := &pgx.Batch{}
	for _, name := range toInsert {
		info := calc.embedded[name]
		batch.Queue(insertEmbeddedFallbackSQL,
			name,
			info.InputCostPerToken,
			info.OutputCostPerToken,
			int32(info.MaxInputTokens),
			int32(info.MaxOutputTokens),
			int32(info.MaxTokens),
			info.Mode,
			info.Provider,
			"embedded",
			info.CacheReadCostPerToken,
			info.CacheCreationCostPerToken,
			info.CacheReadCostPerTokenAbove200k,
			info.CacheCreationCostPerTokenAbove200k,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, execErr := br.Exec(); execErr != nil {
			_ = br.Close()
			return fmt.Errorf("syncEmbeddedFallback: insert item %d: %w", i, execErr)
		}
	}
	if closeErr := br.Close(); closeErr != nil {
		return fmt.Errorf("syncEmbeddedFallback: close batch: %w", closeErr)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("syncEmbeddedFallback: commit: %w", err)
	}

	log.Printf("pricing sync: inserted %d embedded fallback models into DB", len(toInsert))
	return nil
}
