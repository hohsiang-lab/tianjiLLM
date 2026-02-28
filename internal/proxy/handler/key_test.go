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

func TestKeyGenerate_Success(t *testing.T) {
	ms := newMockStore()
	ms.createVerificationTokenFn = func(_ context.Context, arg db.CreateVerificationTokenParams) (db.VerificationToken, error) {
		return db.VerificationToken{Token: arg.Token, KeyName: arg.KeyName}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"key_name": "test-key"})
	req := httptest.NewRequest(http.MethodPost, "/key/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.KeyGenerateHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["key"] == nil {
		t.Fatal("expected key in response")
	}
}

func TestKeyInfo_Success(t *testing.T) {
	ms := newMockStore()
	ms.getVerificationTokenFn = func(_ context.Context, token string) (db.VerificationToken, error) {
		return db.VerificationToken{Token: token}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/key/info?key=sk-test", nil)
	w := httptest.NewRecorder()
	h.KeyInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeyInfo_MissingKey(t *testing.T) {
	h := &Handlers{DB: newMockStore()}
	req := httptest.NewRequest(http.MethodGet, "/key/info", nil)
	w := httptest.NewRecorder()
	h.KeyInfo(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestKeyList_Success(t *testing.T) {
	ms := newMockStore()
	ms.countVerificationTokensFilteredFn = func(_ context.Context, _ db.CountVerificationTokensFilteredParams) (int64, error) {
		return 1, nil
	}
	ms.listVerificationTokensFilteredFn = func(_ context.Context, _ db.ListVerificationTokensFilteredParams) ([]db.VerificationToken, error) {
		return []db.VerificationToken{{Token: "abc"}}, nil
	}
	h := &Handlers{DB: ms}

	req := httptest.NewRequest(http.MethodGet, "/key/list", nil)
	w := httptest.NewRecorder()
	h.KeyList(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["total_count"].(float64) != 1 {
		t.Fatalf("expected total_count 1")
	}
}

func TestKeyDelete_Success(t *testing.T) {
	ms := newMockStore()
	ms.deleteVerificationTokenFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string][]string{"keys": {"sk-1"}})
	req := httptest.NewRequest(http.MethodPost, "/key/delete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.KeyDelete(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeyBlock_Success(t *testing.T) {
	ms := newMockStore()
	ms.blockVerificationTokenFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"key": "sk-1"})
	req := httptest.NewRequest(http.MethodPost, "/key/block", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.KeyBlock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeyUnblock_Success(t *testing.T) {
	ms := newMockStore()
	ms.unblockVerificationTokenFn = func(_ context.Context, _ string) error { return nil }
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"key": "sk-1"})
	req := httptest.NewRequest(http.MethodPost, "/key/unblock", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.KeyUnblock(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestKeyUpdate_Success(t *testing.T) {
	ms := newMockStore()
	ms.updateVerificationTokenFn = func(_ context.Context, arg db.UpdateVerificationTokenParams) (db.VerificationToken, error) {
		return db.VerificationToken{Token: arg.Token}, nil
	}
	h := &Handlers{DB: ms}

	body, _ := json.Marshal(map[string]string{"key": "sk-1"})
	req := httptest.NewRequest(http.MethodPost, "/key/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.KeyUpdate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
