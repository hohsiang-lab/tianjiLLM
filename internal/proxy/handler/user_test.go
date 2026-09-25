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

func TestUserNew_Success(t *testing.T) {
	ms := newMockStore()
	ms.createUserFn = func(_ context.Context, arg db.CreateUserParams) (db.UserTable, error) {
		return db.UserTable{UserID: arg.UserID, UserRole: arg.UserRole}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"user_role": "admin"})
	req := httptest.NewRequest(http.MethodPost, "/user/new", bytes.NewReader(body))
	w := httptest.NewRecorder()

	h.UserNew(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp db.UserTable
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.UserRole != "admin" {
		t.Fatalf("expected role admin, got %s", resp.UserRole)
	}
}

func TestUserNew_DBNil(t *testing.T) {
	h := &Handlers{DB: nil}
	req := httptest.NewRequest(http.MethodPost, "/user/new", bytes.NewReader([]byte("{}")))
	w := httptest.NewRecorder()
	h.UserNew(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestUserList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listUsersFn = func(_ context.Context) ([]db.UserTable, error) {
		return []db.UserTable{{UserID: "u1"}, {UserID: "u2"}}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/user/list", nil)
	w := httptest.NewRecorder()
	h.UserList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string][]db.UserTable
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp["users"]) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp["users"]))
	}
}

func TestUserDelete_Success(t *testing.T) {
	deleted := []string{}
	ms := newMockStore()
	ms.deleteUserFn = func(_ context.Context, userID string) error {
		deleted = append(deleted, userID)
		return nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string][]string{"user_ids": {"u1", "u2"}})
	req := httptest.NewRequest(http.MethodPost, "/user/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.UserDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deletions, got %d", len(deleted))
	}
}
