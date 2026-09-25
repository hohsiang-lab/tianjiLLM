# Requirements Checklist: OpenAI 401 Retry, Refresh Failure, and Disable Behavior

## Completeness

- [x] Scope is limited to direct official OpenAI subscription HTTP paths.
- [x] API-key-only deployments are explicitly out of scope for refresh behavior.
- [x] Custom `api_base` and OpenAI-compatible providers are out of scope.
- [x] 401 forced refresh and same-credential retry are specified.
- [x] Retry-still-401 disable/failover behavior is specified.
- [x] Refresh failure failover behavior is specified.
- [x] All-credentials-failed reauthorization error is specified.
- [x] No-api-key-fallback invariant is specified.
- [x] Redaction requirements are specified.
- [x] Direct handler and proxy transport surfaces are both covered.

## Testability

- [x] 401 refresh retry succeeds scenario has a named failing test.
- [x] 401 refresh failure then failover scenario has a named failing test.
- [x] 401 retry still fails then disable/failover scenario has a named failing test.
- [x] All credentials fail scenario has a named failing test.
- [x] API-key 401 no-refresh regression has a named failing test.
- [x] Proxy transport has named failing tests.
- [x] Metadata redaction has a named failing test.
- [x] Verification commands are listed.

## Safety

- [x] No real OpenAI network calls are allowed.
- [x] No raw access tokens, refresh tokens, bearer strings, JWTs, fallback API keys, or encrypted credential values may be returned or persisted.
- [x] Request retry is bounded to one forced refresh and one retry per credential.
- [x] Streaming after body relay is not retried.
- [x] No schema migration is required.

## SpecKit Gate

- [x] `spec.md` complete.
- [x] `plan.md` complete.
- [x] `research.md` complete.
- [x] `data-model.md` complete.
- [x] `contracts/openai-401-refresh-failover.md` complete.
- [x] `quickstart.md` complete.
- [x] `tasks.md` complete with implementation tasks unchecked.
- [x] `analyze.md` fatal 0 / critical 0 / owner input 0.
