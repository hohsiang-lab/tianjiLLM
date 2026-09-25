# Quickstart: HO-1176 Verification

## Targeted Tests

```bash
go test ./internal/auth/... ./internal/db/... ./internal/proxy/handler/... -run 'TestOpenAISubscriptionCredential|TestCredentialList_Redacts|TestCredentialInfo_Redacts|TestUpdateCredentialValueAndInfo' -v
```

## sqlc Generation

```bash
make generate
```

## Handler Regression Tests

```bash
go test ./internal/proxy/handler/... -v
```

## Manual Inspection Checklist

1. Create or update an OpenAI subscription credential through the implementation helper.
2. Inspect the stored DB row:
   - `credential_type` is `openai_subscription`.
   - `credential_value` is ciphertext and does not contain raw token strings.
   - `credential_info` contains only non-secret metadata.
3. Decrypt `credential_value` locally in a test with the master key and verify `access_token`, `refresh_token`, `expires_at`, and `account_id`.
4. Call `/credentials/list` and `/credentials/info/{credential_id}` through tests or local server.
5. Confirm response body omits `credential_value`, `access_token`, `refresh_token`, and `id_token`.
6. Simulate token refresh with a new `refresh_token`; confirm old refresh token disappears from decrypted stored bundle.
7. Simulate token refresh without `refresh_token`; confirm old refresh token remains.

## No Real OpenAI Requirement

All tests must run offline. Do not require real OpenAI accounts, live token exchange, or OpenAI network access for HO-1176.
