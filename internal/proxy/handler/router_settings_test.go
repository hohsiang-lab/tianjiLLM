package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouterSettingsPatch_NotConfigured(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodPatch, "/settings", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	h.RouterSettingsPatch(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
