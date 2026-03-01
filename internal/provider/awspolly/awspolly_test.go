package awspolly

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("aws_polly")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestTransformRequest(t *testing.T) {
	p := &Provider{baseURL: "https://polly.us-east-1.amazonaws.com"}
	req := &model.ChatCompletionRequest{
		Model:    "Joanna",
		Messages: []model.Message{{Role: "user", Content: "Hello world"}},
	}
	httpReq, err := p.TransformRequest(context.Background(), req, "key")
	require.NoError(t, err)
	assert.Contains(t, httpReq.URL.String(), "/v1/speech")
}

func TestTransformRequest_NoMessages(t *testing.T) {
	p := &Provider{baseURL: "https://polly.us-east-1.amazonaws.com"}
	req := &model.ChatCompletionRequest{Model: "Matthew", Messages: []model.Message{}}
	httpReq, err := p.TransformRequest(context.Background(), req, "key")
	require.NoError(t, err)
	assert.NotNil(t, httpReq)
}

func TestTransformResponse_Success(t *testing.T) {
	p := &Provider{}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte("audio-data"))),
	}
	result, err := p.TransformResponse(context.Background(), resp)
	require.NoError(t, err)
	require.Len(t, result.Choices, 1)
	assert.Contains(t, result.Choices[0].Message.Content.(string), "audio:")
}

func TestTransformResponse_Error(t *testing.T) {
	p := &Provider{}
	resp := &http.Response{
		StatusCode: http.StatusUnauthorized,
		Body:       io.NopCloser(bytes.NewReader([]byte("unauthorized"))),
	}
	_, err := p.TransformResponse(context.Background(), resp)
	require.Error(t, err)
	var tianjiErr *model.TianjiError
	require.ErrorAs(t, err, &tianjiErr)
	assert.Equal(t, "aws_polly", tianjiErr.Provider)
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
	in := map[string]any{"voice": "Joanna"}
	assert.Equal(t, in, p.MapParams(in))
}

func TestSetupHeaders_WithKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "my-key")
	assert.Equal(t, "Bearer my-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestSetupHeaders_NoKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "")
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
	assert.Empty(t, req.Header.Get("Authorization"))
}

func TestGetSupportedParams(t *testing.T) {
	p := &Provider{}
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}
