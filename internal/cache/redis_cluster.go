package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCluster implements Cache using Redis Cluster.
// Uses go-redis/v9 ClusterClient which has identical command interface.
// MGet across slots is NOT atomic — SDK splits automatically.
type RedisCluster struct {
	client *redis.ClusterClient
}

// NewRedisCluster creates a Redis Cluster cache.
func NewRedisCluster(addrs []string, password string) *RedisCluster {
	return &RedisCluster{
		client: redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:    addrs,
			Password: password,
		}),
	}
}

func (r *RedisCluster) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (r *RedisCluster) GetShared(ctx context.Context, key string) ([]byte, error) {
	return r.Get(ctx, key)
}

func (r *RedisCluster) SharedCoordinationAvailable() bool { return true }

func (r *RedisCluster) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCluster) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	result, err := compareAndSetScript.Run(ctx, r.client, []string{key}, expected, boolString(expected != nil), strconv.FormatInt(ttl.Milliseconds(), 10), value).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (r *RedisCluster) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, token, ttl).Result()
}

func (r *RedisCluster) ReleaseLock(ctx context.Context, key, token string) error {
	_, err := releaseLockScript.Run(ctx, r.client, []string{key}, token).Result()
	return err
}

func (r *RedisCluster) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCluster) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	result := make([][]byte, len(vals))
	for i, v := range vals {
		if v != nil {
			if s, ok := v.(string); ok {
				result[i] = []byte(s)
			}
		}
	}
	return result, nil
}

// Close closes the cluster client.
func (r *RedisCluster) Close() error {
	return r.client.Close()
}
