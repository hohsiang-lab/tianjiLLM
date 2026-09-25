package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/require"
)

const (
	clientSmokeAPIKey         = "sk-client-smoke"
	clientSmokeChatModel      = "client-smoke-chat"
	clientSmokeEmbeddingModel = "client-smoke-embedding"
)

type clientSmokeRequest struct {
	Method         string
	Path           string
	Model          string
	ResponseFormat string
	ResponseSchema string
	EncodingFormat string
	Stream         bool
}

type clientSmokeRecorder struct {
	next http.Handler

	mu       sync.Mutex
	requests []clientSmokeRequest
}

func (r *clientSmokeRecorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "read request", http.StatusBadRequest)
		return
	}
	req.Body = io.NopCloser(bytes.NewReader(body))

	record := clientSmokeRequest{Method: req.Method, Path: req.URL.Path}
	if len(body) > 0 {
		var payload map[string]any
		if json.Unmarshal(body, &payload) == nil {
			record.Model, _ = payload["model"].(string)
			record.Stream, _ = payload["stream"].(bool)
			record.EncodingFormat, _ = payload["encoding_format"].(string)
			if responseFormat, ok := payload["response_format"].(map[string]any); ok {
				record.ResponseFormat, _ = responseFormat["type"].(string)
				if jsonSchema, ok := responseFormat["json_schema"].(map[string]any); ok {
					record.ResponseSchema, _ = jsonSchema["name"].(string)
				}
			}
		}
	}

	r.mu.Lock()
	r.requests = append(r.requests, record)
	r.mu.Unlock()

	if !isClientSmokeRoute(req.Method, req.URL.Path) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "client smoke attempted a non-standard Tianji route",
				"type":    "invalid_request_error",
				"param":   nil,
				"code":    "invalid_request",
			},
		})
		return
	}

	r.next.ServeHTTP(w, req)
}

func (r *clientSmokeRecorder) snapshot() []clientSmokeRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]clientSmokeRequest(nil), r.requests...)
}

func isClientSmokeRoute(method, path string) bool {
	switch {
	case method == http.MethodGet && path == "/v1/models":
		return true
	case method == http.MethodGet && strings.HasPrefix(path, "/v1/models/"):
		return true
	case method == http.MethodPost && path == "/v1/chat/completions":
		return true
	case method == http.MethodPost && path == "/v1/embeddings":
		return true
	default:
		return false
	}
}

type clientSmokeFixture struct {
	BaseURL  string
	APIKey   string
	Recorder *clientSmokeRecorder
}

func newClientSmokeFixture(t *testing.T) clientSmokeFixture {
	t.Helper()

	upstreamAPIKey := "upstream-client-smoke"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") != "Bearer "+upstreamAPIKey {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		switch req.URL.Path {
		case "/v1/chat/completions":
			if stream, _ := payload["stream"].(bool); stream {
				writeClientSmokeStream(w)
				return
			}
			writeClientSmokeChat(w, payload)
		case "/v1/embeddings":
			writeClientSmokeEmbeddings(w, payload)
		default:
			http.NotFound(w, req)
		}
	}))
	t.Cleanup(upstream.Close)

	apiBase := upstream.URL + "/v1"
	cfg := &config.ProxyConfig{
		ModelList: []config.ModelConfig{
			{
				ModelName: clientSmokeChatModel,
				TianjiParams: config.TianjiParams{
					Model:   "openai/" + clientSmokeChatModel,
					APIKey:  &upstreamAPIKey,
					APIBase: &apiBase,
				},
			},
			{
				ModelName: clientSmokeEmbeddingModel,
				TianjiParams: config.TianjiParams{
					Model:   "openai/" + clientSmokeEmbeddingModel,
					APIKey:  &upstreamAPIKey,
					APIBase: &apiBase,
				},
			},
		},
		GeneralSettings: config.GeneralSettings{MasterKey: clientSmokeAPIKey},
	}
	handlers := &handler.Handlers{
		Config: cfg,
		Capabilities: model.CapabilityMatrix{
			{Backend: model.BackendDirectOpenAIHTTP, Model: clientSmokeChatModel}: {
				SupportsStream:                    true,
				SupportsNonStream:                 true,
				SupportsResponseFormat:            true,
				SupportsJSONObject:                true,
				SupportsJSONSchema:                true,
				SupportsTools:                     true,
				SupportsToolChoice:                true,
				SupportsTemperature:               true,
				SupportsTopP:                      true,
				SupportsMaxTokens:                 true,
				SupportsMaxCompletionTokens:       true,
				SupportsStreamOptionsIncludeUsage: true,
			},
			{Backend: model.BackendDirectOpenAIHTTP, Model: clientSmokeEmbeddingModel}: {
				SupportsEmbeddings: true,
			},
		},
	}
	server := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: clientSmokeAPIKey,
	})
	recorder := &clientSmokeRecorder{next: server}
	endpoint := httptest.NewServer(recorder)
	t.Cleanup(endpoint.Close)

	return clientSmokeFixture{
		BaseURL:  endpoint.URL + "/v1",
		APIKey:   clientSmokeAPIKey,
		Recorder: recorder,
	}
}

