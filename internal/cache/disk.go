package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DiskCache is a file-based cache for local development.
// Key → filename (SHA256 hash), value → file contents.
// TTL via file mtime check.
type DiskCache struct {
	dir string
}

// NewDiskCache creates a disk-based cache in the given directory.
func NewDiskCache(dir string) (*DiskCache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("disk cache: mkdir: %w", err)
	}
	return &DiskCache{dir: dir}, nil
}

func (d *DiskCache) keyPath(key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(d.dir, hex.EncodeToString(h[:]))
}

func (d *DiskCache) Get(ctx context.Context, key string) ([]byte, error) {
	path := d.keyPath(key)
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Check TTL via mtime — companion .ttl file stores expiry
	ttlPath := path + ".ttl"
	ttlData, err := os.ReadFile(ttlPath)
	if err == nil {
		expiry, err := time.Parse(time.RFC3339Nano, string(ttlData))
		if err == nil && time.Now().After(expiry) {
			os.Remove(path)
			os.Remove(ttlPath)
			return nil, nil
		}
	}

	_ = info
	return os.ReadFile(path)
}

func (d *DiskCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	path := d.keyPath(key)
	if err := os.WriteFile(path, value, 0o644); err != nil {
		return err
	}
	if ttl > 0 {
		expiry := time.Now().Add(ttl).Format(time.RFC3339Nano)
		return os.WriteFile(path+".ttl", []byte(expiry), 0o644)
	}
	return nil
}

func (d *DiskCache) Delete(ctx context.Context, key string) error {
	path := d.keyPath(key)
	os.Remove(path + ".ttl")
	return os.Remove(path)
}

func (d *DiskCache) MGet(ctx context.Context, keys ...string) ([][]byte, error) {
	result := make([][]byte, len(keys))
	for i, key := range keys {
		val, err := d.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		result[i] = val
	}
	return result, nil
}
