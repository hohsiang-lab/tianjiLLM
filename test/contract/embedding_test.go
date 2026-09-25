package contract

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/jina" // register jina provider
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/testutil/openaitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func jinaTestConfig(apiKey, apiBase string) *config.ProxyConfig {
	return &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "jina-embeddings-v3",
			TianjiParams: config.TianjiParams{
				Model:   "jina_ai/jina-embeddings-v3",
				APIKey:  &apiKey,
				APIBase: &apiBase,
			},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: contractMasterKey},
	}
}

func newTestServerFromConfig(t *testing.T, cfg *config.ProxyConfig) *proxy.Server {
	t.Helper()
	return proxy.NewServer(proxy.ServerConfig{
		Handlers:  &handler.Handlers{Config: cfg},
		MasterKey: cfg.GeneralSettings.MasterKey,
	})
}

func serveEmbeddingRequest(t *testing.T, srv http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, newAuthenticatedContractRequest(http.MethodPost, "/v1/embeddings", body))
	return recorder
}

func TestEmbedding_StringInputReturnsNumericVectorAndUsage(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"object":    "embedding",
				"index":     0,
				"embedding": []any{0.1, 0.2, 0.3},
			}},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 3, "total_tokens": 3},
		},
	}, model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"hello","encoding_format":"float"}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response model.EmbeddingResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "list", response.Object)
	assert.Equal(t, contractModel, response.Model)
	require.Len(t, response.Data, 1)
	assert.Equal(t, "embedding", response.Data[0].Object)
	assert.Equal(t, 0, response.Data[0].Index)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, response.Data[0].Embedding)
	assert.Equal(t, 3, response.Usage.PromptTokens)
	assert.Equal(t, 3, response.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, "gpt-4o", upstreamBody["model"])
	assert.Equal(t, "hello", upstreamBody["input"])
	assert.Equal(t, "float", upstreamBody["encoding_format"])
}

func TestEmbedding_ListInputPreservesZeroBasedIndexes(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"object": "embedding", "index": 0, "embedding": []any{0.1, 0.2}},
				map[string]any{"object": "embedding", "index": 1, "embedding": []any{0.3, 0.4}},
			},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 4, "total_tokens": 4},
		},
	}, model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":["one","two"],"encoding_format":"float"}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response model.EmbeddingResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 2)
	assert.Equal(t, 0, response.Data[0].Index)
	assert.Equal(t, 1, response.Data[1].Index)
	assert.Equal(t, 4, response.Usage.PromptTokens)
	assert.Equal(t, 4, response.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, []any{"one", "two"}, upstreamBody["input"])
}

func TestEmbedding_Base64RequestUsesFloatUpstreamAndEncodesResponse(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"object":    "embedding",
				"index":     0,
				"embedding": []any{0.1, 0.2, 0.3},
			}},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 3, "total_tokens": 3},
		},
	}, model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"hello","encoding_format":"base64"}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		Object string `json:"object"`
		Data   []struct {
			Object    string `json:"object"`
			Index     int    `json:"index"`
			Embedding string `json:"embedding"`
		} `json:"data"`
		Model string               `json:"model"`
		Usage model.EmbeddingUsage `json:"usage"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	decoded, err := base64.StdEncoding.DecodeString(response.Data[0].Embedding)
	require.NoError(t, err)
	require.Len(t, decoded, 3*4)
	assert.InDelta(t, 0.1, math.Float32frombits(binary.LittleEndian.Uint32(decoded[0:4])), 0.0001)
	assert.InDelta(t, 0.2, math.Float32frombits(binary.LittleEndian.Uint32(decoded[4:8])), 0.0001)
	assert.InDelta(t, 0.3, math.Float32frombits(binary.LittleEndian.Uint32(decoded[8:12])), 0.0001)
	assert.Equal(t, contractModel, response.Model)
	assert.Equal(t, 3, response.Usage.PromptTokens)
	assert.Equal(t, 3, response.Usage.TotalTokens)

	requests := upstream.Requests()
	require.Len(t, requests, 1)
	var upstreamBody map[string]any
	require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
	assert.Equal(t, "float", upstreamBody["encoding_format"])
}

func TestEmbedding_Base64RejectsFloat32Overflow(t *testing.T) {
	srv, _ := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"object":    "embedding",
				"index":     0,
				"embedding": []any{1e308},
			}},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		},
	}, model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"hello","encoding_format":"base64"}`)

	require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "api_error", response.Error.Type)
	assert.Equal(t, "upstream_error", response.Error.Code)
}

