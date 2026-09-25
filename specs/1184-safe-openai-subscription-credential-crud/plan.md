# Implementation Plan: Safe OpenAI Subscription Credential CRUD

**Branch**: `HO-1184-safe-openai-subscription-credential-crud` | **Date**: 2026-05-08 | **Spec**: [spec.md](spec.md)
**Input**: `/specs/1184-safe-openai-subscription-credential-crud/spec.md`

## Summary

把既有 `/credentials/*` management API 補成明確的 `credential_type=openai_subscription` 安全 CRUD contract：維持 proxy-admin auth、維持 encrypted `credential_value`、維持 list/info redaction、補 metadata-aware update、拒絕 cross-type mutation，並鎖定 idempotent/local-only delete。Todo 階段只產 SpecKit artifacts，不寫 production code。

## Technical Context

**Language/Version**: Go module in TianjiLLM
**Relevant Packages**: `internal/proxy/handler`、`internal/proxy/server.go`、`internal/auth`、`internal/db`、`internal/security/redact`
**Storage**: Existing PostgreSQL `CredentialTable` with `credential_type`、encrypted `credential_value`、`credential_info JSONB`、`organization_id`、timestamps
**Testing**: Go handler/unit tests with existing mock store; offline only
**Constraints**: Todo state is docs-only; no real OpenAI credentials; no remote OpenAI revoke; no schema migration unless implementation proves necessary
**Scope**: Admin CRUD path for local credential rows only

## Current Repo Findings

- `internal/proxy/server.go` already mounts `/credentials/new`、`/credentials/list`、`/credentials/info/{credential_id}`、`/credentials/update`、`/credentials/delete/{credential_id}` behind `s.AuthMiddleware`。
- `internal/auth/rbac.go` maps `/credentials` to `RoleProxyAdmin`。
- `internal/proxy/handler/credentials.go` already defines `CredentialTypeOpenAISubscription`、`OpenAISubscriptionTokenBundle`、`OpenAISubscriptionCredentialInfo`、encrypted create/update helpers、and `redactedCredentialResponse`。
- Current list/info responses already omit `credential_value` and route `credential_info` through redaction。
- Current `CredentialUpdate` accepts only `credential_id` and `credential_value`; HO-1184 should add safe metadata update behavior and explicit cross-type guards。
- Current `CredentialDelete` deletes locally and emits subscription lifecycle audit on known subscription rows; HO-1184 should lock idempotent local-only semantics。
- `internal/db/queries/credential.sql` already has create/get/list/list-by-org/update/update-value-and-info/update-info/delete queries。

## External Evidence

- OpenAI production best practices state API keys are bearer-auth credentials and should be exposed to applications through environment variables or secret-management services, not hard-coded in codebases: <https://platform.openai.com/docs/guides/production-best-practices>
- OpenAI API reference security guidance says API keys should be securely loaded from environment variables or key management services on the server: <https://platform.openai.com/docs/api-reference/debugging-requests>
- OpenAI Help Center exposes API-key deletion as an OpenAI dashboard/account operation; HO-1184 intentionally does not implement remote OpenAI revoke/delete and only deletes Tianji's local row: <https://help.openai.com/en?q=api+key>

## Constitution Check

| Principle | Status | Notes |
| --- | --- | --- |
| Repo reality first | PASS | Plan based on current `/credentials` routes, RBAC, handlers, tests, DB queries, and sibling specs. |
| Research before build | PASS | Existing repo patterns and official OpenAI key-safety/delete docs checked. |
| Failing tests first | PASS | `tasks.md` starts with CRUD/redaction/RBAC tests before implementation tasks. |
| No production code in Todo | PASS | Diff is limited to `specs/1184-safe-openai-subscription-credential-crud/`. |
| sqlc-first DB access | PASS | Existing sqlc credential queries are sufficient unless implementation proves a new query is needed. |
| Secret redaction | PASS | API response contract forbids encrypted and raw secret material. |

## Project Structure

```text
specs/1184-safe-openai-subscription-credential-crud/
|-- spec.md
|-- plan.md
|-- research.md
|-- data-model.md
|-- quickstart.md
|-- analyze.md
|-- contracts/
|   `-- openai-subscription-credential-crud.md
|-- checklists/
|   `-- requirements.md
`-- tasks.md
```

Future implementation scope:

```text
internal/
|-- auth/
|   `-- rbac.go
|-- db/
|   |-- queries/credential.sql
|   |-- credential.sql.go
|   `-- interface.go
`-- proxy/
    |-- server.go
    `-- handler/
        |-- credentials.go
        |-- credential_test.go
        |-- openai_subscription_attribution_test.go
        `-- mock_store_test.go
```

## Proposed API Shape

沿用既有 route family，除非 In Progress 實作發現 compatibility reason 需要 typed alias：

```text
POST   /credentials/new
GET    /credentials/list?organization_id=<optional>
GET    /credentials/info/{credential_id}
POST   /credentials/update
DELETE /credentials/delete/{credential_id}
```

Create request:

```json
{
  "credential_name": "openai-main",
  "credential_type": "openai_subscription",
  "credential_value": "{\"access_token\":\"...\",\"refresh_token\":\"...\",\"expires_at\":\"2026-05-08T00:00:00Z\",\"account_id\":\"acct_123\"}",
  "credential_info": {
    "email": "owner@example.com",
    "scopes": ["openid", "offline_access"],
    "status": "active"
  },
  "organization_id": "optional-org-id"
}
```

