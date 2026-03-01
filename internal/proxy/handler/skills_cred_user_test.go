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

// ---- Skills ----

func TestSkillCreate_Success(t *testing.T) {
	ms := newMockStore()
	ms.createSkillFn = func(_ context.Context, arg db.CreateSkillParams) (db.SkillsTable, error) {
		return db.SkillsTable{SkillID: "s1", DisplayTitle: arg.DisplayTitle}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"display_title": "my-skill"})
	req := httptest.NewRequest(http.MethodPost, "/skill/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SkillCreate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSkillCreate_NoDB(t *testing.T) {
	h := newTestHandlers()
	body, _ := json.Marshal(map[string]string{"display_title": "s"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.SkillCreate(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestSkillGet_Success(t *testing.T) {
	ms := newMockStore()
	ms.getSkillFn = func(_ context.Context, skillID string) (db.SkillsTable, error) {
		return db.SkillsTable{SkillID: skillID, DisplayTitle: "bot"}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("skill_id", "s1")
	req := httptest.NewRequest(http.MethodGet, "/skill/s1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.SkillGet(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSkillList_Success(t *testing.T) {
	ms := newMockStore()
	ms.listSkillsFn = func(_ context.Context, _ db.ListSkillsParams) ([]db.SkillsTable, error) {
		return []db.SkillsTable{{SkillID: "s1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/skill/list", nil)
	w := httptest.NewRecorder()
	h.SkillList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSkillDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteSkillFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("skill_id", "s1")
	req := httptest.NewRequest(http.MethodDelete, "/skill/s1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.SkillDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- Credentials ----

func TestCredentialCreateV2_Success(t *testing.T) {
	ms := newMockStore()
	ms.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: "c1", CredentialName: arg.CredentialName}, nil
	}
	h := &Handlers{DB: ms}
	body, _ := json.Marshal(map[string]string{"credential_name": "openai-key", "credential_type": "api_key", "credential_value": "sk-xxx"})
	req := httptest.NewRequest(http.MethodPost, "/credential/new", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.CredentialNew(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialInfoV2_Success(t *testing.T) {
	ms := newMockStore()
	ms.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialName: "key"}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	req := httptest.NewRequest(http.MethodGet, "/credential/c1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.CredentialInfo(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialListV2_Success(t *testing.T) {
	ms := newMockStore()
	ms.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{{CredentialID: "c1"}}, nil
	}
	h := &Handlers{DB: ms}
	req := httptest.NewRequest(http.MethodGet, "/credential/list", nil)
	w := httptest.NewRecorder()
	h.CredentialList(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialDeleteV2_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteCredentialFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	req := httptest.NewRequest(http.MethodDelete, "/credential/c1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.CredentialDelete(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id}, nil
	}
	ms.updateCredentialFn = func(_ context.Context, _ db.UpdateCredentialParams) error { return nil }
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	body, _ := json.Marshal(map[string]string{"credential_id": "c1", "credential_value": "sk-new"})
	req := httptest.NewRequest(http.MethodPut, "/credential/c1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.CredentialUpdate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

// ---- UserExt ----

func TestUserInfo_Success(t *testing.T) {
	ms := newMockStore()
	ms.getUserFn = func(_ context.Context, userID string) (db.UserTable, error) {
		return db.UserTable{UserID: userID}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", "u1")
	req := httptest.NewRequest(http.MethodGet, "/user/u1", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.UserInfo(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.getUserFn = func(_ context.Context, userID string) (db.UserTable, error) {
		return db.UserTable{UserID: userID}, nil
	}
	ms.updateUserFn = func(_ context.Context, arg db.UpdateUserParams) (db.UserTable, error) {
		return db.UserTable{UserID: arg.UserID}, nil
	}
	h := &Handlers{DB: ms}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", "u1")
	body, _ := json.Marshal(map[string]string{"user_id": "u1", "user_alias": "alice"})
	req := httptest.NewRequest(http.MethodPut, "/user/u1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.UserUpdate(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUserDailyActivity_NoDB(t *testing.T) {
	h := newTestHandlers()
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("user_id", "u1")
	req := httptest.NewRequest(http.MethodGet, "/user/u1/activity", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.UserDailyActivity(w, req)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
