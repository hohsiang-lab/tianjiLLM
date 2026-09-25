package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	registered, err := provider.Get("ollama")
	require.NoError(t, err)
	require.IsType(t, &Provider{}, registered)
}

func TestGetWithBaseURLPreservesOllamaAdapter(t *testing.T) {
	configured, err := provider.GetWithBaseURL("ollama", "http://ollama.test:11434/v1")
	require.NoError(t, err)
	require.IsType(t, &Provider{}, configured)

	req := &model.EmbeddingRequest{
		Model: "qwen3-embedding:0.6b",
		Input: "query text",
		ExtraParams: map[string]any{
			"input_type": "query",
		},
	}
	embeddingProvider, ok := configured.(provider.EmbeddingProvider)
	require.True(t, ok)

	httpReq, err := embeddingProvider.TransformEmbeddingRequest(context.Background(), req, "")
	require.NoError(t, err)
	assert.Equal(t, "http://ollama.test:11434/v1/embeddings", httpReq.URL.String())
	assert.Equal(t, qwen3QueryPrefix+"query text", decodeRequestBody(t, httpReq)["input"])
}

func TestTransformEmbeddingRequest_Qwen3QueryString(t *testing.T) {
	p := NewWithBaseURL("http://ollama.test:11434/v1")
	req := &model.EmbeddingRequest{
		Model: "qwen3-embedding:0.6b",
		Input: "信用卡退款要多久？",
		ExtraParams: map[string]any{
			"input_type": "query",
			"custom":     "preserved",
		},
	}

	httpReq, err := p.TransformEmbeddingRequest(context.Background(), req, "")
	require.NoError(t, err)
	assert.Equal(t, "http://ollama.test:11434/v1/embeddings", httpReq.URL.String())

	body := decodeRequestBody(t, httpReq)
	assert.Equal(t, qwen3QueryPrefix+"信用卡退款要多久？", body["input"])
	assert.Equal(t, "preserved", body["custom"])
	assert.NotContains(t, body, "input_type")

	assert.Equal(t, "信用卡退款要多久？", req.Input, "the original request must not be mutated")
	assert.Equal(t, "query", req.ExtraParams["input_type"], "the original extra params must not be mutated")
}

func TestTransformEmbeddingRequest_Qwen3QueryArrayFromJSON(t *testing.T) {
	var req model.EmbeddingRequest
	require.NoError(t, json.Unmarshal([]byte(`{
		"model": "qwen3-embedding:0.6b",
		"input": ["退款多久", "忘記密碼"],
		"input_type": "query"
	}`), &req))

	httpReq, err := NewWithBaseURL("http://ollama.test/v1").TransformEmbeddingRequest(context.Background(), &req, "")
	require.NoError(t, err)

	body := decodeRequestBody(t, httpReq)
	assert.Equal(t, []any{
		qwen3QueryPrefix + "退款多久",
		qwen3QueryPrefix + "忘記密碼",
	}, body["input"])
	assert.NotContains(t, body, "input_type")
}

func TestTransformEmbeddingRequest_Qwen3PassageAndDocumentRemainUnchanged(t *testing.T) {
	for _, inputType := range []string{"passage", "document"} {
		t.Run(inputType, func(t *testing.T) {
			req := &model.EmbeddingRequest{
				Model: "qwen3-embedding:0.6b",
				Input: []string{"文件一", "文件二"},
				ExtraParams: map[string]any{
					"input_type": inputType,
				},
			}

			httpReq, err := NewWithBaseURL("http://ollama.test/v1").TransformEmbeddingRequest(context.Background(), req, "")
			require.NoError(t, err)

			body := decodeRequestBody(t, httpReq)
			assert.Equal(t, []any{"文件一", "文件二"}, body["input"])
			assert.NotContains(t, body, "input_type")
		})
	}
}

func TestTransformEmbeddingRequest_NonQwenModelDoesNotReceiveQwenPrefix(t *testing.T) {
	req := &model.EmbeddingRequest{
		Model: "embeddinggemma",
		Input: "query text",
		ExtraParams: map[string]any{
			"input_type": "query",
		},
	}

	httpReq, err := NewWithBaseURL("http://ollama.test/v1").TransformEmbeddingRequest(context.Background(), req, "")
	require.NoError(t, err)

	body := decodeRequestBody(t, httpReq)
	assert.Equal(t, "query text", body["input"])
	assert.NotContains(t, body, "input_type")
}

func TestTransformEmbeddingRequest_DoesNotDoublePrefix(t *testing.T) {
	input := qwen3QueryPrefix + "already formatted"
	req := &model.EmbeddingRequest{
		Model: "qwen3-embedding:0.6b",
		Input: input,
		ExtraParams: map[string]any{
			"input_type": "query",
		},
	}

	httpReq, err := NewWithBaseURL("http://ollama.test/v1").TransformEmbeddingRequest(context.Background(), req, "")
	require.NoError(t, err)

	body := decodeRequestBody(t, httpReq)
	assert.Equal(t, input, body["input"])
}

func TestGetSupportedParamsIncludesInputType(t *testing.T) {
	assert.Contains(t, NewWithBaseURL("http://ollama.test/v1").GetSupportedParams(), "input_type")
}

func TestTransformResponsesAttributeOllamaErrors(t *testing.T) {
	p := NewWithBaseURL("http://ollama.test/v1")
	newErrorResponse := func() *http.Response {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body: io.NopCloser(bytes.NewReader([]byte(
				`{"error":{"message":"model unavailable","type":"server_error"}}`,
			))),
		}
	}

	t.Run("chat", func(t *testing.T) {
		_, err := p.TransformResponse(context.Background(), newErrorResponse())
		var tianjiErr *model.TianjiError
		require.ErrorAs(t, err, &tianjiErr)
		assert.Equal(t, "ollama", tianjiErr.Provider)
	})

	t.Run("embedding", func(t *testing.T) {
		_, err := p.TransformEmbeddingResponse(context.Background(), newErrorResponse())
		var tianjiErr *model.TianjiError
		require.ErrorAs(t, err, &tianjiErr)
		assert.Equal(t, "ollama", tianjiErr.Provider)
	})
}

func decodeRequestBody(t *testing.T, req *http.Request) map[string]any {
	t.Helper()
	defer req.Body.Close()

	data, err := io.ReadAll(req.Body)
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(data, &body))
	return body
}
