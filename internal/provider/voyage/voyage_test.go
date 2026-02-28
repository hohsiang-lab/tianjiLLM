package voyage

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("voyage")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("voyage")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestGetRequestURL(t *testing.T) {
	p, _ := provider.Get("voyage")
	url := p.GetRequestURL("test-model")
	assert.NotEmpty(t, url)
}