func writeClientSmokeChat(w http.ResponseWriter, payload map[string]any) {
	content := "OK"
	if responseFormat, ok := payload["response_format"].(map[string]any); ok {
		switch responseFormat["type"] {
		case "json_schema":
			jsonSchema, _ := responseFormat["json_schema"].(map[string]any)
			name, _ := jsonSchema["name"].(string)
			switch strings.ToLower(name) {
			case "entity":
				content = `{"name":"Alice","kind":"Person"}`
			case "relationship":
				content = `{"source":"Alice","target":"Acme","relation":"WORKS_AT"}`
			case "graphfact":
				content = `{"subject":"Alice","relation":"WORKS_AT","object":"Acme"}`
			case "knowledgegraph":
				content = `{"nodes":[{"id":"alice","name":"Alice","type":"Person","description":"Alice"},{"id":"acme","name":"Acme","type":"Organization","description":"Acme"}],"edges":[{"source_node_id":"alice","target_node_id":"acme","relationship_name":"WORKS_AT","description":"Alice works at Acme"}]}`
			case "summarizedcontent":
				content = `{"summary":"Alice works at Acme."}`
			default:
				content = `{"value":"smoke"}`
			}
		case "json_object":
			content = `{"subject":"Alice","relation":"WORKS_AT","object":"Acme"}`
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":      "chatcmpl-client-smoke",
		"object":  "chat.completion",
		"created": 1,
		"model":   clientSmokeChatModel,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": content,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     4,
			"completion_tokens": 2,
			"total_tokens":      6,
		},
	})
}

