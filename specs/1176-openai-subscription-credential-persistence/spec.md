# Feature Specification: Encrypted OpenAI Subscription Credential Persistence

**Feature Branch**: `HO-1176-openai-subscription-credential-persistence`  
**Created**: 2026-05-06  
**Status**: Draft  
**Input**: Linear HO-1176 - `[BE] Encrypted OpenAI subscription credential persistence`

> Informed by memory: no HO-1176-specific memory found in daily notes or memory search. Related repo artifacts checked: HO-1174 OpenAI OAuth PKCE/token bundle primitives and HO-1190 OpenAI OAuth mock harness.

## Scope Boundary

This issue is the credential-storage slice for OpenAI subscription OAuth support. It persists OpenAI OAuth token bundles after a later callback/refresh flow receives them, and it ensures management APIs expose only non-secret metadata.

**In scope**:

- Persist `credential_type = "openai_subscription"` in existing `CredentialTable`.
- Store the secret token bundle in encrypted `CredentialTable.credential_value`.
- Token bundle fields persisted as the secret payload: `access_token`, `refresh_token`, `expires_at`, `account_id`.
- Store non-secret metadata in `CredentialTable.credential_info`: `email`, `scopes`, `status`, `last_refresh_at`, `last_error`, `disabled_reason`.
- Persist a rotated refresh token when a token refresh response returns one.
- Keep credential list/info responses redacted by default: no encrypted blob, raw token bundle, access token, or refresh token.
- Add migration + rollback only if implementation discovers a schema change is actually needed.

**Out of scope**:

- OpenAI OAuth authorize/callback routes, browser redirect handling, PKCE generation, endpoint override plumbing, or mock harness work already covered by HO-1174/HO-1190.
- Token refresh scheduling, retry/failover routing, model selection, or provider request injection.
- UI screens for managing OpenAI subscription credentials.
- Persisting `id_token` or raw OAuth response JSON unless a later issue explicitly adds that scope.
- Storing secrets outside `CredentialTable.credential_value`.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Persist OpenAI Subscription Token Bundle Encrypted (Priority: P1)

As a TianjiLLM backend component, I want to persist an OpenAI subscription OAuth token bundle as an encrypted credential, so the credential can survive restarts without exposing user tokens in plaintext storage or normal API responses.

**Why this priority**: This is the core value of HO-1176. Later callback and refresh features cannot safely use subscription auth until token bundles have a durable encrypted storage path.

**Independent Test**: Unit tests call the OpenAI subscription credential persistence helper with a token bundle and assert the DB receives an encrypted payload that decrypts to the expected JSON only with the master key.

**Acceptance Scenarios**:

1. **Given** an OpenAI OAuth token bundle with access token, refresh token, expiry, and account ID, **When** the backend persists it, **Then** `CredentialTable.credential_type` is `openai_subscription` and `credential_value` is not equal to any raw token string.
2. **Given** the stored `credential_value`, **When** it is decrypted with the configured master key, **Then** the plaintext JSON contains exactly the secret token-bundle fields needed for later refresh/use.
3. **Given** non-secret metadata such as email, scopes, status, and last refresh timestamp, **When** the credential is persisted, **Then** the metadata is stored in `credential_info` and remains separate from the encrypted secret bundle.

---

### User Story 2 - List and Inspect Metadata Without Secret Exposure (Priority: P1)

As a proxy admin, I want credential list/info endpoints to show OpenAI subscription metadata without returning token material, so normal management views can identify credential status without leaking access or refresh tokens.

**Why this priority**: The current credential table contains encrypted secrets. Exposing `credential_value` through list/info responses still leaks sensitive material and violates HO-1176's default redaction requirement.

**Independent Test**: Handler tests seed credentials containing encrypted token payloads and assert `/credentials/list` and `/credentials/info/{id}` responses include `credential_info` metadata but omit `credential_value`, `access_token`, and `refresh_token`.

**Acceptance Scenarios**:

1. **Given** an OpenAI subscription credential exists, **When** `/credentials/list` is requested, **Then** the response includes ID/name/type/org/timestamps and safe metadata but no `credential_value`.
2. **Given** the same credential, **When** `/credentials/info/{credential_id}` is requested, **Then** the response includes safe metadata and omits the encrypted blob and raw token fields.
3. **Given** an existing API-key credential, **When** list/info are requested, **Then** it is also redacted by default and existing management behavior remains compatible.

---

### User Story 3 - Persist Rotated Refresh Tokens Atomically With Metadata (Priority: P1)

As a token refresh path, I want to replace the stored OpenAI subscription token bundle when OpenAI returns a new access token and optional rotated refresh token, so future refreshes use the newest valid token material.

**Why this priority**: Refresh token rotation makes stale refresh tokens dangerous. A refresh response that returns a new refresh token must overwrite the old encrypted payload in the same credential row.

**Independent Test**: Unit tests call the refresh-persistence helper with an existing bundle and a refresh response. When a new refresh token is present, the decrypted stored bundle contains the new refresh token; when absent, it preserves the old one.

