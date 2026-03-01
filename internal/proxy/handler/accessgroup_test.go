package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func TestAccessGroupNew_Success(t *testing.T) {
	ms := newMockStore()
	ms.createAccessGroupFn = func(_ context.Context, arg db.CreateAccessGroupParams) (db.ModelAccessGroup, error) {
		return db.ModelAccessGroup{GroupID: "ag-1", GroupAlias: arg.GroupAlias}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"group_alias": "my-group"})
	req := httptest.NewRequest(http.MethodPost, "/access_group/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AccessGroupNew(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAccessGroupNew_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"group_alias": "g"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AccessGroupNew(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestAccessGroupInfo_Success(t *testing.T) {
	ms := newMockStore()
	ms.getAccessGroupFn = func(_ context.Context, groupID string) (db.ModelAccessGroup, error) {
		alias := "test"
		return db.ModelAccessGroup{GroupID: groupID, GroupAlias: &alias}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("group_id", "ag-1")
	req := httptest.NewRequest(http.MethodGet, "/access_group/info/ag-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AccessGroupInfo(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAccessGroupUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateAccessGroupFn = func(_ context.Context, arg db.UpdateAccessGroupParams) error { return nil }
	ms.getAccessGroupFn = func(_ context.Context, groupID string) (db.ModelAccessGroup, error) {
		return db.ModelAccessGroup{GroupID: groupID}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"group_id": "ag-1", "group_alias": "updated"})
	req := httptest.NewRequest(http.MethodPost, "/access_group/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.AccessGroupUpdate(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAccessGroupDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteAccessGroupFn = func(_ context.Context, groupID string) error { return nil }
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("group_id", "ag-1")
	req := httptest.NewRequest(http.MethodDelete, "/access_group/delete/ag-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.AccessGroupDelete(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
