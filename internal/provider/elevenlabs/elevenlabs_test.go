package elevenlabs

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
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	req := &model.ChatCompletionRequest{
		Model: "21m00Tcm4TlvDq8ikWAM",
		Messages: []model.Message{
			{Role: "user", Content: "Hello, world!"},
		},
	}

	httpReq, err := p.TransformRequest(context.Background(), req, "test-key")
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, httpReq.Method)
	assert.Contains(t, httpReq.URL.String(), "/v1/text-to-speech/21m00Tcm4TlvDq8ikWAM")
	assert.Equal(t, "test-key", httpReq.Header.Get("xi-api-key"))
	assert.Equal(t, "application/json", httpReq.Header.Get("Content-Type"))
	assert.Equal(t, "audio/mpeg", httpReq.Header.Get("Accept"))

	body, _ := io.ReadAll(httpReq.Body)
	assert.Contains(t, string(body), `"text":"Hello, world!"`)
}

func TestTransformResponse_Success(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	audioBytes := []byte("fake-audio-data")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(io.LimitReader(io.NopCloser(nil), 0)),
	}
	// Create a proper response with audio data
	resp.Body = io.NopCloser(io.LimitReader(
		io.NopCloser(
			&bytesReader{data: audioBytes},
		), int64(len(audioBytes)),
	))

	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	assert.Len(t, result.Choices, 1)
	assert.Contains(t, result.Choices[0].Message.Content, "[audio:")
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

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(&bytesReader{data: []byte(`{"detail":{"status":"unauthorized"}}`)}),
	}

	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestGetRequestURL(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	url := p.GetRequestURL("21m00Tcm4TlvDq8ikWAM")
	assert.Equal(t, "https://api.elevenlabs.io/v1/text-to-speech/21m00Tcm4TlvDq8ikWAM", url)
}

func TestGetRequestURL_WithProviderPrefix(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	url := p.GetRequestURL("elevenlabs/21m00Tcm4TlvDq8ikWAM")
	assert.Equal(t, "https://api.elevenlabs.io/v1/text-to-speech/21m00Tcm4TlvDq8ikWAM", url)
}

func TestSetupHeaders(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	p.SetupHeaders(req, "my-api-key")

	assert.Equal(t, "my-api-key", req.Header.Get("xi-api-key"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Equal(t, "audio/mpeg", req.Header.Get("Accept"))
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{baseURL: "https://api.elevenlabs.io"}
	params := p.GetSupportedParams()
	assert.Contains(t, params, "model")
	assert.Contains(t, params, "messages")
}

func TestTransformStreamChunk(t *testing.T) {
	p := &Provider{}
	chunk, done, err := p.TransformStreamChunk(context.Background(), nil)
	assert.Nil(t, chunk)
	assert.True(t, done)
	assert.Error(t, err)
}

func TestMapParams(t *testing.T) {
	p := &Provider{}
	in := map[string]any{"voice": "Rachel"}
	assert.Equal(t, in, p.MapParams(in))
}
