package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestHandlersNoDB tests that all DB-dependent handlers return 503 when DB is nil.
func TestHandlersNoDB(t *testing.T) {
	h := newTestHandlers() // DB is nil

	// key endpoints
	keyEndpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"KeyGenerateHandler", h.KeyGenerateHandler},
		{"KeyInfo", h.KeyInfo},
		{"KeyList", h.KeyList},
		{"KeyDelete", h.KeyDelete},
		{"KeyBlock", h.KeyBlock},
		{"KeyUnblock", h.KeyUnblock},
		{"KeyUpdate", h.KeyUpdate},
	}

	for _, ep := range keyEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/key", strings.NewReader("{}"))
			r.Header.Set("Content-Type", "application/json")
			ep.fn(w, r)
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}

	// spend_global endpoints
	spendGlobalEndpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"GlobalSpend", h.GlobalSpend},
		{"GlobalSpendByKeys", h.GlobalSpendByKeys},
		{"GlobalSpendByModels", h.GlobalSpendByModels},
		{"GlobalSpendByTeams", h.GlobalSpendByTeams},
		{"GlobalSpendByTags", h.GlobalSpendByTags},
		{"GlobalSpendByProvider", h.GlobalSpendByProvider},
		{"GlobalActivity", h.GlobalActivity},
		{"GlobalActivityByModel", h.GlobalActivityByModel},
		{"GlobalSpendReport", h.GlobalSpendReport},
		{"GlobalSpendReset", h.GlobalSpendReset},
		{"CacheHitStats", h.CacheHitStats},
		{"SpendLogs", h.SpendLogs},
	}

	for _, ep := range spendGlobalEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/spend", nil)
			ep.fn(w, r)
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}

	// spend endpoints
	spendEndpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"SpendKeys", h.SpendKeys},
		{"SpendUsers", h.SpendUsers},
		{"SpendByTeams", h.SpendByTeams},
		{"SpendByTags", h.SpendByTags},
		{"SpendByModels", h.SpendByModels},
		{"SpendByEndUsers", h.SpendByEndUsers},
	}

	for _, ep := range spendEndpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("GET", "/spend", nil)
			ep.fn(w, r)
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

// TestVectorStoreEndpoints_NotConfigured tests vectorstore handlers return 501.
func TestVectorStoreEndpoints_NotConfigured(t *testing.T) {
	h := newTestHandlers()

	endpoints := []struct {
		name string
		fn   func(http.ResponseWriter, *http.Request)
	}{
		{"VectorStoreFilesCreate", h.VectorStoreFilesCreate},
		{"VectorStoreFilesList", h.VectorStoreFilesList},
		{"VectorStoreFilesGet", h.VectorStoreFilesGet},
		{"VectorStoreFilesDelete", h.VectorStoreFilesDelete},
		{"VectorStoreSearch", h.VectorStoreSearch},
		{"VectorStoreCreate", h.VectorStoreCreate},
		{"VectorStoreList", h.VectorStoreList},
		{"VectorStoreGet", h.VectorStoreGet},
		{"VectorStoreDelete", h.VectorStoreDelete},
	}

	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", "/v1/vector_stores", nil)
			ep.fn(w, r)
			// Should return 501 (not implemented) since no vector store configured
			assert.Contains(t, []int{http.StatusNotImplemented, http.StatusServiceUnavailable}, w.Code)
		})
	}
}

func TestParseEnd(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/spend?end_date=2025-01-01", nil)
	end := h.parseEnd(r)
	assert.False(t, end.IsZero())
}

func TestParseSince(t *testing.T) {
	h := newTestHandlers()
	r := httptest.NewRequest("GET", "/spend?start_date=2025-01-01", nil)
	since := h.parseSince(r)
	assert.False(t, since.IsZero())
}
