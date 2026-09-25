package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/mcp"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPRESTToolsList(t *testing.T) {
	manager := mcp.NewManager()
	restHandler := &mcp.RESTHandler{Manager: manager}

	srv := httptest.NewServer(restHandler.Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/tools/list")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var body struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Empty(t, body.Tools)
}

func TestMCPRESTToolsCall_ToolNotFound(t *testing.T) {
	manager := mcp.NewManager()
	restHandler := &mcp.RESTHandler{Manager: manager}

	srv := httptest.NewServer(restHandler.Handler())
	defer srv.Close()

	reqBody := `{"name":"nonexistent-tool","arguments":{}}`
	resp, err := http.Post(srv.URL+"/tools/call", "application/json", strings.NewReader(reqBody))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.True(t, body.IsError)
	assert.Contains(t, body.Content[0].Text, "Tool not found")
}

func TestMCPRESTToolsCall_InvalidBody(t *testing.T) {
	manager := mcp.NewManager()
	restHandler := &mcp.RESTHandler{Manager: manager}

	srv := httptest.NewServer(restHandler.Handler())
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/tools/call", "application/json", strings.NewReader("not-json"))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestMCPServerCRUD_NoDB(t *testing.T) {
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: "sk-master",
	})

	tests := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{http.MethodPost, "/mcp_server/", `{"server_id":"test"}`, http.StatusServiceUnavailable},
		{http.MethodGet, "/mcp_server/list", "", http.StatusServiceUnavailable},
		{http.MethodGet, "/mcp_server/abc", "", http.StatusServiceUnavailable},
		{http.MethodPut, "/mcp_server/abc", `{"alias":"new"}`, http.StatusServiceUnavailable},
		{http.MethodDelete, "/mcp_server/abc", "", http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			req.Header.Set("Authorization", "Bearer sk-master")
			w := httptest.NewRecorder()
			srv.ServeHTTP(w, req)
			assert.Equal(t, tt.status, w.Code)
		})
	}
}

func TestMCPRoutes_ProtocolEndpoints(t *testing.T) {
	manager := mcp.NewManager()
	mcpServer := mcp.NewMCPServer(manager)
	mcp.SyncTools(mcpServer, manager)

	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{MasterKey: "sk-master"},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:         handlers,
		MasterKey:        "sk-master",
		MCPSSEHandler:    mcp.NewSSEHandler(mcpServer),
		MCPStreamHandler: mcp.NewStreamableHTTPHandler(mcpServer),
		MCPRESTHandler:   (&mcp.RESTHandler{Manager: manager}).Handler(),
	})

	// REST tools/list should work
	req := httptest.NewRequest(http.MethodGet, "/mcp-rest/tools/list", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Tools []any `json:"tools"`
	}
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Empty(t, body.Tools)
}
