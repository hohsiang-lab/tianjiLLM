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

func TestTeamNew_Success(t *testing.T) {
	ms := newMockStore()
	ms.createTeamFn = func(_ context.Context, arg db.CreateTeamParams) (db.TeamTable, error) {
		return db.TeamTable{TeamID: arg.TeamID, TeamAlias: arg.TeamAlias}, nil
	}
	h := &Handlers{DB: ms}

	alias := "my-team"
	body, _ := json.Marshal(map[string]string{"team_alias": alias})
	req := httptest.NewRequest(http.MethodPost, "/team/new", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.TeamNew(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listTeamsFn = func(_ context.Context) ([]db.TeamTable, error) {
		return []db.TeamTable{{TeamID: "t1"}}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/team/list", nil)
	w := httptest.NewRecorder()
	h.TeamList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTeamDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteTeamFn = func(_ context.Context, teamID string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string][]string{"team_ids": {"t1"}})
	req := httptest.NewRequest(http.MethodPost, "/team/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.TeamDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateTeamFn = func(_ context.Context, arg db.UpdateTeamParams) (db.TeamTable, error) {
		return db.TeamTable{TeamID: arg.TeamID}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1"})
	req := httptest.NewRequest(http.MethodPost, "/team/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.TeamUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamMemberAdd_Success(t *testing.T) {
	ms := newMockStore()
	ms.addTeamMemberFn = func(_ context.Context, arg db.AddTeamMemberParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1", "user_id": "u1"})
	req := httptest.NewRequest(http.MethodPost, "/team/member/add", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.TeamMemberAdd(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTeamMemberDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.removeTeamMemberFn = func(_ context.Context, arg db.RemoveTeamMemberParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"team_id": "t1", "user_id": "u1"})
	req := httptest.NewRequest(http.MethodPost, "/team/member/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.TeamMemberDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
