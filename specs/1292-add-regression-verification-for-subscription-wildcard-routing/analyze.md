# Analyze: HO-1292 Subscription wildcard routing regression verification

## Result

- Fatal: 0
- Critical: 0
- Owner input required: 0
- Governance blocker: production/test implementation cannot start until Linear moves to `In Progress`.

## Linear Scope Coverage

| Linear explicit behavior | Spec coverage | Plan/tasks coverage |
| --- | --- | --- |
| Subscription-backed wildcard routes through ChatGPT Codex backend | US1, FR-001..FR-003, SC-001..SC-002 | T005-T007, T014 |
| Normal API-key OpenAI routes unaffected | US4, FR-009, SC-004 | T012, T017 |
| Prevent direct subscription credential missing-scope route to `api.openai.com/v1/chat/completions` | US2, FR-004, SC-001 | T007-T008, T015 |
| Cover 429/quota and auth diagnostics from Codex transport | US3, FR-005..FR-008, SC-003 | T009-T011, T016 |
| Include deployment verification note for OpenClaw `openai/*` rollout | US5, FR-013, SC-006 | T019-T022 |
| Verification does not require printing secrets | FR-010..FR-011, Secret Safety checklist | T020-T022, T028 |

## Issue Explicit Behavior Map

| Issue noun / verb | Artifact mapping | Alignment result |
| --- | --- | --- |
| `subscription-backed wildcard` | `spec.md` Summary/US1, `data-model.md` `SubscriptionWildcardRouteFixture`, `contracts/` Subscription Codex Wildcard Request | Covered |
| `ChatGPT Codex backend` | `spec.md` FR-002, `plan.md` Architecture, `contracts/` expected path | Covered |
| `normal API-key OpenAI routes are unaffected` | `spec.md` US4/FR-009, `contracts/` API-Key Wildcard Contract, `tasks.md` T012/T017 | Covered |
| `no calls to api.openai.com/v1/chat/completions with ChatGPT OAuth token` | `spec.md` US2/FR-004, `data-model.md` PlatformWrongRouteMock, `tasks.md` T007-T008 | Covered |
| `429/quota and auth diagnostics` | `spec.md` US3/FR-005..FR-008, `plan.md` Diagnostics matrix, `contracts/` Diagnostics Contract | Covered |
| `deployment verification note for openai/* after rollout` | `quickstart.md` OpenClaw section, `tasks.md` Phase 3 | Covered |
| `do not print tokens or secrets` | `quickstart.md` Secret Safety, `checklists/requirements.md`, `tasks.md` T028 | Covered |

## Cross-Artifact Consistency

- `spec.md` defines HO-1292 as regression verification, not a new runtime feature.
- `plan.md` places primary tests at handler route boundary because provider unit tests cannot prove wildcard selection.
- `tasks.md` keeps implementation/test tasks unchecked and blocked by Linear `In Progress`.
- `research.md` records repo evidence, official OpenAI docs, and GitHub prior-art lookup.
- `data-model.md` defines local test fixtures and secret sentinels.
- `quickstart.md` gives future commands and secret-safe deployment verification.
- `contracts/` defines expected subscription and API-key wildcard behavior.

## Out-of-Scope Challenge

- Models UI selector changes are excluded because HO-1291 already owns `openai_subscription_transport` create/edit persistence.
- Credential OAuth lifecycle is excluded because the regression concerns route selection after credentials resolve.
- Real OpenAI/ChatGPT calls are excluded because local mocks prove routing and avoid secret/flake risk.
- Streaming support is excluded unless the wildcard route accidentally bypasses the existing tested `unsupported_streaming` behavior.

## E2E Coverage Map

- PR layer: TianjiLLM routing/transport regression verification.
- Runtime boundary: `/v1/chat/completions` handler -> wildcard config lookup -> subscription credential resolver -> transport selection -> mocked Codex or Platform upstream.
- Test command: `go test ./internal/proxy/handler -run 'TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard|TestChatGPTCodexBackendTransport' -count=1`.
- CI evidence: full PR workflow after implementation.
- Gap/follow-up: real OpenClaw production validation remains deployment evidence, not automated CI, and must avoid secrets.

## Final Scope Confirmation

HO-1292 Todo planning is internally consistent and needs no owner input. Move to `Waiting` after docs-only diff passes, draft PR exists, and the thread is notified with Linear and PR links.