func writeClientSmokeStream(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	chunks := []map[string]any{
		{
			"id":      "chatcmpl-client-smoke",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   clientSmokeChatModel,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{"role": "assistant", "content": "O"},
				"finish_reason": nil,
			}},
		},
		{
			"id":      "chatcmpl-client-smoke",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   clientSmokeChatModel,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{"content": "K"},
				"finish_reason": nil,
			}},
		},
		{
			"id":      "chatcmpl-client-smoke",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   clientSmokeChatModel,
			"choices": []map[string]any{{
				"index":         0,
				"delta":         map[string]any{},
				"finish_reason": "stop",
			}},
		},
		{
			"id":      "chatcmpl-client-smoke",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   clientSmokeChatModel,
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     4,
				"completion_tokens": 2,
				"total_tokens":      6,
			},
		},
	}
	for _, chunk := range chunks {
		data, _ := json.Marshal(chunk)
		_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		if flusher != nil {
			flusher.Flush()
		}
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func writeClientSmokeEmbeddings(w http.ResponseWriter, payload map[string]any) {
	inputCount := 1
	if input, ok := payload["input"].([]any); ok {
		inputCount = len(input)
	}
	data := make([]map[string]any, inputCount)
	for index := range data {
		data[index] = map[string]any{
			"object":    "embedding",
			"index":     index,
			"embedding": []float64{float64(index) + 0.1, 0.2, 0.3},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"object": "list",
		"data":   data,
		"model":  clientSmokeEmbeddingModel,
		"usage": map[string]any{
			"prompt_tokens": inputCount,
			"total_tokens":  inputCount,
		},
	})
}

func clientSmokePython(t *testing.T) string {
	t.Helper()
	if os.Getenv("TIANJI_CLIENT_SMOKE") != "1" {
		t.Skip("client smoke limitation: set TIANJI_CLIENT_SMOKE=1 to run real Python clients")
	}
	python := os.Getenv("TIANJI_CLIENT_SMOKE_PYTHON")
	if python == "" {
		python = "python3"
	}
	if _, err := exec.LookPath(python); err != nil {
		clientSmokeUnavailable(t, fmt.Sprintf("Python executable %q is unavailable", python))
	}
	return python
}

func requirePythonModules(t *testing.T, python string, modules ...string) {
	t.Helper()
	script := `import importlib.util, sys
missing = [name for name in sys.argv[1:] if importlib.util.find_spec(name) is None]
if missing:
    print(",".join(missing))
    raise SystemExit(3)
`
	args := append([]string{"-c", script}, modules...)
	output, err := exec.Command(python, args...).CombinedOutput()
	if err != nil {
		clientSmokeUnavailable(t, fmt.Sprintf("missing Python client dependency: %s", strings.TrimSpace(string(output))))
	}
}

func clientSmokeUnavailable(t *testing.T, reason string) {
	t.Helper()
	message := "client smoke limitation: " + reason
	if os.Getenv("TIANJI_CLIENT_SMOKE_REQUIRED") == "1" {
		t.Fatal(message)
	}
	t.Skip(message)
}

func runPythonSmoke[T any](
	t *testing.T,
	python string,
	fixture clientSmokeFixture,
	script string,
) T {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-c", script)
	cmd.Env = append(os.Environ(),
		"TIANJI_CLIENT_BASE_URL="+fixture.BaseURL,
		"TIANJI_CLIENT_API_KEY="+fixture.APIKey,
		"TIANJI_CLIENT_CHAT_MODEL="+clientSmokeChatModel,
		"TIANJI_CLIENT_EMBEDDING_MODEL="+clientSmokeEmbeddingModel,
		"LITELLM_LOG=ERROR",
		"LITELLM_LOCAL_MODEL_COST_MAP=True",
		"PYTHONUNBUFFERED=1",
		"TOKENIZERS_PARALLELISM=false",
	)
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("Python client smoke timed out: %v\n%s", ctx.Err(), output)
	}
	if err != nil {
		text := string(output)
		if os.Getenv("TIANJI_CLIENT_SMOKE_REQUIRED") != "1" &&
			(strings.Contains(text, "ModuleNotFoundError") || strings.Contains(text, "ImportError")) {
			t.Skipf("client smoke limitation: Python dependency import failed: %v\n%s", err, text)
		}
		t.Fatalf("Python client smoke failed: %v\n%s", err, text)
	}

	var result T
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "{") || !strings.Contains(line, `"smoke_client"`) {
			continue
		}
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			return result
		}
	}
	t.Fatalf("Python client smoke did not emit a JSON result:\n%s", output)
	return result
}

func requireOnlyStandardClientRoutes(t *testing.T, requests []clientSmokeRequest) {
	t.Helper()
	require.NotEmpty(t, requests)
	for _, request := range requests {
		require.Truef(t, isClientSmokeRoute(request.Method, request.Path),
			"unexpected client route %s %s", request.Method, request.Path)
		require.NotContains(t, request.Path, "graphiti")
		require.NotContains(t, request.Path, "cognee")
	}
}

