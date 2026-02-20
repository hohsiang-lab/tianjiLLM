package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
)

// GCSCache stores cache entries as GCS objects with TTL via custom metadata.
type GCSCache struct {
	client *storage.Client
	bucket string
	prefix string
}

// NewGCSCache creates a GCS-backed cache.
func NewGCSCache(client *storage.Client, bucket, prefix string) *GCSCache {
	return &GCSCache{client: client, bucket: bucket, prefix: prefix}
}

func (c *GCSCache) key(k string) string { return c.prefix + k }

func (c *GCSCache) Get(ctx context.Context, key string) ([]byte, error) {
	obj := c.client.Bucket(c.bucket).Object(c.key(key))

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return nil, err
	}

	// Check TTL via custom metadata
	if exStr, ok := attrs.Metadata["expires_at"]; ok {
		exTime, parseErr := time.Parse(time.RFC3339, exStr)
		if parseErr == nil && time.Now().After(exTime) {
			_ = c.Delete(ctx, key)
			return nil, fmt.Errorf("key expired")
		}
	}

	r, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	return io.ReadAll(r)
}

func (c *GCSCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	obj := c.client.Bucket(c.bucket).Object(c.key(key))
	w := obj.NewWriter(ctx)
	w.ContentType = "application/octet-stream"
	w.Metadata = map[string]string{}
	if ttl > 0 {
		w.Metadata["expires_at"] = time.Now().Add(ttl).Format(time.RFC3339)
	}

	if _, err := io.Copy(w, bytes.NewReader(value)); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

func (c *GCSCache) Delete(ctx context.Context, key string) error {
	return c.client.Bucket(c.bucket).Object(c.key(key)).Delete(ctx)
}

func (c *GCSCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
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

var _ Cache = (*GCSCache)(nil)
