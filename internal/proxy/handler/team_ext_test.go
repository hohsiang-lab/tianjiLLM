package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

func TestTeamBlock_Success(t *testing.T) {
	ms := newMockStore()
	ms.blockTeamFn = func(_ context.Context, teamID string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.TeamBlock(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamBlock_MissingID(t *testing.T) {
	ms := newMockStore()
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.TeamBlock(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTeamUnblock_Success(t *testing.T) {
	ms := newMockStore()
	ms.unblockTeamFn = func(_ context.Context, teamID string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.TeamUnblock(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamModelAdd_Success(t *testing.T) {
	ms := newMockStore()
	ms.addTeamModelFn = func(_ context.Context, arg db.AddTeamModelParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1", "model": "gpt-4o"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.TeamModelAdd(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamModelRemove_Success(t *testing.T) {
	ms := newMockStore()
	ms.removeTeamModelFn = func(_ context.Context, arg db.RemoveTeamModelParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1", "model": "gpt-4o"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	h.TeamModelRemove(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
