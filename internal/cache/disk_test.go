package cache

import (
	"context"
	"testing"
	"time"
)

func TestDiskCacheGetSetDelete(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Get non-existent
	val, err := dc.Get(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil, got %v", val)
	}

	// Set + Get
	err = dc.Set(ctx, "k1", []byte("hello"), 1*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	val, err = dc.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "hello" {
		t.Fatalf("got %q, want hello", val)
	}

	// Delete
	err = dc.Delete(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	val, err = dc.Get(ctx, "k1")
	if err != nil && val != nil {
		t.Fatalf("expected nil after delete, got %v, err=%v", val, err)
	}
}

func TestDiskCacheMGet(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_ = dc.Set(ctx, "a", []byte("1"), time.Hour)
	_ = dc.Set(ctx, "b", []byte("2"), time.Hour)

	vals, err := dc.MGet(ctx, "a", "b", "c")
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 {
		t.Fatalf("got %d results, want 3", len(vals))
	}
	if string(vals[0]) != "1" {
		t.Fatalf("vals[0]=%q", vals[0])
	}
	if string(vals[1]) != "2" {
		t.Fatalf("vals[1]=%q", vals[1])
	}
	if vals[2] != nil {
		t.Fatalf("vals[2] should be nil")
	}
}

func TestDiskCacheTTLExpiry(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// Set with very short TTL
	err = dc.Set(ctx, "exp", []byte("data"), 1*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	val, err := dc.Get(ctx, "exp")
	if err != nil {
		t.Fatal(err)
	}
	if val != nil {
		t.Fatalf("expected nil after TTL expiry, got %q", val)
	}
}

func TestDiskCacheSetNoTTL(t *testing.T) {
	dir := t.TempDir()
	dc, err := NewDiskCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	err = dc.Set(ctx, "notl", []byte("data"), 0)
	if err != nil {
		t.Fatal(err)
	}
	val, err := dc.Get(ctx, "notl")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "data" {
		t.Fatalf("got %q", val)
	}
}
