package cache

import (
	"context"
	"time"
)

// Cache defines the interface for all cache backends.
type Cache interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	MGet(ctx context.Context, keys ...string) ([][]byte, error)
}

// CompareAndSetCache provides an atomic write for backends that support it.
// A nil expected value means the key must be absent.
type CompareAndSetCache interface {
	Cache
	CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error)
}

// LockCache provides a short-lived distributed lock for a cache key.
type LockCache interface {
	Cache
	AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error)
	ReleaseLock(ctx context.Context, key, token string) error
}

// SharedCache reads from the shared layer of a multi-level cache.
// It is used when a caller cannot tolerate a stale process-local value.
type SharedCache interface {
	GetShared(ctx context.Context, key string) ([]byte, error)
}

// SharedCoordinationCache marks a cache whose reads and locks are shared across replicas.
type SharedCoordinationCache interface {
	SharedCache
	LockCache
	SharedCoordinationAvailable() bool
}
