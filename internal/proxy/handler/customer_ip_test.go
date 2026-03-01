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
	"github.com/stretchr/testify/assert"
)

// ---- Customer / EndUser ----

func TestEndUserNew_Success(t *testing.T) {
	ms := newMockStore()
	ms.createEndUserFn = func(_ context.Context, arg db.CreateEndUserParams) (db.EndUserTable2, error) {
		return db.EndUserTable2{EndUserID: arg.EndUserID}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"end_user_id": "user-1"})
	req := httptest.NewRequest(http.MethodPost, "/end_user/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.EndUserNew(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestEndUserNew_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"end_user_id": "u"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.EndUserNew(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestEndUserGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getEndUserFn = func(_ context.Context, id string) (db.EndUserTable2, error) {
		return db.EndUserTable2{EndUserID: id}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("end_user_id", "user-1")
	req := httptest.NewRequest(http.MethodGet, "/end_user/user-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.EndUserInfo(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndUserUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateEndUserFn = func(_ context.Context, arg db.UpdateEndUserParams) (db.EndUserTable2, error) {
		return db.EndUserTable2{EndUserID: arg.ID}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("end_user_id", "user-1")
	body, _ := json.Marshal(map[string]string{"id": "user-1", "alias": "alice"})
	req := httptest.NewRequest(http.MethodPut, "/end_user/user-1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.EndUserUpdate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndUserDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteEndUserFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("end_user_id", "user-1")
	req := httptest.NewRequest(http.MethodDelete, "/end_user/user-1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.EndUserDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndUserBlock_Success(t *testing.T) {
	ms := newMockStore()
	ms.blockEndUserFn = func(_ context.Context, id string) (db.EndUserTable2, error) {
		return db.EndUserTable2{EndUserID: id}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"id": "user-1"})
	req := httptest.NewRequest(http.MethodPost, "/end_user/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.EndUserBlock(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndUserUnblock_Success(t *testing.T) {
	ms := newMockStore()
	ms.unblockEndUserFn = func(_ context.Context, id string) (db.EndUserTable2, error) {
		return db.EndUserTable2{EndUserID: id}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"id": "user-1"})
	req := httptest.NewRequest(http.MethodPost, "/end_user/unblock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.EndUserUnblock(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEndUserList_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/end_user/list", nil)
	w := httptest.NewRecorder()
	h.EndUserList(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

// ---- IP Whitelist ----

func TestIPAdd_Success(t *testing.T) {
	ms := newMockStore()
	ms.createIPFn = func(_ context.Context, arg db.CreateIPWhitelistParams) (db.IPWhitelistTable, error) {
		return db.IPWhitelistTable{IpAddress: arg.IpAddress}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"ip_address": "1.2.3.4"})
	req := httptest.NewRequest(http.MethodPost, "/ip/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IPAdd(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestIPAdd_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"ip_address": "1.2.3.4"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IPAdd(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestIPList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listIPFn = func(_ context.Context) ([]db.IPWhitelistTable, error) {
		return []db.IPWhitelistTable{{IpAddress: "1.2.3.4"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/ip/list", nil)
	w := httptest.NewRecorder()
	h.IPList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIPDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteIPFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"ip_address": "1.2.3.4"})
	req := httptest.NewRequest(http.MethodDelete, "/ip/1.2.3.4", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.IPDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
