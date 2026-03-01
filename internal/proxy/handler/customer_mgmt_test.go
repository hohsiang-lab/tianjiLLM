package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEndUserHandlers_NoDB(t *testing.T) {
	h := newTestHandlers()

	fns := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"EndUserNew", h.EndUserNew},
		{"EndUserInfo", h.EndUserInfo},
		{"EndUserList", h.EndUserList},
		{"EndUserUpdate", h.EndUserUpdate},
		{"EndUserDelete", h.EndUserDelete},
		{"EndUserBlock", h.EndUserBlock},
		{"EndUserUnblock", h.EndUserUnblock},
	}
	for _, f := range fns {
		t.Run(f.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			w := httptest.NewRecorder()
			f.fn(w, req)
			if w.Code == http.StatusOK {
				t.Fatalf("%s: expected non-200 with nil DB, got 200", f.name)
			}
		})
	}
}
