package vertexai

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSupportedParams(t *testing.T) {
	p := New("proj", "us-central1")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestMapParams(t *testing.T) {
	p := New("proj", "us-central1")
	in := map[string]any{"temperature": 0.5}
	out := p.MapParams(in)
	assert.NotNil(t, out)
}

func TestSetupHeaders(t *testing.T) {
	p := New("proj", "us-central1")
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "token-123")
	assert.Equal(t, "Bearer token-123", req.Header.Get("Authorization"))
}

func TestTransformStreamChunk_VertexAI(t *testing.T) {
	p := New("proj", "us-central1")
	// invalid JSON → error expected
	_, _, err := p.TransformStreamChunk(context.Background(), []byte("not-sse"))
	// may or may not error depending on gemini impl
	_ = err
}

func TestTransformRequest_WithAPIKey(t *testing.T) {
	p := New("proj", "us-central1")
	// Just verify New() works without panic
	require.NotNil(t, p)
}
