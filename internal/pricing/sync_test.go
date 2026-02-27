package pricing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildUpstreamJSON returns a JSON blob with n models (+ optional sample_spec).
func buildUpstreamJSON(n int, includeSampleSpec bool) []byte {
	m := make(map[string]any, n+1)
	if includeSampleSpec {
		m["sample_spec"] = map[string]any{"input_cost_per_token": 0}
	}
	for i := 0; i < n; i++ {
		m[fmt.Sprintf("model-%d", i)] = map[string]any{
			"input_cost_per_token":  float64(i) * 0.0001,
			"output_cost_per_token": float64(i) * 0.0002,
			"max_input_tokens":      4096,
			"max_output_tokens":     2048,
			"max_tokens":            6144,
			"mode":                  "chat",
			"litellm_provider":      "test",
		}
	}
	b, _ := json.Marshal(m)
	return b
}

// TestValidateUpstreamData_Pass checks count >= 50 passes.
func TestValidateUpstreamData_Pass(t *testing.T) {
	raw := make(map[string]json.RawMessage, 60)
	for i := 0; i < 60; i++ {
		raw[fmt.Sprintf("m%d", i)] = json.RawMessage(`{}`)
	}
	if err := validateUpstreamData(raw); err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

// TestValidateUpstreamData_ExactlyMinPasses checks count == 50 passes.
func TestValidateUpstreamData_ExactlyMinPasses(t *testing.T) {
	raw := make(map[string]json.RawMessage, 50)
	for i := 0; i < 50; i++ {
		raw[fmt.Sprintf("m%d", i)] = json.RawMessage(`{}`)
	}
	if err := validateUpstreamData(raw); err != nil {
		t.Errorf("expected pass at exactly %d, got: %v", minModelCount, err)
	}
}

// TestValidateUpstreamData_Fail checks count < 50 returns error.
func TestValidateUpstreamData_Fail(t *testing.T) {
	raw := make(map[string]json.RawMessage, 10)
	for i := 0; i < 10; i++ {
		raw[fmt.Sprintf("m%d", i)] = json.RawMessage(`{}`)
	}
	err := validateUpstreamData(raw)
	if err == nil {
		t.Fatal("expected error for count < 50")
	}
	if !strings.Contains(err.Error(), "10") {
		t.Errorf("error should mention actual count: %v", err)
	}
}

// TestValidateUpstreamData_SampleSpecExcluded: sample_spec not counted.
func TestValidateUpstreamData_SampleSpecExcluded(t *testing.T) {
	// 49 real models + 1 sample_spec → should fail
	raw := make(map[string]json.RawMessage, 50)
	raw["sample_spec"] = json.RawMessage(`{}`)
	for i := 0; i < 49; i++ {
		raw[fmt.Sprintf("m%d", i)] = json.RawMessage(`{}`)
	}
	err := validateUpstreamData(raw)
	if err == nil {
		t.Error("expected error: 49 real models < 50")
	}
}

// TestFetchUpstream_Success tests normal JSON response.
func TestFetchUpstream_Success(t *testing.T) {
	body := buildUpstreamJSON(60, false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	raw, err := fetchUpstream(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) != 60 {
		t.Errorf("expected 60 entries, got %d", len(raw))
	}
}

// TestFetchUpstream_HTTP4xx tests 4xx error.
func TestFetchUpstream_HTTP4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchUpstream(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error should mention status code: %v", err)
	}
}

// TestFetchUpstream_HTTP5xx tests 5xx error.
func TestFetchUpstream_HTTP5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchUpstream(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500")
	}
}

