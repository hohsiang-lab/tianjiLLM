package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDualCache_MemoryOnly_SetAndGet(t *testing.T) {
	mem := NewMemoryCache()
	dc := NewDualCache(mem, nil)
	ctx := context.Background()

	err := dc.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	val, err := dc.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)
}

func TestDualCache_MemoryOnly_GetMiss(t *testing.T) {
	mem := NewMemoryCache()
	dc := NewDualCache(mem, nil)

	val, err := dc.Get(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestDualCache_MemoryOnly_Delete(t *testing.T) {
	mem := NewMemoryCache()
	dc := NewDualCache(mem, nil)
	ctx := context.Background()

	_ = dc.Set(ctx, "key1", []byte("value1"), time.Hour)
	err := dc.Delete(ctx, "key1")
	require.NoError(t, err)

	val, err := dc.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestDualCache_MemoryOnly_MGet(t *testing.T) {
	mem := NewMemoryCache()
	dc := NewDualCache(mem, nil)
	ctx := context.Background()

	_ = dc.Set(ctx, "a", []byte("1"), time.Hour)
	_ = dc.Set(ctx, "b", []byte("2"), time.Hour)

	vals, err := dc.MGet(ctx, "a", "b", "c")
	require.NoError(t, err)
	assert.Equal(t, []byte("1"), vals[0])
	assert.Equal(t, []byte("2"), vals[1])
	assert.Nil(t, vals[2])
}

func TestDualCache_RedisWriteFailureDoesNotPublishLocalValue(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	cache := NewDualCache(NewMemoryCache(), NewRedisCache(client))
	ctx := context.Background()

	require.NoError(t, cache.Set(ctx, "device-flow", []byte("old"), time.Hour))
	server.Close()

	require.Error(t, cache.Set(ctx, "device-flow", []byte("new"), time.Hour))
	got, err := cache.memory.Get(ctx, "device-flow")
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), got)
}

func TestDualCache_InterfaceCompliance(t *testing.T) {
	var _ Cache = (*DualCache)(nil)
}
