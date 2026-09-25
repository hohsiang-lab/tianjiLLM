package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
)

type testUIAuthRouter struct{}

func (testUIAuthRouter) RegisterRoutes(r chi.Router) {
	r.Get("/probe", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func (testUIAuthRouter) RegisterAuthRoutes(r chi.Router) {
	r.Get("/probe", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
}

func TestServerMountsOptionalUIAuthRoutes(t *testing.T) {
	server := NewServer(ServerConfig{
		Handlers:  &handler.Handlers{},
		MasterKey: "test-master-key",
		UIHandler: testUIAuthRouter{},
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/probe", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
}
