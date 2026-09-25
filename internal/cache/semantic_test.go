package cache

import (
	"testing"
)

func TestFloat32ToBytes(t *testing.T) {
	vec := []float32{1.0, 2.0, 3.0}
	b := float32ToBytes(vec)
	if len(b) != 12 {
		t.Fatalf("expected 12 bytes, got %d", len(b))
	}
}

func TestNewSemanticCacheDefaults(t *testing.T) {
	sc := NewSemanticCache(SemanticCacheConfig{})
	if sc.indexName != "idx:semantic_cache" {
		t.Fatalf("got index %q", sc.indexName)
	}
	if sc.prefix != "cache:semantic:" {
		t.Fatalf("got prefix %q", sc.prefix)
	}
	if sc.threshold != 0.1 {
		t.Fatalf("got threshold %f", sc.threshold)
	}
}

func TestNewSemanticCacheCustom(t *testing.T) {
	sc := NewSemanticCache(SemanticCacheConfig{
		IndexName: "myidx",
		Prefix:    "mypfx:",
		Threshold: 0.5,
	})
	if sc.indexName != "myidx" {
		t.Fatalf("got index %q", sc.indexName)
	}
	if sc.prefix != "mypfx:" {
		t.Fatalf("got prefix %q", sc.prefix)
	}
	if sc.threshold != 0.5 {
		t.Fatalf("got threshold %f", sc.threshold)
	}
}