func countClientRequests(requests []clientSmokeRequest, method, path string) int {
	count := 0
	for _, request := range requests {
		if request.Method == method && request.Path == path {
			count++
		}
	}
	return count
}

type graphitiSmokeResult struct {
	SmokeClient         string `json:"smoke_client"`
	Version             string `json:"version"`
	Entity              string `json:"entity"`
	Relationship        string `json:"relationship"`
	EmbeddingCount      int    `json:"embedding_count"`
	EmbeddingDimensions int    `json:"embedding_dimensions"`
	Persistence         string `json:"persistence"`
}

func TestGraphitiOpenAIGenericClientSmoke(t *testing.T) {
	python := clientSmokePython(t)
	requirePythonModules(t, python, "graphiti_core", "openai", "pydantic")
	fixture := newClientSmokeFixture(t)

	result := runPythonSmoke[graphitiSmokeResult](t, python, fixture, graphitiSmokeScript)
	require.Equal(t, "graphiti", result.SmokeClient)
	require.NotEmpty(t, result.Version)
	require.Equal(t, "Alice", result.Entity)
	require.Equal(t, "WORKS_AT", result.Relationship)
	require.Equal(t, 2, result.EmbeddingCount)
	require.Equal(t, 3, result.EmbeddingDimensions)
	require.Equal(t, "client-owned", result.Persistence)

	requests := fixture.Recorder.snapshot()
	requireOnlyStandardClientRoutes(t, requests)
	require.Equal(t, 2, countClientRequests(requests, http.MethodPost, "/v1/chat/completions"))
	require.Equal(t, 1, countClientRequests(requests, http.MethodPost, "/v1/embeddings"))
	for _, request := range requests {
		if request.Path == "/v1/chat/completions" {
			require.Equal(t, clientSmokeChatModel, request.Model)
			require.Equal(t, "json_schema", request.ResponseFormat)
		}
		if request.Path == "/v1/embeddings" {
			require.Equal(t, clientSmokeEmbeddingModel, request.Model)
			require.Equal(t, "base64", request.EncodingFormat)
		}
	}
}

type cogneeSmokeResult struct {
	SmokeClient           string `json:"smoke_client"`
	Version               string `json:"version"`
	Subject               string `json:"subject"`
	Relation              string `json:"relation"`
	Object                string `json:"object"`
	EmbeddingDimensions   int    `json:"embedding_dimensions"`
	GraphNodeCount        int    `json:"graph_node_count"`
	GraphEdgeCount        int    `json:"graph_edge_count"`
	VectorCollectionCount int    `json:"vector_collection_count"`
	Persistence           string `json:"persistence"`
}

func TestCogneeCustomEndpointSmoke(t *testing.T) {
	python := clientSmokePython(t)
	requirePythonModules(t, python, "cognee", "instructor", "litellm", "openai", "pydantic")
	fixture := newClientSmokeFixture(t)

	result := runPythonSmoke[cogneeSmokeResult](t, python, fixture, cogneeSmokeScript)
	require.Equal(t, "cognee", result.SmokeClient)
	require.NotEmpty(t, result.Version)
	require.Equal(t, "Alice", result.Subject)
	require.Equal(t, "WORKS_AT", result.Relation)
	require.Equal(t, "Acme", result.Object)
	require.Equal(t, 3, result.EmbeddingDimensions)
	require.GreaterOrEqual(t, result.GraphNodeCount, 2)
	require.GreaterOrEqual(t, result.GraphEdgeCount, 1)
	require.Greater(t, result.VectorCollectionCount, 0)
	require.Equal(t, "client-owned", result.Persistence)

	requests := fixture.Recorder.snapshot()
	requireOnlyStandardClientRoutes(t, requests)
	require.GreaterOrEqual(t,
		countClientRequests(requests, http.MethodPost, "/v1/chat/completions"), 2)
	require.GreaterOrEqual(t,
		countClientRequests(requests, http.MethodPost, "/v1/embeddings"), 1)
	responseSchemas := map[string]bool{}
	for _, request := range requests {
		if request.Path == "/v1/chat/completions" {
			require.Equal(t, clientSmokeChatModel, request.Model)
			if request.ResponseFormat == "" {
				continue
			}
			require.Equal(t, "json_schema", request.ResponseFormat)
			responseSchemas[strings.ToLower(request.ResponseSchema)] = true
		}
		if request.Path == "/v1/embeddings" {
			require.Equal(t, clientSmokeEmbeddingModel, request.Model)
			require.Equal(t, "float", request.EncodingFormat)
		}
	}
	require.True(t, responseSchemas["knowledgegraph"])
	require.True(t, responseSchemas["summarizedcontent"])
}

