package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

const nativeFileTTL = 24 * time.Hour

var errNativeFile = errors.New("unknown, expired or inaccessible native file")

type nativeFileBinding struct {
	Owner        string
	CredentialID string
	AccountID    string
	ExpiresAt    time.Time
	Ready        bool
	Size         int64
}

func nativeFileOwner(ctx context.Context) string {
	token, _ := ctx.Value(middleware.ContextKeyTokenHash).(string)
	user, _ := ctx.Value(middleware.ContextKeyUserID).(string)
	org, _ := ctx.Value(middleware.ContextKeyOrgID).(string)
	if token == "" && user == "" {
		return ""
	}
	b, _ := json.Marshal([]string{token, user, org})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func nativeFileKey(id string) string {
	sum := sha256.Sum256([]byte(id))
	return "codex-file-owner:" + hex.EncodeToString(sum[:])
}
func (h *Handlers) saveNativeFileBinding(ctx context.Context, id string, b nativeFileBinding) error {
	if h.Cache == nil || !nativeFileID(id) || b.Owner == "" || b.CredentialID == "" || b.AccountID == "" || !b.ExpiresAt.After(time.Now()) {
		return errNativeFile
	}
	raw, e := json.Marshal(b)
	if e != nil {
		return e
	}
	return h.Cache.Set(ctx, nativeFileKey(id), raw, time.Until(b.ExpiresAt))
}
func (h *Handlers) loadNativeFileBinding(ctx context.Context, id string) (nativeFileBinding, error) {
	var b nativeFileBinding
	if h.Cache == nil || id == "" || nativeFileOwner(ctx) == "" {
		return b, errNativeFile
	}
	raw, e := h.Cache.Get(ctx, nativeFileKey(id))
	if e != nil || json.Unmarshal(raw, &b) != nil || b.Owner != nativeFileOwner(ctx) || !b.ExpiresAt.After(time.Now()) {
		return nativeFileBinding{}, errNativeFile
	}
	return b, nil
}
func nativeJSON(r *http.Request) bool {
	contentType, err := singleRequestContentType(r)
	if err != nil {
		return false
	}
	media, _, e := mime.ParseMediaType(contentType)
	return e == nil && media == "application/json"
}
func nativeFileError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, model.ErrorResponse{Error: model.ErrorDetail{Message: message, Type: "invalid_request_error"}})
}

