package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_Memory(t *testing.T) {
	c, err := NewFromConfig(context.Background(), "memory", nil, "", "")
	require.NoError(t, err)
	assert.IsType(t, &MemoryCache{}, c)
}

func TestNewFromConfig_Disk(t *testing.T) {
	dir := t.TempDir()
	c, err := NewFromConfig(context.Background(), "disk", nil, "", dir)
	require.NoError(t, err)
	assert.IsType(t, &DiskCache{}, c)
}

func TestNewFromConfig_DiskDefaultDir(t *testing.T) {
	c, err := NewFromConfig(context.Background(), "disk", nil, "", "")
	require.NoError(t, err)
	assert.IsType(t, &DiskCache{}, c)
}

func TestNewFromConfig_UnknownType(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "unknown", nil, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown cache type")
}

func TestNewFromConfig_S3(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "s3", nil, "", "")
	assert.Error(t, err)
}

func TestNewFromConfig_GCS(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "gcs", nil, "", "")
	assert.Error(t, err)
}

func TestNewFromConfig_AzureBlob(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "azure_blob", nil, "", "")
	assert.Error(t, err)
}

func TestNewFromConfig_RedisClusterNoAddrs(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "redis_cluster", nil, "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis_cluster requires addrs")
}
