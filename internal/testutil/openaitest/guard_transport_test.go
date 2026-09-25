package openaitest

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNoRealOpenAIGuard_RejectsDefaultHosts(t *testing.T) {
	client := NewGuardedClient()

	_, err := client.Get("https://auth.openai.com/oauth/token")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth.openai.com")

	_, err = client.Get("https://api.openai.com/v1/models")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api.openai.com")
}

func TestNoRealOpenAIGuard_AllowsExplicitMockHost(t *testing.T) {
	called := false
	transport := NewNoRealOpenAIGuard(roundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return nil, errors.New("sentinel")
	}), "127.0.0.1:12345")
	client := &http.Client{Transport: transport}

	_, err := client.Get("http://127.0.0.1:12345/v1/models")
	require.ErrorContains(t, err, "sentinel")
	assert.True(t, called)
}
