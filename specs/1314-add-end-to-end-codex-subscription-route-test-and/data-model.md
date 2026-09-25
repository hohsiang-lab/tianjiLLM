# Data Model: Codex subscription route E2E and credential lookup recovery

## Existing Tables

### `CredentialTable`

Purpose: stores encrypted OpenAI subscription credential bundles.

Relevant fields:

- `credential_id TEXT PRIMARY KEY`
- `credential_name TEXT`
- `credential_type TEXT`
- `credential_value TEXT`
- `credential_info JSONB`
- `organization_id TEXT NULL`
- `created_at TIMESTAMP`
- `created_by TEXT`
- `updated_at TIMESTAMP`
- `updated_by TEXT`

Required fixture state:

- `credential_type = "openai_subscription"`
- `credential_value` is encrypted with the handler `general_settings.master_key`
- decrypted bundle contains fake `access_token`, `refresh_token`, `expires_at`, and `account_id`
- `credential_info.status = "active"` unless the test case is explicitly disabled/failure

### `ProxyModelTable`

Purpose: stores DB-managed runtime model config.

Relevant `tianji_params` fields:

- `model = "openai/*"`
- `openai_subscription_credential_ids = ["<credential_id>"]`
- `openai_subscription_transport = "chatgpt_codex_backend"`
- optional `api_key`/`api_base` only for API-key no-regression tests

## Existing In-Memory Entities

### `OpenAISubscriptionTokenBundle`

Required fields:

- `AccessToken`: fake bearer sent to Codex backend mock only
- `RefreshToken`: fake refresh value used by refresh tests
- `ExpiresAt`: future for fresh tests, near-past/near-expiry for refresh tests
- `AccountID`: sent as `ChatGPT-Account-Id`

### `OpenAISubscriptionCredentialInfo`

Relevant fields:

- `Email`
- `Scopes`
- `Status`
- `LastRefreshAt`
- `LastError`
- `DisabledReason`

Status values used by tests:

- `active`
- `disabled`
- `refresh_failed`

### `resolvedOpenAISubscriptionCredential`

Relevant fields:

- `CredentialID`
- `BearerToken`
- `AccountID`
- `CodexLogin` must remain nil for `chatgpt_codex_backend`

### `chatgptcodex.Payload`

Expected route-level request fields:

- `model = "gpt-5.5"` for caller model `openai/gpt-5.5`
- `store = false`
- `stream = true` for streaming route
- `instructions` from leading system/developer messages when present
- `input` as typed message/content parts

## State Transitions

Credential lookup:

1. Configured credential ID is selected from runtime model params.
2. `GetCredential` loads row by text ID.
3. Missing row maps to `credential_missing`.
4. Non-row query/scan/runtime failure maps to `credential_lookup_failed`.
5. Active row decrypts and validates bundle.
6. Fresh token returns directly; stale token enters refresh path.

Route execution:

1. `openai/*` wildcard resolves `openai/gpt-5.5`.
2. `chatgpt_codex_backend` transport is selected.
3. Credential candidates are ordered.
4. Codex request is built with bearer/account ID.
5. Codex SSE backend mock emits delta/completed events.
6. Tianji returns OpenAI-compatible chat completion chunks and `[DONE]`.

## Secret Safety Rules

- Token fields are fake in tests and still must not appear in responses or assertion error messages.
- Live verification notes must not print decrypted `credential_value`, bearer tokens, refresh tokens, ID tokens, cookies, or raw `Authorization` headers.