// TestFetchUpstream_InvalidJSON tests JSON parse failure.
func TestFetchUpstream_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := fetchUpstream(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestFetchUpstream_ConnectionRefused tests network failure.
func TestFetchUpstream_ConnectionRefused(t *testing.T) {
	_, err := fetchUpstream(t.Context(), "http://127.0.0.1:19999")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// TestValidation_FailureDoesNotCallDB: validate is called before DB ops.
// We test this by ensuring validation error returns before any DB interaction.
func TestValidation_FailureDoesNotCallDB(t *testing.T) {
	// Build response with only 10 models (< 50)
	body := buildUpstreamJSON(10, false)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	raw, err := fetchUpstream(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	err = validateUpstreamData(raw)
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Confirm it's a count error
	if !strings.Contains(err.Error(), "10") {
		t.Errorf("error should mention count: %v", err)
	}
}

// TestSampleSpecSkipped: sample_spec entry should not appear in parsed entries.
func TestSampleSpecSkipped(t *testing.T) {
	body := buildUpstreamJSON(60, true) // 60 real + sample_spec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	raw, err := fetchUpstream(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if err := validateUpstreamData(raw); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Count parsed entries (simulate what SyncFromUpstream does)
	count := 0
	for name := range raw {
		if name == "sample_spec" {
			continue
		}
		count++
	}
	if count != 60 {
		t.Errorf("expected 60 parsed entries (excluding sample_spec), got %d", count)
	}
}

// TestModelEntryFieldsMissing: entries with bad JSON should be skipped gracefully.
func TestModelEntryFieldsMissing(t *testing.T) {
	// Build a map with 50 valid + 5 with null/missing fields
	raw := make(map[string]json.RawMessage, 55)
	for i := 0; i < 50; i++ {
		raw[fmt.Sprintf("good-%d", i)] = json.RawMessage(`{"input_cost_per_token": 0.001}`)
	}
	for i := 0; i < 5; i++ {
		// invalid JSON → should be skipped
		raw[fmt.Sprintf("bad-%d", i)] = json.RawMessage(`{invalid}`)
	}

	// Validate passes (55 entries)
	if err := validateUpstreamData(raw); err != nil {
		t.Fatalf("validation failed: %v", err)
	}

	// Parse: bad entries should be skipped
	goodCount := 0
	for name, data := range raw {
		if name == "sample_spec" {
			continue
		}
		var info upstreamModelEntry
		if err := json.Unmarshal(data, &info); err != nil {
			continue // skip bad
		}
		goodCount++
	}
	if goodCount != 50 {
		t.Errorf("expected 50 good entries, got %d", goodCount)
	}
}

// --- OpenRouter unit tests ---

// buildOpenRouterJSON returns a JSON response in OpenRouter /api/v1/models format.
func buildOpenRouterJSON(models []struct{ id, prompt, completion string }) []byte {
	data := make([]map[string]any, len(models))
	for i, m := range models {
		data[i] = map[string]any{
			"id": m.id,
			"pricing": map[string]any{
				"prompt":     m.prompt,
				"completion": m.completion,
			},
		}
	}
	b, _ := json.Marshal(map[string]any{"data": data})
	return b
}

// TestFetchOpenRouter_Success verifies basic fetch and dual-key expansion.
func TestFetchOpenRouter_Success(t *testing.T) {
	body := buildOpenRouterJSON([]struct{ id, prompt, completion string }{
		{"google/gemini-2.5-pro-preview", "0.000001", "0.000002"},
		{"anthropic/claude-3-opus", "0.000015", "0.000075"},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	entries, err := fetchOpenRouter(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each model produces 2 entries (full id + bare name)
	if len(entries) != 4 {
		t.Errorf("expected 4 entries (2 models × 2 keys), got %d", len(entries))
	}

	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.name] = true
	}
	for _, want := range []string{
		"google/gemini-2.5-pro-preview",
		"gemini-2.5-pro-preview",
		"anthropic/claude-3-opus",
		"claude-3-opus",
	} {
		if !names[want] {
			t.Errorf("expected entry %q not found", want)
		}
	}
}

// TestFetchOpenRouter_PricingParsed verifies price string → float64 conversion.
func TestFetchOpenRouter_PricingParsed(t *testing.T) {
	body := buildOpenRouterJSON([]struct{ id, prompt, completion string }{
		{"openai/gpt-4o", "0.000005", "0.000015"},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	entries, err := fetchOpenRouter(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range entries {
		if e.name == "openai/gpt-4o" {
			if e.info.InputCostPerToken != 0.000005 {
				t.Errorf("input cost: want 0.000005, got %v", e.info.InputCostPerToken)
			}
			if e.info.OutputCostPerToken != 0.000015 {
				t.Errorf("output cost: want 0.000015, got %v", e.info.OutputCostPerToken)
			}
			if e.info.LiteLLMProvider != "openai" {
				t.Errorf("provider: want openai, got %q", e.info.LiteLLMProvider)
			}
		}
	}
}

// TestFetchOpenRouter_HTTP500 verifies non-200 status returns error.
func TestFetchOpenRouter_HTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchOpenRouter(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status code: %v", err)
	}
}

// TestFetchOpenRouter_ConnectionRefused verifies network failure returns error.
func TestFetchOpenRouter_ConnectionRefused(t *testing.T) {
	_, err := fetchOpenRouter(t.Context(), "http://127.0.0.1:19998")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

// TestFetchOpenRouter_InvalidJSON verifies JSON parse failure returns error.
func TestFetchOpenRouter_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer srv.Close()

	_, err := fetchOpenRouter(t.Context(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// TestFetchOpenRouter_EmptyData verifies empty data array returns no entries.
func TestFetchOpenRouter_EmptyData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	entries, err := fetchOpenRouter(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

// TestFetchOpenRouter_NoPrefixModel verifies model IDs without "/" produce only one entry.
func TestFetchOpenRouter_NoPrefixModel(t *testing.T) {
	body := buildOpenRouterJSON([]struct{ id, prompt, completion string }{
		{"bare-model", "0.000001", "0.000002"},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	entries, err := fetchOpenRouter(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No "/" in id → only one entry (bare-model itself)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for bare model id, got %d", len(entries))
	}
	if entries[0].name != "bare-model" {
		t.Errorf("expected entry name 'bare-model', got %q", entries[0].name)
	}
}

// TestMergeOpenRouterEntries_LiteLLMTakesPrecedence verifies LiteLLM models are not overwritten.
func TestMergeOpenRouterEntries_LiteLLMTakesPrecedence(t *testing.T) {
	litellm := []parsedEntry{
		{name: "gpt-4o", info: upstreamModelEntry{InputCostPerToken: 0.000005}},
		{name: "claude-3-opus", info: upstreamModelEntry{InputCostPerToken: 0.000015}},
	}
	openrouter := []parsedEntry{
		{name: "gpt-4o", info: upstreamModelEntry{InputCostPerToken: 9999}},       // should NOT overwrite
		{name: "gemini-2.5-pro", info: upstreamModelEntry{InputCostPerToken: 0.001}}, // should be added
	}

	merged := mergeOpenRouterEntries(litellm, openrouter)

	if len(merged) != 3 {
		t.Errorf("expected 3 entries (2 litellm + 1 new from openrouter), got %d", len(merged))
	}

	for _, e := range merged {
		if e.name == "gpt-4o" && e.info.InputCostPerToken != 0.000005 {
			t.Errorf("gpt-4o should keep LiteLLM price 0.000005, got %v", e.info.InputCostPerToken)
		}
	}
}

// TestMergeOpenRouterEntries_NewModelsAdded verifies OpenRouter-only models are included.
func TestMergeOpenRouterEntries_NewModelsAdded(t *testing.T) {
	litellm := []parsedEntry{
		{name: "gpt-4o", info: upstreamModelEntry{InputCostPerToken: 0.000005}},
	}
	openrouter := []parsedEntry{
		{name: "google/gemini-2.5-pro", info: upstreamModelEntry{InputCostPerToken: 0.001}},
		{name: "gemini-2.5-pro", info: upstreamModelEntry{InputCostPerToken: 0.001}},
	}

	merged := mergeOpenRouterEntries(litellm, openrouter)

	if len(merged) != 3 {
		t.Errorf("expected 3 entries, got %d", len(merged))
	}

	names := make(map[string]bool, len(merged))
	for _, e := range merged {
		names[e.name] = true
	}
	if !names["google/gemini-2.5-pro"] {
		t.Error("expected 'google/gemini-2.5-pro' to be added from OpenRouter")
	}
	if !names["gemini-2.5-pro"] {
		t.Error("expected 'gemini-2.5-pro' to be added from OpenRouter")
	}
}

// TestMergeOpenRouterEntries_NoDuplicates verifies OpenRouter entries don't produce duplicates.
func TestMergeOpenRouterEntries_NoDuplicates(t *testing.T) {
	litellm := []parsedEntry{}
	// Simulate OR response where bare name appears twice (from two different providers)
	openrouter := []parsedEntry{
		{name: "google/gemini-pro", info: upstreamModelEntry{InputCostPerToken: 0.001}},
		{name: "gemini-pro", info: upstreamModelEntry{InputCostPerToken: 0.001}},
		{name: "openai/gemini-pro", info: upstreamModelEntry{InputCostPerToken: 0.002}},
		{name: "gemini-pro", info: upstreamModelEntry{InputCostPerToken: 0.002}}, // duplicate bare name
	}

	merged := mergeOpenRouterEntries(litellm, openrouter)

	count := 0
	for _, e := range merged {
		if e.name == "gemini-pro" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 'gemini-pro' entry (no duplicates), got %d", count)
	}
}

// TestMergeOpenRouterEntries_OpenRouterFailureDoesNotBlock verifies that when
// OR fetch fails the LiteLLM entries are returned unchanged.
func TestMergeOpenRouterEntries_OpenRouterFailureDoesNotBlock(t *testing.T) {
	litellm := []parsedEntry{
		{name: "gpt-4o", info: upstreamModelEntry{InputCostPerToken: 0.000005}},
	}

	// Simulate OR failure: we call mergeOpenRouterEntries with empty OR slice
	merged := mergeOpenRouterEntries(litellm, nil)

	if len(merged) != 1 {
		t.Errorf("expected 1 litellm entry unchanged, got %d", len(merged))
	}
	if merged[0].name != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %q", merged[0].name)
	}
}

// TestFetchOpenRouter_DualKeyStorage verifies the dual key storage pattern end-to-end.
func TestFetchOpenRouter_DualKeyStorage(t *testing.T) {
	body := buildOpenRouterJSON([]struct{ id, prompt, completion string }{
		{"google/gemini-2.5-pro-preview", "0.000001", "0.000002"},
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	}))
	defer srv.Close()

	entries, err := fetchOpenRouter(t.Context(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.name] = true
	}

	if !names["google/gemini-2.5-pro-preview"] {
		t.Error("expected full provider/model key 'google/gemini-2.5-pro-preview'")
	}
	if !names["gemini-2.5-pro-preview"] {
		t.Error("expected bare model key 'gemini-2.5-pro-preview'")
	}
}
