package scheduler

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
	keys map[string]string
	err  error
}

func (m *mockFetcher) FetchKey(_ context.Context, cred string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if key, ok := m.keys[cred]; ok {
		return key, nil
	}
	return "", fmt.Errorf("key not found: %s", cred)
}

type mockSwapper struct {
	mu      sync.Mutex
	swapped map[string]string
}

func (m *mockSwapper) SwapKey(cred, newKey string) {
	m.mu.Lock()
	m.swapped[cred] = newKey
	m.mu.Unlock()
}

func TestProviderKeyRotation_Success(t *testing.T) {
	fetcher := &mockFetcher{
		keys: map[string]string{
			"openai":    "sk-new-openai-key",
			"anthropic": "sk-new-anthropic-key",
		},
	}
	swapper := &mockSwapper{swapped: make(map[string]string)}

	job := &ProviderKeyRotationJob{
		Fetcher:     fetcher,
		Swapper:     swapper,
		Credentials: []string{"openai", "anthropic"},
	}

	err := job.Run(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "sk-new-openai-key", swapper.swapped["openai"])
	assert.Equal(t, "sk-new-anthropic-key", swapper.swapped["anthropic"])
}

func TestProviderKeyRotation_FetchError(t *testing.T) {
	fetcher := &mockFetcher{
		keys: map[string]string{"openai": "sk-new"},
	}
	swapper := &mockSwapper{swapped: make(map[string]string)}

	job := &ProviderKeyRotationJob{
		Fetcher:     fetcher,
		Swapper:     swapper,
		Credentials: []string{"openai", "missing-cred"},
	}

	err := job.Run(context.Background())
	require.NoError(t, err) // continues on individual failures

	// openai should be swapped, missing-cred should not
	assert.Equal(t, "sk-new", swapper.swapped["openai"])
	assert.Empty(t, swapper.swapped["missing-cred"])
}

func TestProviderKeyRotation_EmptyCredentials(t *testing.T) {
	fetcher := &mockFetcher{keys: map[string]string{}}
	swapper := &mockSwapper{swapped: make(map[string]string)}

	job := &ProviderKeyRotationJob{
		Fetcher:     fetcher,
		Swapper:     swapper,
		Credentials: []string{},
	}

	err := job.Run(context.Background())
	require.NoError(t, err)
	assert.Empty(t, swapper.swapped)
}
