# Quickstart: HO-1190 OpenAI Mock Harness

## After implementation starts

1. Add failing tests first:

```bash
go test ./internal/testutil/openaitest/... -v
go test ./internal/provider/openai/... -run 'Test.*EndpointOverrides|Test.*NoRealOpenAI' -v
```

2. Implement `internal/testutil/openaitest` helpers.

3. Re-run targeted verification:

```bash
go test ./internal/testutil/openaitest/... -v
go test ./internal/provider/openai/... -run 'TestOAuthMock|TestUpstreamMock|TestEndpointOverrides|TestNoRealOpenAI|TestExchangeCode' -v
```

4. Confirm no live OpenAI dependency:

```bash
OPENAI_API_KEY= \
go test ./internal/testutil/openaitest/... ./internal/provider/openai/... -run 'Test.*OpenAI.*Mock|Test.*EndpointOverrides|Test.*NoRealOpenAI|TestExchangeCode' -v
```

## Expected implementation shape

```go
oauthMock := openaitest.NewOAuthServer(t, openaitest.OAuthServerOptions{...})
cfg := config.OpenAIOAuthConfig{
    AuthorizeURL: oauthMock.AuthorizeURL(),
    TokenURL: oauthMock.TokenURL(),
    ClientID: "app_test",
}

upstream := openaitest.NewUpstreamServer(t, openaitest.UpstreamServerOptions{...})
provider := openai.NewWithBaseURL(upstream.BaseURL() + "/v1")
```
