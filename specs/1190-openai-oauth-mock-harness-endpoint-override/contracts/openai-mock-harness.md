# Contract: OpenAI Mock Harness

## Package

`internal/testutil/openaitest`

## OAuthServer

### Constructor

```go
func NewOAuthServer(t testing.TB, opts OAuthServerOptions) *OAuthServer
```

`NewOAuthServer` starts a `httptest.Server`, registers cleanup with `t.Cleanup`, and returns endpoint accessors.

### Required Accessors

```go
func (s *OAuthServer) URL() string
func (s *OAuthServer) AuthorizeURL() string
func (s *OAuthServer) TokenURL() string
func (s *OAuthServer) Client() *http.Client
func (s *OAuthServer) Requests() []RecordedRequest
```

### Required Fixtures

```go
type TokenFixture struct {
    Code         string
    RefreshToken string
    Response     map[string]any
    StatusCode   int
}
```

- Authorization-code requests match `grant_type=authorization_code` + `code`.
- Refresh requests match `grant_type=refresh_token` + `refresh_token`.
- Missing fixture returns deterministic 400 JSON.
- Public-client assertion helper fails if `client_secret` or Basic auth is present.

## UpstreamServer

### Constructor

```go
func NewUpstreamServer(t testing.TB, opts UpstreamServerOptions) *UpstreamServer
```

### Required Accessors

```go
func (s *UpstreamServer) URL() string
func (s *UpstreamServer) BaseURL() string
func (s *UpstreamServer) Client() *http.Client
func (s *UpstreamServer) Requests() []RecordedRequest
```

### Required Endpoints

- `GET /v1/models`
- `POST /v1/chat/completions`
- At least one representative existing endpoint fixture from OpenAI provider tests, e.g. embeddings or images.

## Guard Transport

```go
func NewNoRealOpenAIGuard(base http.RoundTripper, allowedHosts ...string) http.RoundTripper
func NewGuardedClient(allowedHosts ...string) *http.Client
```

- Must reject `auth.openai.com` and `api.openai.com` by default.
- Must allow local mock hosts from `httptest.Server.URL`.
- Error message must include the forbidden host and must not include token bodies/secrets.

## RecordedRequest

```go
type RecordedRequest struct {
    Method string
    Path   string
    Host   string
    Header http.Header
    Body   []byte
    Form   url.Values
}
```

Recorded request snapshots must be copies, not references to mutable request/body state.