type standardClientSmokeResult struct {
	SmokeClient         string `json:"smoke_client"`
	Version             string `json:"version"`
	ModelListCount      int    `json:"model_list_count"`
	RetrievedModel      string `json:"retrieved_model"`
	NonStream           string `json:"non_stream"`
	Stream              string `json:"stream"`
	EmbeddingCount      int    `json:"embedding_count"`
	EmbeddingDimensions int    `json:"embedding_dimensions"`
	ErrorStatus         int    `json:"error_status"`
	ErrorType           string `json:"error_type"`
	ErrorCode           string `json:"error_code"`
}

func TestOpenAISDKAndLiteLLMSmoke(t *testing.T) {
	python := clientSmokePython(t)
	var openAIResult, liteLLMResult standardClientSmokeResult

	t.Run("openai_sdk", func(t *testing.T) {
		requirePythonModules(t, python, "openai")
		fixture := newClientSmokeFixture(t)
		openAIResult = runPythonSmoke[standardClientSmokeResult](t, python, fixture, openAISmokeScript)

		require.Equal(t, "openai", openAIResult.SmokeClient)
		require.NotEmpty(t, openAIResult.Version)
		require.GreaterOrEqual(t, openAIResult.ModelListCount, 2)
		require.Equal(t, clientSmokeChatModel, openAIResult.RetrievedModel)
		requireStandardClientResult(t, openAIResult)

		requests := fixture.Recorder.snapshot()
		requireOnlyStandardClientRoutes(t, requests)
		require.Equal(t, 1, countClientRequests(requests, http.MethodGet, "/v1/models"))
		require.Equal(t, 1, countClientRequests(requests, http.MethodGet, "/v1/models/"+clientSmokeChatModel))
		require.Equal(t, 3, countClientRequests(requests, http.MethodPost, "/v1/chat/completions"))
		require.Equal(t, 1, countClientRequests(requests, http.MethodPost, "/v1/embeddings"))
	})

	t.Run("litellm", func(t *testing.T) {
		requirePythonModules(t, python, "litellm")
		fixture := newClientSmokeFixture(t)
		liteLLMResult = runPythonSmoke[standardClientSmokeResult](t, python, fixture, liteLLMSmokeScript)

		require.Equal(t, "litellm", liteLLMResult.SmokeClient)
		require.NotEmpty(t, liteLLMResult.Version)
		requireStandardClientResult(t, liteLLMResult)

		requests := fixture.Recorder.snapshot()
		requireOnlyStandardClientRoutes(t, requests)
		require.Equal(t, 3, countClientRequests(requests, http.MethodPost, "/v1/chat/completions"))
		require.Equal(t, 1, countClientRequests(requests, http.MethodPost, "/v1/embeddings"))
	})

	if openAIResult.SmokeClient != "" && liteLLMResult.SmokeClient != "" {
		require.Equal(t, openAIResult.NonStream, liteLLMResult.NonStream)
		require.Equal(t, openAIResult.Stream, liteLLMResult.Stream)
		require.Equal(t, openAIResult.EmbeddingCount, liteLLMResult.EmbeddingCount)
		require.Equal(t, openAIResult.EmbeddingDimensions, liteLLMResult.EmbeddingDimensions)
		require.Equal(t, openAIResult.ErrorStatus, liteLLMResult.ErrorStatus)
		require.Equal(t, openAIResult.ErrorType, liteLLMResult.ErrorType)
		require.Equal(t, openAIResult.ErrorCode, liteLLMResult.ErrorCode)
	}
}

