# Analyze: HO-1319 Codex CLI `/v1/responses`

## Coverage Matrix

| Requirement | Spec | Plan | Tasks | Status |
| --- | --- | --- | --- | --- |
| Primary `/v1/responses` transport works without fallback loop | US1, FR-001..FR-005, SC-001..SC-003 | Primary Transport | T002-T004, T010-T014, T027 | Covered |
| Responses WebSocket first-frame protocol coverage | US1, FR-002..FR-004, SC-002 | RED Coverage / Primary Transport | T002-T004, T011-T012 | Covered |
| DB-managed `openai/*` wildcard for `openai/gpt-5.5` | US2, FR-006..FR-007, SC-005 | Route Resolution | T005-T006, T015-T018 | Covered |
| Credential health/refresh | US3, FR-008..FR-009, SC-004 | Credential Health | T007-T008, T019-T022 | Covered |
| Chat-completions no regression | US4, FR-010 | Test Plan | T009, T031 | Covered |
| Issue-owned `/v1/responses` regression | FR-011 | RED Coverage | T002-T008 | Covered |
| Live/local full-chain verification | FR-012 | Rollout Notes | T023-T028 | Covered |

## Repo Reality Alignment

- `internal/proxy/server.go` currently registers `POST /responses`, not `GET /responses`.
- `internal/proxy/handler/realtime.go` contains websocket relay logic for Realtime, not `/v1/responses`.
- `internal/proxy/handler/responses.go` delegates to `openAIEndpointProxy`.
- `internal/proxy/handler/assistants.go` currently forces subscription `/v1/responses` route resolution through `openAISubscriptionTransportDirectHTTP`.
- `test/e2e/codex_subscription_route_test.go` covers chat-completions, not `/v1/responses`.

## Consistency Checks

- No production code is requested in Todo phase.
- Tasks are unchecked because implementation has not started.
- Acceptance explicitly rejects silent fallback success.
- Acceptance now explicitly rejects route-only/non-405 websocket fixes without first-frame `response.create` coverage.
- Plan keeps generic OpenAI `/v1/responses` behavior guarded.
- Credential tests include redaction.

## Fatal Issues

0

## Critical Issues

0

## Owner Input Required

0 for Todo scope.

Owner confirmation will be required later only if implementation proposes any fallback-based solution.