func TestEmbedding_ValidatesInputAndEncodingBeforeUpstream(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		param string
		code  string
	}{
		{name: "missing input", body: `{"model":"contract-model"}`, param: "input", code: "invalid_value"},
		{name: "empty string", body: `{"model":"contract-model","input":""}`, param: "input", code: "invalid_value"},
		{name: "empty list", body: `{"model":"contract-model","input":[]}`, param: "input", code: "invalid_value"},
		{name: "mixed list", body: `{"model":"contract-model","input":["one",2]}`, param: "input", code: "invalid_value"},
		{name: "empty list item", body: `{"model":"contract-model","input":["one",""]}`, param: "input", code: "invalid_value"},
		{name: "scalar number", body: `{"model":"contract-model","input":7}`, param: "input", code: "invalid_value"},
		{name: "token array outside string contract", body: `{"model":"contract-model","input":[1,2,3]}`, param: "input", code: "invalid_value"},
		{name: "token batches outside string contract", body: `{"model":"contract-model","input":[[1,2],[3,4]]}`, param: "input", code: "invalid_value"},
		{name: "unsupported encoding", body: `{"model":"contract-model","input":"one","encoding_format":"hex"}`, param: "encoding_format", code: "invalid_value"},
		{name: "non-positive dimensions", body: `{"model":"contract-model","input":"one","dimensions":0}`, param: "dimensions", code: "invalid_value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{},
				model.CapabilityRecord{SupportsEmbeddings: true, SupportsDimensions: true})

			recorder := serveEmbeddingRequest(t, srv, tt.body)

			require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			requireStandardError(t, recorder.Body.Bytes(), tt.param, tt.code)
			assert.Empty(t, upstream.Requests())
		})
	}
}

func TestEmbedding_RejectsBlankModelBeforeWildcardResolution(t *testing.T) {
	upstream := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"object":    "embedding",
				"index":     0,
				"embedding": []any{0.1},
			}},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		},
	})
	apiKey := "sk-upstream"
	apiBase := upstream.BaseURL()
	srv := newTestServerFromConfig(t, &config.ProxyConfig{
		ModelList: []config.ModelConfig{{
			ModelName: "*",
			TianjiParams: config.TianjiParams{
				Model:   "openaicompat/gpt-4o",
				APIKey:  &apiKey,
				APIBase: &apiBase,
			},
		}},
		GeneralSettings: config.GeneralSettings{MasterKey: contractMasterKey},
	})

	for _, body := range []string{
		`{"input":"one"}`,
		`{"model":"   ","input":"one"}`,
	} {
		recorder := serveEmbeddingRequest(t, srv, body)

		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		requireStandardError(t, recorder.Body.Bytes(), "model", "invalid_value")
	}
	assert.Empty(t, upstream.Requests())
}

func TestEmbedding_DimensionsRequiresExplicitCapability(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{},
		model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"one","dimensions":768}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	requireStandardError(t, recorder.Body.Bytes(), "dimensions", "unsupported_parameter")
	assert.Empty(t, upstream.Requests())
}

func TestEmbedding_RejectsWrongVectorDimensions(t *testing.T) {
	srv, _ := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{map[string]any{
				"object":    "embedding",
				"index":     0,
				"embedding": []any{0.1, 0.2},
			}},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
		},
	}, model.CapabilityRecord{
		SupportsEmbeddings: true,
		SupportsDimensions: true,
		AllowedDimensions:  []int{3},
	})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"one","dimensions":3}`)

	require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "api_error", response.Error.Type)
	assert.Equal(t, "upstream_error", response.Error.Code)
}

func TestEmbedding_DoesNotAssumeGlobal1024Dimension(t *testing.T) {
	capability := model.CapabilityRecord{
		SupportsEmbeddings: true,
		SupportsDimensions: true,
		AllowedDimensions:  []int{768},
	}

	t.Run("allowed model-specific dimension", func(t *testing.T) {
		srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
			EmbeddingBody: map[string]any{
				"object": "list",
				"data": []any{map[string]any{
					"object":    "embedding",
					"index":     0,
					"embedding": make([]float64, 768),
				}},
				"model": "upstream-embedding-model",
				"usage": map[string]any{"prompt_tokens": 1, "total_tokens": 1},
			},
		}, capability)

		recorder := serveEmbeddingRequest(t, srv,
			`{"model":"contract-model","input":"one","dimensions":768}`)

		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		requests := upstream.Requests()
		require.Len(t, requests, 1)
		var upstreamBody map[string]any
		require.NoError(t, json.Unmarshal(requests[0].Body, &upstreamBody))
		assert.Equal(t, float64(768), upstreamBody["dimensions"])
	})

	t.Run("1024 is not a global default", func(t *testing.T) {
		srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{}, capability)

		recorder := serveEmbeddingRequest(t, srv,
			`{"model":"contract-model","input":"one","dimensions":1024}`)

		require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		requireStandardError(t, recorder.Body.Bytes(), "dimensions", "invalid_value")
		assert.Empty(t, upstream.Requests())
	})
}

func TestEmbedding_ModelMustSupportEmbeddings(t *testing.T) {
	srv, upstream := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{},
		model.CapabilityRecord{})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"one"}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	requireStandardError(t, recorder.Body.Bytes(), "model", "unsupported_parameter")
	assert.Empty(t, upstream.Requests())
}

func TestEmbedding_RejectsIncompleteUpstreamResult(t *testing.T) {
	srv, _ := newOpenAIContractServer(t, openaitest.UpstreamServerOptions{
		EmbeddingBody: map[string]any{
			"object": "list",
			"data": []any{
				map[string]any{"object": "embedding", "index": 0, "embedding": []any{0.1}},
			},
			"model": "upstream-embedding-model",
			"usage": map[string]any{"prompt_tokens": 2, "total_tokens": 2},
		},
	}, model.CapabilityRecord{SupportsEmbeddings: true})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":["one","two"]}`)

	require.Equal(t, http.StatusBadGateway, recorder.Code, recorder.Body.String())
	var response model.ErrorResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "api_error", response.Error.Type)
	assert.Equal(t, "upstream_error", response.Error.Code)
	assert.NotContains(t, recorder.Body.String(), "expected 2")
}

