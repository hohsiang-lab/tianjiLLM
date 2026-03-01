package cache

import (
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test constructors and key() helper for cloud caches (no actual connection needed)

func TestNewAzureBlobCache(t *testing.T) {
	c := NewAzureBlobCache(nil, "my-container", "prefix/")
	require.NotNil(t, c)
	assert.Equal(t, "my-container", c.container)
	assert.Equal(t, "prefix/", c.prefix)
}

func TestAzureBlobCache_Key(t *testing.T) {
	c := &AzureBlobCache{prefix: "test/"}
	assert.Equal(t, "test/mykey", c.key("mykey"))
}

func TestNewGCSCache(t *testing.T) {
	c := NewGCSCache(nil, "my-bucket", "gcs/")
	require.NotNil(t, c)
	assert.Equal(t, "my-bucket", c.bucket)
	assert.Equal(t, "gcs/", c.prefix)
}

func TestGCSCache_Key(t *testing.T) {
	c := &GCSCache{prefix: "gcs/"}
	assert.Equal(t, "gcs/foo", c.key("foo"))
}

func TestNewS3Cache(t *testing.T) {
	c := NewS3Cache(nil, "my-bucket", "s3/")
	require.NotNil(t, c)
	assert.Equal(t, "my-bucket", c.bucket)
	assert.Equal(t, "s3/", c.prefix)
}

func TestS3Cache_Key(t *testing.T) {
	c := &S3Cache{prefix: "s3/"}
	assert.Equal(t, "s3/bar", c.key("bar"))
}

func TestNewRedisCache(t *testing.T) {
	// Use a mock/nil client - just test constructor
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	c := NewRedisCache(client)
	require.NotNil(t, c)
	assert.NotNil(t, c.Client())
}

func TestInterfaceCompliance_AzureBlob(t *testing.T) {
	var _ Cache = (*AzureBlobCache)(nil)
}

func TestInterfaceCompliance_GCS(t *testing.T) {
	var _ Cache = (*GCSCache)(nil)
}

func TestInterfaceCompliance_S3(t *testing.T) {
	var _ Cache = (*S3Cache)(nil)
}

func TestInterfaceCompliance_Redis(t *testing.T) {
	var _ Cache = (*RedisCache)(nil)
}
