package handler

import (
	"context"
	"encoding/json"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"testing"
	"time"
)

func TestNativeFileOwnership(t *testing.T) {
	h := &Handlers{Cache: cache.NewMemoryCache()}
	owner := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "owner")
	other := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "other")
	record := nativeFileBinding{Owner: nativeFileOwner(owner), CredentialID: "one", AccountID: "account", ExpiresAt: time.Now().Add(time.Hour)}
	if err := h.saveNativeFileBinding(owner, "file-owned", record); err != nil {
		t.Fatal(err)
	}
	if _, err := h.loadNativeFileBinding(owner, "file-owned"); err != nil {
		t.Fatal(err)
	}
	record.ExpiresAt = time.Now().Add(-time.Second)
	raw, _ := json.Marshal(record)
	if err := h.Cache.Set(owner, nativeFileKey("file-expired"), raw, time.Hour); err != nil {
		t.Fatal(err)
	}
	if _, err := h.loadNativeFileBinding(owner, "file-expired"); err == nil {
		t.Fatal("expired cache record accepted")
	}
	record.ExpiresAt = time.Now().Add(time.Hour)
	for _, c := range []struct {
		ctx context.Context
		id  string
	}{{other, "file-owned"}, {owner, "file-unknown"}, {context.Background(), "file-owned"}} {
		if _, err := h.loadNativeFileBinding(c.ctx, c.id); err == nil {
			t.Fatal("ownership gate accepted invalid reference")
		}
	}
	record.ExpiresAt = time.Now().Add(-time.Second)
	if err := h.saveNativeFileBinding(owner, "file-expired", record); err == nil {
		t.Fatal("expired binding saved")
	}
}
