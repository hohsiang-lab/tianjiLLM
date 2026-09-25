# Analyze: HO-1181 SpecKit Consistency

**Date**: 2026-05-07
**Feature**: OpenAI quota/rate-limit header parser and store integration
**Artifacts**: spec.md, plan.md, research.md, data-model.md, quickstart.md, contracts/openai-quota-rate-limit-store.md, checklists/requirements.md, tasks.md

## Result

- Fatal issues: 0
- Critical issues: 0
- Owner input required: 0

## Cross-artifact Checks

### Scope Alignment

PASS. All artifacts scope HO-1181 to backend official OpenAI subscription quota/rate-limit parsing, normalized store integration, and routing consumption. UI, OAuth connect/callback, credential persistence schema, token refresh lifecycle, billing estimates, and custom OpenAI-compatible providers are out of scope.

### Linear Requirement Coverage

PASS.

- Parse OpenAI quota/rate-limit headers: covered by spec US1/US2, plan parser phases, and tasks T001-T010/T020-T026.
- Normalize into existing store shape where possible: covered by spec US3, data model, contract, and tasks T012-T019.
- Update sticky/gate selection data: covered by spec US3, contract routing consumption, and tasks T027-T034.
- Missing headers degrade safely: covered by spec US4 and tasks T004/T005/T042.
- Do not estimate billing/quota locally: covered by scope, FR-013, and research decisions.

### Repo Reality Alignment

PASS. The plan references current TianjiLLM files that own this behavior:

- `internal/proxy/handler/openai_subscription_ratelimit.go`
- `internal/proxy/handler/openai_subscription_routing.go`
- `internal/proxy/handler/openai_subscription_provider_retry.go`
- `internal/callback/ratelimit_store.go`
- `internal/testutil/openaitest/upstream_server.go`

### External Evidence Alignment

PASS. Official OpenAI docs define six rate-limit response headers and duration reset examples. grep-app found Go ecosystem usage of the same header names. Context7 failure is recorded as a tooling limitation and not counted as supporting evidence.

### Testing Alignment

PASS. `tasks.md` starts with failing parser/mock/store/routing tests before production tasks. Tests cover official headers, mock allowed/rejected/reset/utilization cases, store normalization, gate, sticky, lowest-utilization, response attempts, redaction, and no-real-OpenAI constraints.

### Security Alignment

PASS. Artifacts consistently require credential/account ID keys and forbid raw access tokens, refresh tokens, bearer values, JWTs, encrypted credential values, and full sensitive upstream payloads in parser/store/log/error outputs.

## Open Questions

None requiring owner input.

Implementation can choose whether the normalized store lives in `internal/callback` or remains handler-owned behind an interface, as long as gate, sticky, and lowest-utilization all read one normalized state source and tests prove no handler-private divergence.

## Gate Decision

Todo -> Waiting is allowed after:

1. SpecKit artifacts are committed and pushed.
2. Draft PR exists.
3. `git diff --check origin/main...HEAD` passes.
4. Linear is updated to Waiting with PR/issue links.