func (h *Handlers) nativeFilesBase() string {
	base := strings.TrimRight(h.chatGPTCodexBackendTransport().NativeResponsesURL(), "/")
	return strings.TrimSuffix(base, "/codex/responses")
}
func (h *Handlers) nativeFileRequest(ctx context.Context, method, target string, body []byte, contentType string, c resolvedOpenAISubscriptionCredential, headers http.Header) (*http.Response, error) {
	req, e := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(body))
	if e != nil {
		return nil, e
	}
	chatgptcodex.CopyCodexClientHeaders(req, []http.Header{headers})
	req.Header.Set("Authorization", "Bearer "+c.BearerToken)
	req.Header.Set("ChatGPT-Account-Id", c.AccountID)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	client := *h.openAIUpstreamHTTPClient()
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return client.Do(req)
}
func (h *Handlers) nativeFileCandidate(ctx context.Context, b nativeFileBinding) (resolvedOpenAISubscriptionCredential, error) {
	// Re-check current tenant eligibility and account identity before every operation.
	if h.DB == nil {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	row, e := h.DB.GetCredential(ctx, b.CredentialID)
	if e != nil {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	org, has := ctx.Value(middleware.ContextKeyOrgID).(string)
	if !openAISubscriptionCredentialInRequestScope(row, org, has) {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	c, e := h.resolveOpenAISubscriptionCredentialByID(ctx, b.CredentialID, openAISubscriptionTransportChatGPTCodexBackend)
	if e != nil || c.AccountID != b.AccountID {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	selectable, _ := h.filterOpenAISubscriptionCodexUsageSelectable(ctx, openAISubscriptionCandidatesFromResolved([]resolvedOpenAISubscriptionCredential{c}), h.openAISubscriptionNowUTC())
	if len(selectable) != 1 {
		return resolvedOpenAISubscriptionCredential{}, errNativeFile
	}
	return c, nil
}

// NativeFilesCreate preserves the Codex JSON create protocol; multipart stays on FilesUpload.
func (h *Handlers) NativeFilesCreate(w http.ResponseWriter, r *http.Request) {
	if !nativeJSON(r) || h.Cache == nil || nativeFileOwner(r.Context()) == "" {
		nativeFileError(w, 400, "native file ownership storage and authenticated owner required")
		return
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, maxOpenAIRequestBodyBytes+1))
	if e != nil || len(body) > maxOpenAIRequestBodyBytes {
		nativeFileError(w, 413, "native file request too large")
		return
	}
	var request struct {
		Size    int64  `json:"file_size"`
		Name    string `json:"file_name"`
		UseCase string `json:"use_case"`
	}
	if json.Unmarshal(body, &request) != nil || request.Size < 1 || request.Size > maxOpenAIRequestBodyBytes || request.Name == "" || request.UseCase != "codex" {
		nativeFileError(w, 400, "invalid Codex file create request")
		return
	}
	route, e := h.resolveOpenAIEndpointFallbackRoute(r.Context(), false)
	if e != nil || route.Transport != openAISubscriptionTransportChatGPTCodexBackend || len(route.Candidates) == 0 {
		nativeFileError(w, 400, "native subscription route unavailable")
		return
	}
	candidates := h.orderOpenAISubscriptionCodexCandidates(r.Context(), route.TianjiParams, route.Candidates)
	c := candidates[0]
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	resp, e := h.nativeFileRequest(ctx, r.Method, h.nativeFilesBase()+"/files", body, r.Header.Get("Content-Type"), c, r.Header)
	if e != nil {
		nativeFileError(w, 502, "native file create transport failed; do not replay")
		return
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		nativeFileError(w, 502, "native file create response unavailable")
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var result struct {
			ID        string `json:"file_id"`
			UploadURL string `json:"upload_url"`
		}
		if json.Unmarshal(raw, &result) != nil || !nativeFileID(result.ID) || !nativeUploadURL(result.UploadURL) {
			nativeFileError(w, 502, "invalid native upload destination")
			return
		}
		binding := nativeFileBinding{Owner: nativeFileOwner(ctx), CredentialID: c.CredentialID, AccountID: c.AccountID, ExpiresAt: time.Now().Add(nativeFileTTL), Size: request.Size}
		if h.saveNativeFileBinding(ctx, result.ID, binding) != nil {
			nativeFileError(w, 503, "native file ownership could not be saved; do not replay")
			return
		}
	}
	// Upload URL is the upstream's file-scoped capability, not account credentials.
	// No signed URLs/bodies enter callback logs or a local blob store.
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}
func nativeFileID(id string) bool {
	if id == "" || len(id) > 200 {
		return false
	}
	for _, r := range id {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
func nativeUploadURL(raw string) bool {
	u, e := url.Parse(raw)
	return e == nil && u.Scheme == "https" && u.User == nil && u.Port() == "" && (strings.HasSuffix(u.Hostname(), ".oaiusercontent.com") || strings.HasSuffix(u.Hostname(), ".blob.core.windows.net"))
}

func (h *Handlers) NativeFilesFinalize(w http.ResponseWriter, r *http.Request) {
	h.nativeFilesOwnedOperation(w, r, "/uploaded")
}
func (h *Handlers) NativeFilesDownload(w http.ResponseWriter, r *http.Request) {
	h.nativeFilesOwnedOperation(w, r, "/download")
}
func (h *Handlers) NativeFilesDelete(w http.ResponseWriter, r *http.Request) {
	h.nativeFilesOwnedOperation(w, r, "")
}
func (h *Handlers) nativeFilesOwnedOperation(w http.ResponseWriter, r *http.Request, suffix string) {
	id := chi.URLParam(r, "file_id")
	b, e := h.loadNativeFileBinding(r.Context(), id)
	if e != nil || !nativeFileID(id) {
		nativeFileError(w, 404, errNativeFile.Error())
		return
	}
	c, e := h.nativeFileCandidate(r.Context(), b)
	if e != nil {
		nativeFileError(w, 404, errNativeFile.Error())
		return
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, maxOpenAIRequestBodyBytes+1))
	if e != nil || len(body) > maxOpenAIRequestBodyBytes {
		nativeFileError(w, 413, "native file request too large")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
	defer cancel()
	resp, e := h.nativeFileRequest(ctx, r.Method, h.nativeFilesBase()+"/files/"+id+suffix, body, r.Header.Get("Content-Type"), c, r.Header)
	if e != nil {
		nativeFileError(w, 502, "native file request failed; do not replay")
		return
	}
	defer resp.Body.Close()
	raw, e := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if e != nil || len(raw) > 1<<20 {
		nativeFileError(w, 502, "native file response unavailable")
		return
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && suffix == "/uploaded" {
		var result struct {
			Status string `json:"status"`
			Size   int64  `json:"file_size_bytes"`
		}
		if json.Unmarshal(raw, &result) != nil {
			nativeFileError(w, 502, "invalid native finalization response")
			return
		}
		if result.Status == "success" {
			if result.Size != b.Size || result.Size > maxOpenAIRequestBodyBytes {
				nativeFileError(w, 413, "native file size mismatch")
				return
			}
			b.Ready = true
			if h.saveNativeFileBinding(ctx, id, b) != nil {
				nativeFileError(w, 503, "native file readiness could not be saved")
				return
			}
		}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && r.Method == http.MethodDelete {
		b.ExpiresAt = time.Now()
		_ = h.Cache.Delete(ctx, nativeFileKey(id))
	}
	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}
