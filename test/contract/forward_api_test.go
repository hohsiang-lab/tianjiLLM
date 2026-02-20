package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServerWithBaseURL creates a server that resolves the "openai" provider
// to the given base URL, allowing forwarded requests to hit a mock upstream.
func newTestServerWithBaseURL(t *testing.T, baseURL string) *proxy.Server {
	t.Helper()
	apiKey := "sk-test-key"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: "gpt-4o",
				TianjiParams: config.TianjiParams{
					Model:   "openai/gpt-4o",
					APIKey:  &apiKey,
					APIBase: &baseURL,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}

	handlers := &handler.Handlers{Config: cfg}
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func TestFilesUpload(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/files", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer sk-test-key")

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":         "file-abc123",
			"object":     "file",
			"bytes":      1024,
			"created_at": 1700000000,
			"filename":   "data.jsonl",
			"purpose":    "fine-tune",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	req := httptest.NewRequest(http.MethodPost, "/v1/files", strings.NewReader("file data"))
	req.Header.Set("Content-Type", "multipart/form-data")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "file-abc123", resp["id"])
	assert.Equal(t, "file", resp["object"])
}

func TestFilesList(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/files", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":   []any{},
			"object": "list",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBatchesCreate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/batches", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "batch_abc123",
			"object": "batch",
			"status": "validating",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"input_file_id": "file-abc", "endpoint": "/v1/chat/completions", "completion_window": "24h"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/batches", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "batch_abc123", resp["id"])
}

func TestBatchesList(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/batches", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data":   []any{},
			"object": "list",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	req := httptest.NewRequest(http.MethodGet, "/v1/batches", nil)
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestFineTuningCreate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/fine_tuning/jobs", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":     "ftjob-abc123",
			"object": "fine_tuning.job",
			"status": "queued",
			"model":  "gpt-4o-mini",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"training_file": "file-abc", "model": "gpt-4o-mini"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/fine_tuning/jobs", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "ftjob-abc123", resp["id"])
}

func TestRerank(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/rerank", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "gpt-4o", body["model"])

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"index": 0, "relevance_score": 0.95},
				{"index": 1, "relevance_score": 0.42},
			},
			"model": "gpt-4o",
		})
	}))
	defer upstream.Close()

	srv := newTestServerWithBaseURL(t, upstream.URL)

	body := `{"model": "gpt-4o", "query": "test query", "documents": ["doc1", "doc2"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer sk-master")

	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	results, ok := resp["results"].([]any)
	require.True(t, ok)
	assert.Len(t, results, 2)
}
