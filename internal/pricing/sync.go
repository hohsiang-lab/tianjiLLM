package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

// parsedEntry is a resolved model entry ready for DB upsert.
type parsedEntry struct {
	name string
	info upstreamModelEntry
}

// openRouterResponse is the top-level response from OpenRouter's /api/v1/models.
type openRouterResponse struct {
	Data []openRouterModel `json:"data"`
}

// openRouterModel is a single model entry from OpenRouter.
type openRouterModel struct {
	ID      string            `json:"id"`
	Pricing openRouterPricing `json:"pricing"`
}

// openRouterPricing holds per-token pricing strings from OpenRouter.
type openRouterPricing struct {
	Prompt     string `json:"prompt"`
	Completion string `json:"completion"`
}

var syncHTTPClient = &http.Client{Timeout: 30 * time.Second}

// SyncFromUpstream fetches model pricing from the upstream URL, validates it,
// batch-upserts into the DB, and reloads the in-memory calculator.
// Also fetches from openRouterURL (secondary source); failures are logged but non-fatal.
// Returns the number of models synced.
func SyncFromUpstream(ctx context.Context, pool *pgxpool.Pool, queries *db.Queries, calc *Calculator, upstreamURL string, openRouterURL string) (int, error) {
	// Step 1: HTTP fetch LiteLLM
	raw, err := fetchUpstream(ctx, upstreamURL)
	if err != nil {
		return 0, err
	}

	// Step 2: validate model count
	if valErr := validateUpstreamData(raw); valErr != nil {
		return 0, valErr
	}

	// Step 3: parse LiteLLM entries (skip sample_spec, skip bad entries)
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

	// Step 4: fetch OpenRouter and merge (non-fatal on failure)
	if openRouterURL != "" {
		orEntries, orErr := fetchOpenRouter(ctx, openRouterURL)
		if orErr != nil {
			log.Printf("pricing sync: OpenRouter fetch failed (non-fatal): %v", orErr)
		} else {
			entries = mergeOpenRouterEntries(entries, orEntries)
		}
	}

	// Step 5: batch upsert inside a transaction (all-or-nothing)
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

	// Step 6: reload in-memory calculator
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

// fetchOpenRouter fetches model pricing from OpenRouter and returns parsed entries.
// Each model produces two entries: the full id (e.g. "google/gemini-2.5-pro") and
// the bare name without provider prefix (e.g. "gemini-2.5-pro").
func fetchOpenRouter(ctx context.Context, url string) ([]parsedEntry, error) {
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

	var orResp openRouterResponse
	if parseErr := json.Unmarshal(body, &orResp); parseErr != nil {
		return nil, fmt.Errorf("parse openrouter JSON: %w", parseErr)
	}

	entries := make([]parsedEntry, 0, len(orResp.Data)*2)
	for _, m := range orResp.Data {
		if m.ID == "" {
			continue
		}

		provider := ""
		bareName := m.ID
		if idx := strings.Index(m.ID, "/"); idx >= 0 {
			provider = m.ID[:idx]
			bareName = m.ID[idx+1:]
		}

		info := upstreamModelEntry{
			InputCostPerToken:  parseOpenRouterPrice(m.Pricing.Prompt),
			OutputCostPerToken: parseOpenRouterPrice(m.Pricing.Completion),
			Mode:               "chat",
			LiteLLMProvider:    provider,
		}

		// Full id entry: "google/gemini-2.5-pro"
		entries = append(entries, parsedEntry{name: m.ID, info: info})

		// Bare name entry: "gemini-2.5-pro" (only when id contains "/")
		if bareName != m.ID {
			entries = append(entries, parsedEntry{name: bareName, info: info})
		}
	}
	return entries, nil
}

// mergeOpenRouterEntries adds OpenRouter entries that are not already present
// in the LiteLLM entries. LiteLLM takes precedence.
func mergeOpenRouterEntries(litellm []parsedEntry, openrouter []parsedEntry) []parsedEntry {
	existing := make(map[string]struct{}, len(litellm))
	for _, e := range litellm {
		existing[e.name] = struct{}{}
	}

	merged := litellm
	for _, e := range openrouter {
		if _, found := existing[e.name]; !found {
			merged = append(merged, e)
			existing[e.name] = struct{}{} // avoid duplicate bare names from same OR response
		}
	}
	return merged
}

// parseOpenRouterPrice converts an OpenRouter price string (e.g. "0.000001") to float64.
// Returns 0 on parse failure.
func parseOpenRouterPrice(s string) float64 {
	if s == "" || s == "0" {
		return 0
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return f
}
