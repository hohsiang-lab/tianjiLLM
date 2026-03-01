package deepgram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformRequest(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}

	req := &model.ChatCompletionRequest{
		Model: "general",
		Messages: []model.Message{
			{Role: "user", Content: "audio-data-bytes"},
		},
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "test-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Contains(t, httpReq.URL.String(), "/v1/listen")
	assert.Equal(t, "Token test-key", httpReq.Header.Get("Authorization"))
	assert.Equal(t, "audio/wav", httpReq.Header.Get("Content-Type"))
}

func TestTransformResponse_Success(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}

	body := `{
		"results": {
			"channels": [{
				"alternatives": [{
					"transcript": "Hello world",
					"confidence": 0.95
				}]
			}]
		}
	}`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&bytesReader{data: []byte(body)}),
	}

	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "Hello world", result.Choices[0].Message.Content)
}

type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func TestTransformResponse_EmptyResult(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}

	body := `{"results": {"channels": []}}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(&bytesReader{data: []byte(body)}),
	}

	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Equal(t, "", result.Choices[0].Message.Content)
}

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(&bytesReader{data: []byte(`{"err_msg":"invalid credentials"}`)}),
	}

	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
}

func TestGetRequestURL(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}
	url := p.GetRequestURL("general")
	assert.Equal(t, "https://api.deepgram.com/v1/listen", url)
}

func TestSetupHeaders(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	p.SetupHeaders(req, "my-key")

	assert.Equal(t, "Token my-key", req.Header.Get("Authorization"))
	assert.Equal(t, "audio/wav", req.Header.Get("Content-Type"))
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{baseURL: "https://api.deepgram.com"}
	params := p.GetSupportedParams()
	assert.Contains(t, params, "model")
	assert.Contains(t, params, "messages")
	assert.Contains(t, params, "language")
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	chunk, done, err := p.TransformStreamChunk(context.Background(), []byte("data"))
	assert.Nil(t, chunk)
	assert.True(t, done)
	assert.Error(t, err)
}

func TestMapParams(t *testing.T) {
	p := &Provider{}
	in := map[string]any{"language": "en"}
	assert.Equal(t, in, p.MapParams(in))
}
