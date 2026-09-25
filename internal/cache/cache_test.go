package cache

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskCache_SetAndGet(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	require.NoError(t, err)

	ctx := context.Background()

	err = dc.Set(ctx, "key1", []byte("value1"), time.Hour)
	require.NoError(t, err)

	val, err := dc.Get(ctx, "key1")
	require.NoError(t, err)
	assert.Equal(t, []byte("value1"), val)
}

func TestDiskCache_TTLExpiry(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	require.NoError(t, err)

	ctx := context.Background()

	err = dc.Set(ctx, "key1", []byte("value1"), 50*time.Millisecond)
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	val, err := dc.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestDiskCache_Delete(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	require.NoError(t, err)

	ctx := context.Background()

	_ = dc.Set(ctx, "key1", []byte("value1"), time.Hour)
	err = dc.Delete(ctx, "key1")
	require.NoError(t, err)

	val, err := dc.Get(ctx, "key1")
	assert.NoError(t, err)
	assert.Nil(t, val)
}

func TestDiskCache_MGet(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	require.NoError(t, err)

	ctx := context.Background()

	_ = dc.Set(ctx, "a", []byte("1"), time.Hour)
	_ = dc.Set(ctx, "b", []byte("2"), time.Hour)

	vals, err := dc.MGet(ctx, "a", "b", "c")
	require.NoError(t, err)
	assert.Equal(t, []byte("1"), vals[0])
	assert.Equal(t, []byte("2"), vals[1])
	assert.Nil(t, vals[2])
}

func TestDiskCache_FilesCreated(t *testing.T) {
	dir := t.TempDir()
	dc, _ := NewDiskCache(dir)

	_ = dc.Set(context.Background(), "testkey", []byte("testvalue"), time.Hour)

	entries, _ := os.ReadDir(dir)
	assert.True(t, len(entries) > 0)
}

func TestDiskCache_KeyHashing(t *testing.T) {
	dir := t.TempDir()
	dc, _ := NewDiskCache(dir)

	_ = dc.Set(context.Background(), "key/with/slashes", []byte("val"), time.Hour)
	val, _ := dc.Get(context.Background(), "key/with/slashes")
	assert.Equal(t, []byte("val"), val)
}

func TestRedisCluster_InterfaceCompliance(t *testing.T) {
	var _ Cache = (*RedisCluster)(nil)
}

func TestDiskCache_InterfaceCompliance(t *testing.T) {
	var _ Cache = (*DiskCache)(nil)
}

func TestSemanticCache_InterfaceCompliance(t *testing.T) {
	var _ Cache = (*SemanticCache)(nil)
}

func TestDiskCache_CleanupOnDelete(t *testing.T) {
	dir := t.TempDir()
	dc, _ := NewDiskCache(dir)

	ctx := context.Background()
	_ = dc.Set(ctx, "cleanup-test", []byte("data"), time.Hour)

	before, _ := filepath.Glob(filepath.Join(dir, "*"))

	_ = dc.Delete(ctx, "cleanup-test")

	after, _ := filepath.Glob(filepath.Join(dir, "*"))
	assert.Less(t, len(after), len(before))
}
