package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockCache is a simple in-memory cache.Cache for testing.
type mockCache struct {
	data map[string][]byte
	fail bool // if true, all ops return an error
}

var errMock = errors.New("mock error")

func newMockCache() *mockCache { return &mockCache{data: make(map[string][]byte)} }

func (m *mockCache) Get(_ context.Context, key string) ([]byte, error) {
	if m.fail {
		return nil, errMock
	}
	return m.data[key], nil
}
func (m *mockCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	if m.fail {
		return errMock
	}
	m.data[key] = value
	return nil
}
func (m *mockCache) Delete(_ context.Context, key string) error {
	if m.fail {
		return errMock
	}
	delete(m.data, key)
	return nil
}
func (m *mockCache) MGet(_ context.Context, keys ...string) ([][]byte, error) {
	if m.fail {
		return nil, errMock
	}
	result := make([][]byte, len(keys))
	for i, k := range keys {
		result[i] = m.data[k]
	}
	return result, nil
}

func TestCachePing_Success(t *testing.T) {
	h := &Handlers{Cache: newMockCache()}
	req := httptest.NewRequest(http.MethodGet, "/cache/ping", nil)
	w := httptest.NewRecorder()
	h.CachePing(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	assert.Equal(t, "healthy", resp["status"])
}

func TestCachePing_Fail(t *testing.T) {
	mc := newMockCache()
	mc.fail = true
	h := &Handlers{Cache: mc}
	req := httptest.NewRequest(http.MethodGet, "/cache/ping", nil)
	w := httptest.NewRecorder()
	h.CachePing(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCachePing_NilCache(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodGet, "/cache/ping", nil)
	w := httptest.NewRecorder()
	h.CachePing(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCacheDelete_Success(t *testing.T) {
	mc := newMockCache()
	mc.data["k1"] = []byte("v1")
	mc.data["k2"] = []byte("v2")
	h := &Handlers{Cache: mc}

	body, _ := json.Marshal(map[string][]string{"keys": {"k1", "k2"}})
	req := httptest.NewRequest(http.MethodPost, "/cache/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CacheDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, mc.data["k1"])
}

func TestCacheDelete_NilCache(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/cache/delete", bytes.NewReader([]byte(`{"keys":["k1"]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CacheDelete(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestCacheFlushAll_NonRedis(t *testing.T) {
	// mockCache is not *cache.RedisCache, so FlushDB branch is skipped → 200 OK
	h := &Handlers{Cache: newMockCache()}
	req := httptest.NewRequest(http.MethodPost, "/cache/flushall", nil)
	w := httptest.NewRecorder()
	h.CacheFlushAll(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCacheFlushAll_NilCache(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest(http.MethodPost, "/cache/flushall", nil)
	w := httptest.NewRecorder()
	h.CacheFlushAll(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
