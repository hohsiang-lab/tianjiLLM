// Package ui — failing tests for the /ui/api/rate-limit-state endpoint.
// These tests are written BEFORE implementation (TDD / failing-first).
// They cover FR-010, FR-007, FR-008, SC-004, SC-005.
package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/ratelimitstate"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// newHandlerWithStore creates a UIHandler pre-wired with a ratelimitstate store.
// (After implementation UIHandler should have a RateLimitStore field.)
func newHandlerWithStore(store *ratelimitstate.Store) *UIHandler {
	h := &UIHandler{}
	h.RateLimitStore = store // field expected after implementation
	return h
}

// ---------------------------------------------------------------------------
// FR-010 / SC-005: endpoint must require authentication
// ---------------------------------------------------------------------------

// TestRateLimitStateEndpoint_RequiresAuth verifies that an unauthenticated
// request to GET /ui/api/rate-limit-state returns 401 Unauthorized.
func TestRateLimitStateEndpoint_RequiresAuth(t *testing.T) {
	h := newHandlerWithStore(ratelimitstate.New())

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	// No session cookie → should be rejected
	rr := httptest.NewRecorder()
	h.handleRateLimitState(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code,
		"unauthenticated request must be rejected (SC-005)")
}

// ---------------------------------------------------------------------------
// FR-007 / SC-004: no data yet → structured empty-state JSON (not an error)
// ---------------------------------------------------------------------------

// TestRateLimitStateEndpoint_NoData verifies that when no snapshot exists the
// endpoint returns 200 with a JSON body clearly indicating no data.
func TestRateLimitStateEndpoint_NoData(t *testing.T) {
	store := ratelimitstate.New()
	h := newHandlerWithStore(store)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	req = withAuthSession(req) // helper that injects a valid session
	rr := httptest.NewRecorder()
	h.handleRateLimitState(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	// Must signal "no data" without exposing null/0 as real values.
	hasData, _ := body["has_data"].(bool)
	assert.False(t, hasData, "has_data must be false when store is empty (SC-004)")
}

// ---------------------------------------------------------------------------
// FR-002, FR-003, FR-004, FR-005, SC-002: full snapshot is returned correctly
// ---------------------------------------------------------------------------

// TestRateLimitStateEndpoint_WithData verifies that a stored snapshot is
// serialised into the expected JSON shape.
func TestRateLimitStateEndpoint_WithData(t *testing.T) {
	store := ratelimitstate.New()

	now := time.Now().UTC().Truncate(time.Second)
	resetsAt := now.Add(90 * time.Second)

	store.Set(&ratelimitstate.Snapshot{
		CapturedAt: now,
		InputTokens: &ratelimitstate.DimensionState{
			Limit:     200_000,
			Remaining: 150_000,
			ResetsAt:  resetsAt,
		},
		OutputTokens: &ratelimitstate.DimensionState{
			Limit:     80_000,
			Remaining: 79_000,
			ResetsAt:  resetsAt,
		},
		Requests: &ratelimitstate.DimensionState{
			Limit:     2_000,
			Remaining: 1_998,
			ResetsAt:  resetsAt,
		},
	})

	h := newHandlerWithStore(store)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	req = withAuthSession(req)
	rr := httptest.NewRecorder()
	h.handleRateLimitState(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))

	var body struct {
		HasData      bool   `json:"has_data"`
		CapturedAt   string `json:"captured_at"`
		InputTokens  *struct {
			Limit     int64  `json:"limit"`
			Remaining int64  `json:"remaining"`
			ResetsAt  string `json:"resets_at"`
		} `json:"input_tokens"`
		OutputTokens *struct {
			Limit     int64  `json:"limit"`
			Remaining int64  `json:"remaining"`
			ResetsAt  string `json:"resets_at"`
		} `json:"output_tokens"`
		Requests *struct {
			Limit     int64  `json:"limit"`
			Remaining int64  `json:"remaining"`
			ResetsAt  string `json:"resets_at"`
		} `json:"requests"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))

	assert.True(t, body.HasData, "has_data must be true when snapshot exists")

	require.NotNil(t, body.InputTokens, "input_tokens must be present (FR-002)")
	assert.Equal(t, int64(200_000), body.InputTokens.Limit)
	assert.Equal(t, int64(150_000), body.InputTokens.Remaining)
	assert.NotEmpty(t, body.InputTokens.ResetsAt, "resets_at must not be empty")

	require.NotNil(t, body.OutputTokens, "output_tokens must be present (FR-003)")
	assert.Equal(t, int64(80_000), body.OutputTokens.Limit)
	assert.Equal(t, int64(79_000), body.OutputTokens.Remaining)

	require.NotNil(t, body.Requests, "requests must be present (FR-004)")
	assert.Equal(t, int64(2_000), body.Requests.Limit)
	assert.Equal(t, int64(1_998), body.Requests.Remaining)
}

// ---------------------------------------------------------------------------
// FR-008: partial snapshot (some dimensions nil) → those fields null in JSON
// ---------------------------------------------------------------------------

// TestRateLimitStateEndpoint_PartialSnapshot verifies that missing dimensions
// are rendered as null (not as 0 or omitted) so the UI can show "N/A".
func TestRateLimitStateEndpoint_PartialSnapshot(t *testing.T) {
	store := ratelimitstate.New()
	store.Set(&ratelimitstate.Snapshot{
		CapturedAt: time.Now().UTC(),
		InputTokens: &ratelimitstate.DimensionState{
			Limit:     100_000,
			Remaining: 90_000,
			ResetsAt:  time.Now().Add(60 * time.Second).UTC(),
		},
		OutputTokens: nil,
		Requests:     nil,
	})

	h := newHandlerWithStore(store)

	req := httptest.NewRequest(http.MethodGet, "/ui/api/rate-limit-state", nil)
	req = withAuthSession(req)
	rr := httptest.NewRecorder()
	h.handleRateLimitState(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &raw))

	// output_tokens and requests must be present as null (not absent keys)
	outputTokens, outputPresent := raw["output_tokens"]
	assert.True(t, outputPresent, "output_tokens key must be present even when nil (FR-008)")
	assert.Nil(t, outputTokens, "output_tokens value must be null when header was absent")

	requests, requestsPresent := raw["requests"]
	assert.True(t, requestsPresent, "requests key must be present even when nil (FR-008)")
	assert.Nil(t, requests, "requests value must be null when header was absent")
}

// ---------------------------------------------------------------------------
// withAuthSession injects a fake valid session into the request context.
// Implementation must recognise this as an authenticated admin session.
// ---------------------------------------------------------------------------
func withAuthSession(r *http.Request) *http.Request {
	// After implementation, replace this with whatever session-cookie or
	// context-injection mechanism the UIHandler.sessionAuth middleware uses.
	// For now we rely on the handler reading a session value from context;
	// the test helper should set that context value here.
	//
	// Placeholder: set a known test cookie that the test UIHandler accepts.
	r.AddCookie(&http.Cookie{
		Name:  "tianji_session",
		Value: "test-valid-session",
	})
	return r
}