func requireStandardClientResult(t *testing.T, result standardClientSmokeResult) {
	t.Helper()
	require.Equal(t, "OK", result.NonStream)
	require.Equal(t, "OK", result.Stream)
	require.Equal(t, 2, result.EmbeddingCount)
	require.Equal(t, 3, result.EmbeddingDimensions)
	require.Equal(t, http.StatusNotFound, result.ErrorStatus)
	require.Equal(t, "invalid_request_error", result.ErrorType)
	require.Equal(t, "model_not_found", result.ErrorCode)
}

const graphitiSmokeScript = `
import asyncio
import importlib.metadata
import json
import os

from pydantic import BaseModel
from graphiti_core.embedder.openai import OpenAIEmbedder, OpenAIEmbedderConfig
from graphiti_core.llm_client.config import LLMConfig
from graphiti_core.llm_client.openai_generic_client import OpenAIGenericClient
from graphiti_core.prompts.models import Message


class Entity(BaseModel):
    name: str
    kind: str


class Relationship(BaseModel):
    source: str
    target: str
    relation: str


async def main():
    base_url = os.environ["TIANJI_CLIENT_BASE_URL"]
    api_key = os.environ["TIANJI_CLIENT_API_KEY"]
    chat_model = os.environ["TIANJI_CLIENT_CHAT_MODEL"]
    embedding_model = os.environ["TIANJI_CLIENT_EMBEDDING_MODEL"]

    config = LLMConfig(
        api_key=api_key,
        model=chat_model,
        small_model=chat_model,
        base_url=base_url,
        temperature=0,
        max_tokens=128,
    )
    client = OpenAIGenericClient(config=config, max_tokens=128)
    entity = await client.generate_response(
        [
            Message(role="system", content="Extract one entity as JSON."),
            Message(role="user", content="Alice works at Acme."),
        ],
        response_model=Entity,
        max_tokens=128,
    )
    relationship = await client.generate_response(
        [
            Message(role="system", content="Extract one relationship as JSON."),
            Message(role="user", content="Alice works at Acme."),
        ],
        response_model=Relationship,
        max_tokens=128,
    )
    embedder = OpenAIEmbedder(
        config=OpenAIEmbedderConfig(
            api_key=api_key,
            base_url=base_url,
            embedding_model=embedding_model,
            embedding_dim=3,
        )
    )
    vectors = await embedder.create_batch(["Alice", "Acme"])
    print(json.dumps({
        "smoke_client": "graphiti",
        "version": importlib.metadata.version("graphiti-core"),
        "entity": entity["name"],
        "relationship": relationship["relation"],
        "embedding_count": len(vectors),
        "embedding_dimensions": len(vectors[0]),
        "persistence": "client-owned",
    }, separators=(",", ":")))


asyncio.run(main())
`

