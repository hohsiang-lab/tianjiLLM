# Analyze: HO-1179 SpecKit Consistency

**Date**: 2026-05-07
**Feature**: Anthropic-like sticky/failover for OpenAI subscriptions
**Artifacts**: spec.md, plan.md, research.md, data-model.md, quickstart.md, contracts/openai-subscription-routing.md, checklists/requirements.md, tasks.md

## Result

- Fatal issues: 0
- Critical issues: 0
- Owner input required: 0

## Cross-artifact Checks

### Scope alignment

PASS. All artifacts limit HO-1179 to backend OpenAI subscription routing over configured credential IDs. UI, OAuth connect/callback, credential persistence, refresh internals, custom `api_base`, and production implementation are explicitly out of scope for Todo state.

### Linear requirement coverage

PASS.

- Sticky routing: covered by spec US2, plan strategy section, contract strategy section, tasks T005-T008 and T027-T033.
- Failover: covered by spec US4, plan failover semantics, contract failover section, tasks T013-T016 and T041-T048.
- Disabled exclusion: covered by spec US1, contract candidate construction, tasks T002 and T023.
- Quota/rate-limit gate where available: covered by spec US3, plan rate-limit gate, research official docs, tasks T009-T012 and T034-T040.
- Org-wide sticky scope: covered by spec FR-008, data model routing key, tasks T027-T028.
- Missing/unusable explicit errors and no fallback: covered by spec FR-004 to FR-006, contract error/no-fallback sections, tasks T003-T004 and T025.

### Repo reality alignment

PASS. The plan references current TianjiLLM files that own the behavior:

- `internal/proxy/handler/openai_subscription_resolution.go`
- `internal/proxy/handler/openai_subscription_refresh.go`
- `internal/proxy/handler/native_upstream.go`
- `internal/proxy/handler/native_format.go`
- `internal/config/validate.go`
- `internal/proxy/middleware/auth.go`

### Provider semantics alignment

PASS. Artifacts do not copy Anthropic 5h/7d/sonnet quota math into OpenAI. OpenAI gating is limited to official `x-ratelimit-*` response headers and explicit credential failures.

### Testing alignment

PASS. `tasks.md` starts with failing tests and keeps all implementation tasks unchecked. Test coverage maps to resolver, disabled exclusion, no-fallback, org-sticky, OpenAI header parsing, failover, streaming boundary, and regression behavior.

### Security/privacy alignment

PASS. Candidate identity and state are keyed by credential/account ID, not raw bearer/access/refresh tokens. Error contract requires sanitized reason codes and forbids token leakage.

## Open Questions

None for Todo scope confirmation.
