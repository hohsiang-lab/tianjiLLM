package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func mockHandlers(m *mockStore) *Handlers {
	return &Handlers{Config: &config.ProxyConfig{}, DB: m, Callbacks: callback.NewRegistry()}
}

func TestBudgetNew_Success(t *testing.T) {
	m := newMockStore()
	m.createBudgetFn = func(_ context.Context, arg db.CreateBudgetParams) (db.BudgetTable, error) {
		return db.BudgetTable{BudgetID: arg.BudgetID}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/budget/new", strings.NewReader(`{"max_budget":100}`))
	r.Header.Set("Content-Type", "application/json")
	h.BudgetNew(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBudgetInfo_Success(t *testing.T) {
	m := newMockStore()
	m.getBudgetFn = func(_ context.Context, id string) (db.BudgetTable, error) {
		return db.BudgetTable{BudgetID: id}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/budget/info?budget_id=b-1", nil)
	h.BudgetInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBudgetInfo_MissingID(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/budget/info", nil)
	h.BudgetInfo(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetList_Success(t *testing.T) {
	m := newMockStore()
	m.listBudgetsFn = func(_ context.Context) ([]db.BudgetTable, error) {
		return []db.BudgetTable{{BudgetID: "b-1"}}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/budget/list", nil)
	h.BudgetList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBudgetUpdate_Success(t *testing.T) {
	m := newMockStore()
	m.updateBudgetFn = func(_ context.Context, arg db.UpdateBudgetParams) (db.BudgetTable, error) {
		return db.BudgetTable{BudgetID: arg.BudgetID}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/budget/update", strings.NewReader(`{"budget_id":"b-1","max_budget":200}`))
	r.Header.Set("Content-Type", "application/json")
	h.BudgetUpdate(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBudgetUpdate_MissingID(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/budget/update", strings.NewReader(`{"max_budget":200}`))
	r.Header.Set("Content-Type", "application/json")
	h.BudgetUpdate(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteBudgetFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/budget/delete", strings.NewReader(`{"budget_id":"b-1"}`))
	r.Header.Set("Content-Type", "application/json")
	h.BudgetDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBudgetDelete_MissingID(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/budget/delete", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	h.BudgetDelete(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestBudgetSettings(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/budget/settings", nil)
	h.BudgetSettings(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
