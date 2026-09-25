package cache

import (
	"context"
	"fmt"
)

// NewFromConfig creates a Cache from config parameters.
// Supported types: redis (standalone/cluster/sentinel auto-detected), memory, disk, dual.
func NewFromConfig(ctx context.Context, cacheType string, addrs []string, password string, diskDir string) (Cache, error) {
	switch cacheType {
	case "redis":
		client, err := NewRedisClient(ctx)
		if err != nil {
			return nil, fmt.Errorf("redis cache: %w", err)
		}
		mem := NewMemoryCache()
		return NewDualCache(mem, NewRedisCache(client)), nil

	case "redis_cluster":
		if len(addrs) == 0 {
			return nil, fmt.Errorf("redis_cluster requires addrs")
		}
		cluster := NewRedisCluster(addrs, password)
		return cluster, nil

	case "memory":
		return NewMemoryCache(), nil

	case "disk":
		if diskDir == "" {
			diskDir = "/tmp/tianji-cache"
		}
		return NewDiskCache(diskDir)

	case "s3":
		return nil, fmt.Errorf("s3 cache requires SDK client — use cache.NewS3Cache() directly")

	case "gcs":
		return nil, fmt.Errorf("gcs cache requires SDK client — use cache.NewGCSCache() directly")

	case "azure_blob":
		return nil, fmt.Errorf("azure_blob cache requires SDK client — use cache.NewAzureBlobCache() directly")

	default:
		return nil, fmt.Errorf("unknown cache type: %s", cacheType)
	}
}
