package cache

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	data      []byte
	expiresAt time.Time
}

// MemoryCache is an in-memory cache using sync.Map with TTL support.
type MemoryCache struct {
	mu    sync.RWMutex
	items map[string]memoryEntry
}

// NewMemoryCache creates a new in-memory cache and starts a background
// cleanup goroutine.
func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{
		items: make(map[string]memoryEntry),
	}
	go mc.cleanup()
	return mc
}

func (m *MemoryCache) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	entry, ok := m.items[key]
	m.mu.RUnlock()

	if !ok || time.Now().After(entry.expiresAt) {
		return nil, nil
	}
	return entry.data, nil
}

func (m *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	m.mu.Lock()
	m.items[key] = memoryEntry{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
	return nil
}

func (m *MemoryCache) MGet(_ context.Context, keys ...string) ([][]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	results := make([][]byte, len(keys))
	for i, key := range keys {
		if entry, ok := m.items[key]; ok && now.Before(entry.expiresAt) {
			results[i] = entry.data
		}
	}
	return results, nil
}

func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		m.mu.Lock()
		for k, v := range m.items {
			if now.After(v.expiresAt) {
				delete(m.items, k)
			}
		}
		m.mu.Unlock()
	}
}
