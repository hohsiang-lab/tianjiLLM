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

func TestGuardrailCreate_Success(t *testing.T) {
	m := newMockStore()
	m.createGuardrailConfigFn = func(_ context.Context, arg db.CreateGuardrailConfigParams) (db.GuardrailConfigTable, error) {
		return db.GuardrailConfigTable{GuardrailName: arg.GuardrailName}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/guardrails", strings.NewReader(`{"guardrail_name":"test","guardrail_type":"regex","config":{},"failure_policy":"block"}`))
	r.Header.Set("Content-Type", "application/json")
	h.GuardrailCreate(w, r)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestGuardrailGet_Success(t *testing.T) {
	m := newMockStore()
	m.getGuardrailConfigFn = func(_ context.Context, id string) (db.GuardrailConfigTable, error) {
		return db.GuardrailConfigTable{ID: id}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/guardrails/g1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "g1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.GuardrailGet(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGuardrailList_Success(t *testing.T) {
	m := newMockStore()
	m.listGuardrailConfigsFn = func(_ context.Context) ([]db.GuardrailConfigTable, error) {
		return []db.GuardrailConfigTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/guardrails/list", nil)
	h.GuardrailList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGuardrailUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateGuardrailConfigFn = func(_ context.Context, arg db.UpdateGuardrailConfigParams) (db.GuardrailConfigTable, error) {
		return db.GuardrailConfigTable{ID: arg.ID}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/guardrails/g1", strings.NewReader(`{"guardrail_name":"up","guardrail_type":"regex","config":{},"failure_policy":"block"}`))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "g1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.GuardrailUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGuardrailDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteGuardrailConfigFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/guardrails/g1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "g1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.GuardrailDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
