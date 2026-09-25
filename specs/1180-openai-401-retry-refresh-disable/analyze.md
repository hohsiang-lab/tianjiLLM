# Analyze: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Cross-Artifact Consistency

### Scope Alignment

PASS. `spec.md`, `plan.md`, `tasks.md`, and the contract all scope HO-1180 to direct official OpenAI subscription HTTP paths. They exclude UI, automatic OAuth reconnect, custom `api_base`, OpenAI-compatible providers, API-key-only deployments, and Codex app-server transport.

### Dependency Alignment

PASS. The plan depends on HO-1167, HO-1176, HO-1177, HO-1178, HO-1179, HO-1183, and HO-1190, which matches the current `origin/main` repo shape after HO-1179.

### Retry Semantics

PASS. The artifacts consistently say 401 must not be added to generic retry status. It is a subscription-aware forced-refresh path with one refresh and one retry per credential.

### Failover Semantics

PASS. The artifacts consistently require:

- forced refresh failure tries next credential
- post-refresh 401 disables/marks credential unavailable and tries next credential
- request fails only when all configured credentials fail
- no `api_key` fallback when subscription IDs exist

### Redaction Semantics

PASS. The artifacts consistently require HO-1183 redaction for upstream 401 body, refresh failures, persisted metadata, aggregate errors, logs, callbacks, and audit/error sinks.

### Test-First Gate

PASS. `tasks.md` starts with failing direct-handler, proxy-transport, forced-refresh, metadata, and API-key regression tests before implementation tasks.

## Open Questions

None requiring owner input.

Implementation can choose exact helper names and whether auth-failed metadata is written through an existing failure helper or a narrow new helper, as long as tests prove `status=disabled`, `disabled_reason=auth_failed_after_refresh`, and redaction.

## Risks Checked

- **Risk**: 401 generic retry causes stale-token loops.
  **Status**: Mitigated by explicit requirement not to add 401 to `openAISubscriptionRetryableStatus`.

- **Risk**: API-key routes accidentally refresh.
  **Status**: Mitigated by branching only when subscription candidate `credentialID` is non-empty.

- **Risk**: Direct handlers and proxy transport diverge.
  **Status**: Mitigated by test tasks covering both `doOpenAISubscriptionProviderRequest` and `openAISubscriptionFailoverTransport`.

- **Risk**: Sensitive upstream auth errors leak.
  **Status**: Mitigated by metadata/redaction tasks and explicit negative assertions.

## Gate Decision

Todo -> Waiting is allowed after:

1. SpecKit artifacts are committed and pushed.
2. Draft PR exists.
3. `git diff --check origin/main...HEAD` passes.
4. Linear is updated to Waiting with PR/issue links.
