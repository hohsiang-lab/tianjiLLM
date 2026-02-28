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

func TestPolicyCreate_Success(t *testing.T) {
	m := newMockStore()
	m.createPolicyFn = func(_ context.Context, arg db.CreatePolicyParams) (db.PolicyTable, error) {
		return db.PolicyTable{ID: "p1", Name: arg.Name}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/policy", strings.NewReader(`{"name":"test","conditions":{}}`))
	r.Header.Set("Content-Type", "application/json")
	h.PolicyCreate(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPolicyGet_Success(t *testing.T) {
	m := newMockStore()
	m.getPolicyFn = func(_ context.Context, id string) (db.PolicyTable, error) {
		return db.PolicyTable{ID: id}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/policy/p1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.PolicyGet(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyList_Success(t *testing.T) {
	m := newMockStore()
	m.listPoliciesFn = func(_ context.Context) ([]db.PolicyTable, error) {
		return []db.PolicyTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/policy/list", nil)
	h.PolicyList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updatePolicyFn = func(_ context.Context, arg db.UpdatePolicyParams) (db.PolicyTable, error) {
		return db.PolicyTable{ID: arg.ID}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/policy/p1", strings.NewReader(`{"name":"updated"}`))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.PolicyUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deletePolicyFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/policy/p1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "p1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.PolicyDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyAttachmentCreate_Success(t *testing.T) {
	m := newMockStore()
	m.createPolicyAttachmentFn = func(_ context.Context, arg db.CreatePolicyAttachmentParams) (db.PolicyAttachmentTable, error) {
		return db.PolicyAttachmentTable{ID: "pa1"}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/policy_attachment", strings.NewReader(`{"policy_name":"test","target_type":"key","target_id":"k1"}`))
	r.Header.Set("Content-Type", "application/json")
	h.PolicyAttachmentCreate(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestPolicyAttachmentGet_Success(t *testing.T) {
	m := newMockStore()
	m.getPolicyAttachmentFn = func(_ context.Context, id string) (db.PolicyAttachmentTable, error) {
		return db.PolicyAttachmentTable{ID: id}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/policy_attachment/pa1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "pa1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.PolicyAttachmentGet(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyAttachmentList_Success(t *testing.T) {
	m := newMockStore()
	m.listPolicyAttachmentsFn = func(_ context.Context) ([]db.PolicyAttachmentTable, error) {
		return []db.PolicyAttachmentTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/policy_attachment/list", nil)
	h.PolicyAttachmentList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPolicyAttachmentDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deletePolicyAttachmentFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/policy_attachment/pa1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "pa1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.PolicyAttachmentDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
