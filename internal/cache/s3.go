package cache

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Cache stores cache entries as S3 objects with TTL via Expires metadata.
type S3Cache struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Cache creates an S3-backed cache.
func NewS3Cache(client *s3.Client, bucket, prefix string) *S3Cache {
	return &S3Cache{client: client, bucket: bucket, prefix: prefix}
}

func (c *S3Cache) key(k string) string { return c.prefix + k }

func (c *S3Cache) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(c.key(key)),
	})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()

	// Check TTL via Expires metadata
	if out.Metadata != nil {
		if exStr, ok := out.Metadata["expires_at"]; ok {
			if exTime, err := time.Parse(time.RFC3339, exStr); err == nil {
				if time.Now().After(exTime) {
					_ = c.Delete(ctx, key)
					return nil, fmt.Errorf("key expired")
				}
			}
		}
	}

	return io.ReadAll(out.Body)
}

func (c *S3Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	metadata := map[string]string{}
	if ttl > 0 {
		metadata["expires_at"] = time.Now().Add(ttl).Format(time.RFC3339)
		metadata["ttl_seconds"] = strconv.Itoa(int(ttl.Seconds()))
	}

	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(c.bucket),
		Key:         aws.String(c.key(key)),
		Body:        bytes.NewReader(value),
		ContentType: aws.String("application/octet-stream"),
		Metadata:    metadata,
	})
	return err
}

func (c *S3Cache) Delete(ctx context.Context, key string) error {
	_, err := c.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.bucket),
		Key:    aws.String(c.key(key)),
	})
	return err
}

func (c *S3Cache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
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

// Ping checks S3 bucket accessibility.
func (c *S3Cache) Ping(ctx context.Context) error {
	_, err := c.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(c.bucket),
		MaxKeys: aws.Int32(1),
	})
	return err
}

// Ensure S3Cache implements Cache
var _ Cache = (*S3Cache)(nil)

// Suppress unused import for types package (used in potential future extensions)
var _ = types.ObjectStorageClassStandard
