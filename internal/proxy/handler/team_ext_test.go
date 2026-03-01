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

func strPtr(s string) *string { return &s }

func TestTeamInfo_Success(t *testing.T) {
	ms := newMockStore()
	ms.getTeamFn = func(_ context.Context, teamID string) (db.TeamTable, error) {
		return db.TeamTable{TeamID: teamID, TeamAlias: strPtr("test-team")}, nil
	}
	h := &Handlers{DB: ms}

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("team_id", "t-001")
	req := httptest.NewRequest(http.MethodGet, "/team/info/t-001", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.TeamInfo(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamInfo_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.TeamInfo(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestTeamInfo_MissingID(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/team/info/", nil)
	w := httptest.NewRecorder()
	h.TeamInfo(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamBlock_Success(t *testing.T) {
	ms := newMockStore()
	ms.blockTeamFn = func(_ context.Context, teamID string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t-001"})
	req := httptest.NewRequest(http.MethodPost, "/team/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamBlock(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamBlock_MissingID(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/team/block", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamBlock(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamUnblock_Success(t *testing.T) {
	ms := newMockStore()
	ms.unblockTeamFn = func(_ context.Context, teamID string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t-001"})
	req := httptest.NewRequest(http.MethodPost, "/team/unblock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamUnblock(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTeamModelAdd_Success(t *testing.T) {
	ms := newMockStore()
	ms.addTeamModelFn = func(_ context.Context, arg db.AddTeamModelParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t-001", "model": "gpt-4o"})
	req := httptest.NewRequest(http.MethodPost, "/team/model/add", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamModelAdd(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamModelRemove_Success(t *testing.T) {
	ms := newMockStore()
	ms.removeTeamModelFn = func(_ context.Context, arg db.RemoveTeamModelParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t-001", "model": "gpt-4o"})
	req := httptest.NewRequest(http.MethodPost, "/team/model/remove", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamModelRemove(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamMemberUpdate_MissingFields(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/team/member/update", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.TeamMemberUpdate(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamAvailable_NoDB(t *testing.T) {
	h := newTestHandlers()
	req := httptest.NewRequest(http.MethodGet, "/team/available", nil)
	w := httptest.NewRecorder()
	h.TeamAvailable(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
