package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestTagNew_Success(t *testing.T) {
	m := newMockStore()
	m.createTagFn = func(_ context.Context, arg db.CreateTagParams) (db.TagTable, error) {
		return db.TagTable{Name: arg.Name}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/tag/new", strings.NewReader(`{"name":"test-tag"}`))
	r.Header.Set("Content-Type", "application/json")
	h.TagNew(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestTagInfo_Success(t *testing.T) {
	m := newMockStore()
	m.getTagFn = func(_ context.Context, id string) (db.TagTable, error) {
		return db.TagTable{ID: id, Name: "mytag"}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/tag/info/t1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "t1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.TagInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTagList_Success(t *testing.T) {
	m := newMockStore()
	m.listTagsFn = func(_ context.Context) ([]db.TagTable, error) {
		return []db.TagTable{{Name: "a"}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/tag/list", nil)
	h.TagList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTagUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateTagFn = func(_ context.Context, arg db.UpdateTagParams) (db.TagTable, error) {
		return db.TagTable{ID: arg.ID, Name: arg.Name}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/tag/update", strings.NewReader(`{"id":"t1","name":"updated"}`))
	r.Header.Set("Content-Type", "application/json")
	h.TagUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTagDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteTagFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/tag/delete", strings.NewReader(`{"id":"t1"}`))
	r.Header.Set("Content-Type", "application/json")
	h.TagDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