Update request should remain backward compatible for API-key rows while supporting subscription-safe metadata:

```json
{
  "credential_id": "cred_123",
  "credential_value": "{\"access_token\":\"...\",\"refresh_token\":\"...\",\"expires_at\":\"2026-05-08T00:00:00Z\",\"account_id\":\"acct_123\"}",
  "credential_info": {
    "status": "active",
    "last_refresh_at": "2026-05-08T00:00:00Z"
  }
}
```

Response:

```json
{
  "credential_id": "cred_123",
  "credential_name": "openai-main",
  "credential_type": "openai_subscription",
  "credential_info": {
    "email": "owner@example.com",
    "status": "active"
  },
  "organization_id": "org_123",
  "created_at": "timestamp",
  "updated_at": "timestamp"
}
```

## Data Flow

```text
Authenticated proxy-admin request
  -> existing AuthMiddleware + RBAC /credentials gate
  -> handler validates credential_type/openai_subscription scope
  -> sanitize credential_info through shared redaction boundary
  -> validate OpenAISubscriptionTokenBundle when credential_value is supplied
  -> auth.Encrypt(canonical bundle JSON, masterKey)
  -> sqlc Store method
  -> redactCredential response mapper
  -> safe JSON response
```

Delete:

```text
Authenticated proxy-admin DELETE
  -> optional GetCredential for lifecycle audit context
  -> DeleteCredential local DB call
  -> success even when row was already absent
  -> no OpenAI remote revoke/logout/delete
  -> redacted lifecycle audit only when existing subscription context is known
```

## Design Decisions

### Reuse existing `/credentials` routes

Repo already has authenticated credential CRUD and RBAC maps `/credentials` to `RoleProxyAdmin`。HO-1184 should harden this surface instead of adding parallel routes。

### Keep `credential_value` write-only

Management APIs may accept secret material for create/update, but responses must only return redacted metadata。這與 HO-1176、HO-1183 和 OpenAI key-safety guidance 對齊。

### Update metadata and value together when supplied

Current helper-level value+info updates already exist for refresh persistence。Admin CRUD should support safe metadata updates, but reject secret-bearing metadata。

### Delete is local-only and idempotent

Deleting Tianji's credential row must not imply OpenAI account/API-key revocation。Remote revoke is out of scope and should stay absent from tests and code paths。

### Cross-type mutation is rejected for subscription path

Existing `api_key` rows must not be mutated as OpenAI subscription credentials。Update should inspect existing row and reject mismatch before encryption/write。

## Failing Tests

| Test | File | Assertion |
| --- | --- | --- |
| `TestCredentialNew_OpenAISubscriptionCRUDResponseIsRedacted` | `internal/proxy/handler/credential_test.go` | Create response omits `credential_value` and token fields. |
| `TestCredentialNew_OpenAISubscriptionRejectsSecretMetadata` | `internal/proxy/handler/credential_test.go` | Secret-bearing metadata is rejected before DB write. |
| `TestCredentialList_OpenAISubscriptionSafeFieldsOnly` | `internal/proxy/handler/credential_test.go` | List response includes safe metadata and omits encrypted/raw secrets. |
| `TestCredentialInfo_OpenAISubscriptionSafeFieldsOnly` | `internal/proxy/handler/credential_test.go` | Info response includes safe metadata and omits encrypted/raw secrets. |
| `TestCredentialUpdate_OpenAISubscriptionUpdatesValueAndInfo` | `internal/proxy/handler/credential_test.go` | Update validates/encrypts bundle and persists sanitized metadata. |
| `TestCredentialUpdate_OpenAISubscriptionRejectsTypeMismatch` | `internal/proxy/handler/credential_test.go` | Updating an `api_key` row as subscription fails. |
| `TestCredentialUpdate_OpenAISubscriptionRejectsTypeChange` | `internal/proxy/handler/credential_test.go` | Request cannot change existing `credential_type`. |
| `TestCredentialDelete_OpenAISubscriptionIdempotentLocalOnly` | `internal/proxy/handler/credential_test.go` | Existing and missing deletes return success and no remote OpenAI client is invoked. |
| `TestCredentialsRouteRequiresProxyAdmin` | auth/server test location chosen during implementation | `/credentials` remains `RoleProxyAdmin` protected. |

## Verification Commands

```bash
go test ./internal/proxy/handler/... -run 'TestCredential(New|List|Info|Update|Delete)_OpenAISubscription|TestOpenAISubscriptionCredential' -count=1 -v
go test ./internal/auth/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Implementation Phases

### Phase 1: Tests First

Add failing handler/RBAC tests for create/list/info/update/delete, redaction, cross-type rejection, idempotent local delete, and no remote OpenAI interaction。

### Phase 2: Request/Response Contract Hardening

Adjust request structs and response mapping only as needed to support safe metadata update while preserving existing API-key compatibility。

### Phase 3: Persistence and Delete Semantics

Reuse existing sqlc-backed credential methods, keep delete local-only/idempotent, and avoid schema changes unless tests prove existing query surface insufficient。

### Phase 4: Verification

Run targeted handler/auth tests, `git diff --check origin/main...HEAD`, and confirm Todo diff remains docs-only before moving to Waiting。
