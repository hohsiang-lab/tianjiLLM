# Requirements Checklist: Safe OpenAI Subscription Credential CRUD

## Scope

- [x] Scope covers create/list/info/update/delete for `credential_type=openai_subscription`。
- [x] Scope keeps delete local-only with no remote OpenAI revoke。
- [x] Scope excludes UI, OAuth callback, refresh scheduler, routing/failover, spend attribution, and quota parsing。
- [x] Scope preserves existing auth/session/permission conventions。

## Security

- [x] Spec forbids returning `credential_value`。
- [x] Spec forbids raw access token, refresh token, id token, bearer, JWT, encrypted blob, and account payload leakage。
- [x] Spec requires metadata sanitization on create/update and response。
- [x] Spec requires sanitized error responses。

## Data and API

- [x] Existing `CredentialTable` is identified as the storage target。
- [x] No schema migration is planned unless implementation proves one necessary。
- [x] Existing `/credentials` route family is the preferred API surface。
- [x] Existing sqlc-backed credential queries are the preferred DB path。

## Tests

- [x] Create success/error and redaction paths are listed。
- [x] List/info redaction paths are listed。
- [x] Update value/metadata/type mismatch paths are listed。
- [x] Delete idempotent local-only path is listed。
- [x] Permission behavior test is listed。
- [x] Tests are offline and synthetic。

## Open Questions

- [x] No owner input required before Waiting; implementation can start after Linear moves to In Progress。
