package handler

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
)

func TestCacheType_Memory(t *testing.T) {
	dc, err := cache.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := cacheType(dc)
	if got != "memory" {
		t.Fatalf("cacheType = %q, want memory", got)
	}
}
