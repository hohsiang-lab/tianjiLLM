package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestModelNew_Success(t *testing.T) {
	m := newMockStore()
	m.createProxyModelFn = func(_ context.Context, arg db.CreateProxyModelParams) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: arg.ModelID, ModelName: arg.ModelName, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/new", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4","tianji_params":{},"model_info":{}}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelNew(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestModelInfo_ByID(t *testing.T) {
	m := newMockStore()
	m.getProxyModelFn = func(_ context.Context, id string) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: id, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/model/info?model_id=m1", nil)
	h.ModelInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelInfo_List(t *testing.T) {
	m := newMockStore()
	m.listProxyModelsFn = func(_ context.Context) ([]db.ProxyModelTable, error) {
		return []db.ProxyModelTable{{ModelID: "m1", TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/model/info", nil)
	h.ModelInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateProxyModelFn = func(_ context.Context, arg db.UpdateProxyModelParams) (db.ProxyModelTable, error) {
		return db.ProxyModelTable{ModelID: arg.ModelID, TianjiParams: []byte("{}"), ModelInfo: []byte("{}")}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/update", strings.NewReader(`{"model_id":"m1","model_name":"gpt-4o","tianji_params":{},"model_info":{}}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestModelDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteProxyModelFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/model/delete", strings.NewReader(`{"model_id":"m1"}`))
	r.Header.Set("Content-Type", "application/json")
	h.ModelDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
