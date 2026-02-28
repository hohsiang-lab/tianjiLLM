package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func TestSpendKeys_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByKeyFn = func(_ context.Context, arg db.GetSpendByKeyParams) ([]db.GetSpendByKeyRow, error) {
		return []db.GetSpendByKeyRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/keys?key=k1&since=2024-01-01T00:00:00Z", nil)
	w := httptest.NewRecorder()
	h.SpendKeys(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSpendKeys_MissingKey(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/keys", nil)
	w := httptest.NewRecorder()
	h.SpendKeys(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSpendUsers_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByUserFn = func(_ context.Context, arg db.GetSpendByUserParams) ([]db.GetSpendByUserRow, error) {
		return []db.GetSpendByUserRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/users?user=u1", nil)
	w := httptest.NewRecorder()
	h.SpendUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSpendUsers_MissingUser(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/users", nil)
	w := httptest.NewRecorder()
	h.SpendUsers(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSpendByTeams_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByTeamFn = func(_ context.Context, _ pgtype.Timestamptz) ([]db.GetSpendByTeamRow, error) {
		return []db.GetSpendByTeamRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/teams", nil)
	w := httptest.NewRecorder()
	h.SpendByTeams(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSpendByTags_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByTagFn = func(_ context.Context, _ pgtype.Timestamptz) ([]db.GetSpendByTagRow, error) {
		return []db.GetSpendByTagRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/tags", nil)
	w := httptest.NewRecorder()
	h.SpendByTags(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSpendByModels_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByModelFn = func(_ context.Context, _ pgtype.Timestamptz) ([]db.GetSpendByModelRow, error) {
		return []db.GetSpendByModelRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/models", nil)
	w := httptest.NewRecorder()
	h.SpendByModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSpendByEndUsers_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSpendByEndUserFn = func(_ context.Context, _ pgtype.Timestamptz) ([]db.GetSpendByEndUserRow, error) {
		return []db.GetSpendByEndUserRow{}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/spend/end_users", nil)
	w := httptest.NewRecorder()
	h.SpendByEndUsers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSpendByTeams_DBNil(t *testing.T) {
	h := &Handlers{DB: nil}
	req := httptest.NewRequest(http.MethodGet, "/spend/teams", nil)
	w := httptest.NewRecorder()
	h.SpendByTeams(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// Suppress unused import warning
var _ = json.Marshal
