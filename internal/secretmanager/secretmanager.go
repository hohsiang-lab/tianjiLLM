package secretmanager

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SecretManager resolves credential references from external vaults.
type SecretManager interface {
	Name() string
	Get(ctx context.Context, path string) (string, error)
	Health(ctx context.Context) error
}

// Factory creates a SecretManager from config.
type Factory func(cfg map[string]any) (SecretManager, error)

var (
	factoryMu sync.RWMutex
	factories = make(map[string]Factory)
)

// Register adds a secret manager factory.
func Register(name string, f Factory) {
	factoryMu.Lock()
	factories[name] = f
	factoryMu.Unlock()
}

// New creates a SecretManager by name from registered factories.
func New(name string, cfg map[string]any) (SecretManager, error) {
	factoryMu.RLock()
	f, ok := factories[name]
	factoryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown secret manager: %s", name)
	}
	return f(cfg)
}

// Names returns all registered secret manager names.
func Names() []string {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	names := make([]string, 0, len(factories))
	for name := range factories {
		names = append(names, name)
	}
	return names
}

// cachedEntry holds a cached secret value with expiration.
type cachedEntry struct {
	value     string
	expiresAt time.Time
}

// CachedSecretManager wraps a SecretManager with per-instance in-memory caching.
// Matches Python LiteLLM's InMemoryCache pattern — no cross-instance sync.
type CachedSecretManager struct {
	inner SecretManager
	ttl   time.Duration
	mu    sync.RWMutex
	cache map[string]cachedEntry
}

// NewCachedSecretManager wraps sm with a TTL cache. Default TTL is 86400s (24h).
func NewCachedSecretManager(sm SecretManager, ttl time.Duration) *CachedSecretManager {
	if ttl <= 0 {
		ttl = 86400 * time.Second
	}
	return &CachedSecretManager{
		inner: sm,
		ttl:   ttl,
		cache: make(map[string]cachedEntry),
	}
}

func (c *CachedSecretManager) Name() string { return c.inner.Name() }

func (c *CachedSecretManager) Get(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("secret path cannot be empty")
	}

	c.mu.RLock()
	entry, ok := c.cache[path]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return entry.value, nil
	}

	val, err := c.inner.Get(ctx, path)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.cache[path] = cachedEntry{
		value:     val,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return val, nil
}

func (c *CachedSecretManager) Health(ctx context.Context) error {
	return c.inner.Health(ctx)
}
