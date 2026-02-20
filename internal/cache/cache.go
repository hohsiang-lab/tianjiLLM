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
