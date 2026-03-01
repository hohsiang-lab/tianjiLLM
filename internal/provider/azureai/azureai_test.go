package azureai

import (
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("azure_ai")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("azure_ai")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
	assert.Contains(t, params, "model")
}

func TestSetupHeaders_WithKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "azure-key")
	assert.Equal(t, "Bearer azure-key", req.Header.Get("Authorization"))
}

func TestSetupHeaders_NoKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "")
	assert.Empty(t, req.Header.Get("Authorization"))
}
