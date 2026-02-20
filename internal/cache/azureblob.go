package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

// AzureBlobCache stores cache entries as Azure Blob objects with TTL via metadata.
type AzureBlobCache struct {
	client    *azblob.Client
	container string
	prefix    string
}

// NewAzureBlobCache creates an Azure Blob-backed cache.
func NewAzureBlobCache(client *azblob.Client, container, prefix string) *AzureBlobCache {
	return &AzureBlobCache{client: client, container: container, prefix: prefix}
}

func (c *AzureBlobCache) key(k string) string { return c.prefix + k }

func (c *AzureBlobCache) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := c.client.DownloadStream(ctx, c.container, c.key(key), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check TTL via metadata
	if resp.Metadata != nil {
		if exStr, ok := resp.Metadata["expires_at"]; ok && exStr != nil {
			if exTime, err := time.Parse(time.RFC3339, *exStr); err == nil {
				if time.Now().After(exTime) {
					_ = c.Delete(ctx, key)
					return nil, fmt.Errorf("key expired")
				}
			}
		}
	}

	return io.ReadAll(resp.Body)
}

func (c *AzureBlobCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	metadata := map[string]*string{}
	if ttl > 0 {
		expiresAt := time.Now().Add(ttl).Format(time.RFC3339)
		metadata["expires_at"] = &expiresAt
	}

	_, err := c.client.UploadStream(ctx, c.container, c.key(key), bytes.NewReader(value), &azblob.UploadStreamOptions{
		Metadata: metadata,
	})
	return err
}

func (c *AzureBlobCache) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteBlob(ctx, c.container, c.key(key), nil)
	return err
}

func (c *AzureBlobCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	results := make([][]byte, len(keys))
	for i, k := range keys {
		val, err := c.Get(ctx, k)
		if err != nil {
			continue
		}
		results[i] = val
	}
	return results, nil
}

var _ Cache = (*AzureBlobCache)(nil)
