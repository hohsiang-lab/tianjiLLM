package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/stretchr/testify/require"
)

// Mechanics only: the fake upstream is not evidence of provider capability.
func TestNativeFilesLocalLifecycle(t *testing.T) {
	calls := 0
	editBody := `{ "model":"gpt-image-2", "prompt":"synthetic", "images":[{"file_id":"file_owned"}], "future_native":{"keep":true} }`
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "Bearer access-secret", r.Header.Get("Authorization"))
		require.Equal(t, "acct_image", r.Header.Get("ChatGPT-Account-Id"))
		require.Equal(t, "client-version", r.Header.Get("Version"))
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/backend-api/files":
			_, _ = io.WriteString(w, `{"file_id":"file_owned","upload_url":"https://synthetic.oaiusercontent.com/upload","status":"success"}`)
		case "/backend-api/files/file_owned/uploaded":
			_, _ = io.WriteString(w, `{"status":"success","file_size_bytes":7}`)
		case "/backend-api/codex/images/edits":
			b, _ := io.ReadAll(r.Body)
			require.Equal(t, editBody, string(b))
			require.Equal(t, "application/json; charset=utf-8", r.Header.Get("Content-Type"))
			_, _ = io.WriteString(w, `{"data":[],"future_response":true}`)
		case "/backend-api/files/file_owned":
			// Preserve a real refusal, and keep ownership so it can be reconciled.
			w.WriteHeader(404)
			_, _ = io.WriteString(w, `{"error":"not_found"}`)
		default:
			t.Errorf("unexpected native path %s", r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer backend.Close()
	h := newImageCodexHarness(t, backend.URL)
	h.Cache = cache.NewMemoryCache()
	ctx := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "owner")
	invoke := func(method, path, body, owner string, fn http.HandlerFunc) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(context.WithValue(ctx, middleware.ContextKeyTokenHash, owner))
		r.Header.Set("Content-Type", "application/json; charset=utf-8")
		r.Header.Set("Version", "client-version")
		r.Header.Set("Authorization", "Bearer caller-not-upstream")
		r.Header.Set("ChatGPT-Account-Id", "caller-not-account")
		route := chi.NewRouteContext()
		route.URLParams.Add("file_id", "file_owned")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	w := invoke("POST", "/backend-api/files", `{ "file_name":"synthetic.png", "file_size":7,"use_case":"codex","future":true }`, "owner", h.NativeFilesCreate)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	b, err := h.loadNativeFileBinding(ctx, "file_owned")
	require.NoError(t, err)
	require.False(t, b.Ready)
	w = invoke("POST", "/v1/images/edits", editBody, "owner", h.ImagesEdit)
	require.Equal(t, 404, w.Code)
	require.Equal(t, 1, calls)
	w = invoke("POST", "/backend-api/files/file_owned/uploaded", `{}`, "other", h.NativeFilesFinalize)
	require.Equal(t, 404, w.Code)
	require.Equal(t, 1, calls)
	w = invoke("POST", "/backend-api/files/file_owned/uploaded", `{}`, "owner", h.NativeFilesFinalize)
	require.Equal(t, 200, w.Code, w.Body.String())
	b, err = h.loadNativeFileBinding(ctx, "file_owned")
	require.NoError(t, err)
	require.True(t, b.Ready)
	w = invoke("POST", "/v1/images/edits", editBody, "owner", h.ImagesEdit)
	require.Equal(t, 200, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "future_response")
	w = invoke("DELETE", "/backend-api/files/file_owned", ``, "owner", h.NativeFilesDelete)
	require.Equal(t, 404, w.Code)
	_, err = h.loadNativeFileBinding(ctx, "file_owned")
	require.NoError(t, err)
	require.Equal(t, 4, calls)
}

func TestNativeImageAffinityFailsClosed(t *testing.T) {
	h := &Handlers{Cache: cache.NewMemoryCache()}
	ctx := context.WithValue(context.Background(), middleware.ContextKeyTokenHash, "owner")
	record := nativeFileBinding{Owner: nativeFileOwner(ctx), CredentialID: "one", AccountID: "account", Ready: true, ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, h.saveNativeFileBinding(ctx, "file_one", record))
	record.CredentialID = "two"
	require.NoError(t, h.saveNativeFileBinding(ctx, "file_two", record))
	candidates := []resolvedOpenAISubscriptionCredential{{CredentialID: "one", AccountID: "account"}}
	for _, body := range []string{`{"images":[{"file_id":"unknown"}]}`, `{"images":[{"file_id":"file_one"},{"file_id":"file_two"}]}`, `{"images":[{"file_id":"file_one"}],"mask":{"file_id":"unknown"}}`, `{"images":[{"image_url":"https://example.com/private"}]}`} {
		_, err := h.nativeImageCandidate(ctx, []byte(body), candidates)
		require.Error(t, err)
	}
	_, err := h.nativeImageCandidate(ctx, []byte(`{"images":[{"file_id":"file_one"}],"future":{"file_id":"unknown"}}`), candidates)
	require.Error(t, err)
	_, err = h.nativeImageCandidate(ctx, []byte(`{"images":[{"file_id":"file_one"}]}`), []resolvedOpenAISubscriptionCredential{{CredentialID: "one", AccountID: "changed"}})
	require.Error(t, err)
	otherTenant := context.WithValue(ctx, middleware.ContextKeyOrgID, "other-org")
	_, err = h.nativeImageCandidate(otherTenant, []byte(`{"images":[{"file_id":"file_one"}]}`), candidates)
	require.Error(t, err)
	record.Owner = nativeFileOwner(ctx)
	record.CredentialID = "one"
	record.ExpiresAt = time.Now().Add(time.Hour)
	record.Ready = false
	require.NoError(t, h.saveNativeFileBinding(ctx, "file_one", record))
	_, err = h.nativeImageCandidate(ctx, []byte(`{"images":[{"file_id":"file_one"}]}`), candidates)
	require.Error(t, err)
}
