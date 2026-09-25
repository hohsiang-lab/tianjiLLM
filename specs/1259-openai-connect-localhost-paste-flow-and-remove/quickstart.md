# Quickstart: HO-1259 OpenAI localhost paste flow

## Manual Product Flow

1. Sign in to TianjiLLM UI.
2. Open `/ui/credentials`.
3. Click `Connect OpenAI`; Tianji keeps the original tab on `/ui/credentials` and opens OpenAI authorization in a new tab/window.
4. Complete OpenAI authorization in the new tab/window.
5. The new tab/window lands on `http://localhost:1455/auth/callback?code=...&state=...`.
6. Copy the full browser URL from the new tab/window.
7. Return to the original `/ui/credentials` tab.
8. Click `Paste callback URL`, paste the full URL in the modal, and submit.
9. TianjiLLM parses the URL server-side, exchanges token using stored `redirect_uri`, and saves the OpenAI subscription credential.

## Expected Config

Minimal OpenAI OAuth config should not require `public_base_url`:

```yaml
general_settings:
  openai_oauth:
    enabled: true
```

Optional test/future override:

```yaml
general_settings:
  openai_oauth:
    enabled: true
    redirect_uri: http://localhost:1455/auth/callback
    authorize_url: http://127.0.0.1:18080/oauth/authorize
    token_url: http://127.0.0.1:18080/oauth/token
    client_id: app_test
```

`proxy.yml` / proxy YAML config artifacts should also remove `general_settings.public_base_url`; OpenAI OAuth must rely on the explicit `openai_oauth.redirect_uri` value above or the default localhost callback.

## Verification Commands

```bash
go test ./internal/config/... ./internal/openaioauth/... ./internal/ui/... ./internal/proxy/handler/... -run 'Test.*OpenAI|Test.*OAuth' -count=1
go test ./internal/provider/openai/... -run 'TestBuildAuthorizeURL|TestExchangeCode' -count=1
go test ./test/e2e -tags e2e -run 'TestOpenAIConnect|TestOpenAICallback' -count=1
rg "PublicBaseURL|public_base_url|DeriveOpenAIRedirectURI|ValidateOpenAIRedirectURI" internal test cmd config || true
find . -maxdepth 4 \( -name 'proxy.yml' -o -name 'proxy.yaml' -o -name 'proxy*.yml' -o -name 'proxy*.yaml' \) -print
git diff --check origin/main...HEAD
```

## Expected Search Result After Implementation

Live runtime OpenAI OAuth code must have no dependency on:

- `GeneralSettings.PublicBaseURL`
- `public_base_url`
- `DeriveOpenAIRedirectURI`
- `ValidateOpenAIRedirectURI`

Historical specs may still mention those names only as past context.
