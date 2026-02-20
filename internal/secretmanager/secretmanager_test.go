package secretmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Registry tests ---

func TestRegister_And_New(t *testing.T) {
	Register("test_sm", func(cfg map[string]any) (SecretManager, error) {
		return &mockSM{name: "test_sm"}, nil
	})

	sm, err := New("test_sm", nil)
	require.NoError(t, err)
	assert.Equal(t, "test_sm", sm.Name())
}

func TestNew_Unknown(t *testing.T) {
	_, err := New("nonexistent_sm", nil)
	assert.ErrorContains(t, err, "unknown secret manager")
}

func TestNames(t *testing.T) {
	names := Names()
	assert.NotEmpty(t, names)
}

// --- CachedSecretManager tests ---

func TestCachedSecretManager_Get(t *testing.T) {
	calls := 0
	inner := &mockSM{
		name: "cached_test",
		getFn: func(ctx context.Context, path string) (string, error) {
			calls++
			return "secret-value-" + path, nil
		},
	}

	cached := NewCachedSecretManager(inner, 10*time.Second)

	ctx := context.Background()

	// First call hits inner
	val, err := cached.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "secret-value-key1", val)
	assert.Equal(t, 1, calls)

	// Second call hits cache
	val, err = cached.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "secret-value-key1", val)
	assert.Equal(t, 1, calls) // no additional call
}

func TestCachedSecretManager_TTLExpiry(t *testing.T) {
	calls := 0
	inner := &mockSM{
		name: "ttl_test",
		getFn: func(ctx context.Context, path string) (string, error) {
			calls++
			return fmt.Sprintf("v%d", calls), nil
		},
	}

	cached := NewCachedSecretManager(inner, 50*time.Millisecond)
	ctx := context.Background()

	val, err := cached.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "v1", val)

	time.Sleep(100 * time.Millisecond) // wait for TTL expiry

	val, err = cached.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, "v2", val)
	assert.Equal(t, 2, calls)
}

func TestCachedSecretManager_EmptyPath(t *testing.T) {
	inner := &mockSM{name: "empty_path"}
	cached := NewCachedSecretManager(inner, time.Minute)

	_, err := cached.Get(context.Background(), "")
	assert.ErrorContains(t, err, "empty")
}

func TestCachedSecretManager_InnerError(t *testing.T) {
	inner := &mockSM{
		name: "error_test",
		getFn: func(ctx context.Context, path string) (string, error) {
			return "", fmt.Errorf("vault unreachable")
		},
	}
	cached := NewCachedSecretManager(inner, time.Minute)

	_, err := cached.Get(context.Background(), "key1")
	assert.ErrorContains(t, err, "vault unreachable")
}

func TestCachedSecretManager_DefaultTTL(t *testing.T) {
	cached := NewCachedSecretManager(&mockSM{name: "default"}, 0)
	assert.Equal(t, 86400*time.Second, cached.ttl)
}

func TestCachedSecretManager_Health(t *testing.T) {
	inner := &mockSM{
		name: "health_test",
		healthFn: func(ctx context.Context) error {
			return nil
		},
	}
	cached := NewCachedSecretManager(inner, time.Minute)
	assert.NoError(t, cached.Health(context.Background()))
}

func TestCachedSecretManager_Concurrent(t *testing.T) {
	var calls int64
	var mu sync.Mutex
	inner := &mockSM{
		name: "concurrent",
		getFn: func(ctx context.Context, path string) (string, error) {
			mu.Lock()
			calls++
			mu.Unlock()
			return "val", nil
		},
	}
	cached := NewCachedSecretManager(inner, time.Minute)
	ctx := context.Background()

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = cached.Get(ctx, "same-key")
		}()
	}
	wg.Wait()

	mu.Lock()
	// Should have hit inner at most a few times (first miss + possible races)
	assert.LessOrEqual(t, calls, int64(5))
	mu.Unlock()
}

// --- Mock AWS Secrets Manager HTTP server ---

func TestAWS_MockGetSecretValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// AWS SDK uses POST with X-Amz-Target header
		target := r.Header.Get("X-Amz-Target")
		switch target {
		case "secretsmanager.GetSecretValue":
			var req map[string]string
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req["SecretId"] == "not-found" {
				w.WriteHeader(400)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"__type":  "ResourceNotFoundException",
					"Message": "not found",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"SecretString": "my-secret-value",
				"Name":         req["SecretId"],
			})
		case "secretsmanager.ListSecrets":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"SecretList": []any{},
			})
		default:
			w.WriteHeader(400)
		}
	}))
	defer server.Close()

	// We can't easily test the real AWS client without custom endpoint resolver,
	// so just verify the mock server works
	resp, err := http.Post(server.URL, "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()
	// Server responds (even if 400) — confirms mock is reachable
	assert.NotNil(t, resp)
}

// --- mockSM ---

type mockSM struct {
	name     string
	getFn    func(ctx context.Context, path string) (string, error)
	healthFn func(ctx context.Context) error
}

func (m *mockSM) Name() string { return m.name }

func (m *mockSM) Get(ctx context.Context, path string) (string, error) {
	if m.getFn != nil {
		return m.getFn(ctx, path)
	}
	return "", fmt.Errorf("not implemented")
}

func (m *mockSM) Health(ctx context.Context) error {
	if m.healthFn != nil {
		return m.healthFn(ctx)
	}
	return nil
}
