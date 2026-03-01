package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAgentHandlers_NoDB(t *testing.T) {
	h := newTestHandlers() // DB is nil

	tests := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
		body string
	}{
		{"AgentCreate", h.AgentCreate, `{"agent_name":"test"}`},
		{"AgentGet", h.AgentGet, ""},
		{"AgentList", h.AgentList, ""},
		{"AgentUpdate", h.AgentUpdate, `{"agent_name":"test"}`},
		{"AgentPatch", h.AgentPatch, `{}`},
		{"AgentDelete", h.AgentDelete, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(http.MethodGet, "/", nil)
			}
			w := httptest.NewRecorder()
			tt.fn(w, req)
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with nil DB, got 200", tt.name)
			}
		})
	}
}
