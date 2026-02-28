package nvidia

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("nvidia_nim")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("nvidia_nim")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestGetRequestURL(t *testing.T) {
	p, _ := provider.Get("nvidia_nim")
	url := p.GetRequestURL("test-model")
	assert.NotEmpty(t, url)
}
