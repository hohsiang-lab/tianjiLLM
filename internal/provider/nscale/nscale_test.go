package nscale

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderRegistered(t *testing.T) {
	p, err := provider.Get("nscale")
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestGetSupportedParams(t *testing.T) {
	p, _ := provider.Get("nscale")
	params := p.GetSupportedParams()
	assert.NotEmpty(t, params)
}

func TestGetRequestURL(t *testing.T) {
	p, _ := provider.Get("nscale")
	url := p.GetRequestURL("test-model")
	assert.NotEmpty(t, url)
}
