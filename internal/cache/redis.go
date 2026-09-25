package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements Cache using go-redis v9.
type RedisCache struct {
	client redis.UniversalClient
}

var compareAndSetScript = redis.NewScript(`
local current = redis.call("GET", KEYS[1])
local expected_present = ARGV[2] == "1"
if (expected_present and current and current == ARGV[1]) or (not expected_present and not current) then
  local ttl_ms = tonumber(ARGV[3])
  if ttl_ms > 0 then
    redis.call("SET", KEYS[1], ARGV[4], "PX", ttl_ms)
  else
    redis.call("SET", KEYS[1], ARGV[4])
  end
  return 1
end
return 0
`)

var releaseLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

// NewRedisCache creates a new Redis-backed cache.
func NewRedisCache(client redis.UniversalClient) *RedisCache {
	return &RedisCache{client: client}
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

func (r *RedisCache) GetShared(ctx context.Context, key string) ([]byte, error) {
	return r.Get(ctx, key)
}

func (r *RedisCache) SharedCoordinationAvailable() bool { return true }

func (r *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *RedisCache) CompareAndSet(ctx context.Context, key string, expected, value []byte, ttl time.Duration) (bool, error) {
	result, err := compareAndSetScript.Run(ctx, r.client, []string{key}, expected, boolString(expected != nil), strconv.FormatInt(ttl.Milliseconds(), 10), value).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (r *RedisCache) AcquireLock(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, token, ttl).Result()
}

func (r *RedisCache) ReleaseLock(ctx context.Context, key, token string) error {
	_, err := releaseLockScript.Run(ctx, r.client, []string{key}, token).Result()
	return err
}

func (r *RedisCache) Delete(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *RedisCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	results := make([][]byte, len(vals))
	for i, v := range vals {
		if v != nil {
			if s, ok := v.(string); ok {
				results[i] = []byte(s)
			}
		}
	}
	return results, nil
}

// Client returns the underlying Redis client for advanced operations
// (Lua scripts, pub/sub, etc.).
func (r *RedisCache) Client() redis.UniversalClient {
	return r.client
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
