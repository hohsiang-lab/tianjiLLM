package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
)

func phase6SkillsServer() *proxy.Server {
	handlers := &handler.Handlers{
		Config: &config.ProxyConfig{
			GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
		},
	}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})
}

func TestPhase6_SkillCreate_NoDB(t *testing.T) {
	srv := phase6SkillsServer()

	body := `{"display_title":"My Skill","description":"A test skill"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_SkillCreate_MissingTitle_NoDB(t *testing.T) {
	// Without DB, the handler returns 503 before validating input
	srv := phase6SkillsServer()

	body := `{"description":"No title"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/skills", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_SkillList_NoDB(t *testing.T) {
	srv := phase6SkillsServer()

	req := httptest.NewRequest(http.MethodGet, "/v1/skills", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_SkillGet_NoDB(t *testing.T) {
	srv := phase6SkillsServer()

	req := httptest.NewRequest(http.MethodGet, "/v1/skills/some-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_SkillDelete_NoDB(t *testing.T) {
	srv := phase6SkillsServer()

	req := httptest.NewRequest(http.MethodDelete, "/v1/skills/some-id", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestPhase6_Skills_Unauthorized(t *testing.T) {
	srv := phase6SkillsServer()

	req := httptest.NewRequest(http.MethodGet, "/v1/skills", nil)
	// No Authorization header

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
