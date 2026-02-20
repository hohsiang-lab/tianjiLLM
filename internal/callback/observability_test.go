package callback

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogData() LogData {
	return LogData{
		Model:            "gpt-4",
		Provider:         "openai",
		StartTime:        time.Now().Add(-time.Second),
		EndTime:          time.Now(),
		Latency:          time.Second,
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
		Cost:             0.01,
		UserID:           "user-123",
		TeamID:           "team-456",
		Request: &model.ChatCompletionRequest{
			Model: "gpt-4",
			Messages: []model.Message{
				{Role: "user", Content: "Hello"},
			},
		},
		Response: &model.ModelResponse{
			Choices: []model.Choice{
				{Message: &model.Message{Role: "assistant", Content: "Hi there"}},
			},
		},
	}
}

func TestLangsmithLogger_BatchPost(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/runs/batch", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewLangsmithLogger("test-key", server.URL, "test-project")
	cb.batchSize = 1
	cb.Start()
	defer cb.Stop()

	cb.LogSuccess(testLogData())

	time.Sleep(200 * time.Millisecond)
	require.NotEmpty(t, received)

	var payload map[string]any
	err := json.Unmarshal(received, &payload)
	require.NoError(t, err)
	runs, ok := payload["post"].([]any)
	assert.True(t, ok)
	assert.Len(t, runs, 1)
}

func TestHeliconeLogger_LogPost(t *testing.T) {
	var receivedPath string
	var receivedBody []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHeliconeLogger("test-helicone-key", server.URL)
	cb.LogSuccess(testLogData())

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, "/oai/v1/log", receivedPath)
	assert.NotEmpty(t, receivedBody)

	var payload map[string]any
	err := json.Unmarshal(receivedBody, &payload)
	require.NoError(t, err)
	assert.Contains(t, payload, "providerRequest")
	assert.Contains(t, payload, "providerResponse")
	assert.Contains(t, payload, "timing")
}

func TestHeliconeLogger_AnthropicPath(t *testing.T) {
	var receivedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewHeliconeLogger("test-key", server.URL)
	data := testLogData()
	data.Provider = "anthropic"
	cb.LogSuccess(data)

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, "/anthropic/v1/log", receivedPath)
}

func TestBraintrustLogger_BatchSend(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewBraintrustLogger("test-key", server.URL, "test-proj")
	cb.batchSize = 1
	cb.Start()
	defer cb.Stop()

	cb.LogSuccess(testLogData())

	time.Sleep(200 * time.Millisecond)
	assert.NotEmpty(t, received)
}

func TestMLflowLogger_Lifecycle(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		if strings.Contains(r.URL.Path, "create") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"run": map[string]any{"info": map[string]any{"run_id": "run-123"}},
			})
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	cb := NewMLflowLogger(server.URL, "exp-1")
	cb.batchSize = 1
	cb.Start()
	defer cb.Stop()

	cb.LogSuccess(testLogData())

	time.Sleep(500 * time.Millisecond)
	assert.GreaterOrEqual(t, len(paths), 3)
	assert.Contains(t, paths[0], "create")
	assert.Contains(t, paths[1], "log-batch")
	assert.Contains(t, paths[2], "update")
}

func TestArizePhoenixCallback_OpenInferenceAttributes(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := &ArizePhoenixCallback{
		endpoint:    server.URL,
		client:      http.DefaultClient,
		headers:     map[string]string{},
		projectName: "test-project",
	}

	cb.LogSuccess(testLogData())

	time.Sleep(100 * time.Millisecond)
	require.NotEmpty(t, received)

	body := string(received)
	assert.Contains(t, body, "openinference.span.kind")
	assert.Contains(t, body, "llm.model_name")
	assert.Contains(t, body, "llm.token_count.prompt")
	assert.Contains(t, body, "openinference.project.name")
}

func TestWandbLogger_BatchSend(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cb := NewWandbLogger("test-key", "test-proj", "test-entity")
	cb.baseURL = server.URL
	cb.batchSize = 1
	cb.Start()
	defer cb.Stop()

	cb.LogSuccess(testLogData())

	time.Sleep(200 * time.Millisecond)
	assert.NotEmpty(t, received)
}

// Interface compliance
var (
	_ CustomLogger = (*LangsmithLogger)(nil)
	_ CustomLogger = (*HeliconeLogger)(nil)
	_ CustomLogger = (*BraintrustLogger)(nil)
	_ CustomLogger = (*MLflowLogger)(nil)
	_ CustomLogger = (*WandbLogger)(nil)
	_ CustomLogger = (*ArizePhoenixCallback)(nil)
)