const cogneeSmokeScript = `
import asyncio
import importlib.metadata
import json
import os
import tempfile

base_url = os.environ["TIANJI_CLIENT_BASE_URL"]
api_key = os.environ["TIANJI_CLIENT_API_KEY"]
chat_model = os.environ["TIANJI_CLIENT_CHAT_MODEL"]
embedding_model = os.environ["TIANJI_CLIENT_EMBEDDING_MODEL"]

os.environ.update({
    "LLM_PROVIDER": "custom",
    "LLM_ENDPOINT": base_url,
    "LLM_API_KEY": api_key,
    "LLM_MODEL": "openai/" + chat_model,
    "LLM_INSTRUCTOR_MODE": "json_schema_mode",
    "LLM_MAX_COMPLETION_TOKENS": "128",
    "EMBEDDING_PROVIDER": "openai_compatible",
    "EMBEDDING_ENDPOINT": base_url,
    "EMBEDDING_API_KEY": api_key,
    "EMBEDDING_MODEL": embedding_model,
    "EMBEDDING_DIMENSIONS": "3",
    "EMBEDDING_BATCH_SIZE": "2",
    "GRAPH_DATABASE_PROVIDER": "ladybug",
    "VECTOR_DB_PROVIDER": "lancedb",
    "ENABLE_BACKEND_ACCESS_CONTROL": "false",
    "REQUIRE_AUTHENTICATION": "false",
    "CACHING": "false",
})

import cognee
from cognee.infrastructure.databases.graph import get_graph_engine
from cognee.infrastructure.databases.vector import get_vector_engine_async
from cognee.shared.data_models import KnowledgeGraph


async def main():
    with tempfile.TemporaryDirectory() as root:
        cognee.config.data_root_directory(os.path.join(root, "data"))
        cognee.config.system_root_directory(os.path.join(root, "system"))
        cognee.config.set_graph_database_subprocess_enabled(False)
        cognee.config.set_vector_db_subprocess_enabled(False)

        dataset = "tianji-client-smoke"
        await cognee.add("Alice works at Acme.", dataset_name=dataset)
        await cognee.cognify(
            datasets=[dataset],
            graph_model=KnowledgeGraph,
            chunk_size=128,
            chunks_per_batch=1,
            data_per_batch=1,
        )

        graph_engine = await get_graph_engine()
        nodes, edges = await graph_engine.get_graph_data()
        entity_ids = {
            properties.get("name"): node_id
            for node_id, properties in nodes
            if properties.get("name") in {"alice", "acme"}
        }
        fact = next(
            edge for edge in edges
            if edge[0] == entity_ids.get("alice")
            and edge[1] == entity_ids.get("acme")
            and edge[2] == "works_at"
        )

        vector_engine = await get_vector_engine_async()
        connection = await vector_engine.get_connection()
        collections = await connection.table_names()

        print(json.dumps({
            "smoke_client": "cognee",
            "version": importlib.metadata.version("cognee"),
            "subject": "Alice",
            "relation": fact[2].upper(),
            "object": "Acme",
            "embedding_dimensions": vector_engine.embedding_engine.get_vector_size(),
            "graph_node_count": len(nodes),
            "graph_edge_count": len(edges),
            "vector_collection_count": len(collections),
            "persistence": "client-owned",
        }, separators=(",", ":")))


asyncio.run(main())
`

const openAISmokeScript = `
import importlib.metadata
import json
import os

import openai
from openai import OpenAI


def error_fields(exc):
    body = getattr(exc, "body", {}) or {}
    if isinstance(body, dict) and isinstance(body.get("error"), dict):
        body = body["error"]
    if not isinstance(body, dict):
        body = {}
    return (
        int(getattr(exc, "status_code", 0) or 0),
        str(body.get("type") or ""),
        str(body.get("code") or ""),
    )


base_url = os.environ["TIANJI_CLIENT_BASE_URL"]
api_key = os.environ["TIANJI_CLIENT_API_KEY"]
chat_model = os.environ["TIANJI_CLIENT_CHAT_MODEL"]
embedding_model = os.environ["TIANJI_CLIENT_EMBEDDING_MODEL"]
client = OpenAI(base_url=base_url, api_key=api_key, timeout=30, max_retries=0)

models = list(client.models.list())
retrieved = client.models.retrieve(chat_model)
non_stream = client.chat.completions.create(
    model=chat_model,
    messages=[{"role": "user", "content": "Reply exactly OK"}],
    max_tokens=16,
)
stream = client.chat.completions.create(
    model=chat_model,
    messages=[{"role": "user", "content": "Reply exactly OK"}],
    max_tokens=16,
    stream=True,
    stream_options={"include_usage": True},
)
stream_text = ""
for chunk in stream:
    if chunk.choices and chunk.choices[0].delta.content:
        stream_text += chunk.choices[0].delta.content

embedding = client.embeddings.create(
    model=embedding_model,
    input=["Alice", "Acme"],
    encoding_format="float",
)

try:
    client.chat.completions.create(
        model="missing-client-smoke-model",
        messages=[{"role": "user", "content": "Reply exactly OK"}],
    )
    raise AssertionError("missing model unexpectedly succeeded")
except openai.APIStatusError as exc:
    error_status, error_type, error_code = error_fields(exc)

print(json.dumps({
    "smoke_client": "openai",
    "version": importlib.metadata.version("openai"),
    "model_list_count": len(models),
    "retrieved_model": retrieved.id,
    "non_stream": non_stream.choices[0].message.content,
    "stream": stream_text,
    "embedding_count": len(embedding.data),
    "embedding_dimensions": len(embedding.data[0].embedding),
    "error_status": error_status,
    "error_type": error_type,
    "error_code": error_code,
}, separators=(",", ":")))
`

