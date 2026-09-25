package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisCacheCompareAndSet(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewRedisCache(client)

	updated, err := cache.CompareAndSet(context.Background(), "catalog", nil, []byte("first"), time.Minute)
	require.NoError(t, err)
	require.True(t, updated)

	updated, err = cache.CompareAndSet(context.Background(), "catalog", []byte("stale"), []byte("second"), time.Minute)
	require.NoError(t, err)
	require.False(t, updated)
	value, err := cache.Get(context.Background(), "catalog")
	require.NoError(t, err)
	require.Equal(t, []byte("first"), value)

	updated, err = cache.CompareAndSet(context.Background(), "catalog", []byte("first"), []byte("second"), time.Minute)
	require.NoError(t, err)
	require.True(t, updated)
	value, err = cache.Get(context.Background(), "catalog")
	require.NoError(t, err)
	require.Equal(t, []byte("second"), value)
}

func TestRedisCacheLockOwnership(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := NewRedisCache(client)
	ctx := context.Background()

	acquired, err := cache.AcquireLock(ctx, "catalog:lock", "owner-a", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	acquired, err = cache.AcquireLock(ctx, "catalog:lock", "owner-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.NoError(t, cache.ReleaseLock(ctx, "catalog:lock", "owner-b"))
	acquired, err = cache.AcquireLock(ctx, "catalog:lock", "owner-b", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.NoError(t, cache.ReleaseLock(ctx, "catalog:lock", "owner-a"))
	acquired, err = cache.AcquireLock(ctx, "catalog:lock", "owner-b", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
}

func TestDualCacheGetSharedBypassesStaleLocalValue(t *testing.T) {
	server := miniredis.RunT(t)
	clientA := redis.NewClient(&redis.Options{Addr: server.Addr()})
	clientB := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = clientA.Close()
		_ = clientB.Close()
	})

	first := NewDualCache(NewMemoryCache(), NewRedisCache(clientA))
	second := NewDualCache(NewMemoryCache(), NewRedisCache(clientB))
	ctx := context.Background()
	require.NoError(t, first.Set(ctx, "catalog", []byte("old"), time.Minute))
	require.Equal(t, []byte("old"), mustCacheGet(t, second, ctx, "catalog"))
	require.True(t, mustCacheCompareAndSet(t, first, ctx, "catalog", []byte("old"), []byte("new")))

	require.Equal(t, []byte("old"), mustCacheGet(t, second, ctx, "catalog"))
	value, err := second.GetShared(ctx, "catalog")
	require.NoError(t, err)
	require.Equal(t, []byte("new"), value)
}

func mustCacheGet(t *testing.T, cache Cache, ctx context.Context, key string) []byte {
	t.Helper()
	value, err := cache.Get(ctx, key)
	require.NoError(t, err)
	return value
}

func mustCacheCompareAndSet(t *testing.T, cache CompareAndSetCache, ctx context.Context, key string, expected, value []byte) bool {
	t.Helper()
	updated, err := cache.CompareAndSet(ctx, key, expected, value, time.Minute)
	require.NoError(t, err)
	return updated
}
