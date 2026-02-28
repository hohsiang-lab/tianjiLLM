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

func TestOrgNew_Success(t *testing.T) {
	ms := newMockStore()
	ms.createOrganizationFn = func(_ context.Context, arg db.CreateOrganizationParams) (db.OrganizationTable, error) {
		return db.OrganizationTable{OrganizationID: arg.OrganizationID, OrganizationAlias: arg.OrganizationAlias}, nil
	}
	h := &Handlers{DB: ms}

	alias := "test-org"
	body, _ := json.Marshal(map[string]string{"organization_alias": alias})
	req := httptest.NewRequest(http.MethodPost, "/organization/new", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OrgNew(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgInfo_Success(t *testing.T) {
	ms := newMockStore()
	ms.getOrganizationFn = func(_ context.Context, orgID string) (db.OrganizationTable, error) {
		return db.OrganizationTable{OrganizationID: orgID}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/organization/info?organization_id=org-1", nil)
	w := httptest.NewRecorder()
	h.OrgInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgInfo_MissingID(t *testing.T) {
	h := &Handlers{DB: newMockStore()}
	req := httptest.NewRequest(http.MethodGet, "/organization/info", nil)
	w := httptest.NewRecorder()
	h.OrgInfo(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestOrgUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateOrganizationFn = func(_ context.Context, arg db.UpdateOrganizationParams) (db.OrganizationTable, error) {
		return db.OrganizationTable{OrganizationID: arg.OrganizationID}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"organization_id": "org-1"})
	req := httptest.NewRequest(http.MethodPost, "/organization/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OrgUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteOrganizationFn = func(_ context.Context, orgID string) error { return nil }
	h := &Handlers{DB: ms}

	// chi URL param
	req := httptest.NewRequest(http.MethodDelete, "/organization/delete/org-1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("organization_id", "org-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()
	h.OrgDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgMemberAdd_Success(t *testing.T) {
	ms := newMockStore()
	ms.addOrgMemberFn = func(_ context.Context, arg db.AddOrgMemberParams) (db.OrganizationMembership, error) {
		return db.OrganizationMembership{UserID: arg.UserID, OrganizationID: arg.OrganizationID}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"user_id": "u1", "organization_id": "org-1"})
	req := httptest.NewRequest(http.MethodPost, "/organization/member_add", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OrgMemberAdd(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgMemberUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateOrgMemberFn = func(_ context.Context, arg db.UpdateOrgMemberParams) (db.OrganizationMembership, error) {
		return db.OrganizationMembership{UserID: arg.UserID, OrganizationID: arg.OrganizationID}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"user_id": "u1", "organization_id": "org-1"})
	req := httptest.NewRequest(http.MethodPatch, "/organization/member_update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OrgMemberUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestOrgMemberDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteOrgMemberFn = func(_ context.Context, arg db.DeleteOrgMemberParams) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"user_id": "u1", "organization_id": "org-1"})
	req := httptest.NewRequest(http.MethodDelete, "/organization/member_delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.OrgMemberDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
