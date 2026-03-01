package githubcopilot

import (
	"net/http"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("github_copilot")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("github_copilot")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestSetupHeaders_WithKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "copilot-token")
	assert.Equal(t, "Bearer copilot-token", req.Header.Get("Authorization"))
	assert.Equal(t, "tianjiLLM/1.0", req.Header.Get("Editor-Version"))
	assert.Equal(t, "tianjiLLM", req.Header.Get("Copilot-Integration-Id"))
}

func TestSetupHeaders_NoKey(t *testing.T) {
	p := &Provider{}
	req, _ := http.NewRequest(http.MethodPost, "https://example.com", nil)
	p.SetupHeaders(req, "")
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "tianjiLLM/1.0", req.Header.Get("Editor-Version"))
}
