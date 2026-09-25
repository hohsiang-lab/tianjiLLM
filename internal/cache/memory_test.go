package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_SetAndGet(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	err := mc.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	val, err := mc.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)
}

func TestMemoryCache_GetMiss(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	val, err := mc.Get(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, val)
}

func TestMemoryCache_TTLExpiry(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	err := mc.Set(ctx, "key1", []byte("value1"), 50*time.Millisecond)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	val, err := mc.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestMemoryCache_Delete(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	_ = mc.Set(ctx, "key1", []byte("value1"), time.Hour)
	err := mc.Delete(ctx, "key1")
	require.NoError(t, err)

	val, err := mc.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestMemoryCache_DeleteNonexistent(t *testing.T) {
	mc := NewMemoryCache()
	err := mc.Delete(context.Background(), "nonexistent")
	assert.NoError(t, err)
}

func TestMemoryCache_MGet(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	_ = mc.Set(ctx, "a", []byte("1"), time.Hour)
	_ = mc.Set(ctx, "b", []byte("2"), time.Hour)

	vals, err := mc.MGet(ctx, "a", "b", "c")
	require.NoError(t, err)
	assert.Equal(t, []byte("1"), vals[0])
	assert.Equal(t, []byte("2"), vals[1])
	assert.Nil(t, vals[2])
}

func TestMemoryCache_MGetExpired(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	_ = mc.Set(ctx, "a", []byte("1"), 50*time.Millisecond)
	_ = mc.Set(ctx, "b", []byte("2"), time.Hour)

	time.Sleep(100 * time.Millisecond)

	vals, err := mc.MGet(ctx, "a", "b")
	require.NoError(t, err)
	assert.Nil(t, vals[0])
	assert.Equal(t, []byte("2"), vals[1])
}

func TestMemoryCache_Overwrite(t *testing.T) {
	mc := NewMemoryCache()
	ctx := context.Background()

	_ = mc.Set(ctx, "key", []byte("v1"), time.Hour)
	_ = mc.Set(ctx, "key", []byte("v2"), time.Hour)

	val, err := mc.Get(ctx, "key")
	require.NoError(t, err)
	assert.Equal(t, []byte("v2"), val)
}

func TestMemoryCache_InterfaceCompliance(t *testing.T) {
	var _ Cache = (*MemoryCache)(nil)
}
