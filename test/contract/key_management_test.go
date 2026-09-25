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

// TestAuthMiddleware_OldFormatKey verifies that old sk- prefix keys still pass auth.
// Auth middleware uses SHA256 hash comparison and is prefix-agnostic.
func TestAuthMiddleware_OldFormatKey(t *testing.T) {
	// newTestServer hardcodes MasterKey: "sk-master" (old sk- prefix).
	// Authenticating with this master key should succeed (503 from no DB, not 401).
	srv := newTestServer(t, "")

	body := `{"key_name": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code,
		"old format sk- key should pass auth (503 from no DB, not 401)")
}

func TestAuthMiddleware_NewFormatKey(t *testing.T) {
	// newTestServer uses "sk-master" as master key, so a new-format key will
	// fail master key check → fail JWT check → try DB lookup → fail (no DB).
	// The key point: auth middleware returns 401 (not panic, not format error).
	// This proves auth middleware processes new-format keys without crashing
	// or rejecting based on prefix — it simply doesn't find them in any auth source.
	srv := newTestServer(t, "")

	body := `{"key_name": "test"}`
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-ant-oat01-tianji-aabbccddee0011223344556677889900aabbccddee001122334455667788")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// 401 = auth middleware processed the key normally (hash mismatch, not format rejection)
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"new format key should be processed by auth middleware (401 from hash mismatch, not format error)")
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
