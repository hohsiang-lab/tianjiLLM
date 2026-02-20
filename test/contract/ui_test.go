package contract

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ui"
)

func newUITestServer(t *testing.T) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:  "openai/gpt-4o",
					APIKey: &apiKey,
				},
			},
			{
				ModelName:    "claude-sonnet",
				TianjiParams: config.TianjiParams{Model: "anthropic/claude-sonnet-4-5-20250929"},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	uiHandler := &ui.UIHandler{
		Config:    cfg,
		MasterKey: cfg.GeneralSettings.MasterKey,
	}

	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: cfg.GeneralSettings.MasterKey,
		UIHandler: uiHandler,
	})
}

func TestUI_LoginPageRenders(t *testing.T) {
	srv := newUITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui/login", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "TianjiLLM Admin")
	assert.Contains(t, w.Body.String(), "api_key")
}

func TestUI_LoginPostInvalidKey(t *testing.T) {
	srv := newUITestServer(t)

	form := url.Values{"api_key": {"wrong-key"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid API key")
}

func TestUI_LoginPostValidKey(t *testing.T) {
	srv := newUITestServer(t)

	form := url.Values{"api_key": {"sk-master"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/ui/", w.Header().Get("Location"))

	// Verify session cookie is set
	cookies := w.Result().Cookies()
	var found bool
	for _, c := range cookies {
		if c.Name == "tianji_session" {
			found = true
			assert.True(t, c.HttpOnly)
			break
		}
	}
	require.True(t, found, "session cookie should be set")
}

func TestUI_DashboardRequiresAuth(t *testing.T) {
	srv := newUITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("Location"))
}

func TestUI_DashboardWithSession(t *testing.T) {
	srv := newUITestServer(t)

	// Login first
	form := url.Values{"api_key": {"sk-master"}}
	loginReq := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	srv.ServeHTTP(loginW, loginReq)
	require.Equal(t, http.StatusSeeOther, loginW.Code)

	// Get session cookie
	var sessionCookie *http.Cookie
	for _, c := range loginW.Result().Cookies() {
		if c.Name == "tianji_session" {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// Access dashboard with session
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	req.AddCookie(sessionCookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Dashboard")
	assert.Contains(t, w.Body.String(), "Total Keys")
	assert.Contains(t, w.Body.String(), "Active Models")
}

func TestUI_ModelsPage(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/models", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "gpt-4o")
	assert.Contains(t, body, "claude-sonnet")
	// DB fallback shows warning banner
	assert.Contains(t, body, "Database unavailable")
	// API key is masked in config fallback
	assert.NotContains(t, body, "sk-test-key")
}

func TestUI_ModelsTable_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/models/table", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "gpt-4o")
	assert.Contains(t, body, "claude-sonnet")
}

func TestUI_ModelsCreate_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{
		"model_name": {"test-model"},
		"model":      {"openai/gpt-4o"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/models/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUI_ModelsEdit_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/models/edit?model_id=some-id", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUI_ModelsUpdate_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{
		"model_id":   {"some-id"},
		"model_name": {"updated"},
		"model":      {"openai/gpt-4o"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/models/update", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUI_ModelsDelete_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	form := url.Values{"model_id": {"some-id"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/models/delete", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestUI_ModelsRequiresAuth(t *testing.T) {
	srv := newUITestServer(t)

	// Full page without auth
	req := httptest.NewRequest(http.MethodGet, "/ui/models", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code)

	// HTMX partial without auth
	req = httptest.NewRequest(http.MethodGet, "/ui/models/table", nil)
	req.Header.Set("HX-Request", "true")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("HX-Redirect"))
}

func TestUI_KeysPage_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/keys", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "API Keys")
	assert.Contains(t, w.Body.String(), "No keys found")
}

func TestUI_UsagePage_NoDB(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodGet, "/ui/usage", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Usage View")
}

func TestUI_HTMX_AuthRedirect(t *testing.T) {
	srv := newUITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui/keys/table", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("HX-Redirect"))
}

func TestUI_Logout(t *testing.T) {
	srv := newUITestServer(t)
	cookie := loginAndGetCookie(t, srv)

	req := httptest.NewRequest(http.MethodPost, "/ui/logout", nil)
	req.AddCookie(cookie)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/ui/login", w.Header().Get("Location"))

	// Verify the session cookie is cleared
	for _, c := range w.Result().Cookies() {
		if c.Name == "tianji_session" {
			assert.True(t, c.MaxAge < 0, "session cookie should be expired")
		}
	}
}

func TestUI_StaticAssets(t *testing.T) {
	srv := newUITestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui/static/js/htmx.min.js", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "htmx")
}

func loginAndGetCookie(t *testing.T, srv *proxy.Server) *http.Cookie {
	t.Helper()
	form := url.Values{"api_key": {"sk-master"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)

	for _, c := range w.Result().Cookies() {
		if c.Name == "tianji_session" {
			return c
		}
	}
	t.Fatal("session cookie not found after login")
	return nil
}
