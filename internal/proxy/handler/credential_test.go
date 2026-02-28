package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/stretchr/testify/assert"
)

func TestCredentialNew_Success(t *testing.T) {
	m := newMockStore()
	m.createCredentialFn = func(_ context.Context, arg db.CreateCredentialParams) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: arg.CredentialID, CredentialName: arg.CredentialName}, nil
	}
	h := mockHandlers(m)
	h.Config = &config.ProxyConfig{}
	h.Config.GeneralSettings.MasterKey = "test-master-key-32-bytes-long!!!"
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{"credential_name":"aws","credential_value":"secret123"}`))
	r.Header.Set("Content-Type", "application/json")
	h.CredentialNew(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialNew_MissingFields(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("POST", "/credentials/new", strings.NewReader(`{"credential_name":"aws"}`))
	r.Header.Set("Content-Type", "application/json")
	h.CredentialNew(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialList_Success(t *testing.T) {
	m := newMockStore()
	m.listCredentialsFn = func(_ context.Context) ([]db.CredentialTable, error) {
		return []db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list", nil)
	h.CredentialList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialList_ByOrg(t *testing.T) {
	m := newMockStore()
	m.listCredentialsByOrgFn = func(_ context.Context, orgID *string) ([]db.CredentialTable, error) {
		return []db.CredentialTable{}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/list?organization_id=org-1", nil)
	h.CredentialList(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialInfo_Success(t *testing.T) {
	m := newMockStore()
	m.getCredentialFn = func(_ context.Context, id string) (db.CredentialTable, error) {
		return db.CredentialTable{CredentialID: id, CredentialName: "aws"}, nil
	}
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.CredentialInfo(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCredentialInfo_MissingID(t *testing.T) {
	h := mockHandlers(newMockStore())
	w := httptest.NewRecorder()
	r := httptest.NewRequest("GET", "/credentials/info/", nil)
	h.CredentialInfo(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCredentialDelete_Success(t *testing.T) {
	m := newMockStore()
	m.deleteCredentialFn = func(_ context.Context, _ string) error { return nil }
	h := mockHandlers(m)
	w := httptest.NewRecorder()
	r := httptest.NewRequest("DELETE", "/credentials/delete/c1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("credential_id", "c1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	h.CredentialDelete(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}