**Acceptance Scenarios**:

1. **Given** an existing stored refresh token and a refresh response with a new refresh token, **When** persistence runs, **Then** the encrypted stored bundle contains the new refresh token and not the old one.
2. **Given** an existing stored refresh token and a refresh response without a new refresh token, **When** persistence runs, **Then** the stored bundle preserves the previous refresh token while updating access token and expiry.
3. **Given** a refresh failure, **When** the credential metadata is updated, **Then** `last_error`, `status`, and `disabled_reason` are updated in `credential_info` without modifying the encrypted token bundle unless explicitly requested.

## Edge Cases

- Empty access token, refresh token, expiry, or account ID must be rejected before encryption.
- `credential_info` must be valid JSON object data; malformed metadata must return a client/config error before DB write.
- Metadata must never contain `access_token`, `refresh_token`, `id_token`, or encrypted `credential_value` copies.
- Decryption with the wrong master key must fail in tests and must not be treated as a missing credential.
- If OpenAI returns a rotated refresh token, the old refresh token must not survive in the stored encrypted payload.
- If OpenAI omits `refresh_token` during refresh, the previous refresh token must remain in the encrypted payload.
- Existing `api_key` credential create/update paths must remain supported.
- No schema migration is required if existing `credential_type`, `credential_value`, and `credential_info` columns satisfy the feature.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST persist OpenAI subscription credentials with `credential_type = "openai_subscription"`.
- **FR-002**: System MUST encrypt the OpenAI subscription token bundle before storing it in `CredentialTable.credential_value`.
- **FR-003**: The encrypted token bundle plaintext MUST include `access_token`, `refresh_token`, `expires_at`, and `account_id`.
- **FR-004**: System MUST store non-secret metadata in `CredentialTable.credential_info`, including `email`, `scopes`, `status`, `last_refresh_at`, `last_error`, and `disabled_reason` when values are available.
- **FR-005**: Credential list/info responses MUST NOT expose `credential_value` by default.
- **FR-006**: Credential list/info responses MUST NOT expose raw `access_token`, `refresh_token`, or `id_token` fields by default.
- **FR-007**: Credential list/info responses MUST expose safe metadata from `credential_info` for admin inspection.
- **FR-008**: System MUST update stored token bundles when a refresh succeeds.
- **FR-009**: System MUST overwrite the stored refresh token when the refresh response returns a rotated refresh token.
- **FR-010**: System MUST preserve the previous refresh token when a successful refresh response omits `refresh_token`.
- **FR-011**: System MUST update refresh metadata such as `last_refresh_at`, `last_error`, `status`, and `disabled_reason` without leaking secrets.
- **FR-012**: System MUST use sqlc-defined queries for any new credential update/read behavior.
- **FR-013**: System MUST add migration and rollback files only if a schema change is required; otherwise the plan MUST explicitly record that no schema migration is needed.
- **FR-014**: Existing `api_key` credential behavior MUST remain compatible, except for safer default redaction in list/info.

### Key Entities

- **OpenAI Subscription Token Bundle**: Secret JSON payload encrypted into `credential_value`; contains `access_token`, `refresh_token`, `expires_at`, and `account_id`.
- **OpenAI Subscription Credential Metadata**: Non-secret JSON object stored in `credential_info`; contains `email`, `scopes`, `status`, `last_refresh_at`, `last_error`, and `disabled_reason`.
- **CredentialTable Row**: Existing durable credential storage row keyed by `credential_id`; reuses `credential_type`, `credential_value`, and `credential_info`.
- **Refresh Persistence Update**: A single logical update that writes a new encrypted token bundle and refreshed metadata for an existing credential.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Tests prove persisted OpenAI subscription token bundles are encrypted at rest and decrypt to the expected secret payload only with the configured master key.
- **SC-002**: Tests prove `/credentials/list` and `/credentials/info/{id}` responses omit `credential_value`, `access_token`, `refresh_token`, and `id_token`.
- **SC-003**: Tests prove safe metadata fields are returned for OpenAI subscription credentials.
- **SC-004**: Tests prove rotated refresh tokens overwrite old refresh tokens in the encrypted stored payload.
- **SC-005**: Tests prove refresh responses without `refresh_token` preserve the existing stored refresh token.
- **SC-006**: `go test` for credential handler/service/db packages passes offline without real OpenAI credentials or network access.

## Assumptions

- Existing `CredentialTable` columns are sufficient: `credential_type` classifies the credential, `credential_value` stores NaCl SecretBox ciphertext, and `credential_info JSONB` stores safe metadata.
- Existing HO-1174 `openai.TokenBundle` uses `expires_in`; this issue should persist an absolute `expires_at` timestamp for durable refresh decisions.
- OpenAI/Codex managed auth treats token caches as password-equivalent and refreshes/persists updated token bundles; Tianji should follow the same secret-handling principle by keeping token material encrypted and redacted.
