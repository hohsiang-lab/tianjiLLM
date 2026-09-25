# Research: OpenAI Subscription Spend and Audit Attribution

## Repo Evidence

### Spend/callback path

- `internal/callback/callback.go` defines `LogData` with model, provider, API key hash, user/team/org, latency, token counts, cost, requester IP, and `UpstreamTokenKey`.
- `internal/spend/tracker.go` maps `LogData` into `SpendRecord`, then into `db.CreateSpendLogParams`.
- `internal/db/queries/spend_logs.sql` inserts `provider`, `organization_id`, and `upstream_token_key`, but no dedicated `credential_id`.
- `spend.Tracker.Record` updates virtual key spend only when `rec.APIKey` is non-empty. This should remain API-key/virtual-key behavior, not subscription bearer behavior.

### Subscription route/attempt path

- `internal/proxy/handler/openai_subscription_resolution.go` resolves configured subscription credential IDs into `resolvedOpenAISubscriptionCredential{CredentialID, BearerToken, AccountID}`.
- `internal/proxy/handler/openai_subscription_routing.go` orders candidates, gates rate-limited credentials, and supports round-robin/sticky/lowest-utilization selection.
- `internal/proxy/handler/openai_subscription_provider_retry.go` builds `openAISubscriptionProviderAttempt{credentialID, apiKey}` and can fail over across candidates.
- The selected `credentialID` is currently local to the attempt loop and is not part of `callback.LogData`.

### Existing audit/redaction path

- `internal/proxy/handler/audit_helper.go` sanitizes audit payloads through `redact.JSONValue` before `InsertAuditLog`.
- `internal/proxy/handler/audit_helper.go` also sanitizes management event payloads before dispatch.
- `internal/proxy/handler/chat.go` uses `redact.String(err.Error())` before `InsertErrorLog`.
- HO-1183 artifacts specify a shared redaction boundary for token/JWT/account payloads and safe credential metadata fields.

## Official OpenAI Evidence

### Authentication and organization/project attribution

OpenAI API reference states API keys are sent via HTTP bearer auth and that callers can specify `OpenAI-Organization` / `OpenAI-Project` headers. Usage from such requests counts against the specified organization/project. Source: https://platform.openai.com/docs/api-reference/authentication

Implication for HO-1182: Tianji should preserve its internal `organization_id` attribution separately from bearer material and should not store bearer values in spend/audit fields.

### Secret handling

OpenAI production guidance says API keys are secrets and should be loaded from environment variables or secret management rather than exposed in code or public repositories. Source: https://platform.openai.com/docs/guides/production-best-practices

Implication for HO-1182: subscription access/refresh tokens are at least as sensitive as API keys and must not enter audit/callback/spend metadata.

### Rate/spend distinction

OpenAI rate-limit guidance distinguishes request/token rate limits from usage/spend limits and lists rate-limit response headers. Source: https://platform.openai.com/docs/guides/rate-limits

Implication for HO-1182: this issue should attribute local spend records and lifecycle audits only. It should not infer monthly subscription allowance, billing quota, or remaining spend from rate-limit headers.

## Decisions

### D001 - Store subscription credential attribution in metadata first

**Decision**: Use `SpendLogs.metadata` for subscription credential attribution unless implementation proves a dedicated column is necessary.

**Rationale**: Current schema already has `provider`, `organization_id`, and `upstream_token_key`, but no `credential_id`. A docs-planned migration would be premature for a narrow attribution slice.

### D002 - Do not overload `api_key`

**Decision**: Keep `SpendLogs.api_key` and `callback.LogData.APIKey` semantics tied to client virtual key hash / API-key flow.

**Rationale**: `spend.Tracker.Record` uses `rec.APIKey` for `UpdateVerificationTokenSpend`. Putting subscription bearer or credential ID in this field would corrupt budget accounting.

### D003 - Add a safe subscription attribution object

**Decision**: Add a safe object or fields for subscription attribution containing `credential_id`, `provider`, `organization_id`, `action`, `status`, `reason_code`, and optional safe metadata.

**Rationale**: This avoids passing full credential rows, token bundles, request structs, or raw errors into asynchronous logging and audit paths.

### D004 - Attribute actual attempt outcome

**Decision**: Set spend attribution from the credential that actually served the request after failover/retry, not from the first configured credential ID.

**Rationale**: HO-1179/1180 can try multiple credentials. Spend attribution must match the serving account.

### D005 - Lifecycle audit belongs at operation boundaries

**Decision**: Emit audit events from connect/refresh/delete/test/disable handler/service boundaries after final action status is known.

**Rationale**: Low-level helpers do not always know actor, organization, route, or final operation intent.

## Alternatives Considered

### Add `credential_id` column to `SpendLogs`

Rejected for Todo planning as the default. It may become necessary if UI/filtering requires SQL-level credential filtering, but Linear only asks for attribution and regression coverage. Metadata satisfies the requirement with lower migration risk.

### Use `upstream_token_key` for OpenAI subscription credential ID

Rejected. `upstream_token_key` is an existing Anthropic OAuth token hash-style attribution field. Reusing it for OpenAI credential IDs would blur provider-specific semantics and could break existing filters.

### Audit full credential request/response structs and rely on redaction

Rejected. Redaction is a safety net, not a reason to persist broad secret-bearing structs. Use narrow safe payloads first, then redact.

## Open Questions

None requiring owner scope confirmation.

Implementation can choose exact field names as long as they satisfy the contract in `contracts/subscription-attribution.md` and tests prove selected-credential attribution, lifecycle audit, redaction, and API-key regression.
