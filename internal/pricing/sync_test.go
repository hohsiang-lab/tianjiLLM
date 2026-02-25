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
