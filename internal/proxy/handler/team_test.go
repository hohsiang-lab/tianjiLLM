package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
)

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