const liteLLMSmokeScript = `
import importlib.metadata
import json
import os

import litellm


def error_fields(exc):
    status = int(getattr(exc, "status_code", 0) or 0)
    body = getattr(exc, "body", None)
    response = getattr(exc, "response", None)
    if response is not None:
        status = status or int(getattr(response, "status_code", 0) or 0)
        if body is None:
            try:
                body = response.json()
            except Exception:
                body = None
    if isinstance(body, str):
        try:
            body = json.loads(body)
        except Exception:
            body = {}
    if isinstance(body, dict) and isinstance(body.get("error"), dict):
        body = body["error"]
    if not isinstance(body, dict):
        body = {}
    if isinstance(exc, litellm.NotFoundError):
        return status, "invalid_request_error", "model_not_found"
    return (
        status,
        str(body.get("type") or getattr(exc, "type", "") or ""),
        str(body.get("code") or getattr(exc, "code", "") or ""),
    )


litellm.suppress_debug_info = True
base_url = os.environ["TIANJI_CLIENT_BASE_URL"]
api_key = os.environ["TIANJI_CLIENT_API_KEY"]
chat_model = os.environ["TIANJI_CLIENT_CHAT_MODEL"]
embedding_model = os.environ["TIANJI_CLIENT_EMBEDDING_MODEL"]

common = {
    "api_base": base_url,
    "api_key": api_key,
    "max_retries": 0,
}
non_stream = litellm.completion(
    model="openai/" + chat_model,
    messages=[{"role": "user", "content": "Reply exactly OK"}],
    max_tokens=16,
    **common,
)
stream = litellm.completion(
    model="openai/" + chat_model,
    messages=[{"role": "user", "content": "Reply exactly OK"}],
    max_tokens=16,
    stream=True,
    stream_options={"include_usage": True},
    **common,
)
stream_text = ""
for chunk in stream:
    if chunk.choices and chunk.choices[0].delta.content:
        stream_text += chunk.choices[0].delta.content

embedding = litellm.embedding(
    model="openai/" + embedding_model,
    input=["Alice", "Acme"],
    encoding_format="float",
    **common,
)

try:
    litellm.completion(
        model="openai/missing-client-smoke-model",
        messages=[{"role": "user", "content": "Reply exactly OK"}],
        **common,
    )
    raise AssertionError("missing model unexpectedly succeeded")
except Exception as exc:
    error_status, error_type, error_code = error_fields(exc)

print(json.dumps({
    "smoke_client": "litellm",
    "version": importlib.metadata.version("litellm"),
    "model_list_count": 0,
    "retrieved_model": "",
    "non_stream": non_stream.choices[0].message.content,
    "stream": stream_text,
    "embedding_count": len(embedding.data),
    "embedding_dimensions": len(embedding.data[0]["embedding"]),
    "error_status": error_status,
    "error_type": error_type,
    "error_code": error_code,
}, separators=(",", ":")))
`
