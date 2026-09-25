package vertexai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVertexAI_TransformRequest(t *testing.T) {
	p := New("my-project", "us-central1")

	req := &model.ChatCompletionRequest{
		Model: "gemini-2.0-flash",
		Messages: []model.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "test-token")
	require.NoError(t, err)

	assert.Contains(t, httpReq.URL.String(), "us-central1-aiplatform.googleapis.com")
	assert.Contains(t, httpReq.URL.String(), "projects/my-project")
	assert.Contains(t, httpReq.URL.String(), "gemini-2.0-flash:generateContent")
	assert.Equal(t, "Bearer test-token", httpReq.Header.Get("Authorization"))
}

func TestVertexAI_TransformResponse(t *testing.T) {
	geminiResp := map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]any{
						{"text": "Hello from Vertex AI"},
					},
					"role": "model",
				},
				"finishReason": "STOP",
			},
		},
		"usageMetadata": map[string]any{
			"promptTokenCount":     10,
			"candidatesTokenCount": 5,
			"totalTokenCount":      15,
		},
	}

	body, _ := json.Marshal(geminiResp)
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(body)),
	}

	p := New("my-project", "us-central1")
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)

	assert.Len(t, result.Choices, 1)
	assert.Equal(t, "Hello from Vertex AI", result.Choices[0].Message.Content)
	assert.Equal(t, 15, result.Usage.TotalTokens)
}

func TestVertexAI_GetRequestURL(t *testing.T) {
	p := New("my-project", "europe-west4")
	url := p.GetRequestURL("gemini-2.0-flash")
	assert.Contains(t, url, "europe-west4-aiplatform.googleapis.com")
	assert.Contains(t, url, "projects/my-project")
	assert.Contains(t, url, "locations/europe-west4")
}

func TestVertexAI_DefaultLocation(t *testing.T) {
	p := New("proj", "")
	assert.Equal(t, "us-central1", p.location)
}

func TestSageMaker_MockResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(model.ModelResponse{
			Object: "chat.completion",
			Choices: []model.Choice{
				{
					Index:   0,
					Message: &model.Message{Role: "assistant", Content: "Hi from SageMaker"},
				},
			},
		})
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	require.NoError(t, err)

	var result model.ModelResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "Hi from SageMaker", result.Choices[0].Message.Content)
}

// Interface compliance
var _ interface {
	TransformRequest(context.Context, *model.ChatCompletionRequest, string) (*http.Request, error)
} = (*Provider)(nil)
