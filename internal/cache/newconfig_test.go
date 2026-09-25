package cache

import (
	"context"
	"testing"
)

func TestNewFromConfigMemory(t *testing.T) {
	c, err := NewFromConfig(context.Background(), "memory", nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewFromConfigDisk(t *testing.T) {
	dir := t.TempDir()
	c, err := NewFromConfig(context.Background(), "disk", nil, "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewFromConfigDiskDefault(t *testing.T) {
	c, err := NewFromConfig(context.Background(), "disk", nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestNewFromConfigUnknown(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "foobar", nil, "", "")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestNewFromConfigS3(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "s3", nil, "", "")
	if err == nil {
		t.Fatal("expected error for s3")
	}
}

func TestNewFromConfigGCS(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "gcs", nil, "", "")
	if err == nil {
		t.Fatal("expected error for gcs")
	}
}

func TestNewFromConfigAzureBlob(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "azure_blob", nil, "", "")
	if err == nil {
		t.Fatal("expected error for azure_blob")
	}
}

func TestNewFromConfigRedisClusterNoAddrs(t *testing.T) {
	_, err := NewFromConfig(context.Background(), "redis_cluster", nil, "", "")
	if err == nil {
		t.Fatal("expected error for redis_cluster without addrs")
	}
}
