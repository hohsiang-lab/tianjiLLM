package contract

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyGenerate_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"key_name": "test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Without DB, should return 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestKeyGenerate_NoAuth(t *testing.T) {
	srv := newTestServer(t, "")

	body := `{"key_name": "test-key"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestKeyList_NoDB(t *testing.T) {
	srv := newTestServer(t, "")

	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// --- UI Create Key with Models multi-select tests (NoDB contract tests) ---
// These tests verify the form parsing and routing reach the handler.
// Full DB-backed model storage is validated in E2E tests (test/e2e/key_models_multiselect_test.go).

// TestCreateKeyWithSpecificModels_NoDB verifies that a POST to /ui/keys/create with
// all_models=0 and specific models selected is correctly parsed and reaches the handler
// (which returns 503 when DB is unavailable).
func TestCreateKeyWithSpecificModels_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{
		"key_alias":  {"test-specific-models"},
		"all_models": {"0"},
		"models":     {"gpt-4", "claude-3"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/keys/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// No DB → 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestCreateKeyWithAllModels_NoDB verifies that a POST with all_models=1 (unrestricted)
// is correctly handled (503 when no DB).
func TestCreateKeyWithAllModels_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{
		"key_alias":  {"test-all-models"},
		"all_models": {"1"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/keys/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// No DB → 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// TestCreateKeyNoModelCheckbox_NoDB verifies that a POST with all_models=0 and no
// individual model checkboxes submitted is handled correctly (503 when no DB).
func TestCreateKeyNoModelCheckbox_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{
		"key_alias":  {"test-no-model-checkbox"},
		"all_models": {"0"},
		// no "models" values submitted
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/keys/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// No DB → 503
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
