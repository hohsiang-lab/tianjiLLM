package snowflake

import (
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("snowflake")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("snowflake")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestSetupHeaders_WithKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "snow-key")
	assert.Equal(t, "Bearer snow-key", req.Header.Get("Authorization"))
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestSetupHeaders_NoKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "")
	assert.Empty(t, req.Header.Get("Authorization"))
}
