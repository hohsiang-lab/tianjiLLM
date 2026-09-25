# Research: OpenAI Credential Test/Refresh/Disable Lifecycle APIs

## Repo Reality

- `internal/proxy/handler/credentials.go` already defines `CredentialTypeOpenAISubscription`、`OpenAISubscriptionTokenBundle`、`OpenAISubscriptionCredentialInfo` and safe CRUD behavior。
- HO-1184 spec requires subscription CRUD responses to omit `credential_value` and token fields。
- `internal/proxy/handler/openai_subscription_refresh.go` already implements normal refresh, forced refresh, refresh failure metadata, and auth-failed disable metadata。
- `internal/proxy/handler/openai_subscription_audit.go` already provides `auditOpenAISubscriptionLifecycle`。
- `internal/testutil/openaitest/upstream_server.go` already supports `GET /v1/models` as mock upstream。
- `internal/testutil/openaitest/guard_transport.go` blocks accidental real OpenAI host calls in tests。

## Official OpenAI Docs

- OpenAI API auth uses HTTP Bearer authentication: `Authorization: Bearer OPENAI_API_KEY`。
- `GET /v1/models` lists currently available models and returns basic model metadata such as `id`、`object`、`created`、`owned_by`。
- These docs support using `/v1/models` as a lightweight credential reachability test, provided tests route through a mock upstream and never use real credentials。

Sources:

- `https://developers.openai.com/api/reference/overview#authentication`
- `https://developers.openai.com/api/reference/resources/models/methods/list`

## Decisions

### Use `/v1/models` for test API

Decision: test API should call `GET /v1/models` with the subscription bearer after refresh-if-needed。

Rationale: It is lightweight, officially documented, and already represented in the local OpenAI upstream mock harness。

### Keep lifecycle under credential management routes

Decision: lifecycle endpoints belong under `/credentials/openai-subscription/{credential_id}/...`。

Rationale: They mutate or inspect local credential lifecycle state and must inherit proxy-admin credential-management auth/RBAC。

### Refresh API is force-refresh

Decision: explicit refresh API should bypass local freshness and force a refresh grant。

Rationale: Operator-triggered refresh is a lifecycle action; no-op on fresh token would make it impossible to verify refresh-token validity.

### Disable is local-only

Decision: disable API updates local metadata only, not remote OpenAI state。

Rationale: HO-1184 delete is local-only and no sibling issue owns remote revoke. Remote revoke/delete would require a separate explicit contract and OpenAI-side capability review。

### Return safe summaries, not raw upstream bodies

Decision: lifecycle responses should expose stable `status` and `reason_code` plus narrow safe metadata only。

Rationale: Prior HO-1182/1183/1184 artifacts treat credential lifecycle responses as secret egress boundaries。

## Open Questions

None requiring owner input at Todo planning time。
