# Quickstart: OpenAI OAuth PKCE Primitives and Provider Config

## Verification Only — No Real OpenAI Calls

1. Run config tests:

   ```bash
   go test ./internal/config/... -run 'TestOpenAIOAuth|TestDeriveOpenAI|TestValidateOpenAI' -v
   ```

2. Run OpenAI OAuth primitive/request tests:

   ```bash
   go test ./internal/provider/openai/... -run 'TestGeneratePKCE|TestGenerateState|TestBuildAuthorizeURL|TestExchangeCode' -v
   ```

3. Run existing OpenAI provider regression tests:

   ```bash
   go test ./internal/provider/openai/... -run 'TestTransformRequest' -v
   ```

4. Confirm no test hits live OpenAI:

   - Token exchange tests must use `httptest.NewServer`.
   - Endpoint override tests must assert mock URLs exactly.
   - There must be no dependency on real OpenAI account, browser, cookies, CLI auth, or network availability.

## Expected Config Shape (Draft)

Draft config shape is deployment-level under `general_settings`, aligned to the Linear issue's "Tianji public base URL" wording:

```yaml
general_settings:
  public_base_url: https://tianji.example.com
  openai_oauth:
    client_id: app_EMoamEEZ73f0CkXaXp7hrann
    issuer_url: https://auth.openai.com
    authorize_url: https://auth.openai.com/oauth/authorize
    token_url: https://auth.openai.com/oauth/token
    scopes:
      - openid
      - profile
      - email
      - offline_access
      - api.connectors.read
      - api.connectors.invoke
```

## Expected Token Exchange Form

```text
grant_type=authorization_code
code=<callback code>
redirect_uri=https://tianji.example.com/oauth/openai/callback
client_id=<public client id>
code_verifier=<pkce verifier>
```

`client_secret` must be absent.
