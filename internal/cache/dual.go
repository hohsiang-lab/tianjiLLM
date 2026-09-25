package cache

import (
	"context"
	"time"
)

// DualCache implements the three-layer cache architecture:
// Read: In-Memory (µs) → Redis (ms)
// Write: In-Memory + Redis
type DualCache struct {
	memory *MemoryCache
	redis  *RedisCache
}

// NewDualCache creates a new dual-layer cache.
func NewDualCache(memory *MemoryCache, redisCache *RedisCache) *DualCache {
	return &DualCache{memory: memory, redis: redisCache}
}

func (d *DualCache) Get(ctx context.Context, key string) ([]byte, error) {
	// Try in-memory first
	val, err := d.memory.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		return val, nil
	}

	// Fall back to Redis
	if d.redis == nil {
		return nil, nil
	}
	val, err = d.redis.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val != nil {
		// Backfill memory cache
		_ = d.memory.Set(ctx, key, val, 5*time.Minute)
	}
	return val, nil
}

func (d *DualCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	if d.redis == nil {
		return d.memory.GetShared(ctx, key)
	}
	return d.redis.GetShared(ctx, key)
}

func (d *DualCache) SharedCoordinationAvailable() bool { return d != nil && d.redis != nil }

func (d *DualCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if d.redis != nil {
		if err := d.redis.Set(ctx, key, value, ttl); err != nil {
			return err
		}
	}
	return d.memory.Set(ctx, key, value, ttl)
}

func (d *DualCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	if d.redis == nil {
		return d.memory.CompareAndSet(ctx, key, expected, value, ttl)
	}
	updated, err := d.redis.CompareAndSet(ctx, key, expected, value, ttl)
	if err != nil {
		return false, err
	}
	if updated {
		_ = d.memory.Set(ctx, key, value, ttl)
		return true, nil
	}
	current, getErr := d.redis.Get(ctx, key)
	if getErr != nil {
		return false, getErr
	}
	if current == nil {
		_ = d.memory.Delete(ctx, key)
	} else {
		_ = d.memory.Set(ctx, key, current, 5*time.Minute)
	}
	return false, nil
}

func (d *DualCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	if d.redis == nil {
		return d.memory.AcquireLock(ctx, key, token, ttl)
	}
	return d.redis.AcquireLock(ctx, key, token, ttl)
}

func (d *DualCache) ReleaseLock(ctx context.Context, key, token string) error {
	if d.redis == nil {
		return d.memory.ReleaseLock(ctx, key, token)
	}
	return d.redis.ReleaseLock(ctx, key, token)
}

func (d *DualCache) Delete(ctx context.Context, key string) error {
	if d.redis != nil {
		if err := d.redis.Delete(ctx, key); err != nil {
			return err
		}
	}
	return d.memory.Delete(ctx, key)
}

func (d *DualCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	results, err := d.memory.MGet(ctx, keys...)
	if err != nil {
		return nil, err
	}

	// Find misses
	if d.redis == nil {
		return results, nil
	}

	var missKeys []string
	var missIndices []int
	for i, v := range results {
		if v == nil {
			missKeys = append(missKeys, keys[i])
			missIndices = append(missIndices, i)
		}
	}

	if len(missKeys) == 0 {
		return results, nil
	}

	redisResults, err := d.redis.MGet(ctx, missKeys...)
	if err != nil {
		return results, nil // degrade gracefully
	}

	for i, val := range redisResults {
		if val != nil {
			idx := missIndices[i]
			results[idx] = val
			_ = d.memory.Set(ctx, keys[idx], val, 5*time.Minute)
		}
	}

	return results, nil
}
