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

func (d *DualCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := d.memory.Set(ctx, key, value, ttl); err != nil {
		return err
	}
	if d.redis != nil {
		return d.redis.Set(ctx, key, value, ttl)
	}
	return nil
}

func (d *DualCache) Delete(ctx context.Context, key string) error {
	if err := d.memory.Delete(ctx, key); err != nil {
		return err
	}
	if d.redis != nil {
		return d.redis.Delete(ctx, key)
	}
	return nil
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
