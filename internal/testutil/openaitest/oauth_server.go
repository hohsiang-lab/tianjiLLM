package openaitest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type OAuthServerOptions struct {
	TokenFixtures []TokenFixture
	DeviceAuth    *DeviceAuthOptions
}

type DeviceAuthOptions struct {
	UserCodeResponse map[string]any
	PollResponses    []DevicePollResponse
}

type DevicePollResponse struct {
	StatusCode int
	Response   map[string]any
}

type TokenFixture struct {
	Code         string
	RefreshToken string
	Response     map[string]any
	StatusCode   int
}

type OAuthServer struct {
	server     *httptest.Server
	fixtures   []TokenFixture
	deviceAuth *DeviceAuthOptions

	mu              sync.Mutex
	requests        []RecordedRequest
	devicePollIndex int
}

func NewOAuthServer(t testing.TB, opts OAuthServerOptions) *OAuthServer {
	t.Helper()
	s := &OAuthServer{fixtures: append([]TokenFixture(nil), opts.TokenFixtures...), deviceAuth: opts.DeviceAuth}
	s.server = httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(s.server.Close)
	return s
}

func (s *OAuthServer) URL() string { return s.server.URL }

func (s *OAuthServer) Host() string {
	parsed, err := url.Parse(s.server.URL)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func (s *OAuthServer) AuthorizeURL() string { return s.server.URL + "/oauth/authorize" }

func (s *OAuthServer) TokenURL() string { return s.server.URL + "/oauth/token" }

func (s *OAuthServer) DeviceUserCodeURL() string {
	return s.server.URL + "/api/accounts/deviceauth/usercode"
}

func (s *OAuthServer) DeviceTokenURL() string {
	return s.server.URL + "/api/accounts/deviceauth/token"
}

func (s *OAuthServer) DeviceVerificationURL() string { return s.server.URL + "/codex/device" }

func (s *OAuthServer) DeviceRedirectURL() string { return s.server.URL + "/deviceauth/callback" }

func (s *OAuthServer) Client() *http.Client { return s.server.Client() }

func (s *OAuthServer) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RecordedRequest, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *OAuthServer) AssertPublicClientPKCE(t testing.TB, req RecordedRequest) {
	t.Helper()
	if _, ok := req.Form["client_secret"]; ok {
		t.Fatalf("OpenAI OAuth public-client request must not include client_secret")
	}
	if strings.HasPrefix(req.Header.Get("Authorization"), "Basic ") {
		t.Fatalf("OpenAI OAuth public-client request must not use Basic authorization")
	}
}

func (s *OAuthServer) PostTokenForm(t testing.TB, fields map[string]string) map[string]any {
	t.Helper()
	form := url.Values{}
	for key, value := range fields {
		form.Set(key, value)
	}
	resp, err := s.Client().Post(s.TokenURL(), "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("post token form: %v", err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		t.Fatalf("token endpoint status %d: %v", resp.StatusCode, body)
	}
	return body
}

func (s *OAuthServer) handle(w http.ResponseWriter, r *http.Request) {
	recorded := recordRequest(r)
	s.mu.Lock()
	s.requests = append(s.requests, recorded)
	s.mu.Unlock()

	switch r.URL.Path {
	case "/oauth/authorize":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>Mock OpenAI authorize</title>"))
	case "/api/accounts/deviceauth/usercode":
		s.handleDeviceUserCode(w, r)
	case "/api/accounts/deviceauth/token":
		s.handleDeviceToken(w, r)
	case "/codex/device", "/deviceauth/callback":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><title>Mock OpenAI device authorization</title>"))
	case "/oauth/token":
		s.handleToken(w, recorded)
	default:
		http.Error(w, fmt.Sprintf("unexpected OpenAI OAuth mock path: %s", r.URL.Path), http.StatusNotFound)
	}
}

func (s *OAuthServer) handleDeviceUserCode(w http.ResponseWriter, r *http.Request) {
	if s.deviceAuth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	response := s.deviceAuth.UserCodeResponse
	if response == nil {
		response = map[string]any{"device_auth_id": "[REDACTED]", "user_code": "ABCD-EFGH", "interval": "5"}
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *OAuthServer) handleDeviceToken(w http.ResponseWriter, r *http.Request) {
	if s.deviceAuth == nil {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.mu.Lock()
	index := s.devicePollIndex
	s.devicePollIndex++
	responses := s.deviceAuth.PollResponses
	s.mu.Unlock()
	if index >= len(responses) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "authorization_pending"})
		return
	}
	fixture := responses[index]
	status := fixture.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	response := fixture.Response
	if response == nil {
		response = map[string]any{"error": "authorization_pending"}
	}
	writeJSON(w, status, response)
}

func (s *OAuthServer) handleToken(w http.ResponseWriter, req RecordedRequest) {
	fixture, ok := s.findTokenFixture(req.Form)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":             "fixture_not_found",
			"error_description": "no OpenAI OAuth token fixture matched request",
		})
		return
	}
	status := fixture.StatusCode
	if status == 0 {
		status = http.StatusOK
	}
	response := fixture.Response
	if response == nil {
		response = map[string]any{"access_token": "[REDACTED]", "token_type": "Bearer", "expires_in": 3600}
	}
	writeJSON(w, status, response)
}

func (s *OAuthServer) findTokenFixture(form url.Values) (TokenFixture, bool) {
	grantType := form.Get("grant_type")
	for _, fixture := range s.fixtures {
		switch grantType {
		case "authorization_code":
			if fixture.Code != "" && fixture.Code == form.Get("code") {
				return fixture, true
			}
		case "refresh_token":
			if fixture.RefreshToken != "" && fixture.RefreshToken == form.Get("refresh_token") {
				return fixture, true
			}
		}
	}
	return TokenFixture{}, false
}

func writeJSON(w http.ResponseWriter, status int, value map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(value)
	_, _ = w.Write(buf.Bytes())
}