func TestEmbedding_UpstreamFailureIsSanitized(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Authorization: Bearer sk-upstream-secret at https://internal.example/v1","type":"invalid_request_error","code":"provider_specific"}}`))
	}))
	defer upstream.Close()

	srv := newOpenAIContractProxy(upstream.URL+"/v1",
		model.CapabilityRecord{SupportsEmbeddings: true})
	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"contract-model","input":"one"}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	requireStandardError(t, recorder.Body.Bytes(), "", "invalid_request")
	assert.NotContains(t, recorder.Body.String(), "sk-upstream-secret")
	assert.NotContains(t, recorder.Body.String(), "internal.example")
	assert.NotContains(t, recorder.Body.String(), "provider_specific")
}

func TestEmbedding_InvalidModel(t *testing.T) {
	srv, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})

	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"nonexistent-embedding","input":"hello"}`)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	requireStandardError(t, recorder.Body.Bytes(), "", "model_not_found")
}

func TestEmbedding_InvalidJSON(t *testing.T) {
	srv, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})

	recorder := serveEmbeddingRequest(t, srv, `{broken`)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	requireStandardError(t, recorder.Body.Bytes(), "", "invalid_request")
}

func TestEmbedding_NoAuth(t *testing.T) {
	srv, _ := newDirectOpenAIContractServer(t, openaitest.UpstreamServerOptions{})
	req := httptest.NewRequest(http.MethodPost, "/v1/embeddings",
		strings.NewReader(`{"model":"contract-model","input":"hello"}`))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	srv.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestEmbedding_JinaExtraParams(t *testing.T) {
	var capturedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],"model":"jina-embeddings-v3","usage":{"total_tokens":3}}`))
	}))
	defer upstream.Close()

	cfg := jinaTestConfig("sk-jina-test", upstream.URL)
	srv := newTestServerFromConfig(t, cfg)
	recorder := serveEmbeddingRequest(t, srv, `{
		"model":"jina-embeddings-v3",
		"input":"hello",
		"encoding_format":"float",
		"task":"retrieval.query",
		"normalized":true
	}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotEmpty(t, capturedBody)
	var upstreamRequest map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &upstreamRequest))
	assert.Equal(t, "float", upstreamRequest["encoding_format"])
	assert.Equal(t, "retrieval.query", upstreamRequest["task"])
	assert.Equal(t, true, upstreamRequest["normalized"])
	var response model.EmbeddingResponse
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, 3, response.Usage.PromptTokens)
	assert.Equal(t, 3, response.Usage.TotalTokens)
}

func TestEmbedding_JinaBase64UsesStandardFloatTranslation(t *testing.T) {
	var capturedBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],"model":"jina-embeddings-v3","usage":{"prompt_tokens":3,"total_tokens":3}}`))
	}))
	defer upstream.Close()

	srv := newTestServerFromConfig(t, jinaTestConfig("sk-jina-test", upstream.URL))
	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"jina-embeddings-v3","input":"hello","encoding_format":"base64"}`)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.NotEmpty(t, capturedBody)
	var upstreamRequest map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &upstreamRequest))
	assert.Equal(t, "float", upstreamRequest["encoding_format"])
	var response struct {
		Data []struct {
			Embedding string `json:"embedding"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.NotEmpty(t, response.Data[0].Embedding)
}

func TestEmbedding_JinaDimensionsRequireExplicitModelCapability(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls++
	}))
	defer upstream.Close()

	srv := newTestServerFromConfig(t, jinaTestConfig("sk-jina-test", upstream.URL))
	recorder := serveEmbeddingRequest(t, srv,
		`{"model":"jina-embeddings-v3","input":"hello","dimensions":1024}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	requireStandardError(t, recorder.Body.Bytes(), "dimensions", "unsupported_parameter")
	assert.Zero(t, calls)
}
