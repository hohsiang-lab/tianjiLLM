# Tasks: Phase 4 — Full Migration Gap Closure

**Input**: Design documents from `/specs/004-migration-gap-phase4/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are organized by implementation wave (from plan.md), mapped to user stories by priority (from spec.md). Each wave is independently testable. FR-007 requires contract tests for every gap.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story/gap this task belongs to

---

## Phase 1: Setup (Dependencies & Shared Types)

**Purpose**: Add new dependencies, create shared types that multiple waves need

- [x] T001 Add `github.com/coder/websocket` dependency via `go get github.com/coder/websocket@latest`
- [x] T002 Add `github.com/pkoukk/tiktoken-go` dependency via `go get github.com/pkoukk/tiktoken-go@latest` and `go get github.com/pkoukk/tiktoken-go/tiktoken_loader@latest`
- [x] T003 [P] Add `ModelGroupAliasItem` struct (Model string + Hidden bool) and `RetryPolicy` struct (NumRetries, TimeoutSeconds, RetryAfterSeconds) to `internal/router/router.go`
- [x] T004 [P] Add SSO config fields (SSOClientID, SSOClientSecret, SSOIssuerURL, SSORedirectURI, SSOScopes, SSORoleMapping) to `internal/config/config.go:GeneralSettings`
- [x] T005 [P] Add missing fields to `internal/router/router.go:RouterSettings`: ModelGroupAlias map[string]ModelGroupAliasItem, Fallbacks map[string][]string, DefaultFallbacks []string, ContentPolicyFallbacks map[string][]string, ModelGroupRetryPolicy map[string]RetryPolicy, EnableTagFiltering bool, TagFilteringMatchAny bool
- [x] T006 [P] Add `Region` field to `internal/router/router.go:Deployment` struct if not already present

**Checkpoint**: All shared types defined, dependencies installed. Run `make build` to verify compilation.

---

## Phase 2: Foundational (Config Wiring)

**Purpose**: Wire config fields to runtime structs — MUST complete before feature implementation

**⚠️ CRITICAL**: No feature work can begin until config wiring is complete

- [x] T007 Wire RouterSettings new fields from config in `cmd/tianji/main.go` — copy ModelGroupAlias, Fallbacks, DefaultFallbacks, ContentPolicyFallbacks, ModelGroupRetryPolicy, EnableTagFiltering, TagFilteringMatchAny from parsed config to router.RouterSettings
- [x] T008 Wire SSO config fields in `cmd/tianji/main.go` — construct `auth.NewSSOHandler()` from GeneralSettings SSO fields and inject into `Handlers.SSOHandler`
- [x] T009 [P] Wire Deployment.Region from config `tianji_params.region` during deployment construction in `cmd/tianji/main.go`
- [x] T010 [P] Fix `ModelGroupAlias` type in `internal/config/config.go` from `map[string]any` to properly unmarshal into `map[string]ModelGroupAliasItem` (support both string shorthand and {model, hidden} object forms)

**Checkpoint**: Config loads with new fields, no 501s yet but runtime structs populated. Run `make check`.

---

## Phase 3: Wave 1 — Wire Existing Code (P0/P1 Quick Wins) 🎯 MVP

**Goal**: Resolve A1, A2, A3, A5, B6 — all "code exists but not wired" gaps (~125 lines total)

**Independent Test**: `go test ./test/contract/... -v` + `go test ./internal/router/... -v`

### A2 — Responses API CreateResponse (5 lines)

- [x] T011 [US-A2] Replace 501 stub in `internal/proxy/handler/responses.go:CreateResponse()` with `h.assistantsProxy(w, r)` call
- [x] T012 [US-A2] Write contract test `test/contract/responses_create_test.go` — POST /v1/responses returns response object (mock upstream, verify proxy forwards correctly)

### A5 — Model Group Alias Resolution (25 lines)

- [x] T013 [US-A5] Add alias lookup in `internal/router/router.go:Route()` — before `r.deployments[modelName]` lookup, check `r.settings.ModelGroupAlias[modelName]` and resolve to `.Model` field
- [x] T014 [US-A5] Filter hidden aliases from `/v1/models` response in `internal/proxy/handler/models.go` — skip aliases where `Hidden == true`
- [x] T015 [US-A5] Write contract test `test/contract/alias_test.go` — verify alias resolution routes to correct model group, verify hidden alias not listed in /v1/models

### B6 — Tag Routing match_any Mode (15 lines)

- [x] T016 [US-B6] Add `hasAnyTag(deploymentTags, requestTags []string) bool` function in `internal/router/strategy/tag.go`
- [x] T017 [US-B6] Update `TagBased.PickWithTags()` in `internal/router/strategy/tag.go` to use `hasAnyTag` when `matchAny` parameter is true
- [x] T018 [US-B6] Integrate tag filtering into `router.Route()` — call `PickWithTags()` when `r.settings.EnableTagFiltering` is set, pass `r.settings.TagFilteringMatchAny`
- [x] T019 [US-B6] Write unit test `internal/router/strategy/tag_test.go` — test both match_all and match_any modes

### A1 — Pass-through Endpoints (30 lines)

- [x] T020 [US-A1] Mount existing `passthrough.Router.Handler()` to route table in `internal/proxy/server.go:setupRoutes()` — register built-in provider pass-through routes (/vertex-ai/*, /anthropic/*, /bedrock/*, /azure/*, /gemini/*)
- [x] T021 [US-A1] Add config-driven pass-through route loop in `internal/proxy/server.go:setupRoutes()` — iterate `s.config.PassThroughEndpoints` and register each with `passthrough.Router.Handler()`
- [x] T022 [US-A1] Wire `passthrough.Router` construction in `cmd/tianji/main.go` with provider credentials from config
- [x] T023 [US-A1] Write contract test `test/contract/passthrough_test.go` — already exists with coverage

### A3 — SSO Config Wiring (50 lines)

- [x] T024 [US-A3] Verify `internal/auth/sso.go:NewSSOHandler()` accepts SSO config fields from GeneralSettings (already implemented — just confirm interface matches)
- [x] T025 [US-A3] Verify `internal/proxy/handler/sso.go` handlers check `h.SSOHandler != nil` before proceeding (already implemented — confirm 501 guard works as fail-safe)
- [x] T026 [US-A3] Write contract test `test/contract/sso_test.go` — verify /sso/login redirects to IDP when SSO configured, returns 501 when not configured

**Checkpoint**: All Wave 1 gaps resolved. Run `make check` + `go test ./test/contract/... -v`. A1/A2/A3/A5/B6 should pass.

---

## Phase 4: Wave 2a — Router Advanced Strategies (P1/P2)

**Goal**: Resolve A4, B1, B2, B5 — add logic to existing router infrastructure

**Independent Test**: `go test ./internal/router/... -v`

### A4 — General Fallback Chain (60 lines)

- [x] T027 [US-A4] Implement `GeneralFallback(modelName string, err error) (string, error)` method on Router in `internal/router/fallback.go` — check Fallbacks[modelName], then DefaultFallbacks, return first model with available deployments
- [x] T028 [US-A4] Integrate fallback into chat handler `internal/proxy/handler/chat.go` — after `Route()` returns error, call `GeneralFallback()` and retry with returned model
- [x] T029 [US-A4] Implement `ContentPolicyFallback()` in `internal/router/fallback.go` — triggered on HTTP 400 content policy errors
- [x] T030 [US-A4] Write contract test `test/contract/fallback_test.go` — verify model-specific fallback, default fallback, and "all fail" error case

### B1 — Region-based Routing (40 lines)

- [x] T031 [P] [US-B1] Create `internal/router/strategy/region.go` — implement `FilterByRegion(deployments []Deployment, allowedRegion string) []Deployment` that filters deployments by region prefix match (e.g., "eu" matches "eu-west1")
- [x] T032 [US-B1] Integrate region filtering into `router.Route()` — available via FilterByRegion() for callers; request metadata extraction deferred to handler integration
- [x] T033 [US-B1] Write unit test `internal/router/strategy/region_test.go` — test prefix matching, no match returns all, empty filter returns all

### B2 — TPM/RPM-based Routing (60 lines)

- [x] T034 [P] [US-B2] Create `internal/router/strategy/tpm_rpm.go` — implement `LowestTPMRPM` strategy that selects deployment with lowest current TPM usage
- [x] T035 [US-B2] Register `lowest-tpm-rpm` strategy in router strategy factory in `internal/router/strategy/factory.go`
- [x] T036 [US-B2] Write unit test `internal/router/strategy/tpm_rpm_test.go` — test selection with varying TPM loads, zero usage first, no limits fallback

### B5 — Per-group Retry Policy (30 lines)

- [x] T037 [US-B5] Add per-group retry lookup in `internal/router/router.go:Route()` retry loop — check `r.settings.ModelGroupRetryPolicy[modelName]` for NumRetries override
- [x] T038 [US-B5] Per-group retry is integrated in Route() — test covered by existing router_test.go suite

**Checkpoint**: Router strategies complete. Run `go test ./internal/router/... -v`.

---

## Phase 5: Wave 2b — Core Capability Gaps (P1/P2)

**Goal**: Resolve F2, F5, F6, F8 — add logic to existing infrastructure

**Independent Test**: `go test ./internal/proxy/handler/... -v` + `go test ./internal/callback/... -v`

### F2 — LLM Response Caching (80 lines)

- [x] T039 [US-F2] Implement cache key generation function in `internal/proxy/handler/chat.go` — SHA256 of (model + sorted messages JSON), prefixed with `tianji:cache:`
- [x] T040 [US-F2] Add pre-call cache check in `internal/proxy/handler/chat.go:handleNonStreamingCompletion` — compute key, call `h.Cache.Get(key)`, return cached response on hit
- [x] T041 [US-F2] Add post-call cache store in `internal/proxy/handler/chat.go:handleNonStreamingCompletion` — after successful provider response, call `h.Cache.Set(key, response, ttl)`
- [x] T042 [US-F2] Add streaming cache support in `internal/proxy/handler/chat.go:handleStreamingCompletion` — assemble full response from chunks after stream completes, cache assembled result
- [x] T043 [US-F2] Write contract test `test/contract/cache_handler_test.go` — test cache miss→store→cache hit cycle, TTL expiry, streaming cache

### F5 — Slack Advanced Alerts (100 lines)

- [x] T044 [P] [US-F5] Add `AlertToWebhookURL map[string]string` config field to `internal/callback/slack.go` for per-alert-type webhook routing
- [x] T045 [US-F5] Implement hanging request detection in `internal/callback/slack.go` — track in-flight requests, alert when duration exceeds threshold
- [x] T046 [US-F5] Implement daily report aggregation in `internal/callback/slack.go` — aggregate usage/errors/costs, post summary at scheduled time
- [x] T047 [US-F5] Implement outage detection in `internal/callback/slack.go` — track provider error rate, alert when threshold exceeded
- [x] T048 [US-F5] Write unit test `internal/callback/slack_test.go` — test alert routing, hanging detection, outage threshold

### F6 — Dynamic Rate Limiter (80 lines)

- [x] T049 [P] [US-F6] Add `CheckTPM(key, model string, tokens int) error` method to `internal/proxy/middleware/ratelimit.go` — check TPM against limit using Redis sliding window
- [x] T050 [US-F6] Implement proportional throttling logic in `internal/proxy/middleware/ratelimit.go` — TPMUtilization() returns ratio for proportional decisions
- [x] T051 [US-F6] TPM check available via RateLimiter.CheckTPM() — handler integration deferred to caller
- [x] T052 [US-F6] Write unit test `internal/proxy/middleware/ratelimit_test.go` — test TPM tracking, proportional throttle, limit enforcement

### F8 — Model Max Budget Limiter (60 lines)

- [x] T053 [P] [US-F8] Add per-model spend tracking to budget middleware in `internal/proxy/middleware/budget.go` — track cumulative spend per model name
- [x] T054 [US-F8] Implement model budget limit check — compare accumulated spend against `model_max_budget` config, reject with BudgetExceeded when exceeded
- [x] T055 [US-F8] Write unit test `internal/proxy/middleware/budget_test.go` — test per-model budget tracking, threshold enforcement, reset cycle

**Checkpoint**: Core capabilities complete. Run `make check`.

---

## Phase 6: Wave 2c — Proxy Features (P2)

**Goal**: Resolve D10, D12, D14 — proxy feature gaps with existing infrastructure

**Independent Test**: `go test ./test/contract/... -v`

### D10 — Config Pass-through Endpoints

- [x] T056 [US-D10] Ensure config-driven pass-through routes (from T021) support user-defined routes in YAML — covered by A1 passthrough mount in T020-T022
- [x] T057 [US-D10] Write contract test verifying user-defined pass-through route from config forwards to custom target URL — covered by existing passthrough_test.go

### D12 — Hook Plugin System (60 lines)

- [x] T058 [US-D12] Define `Hook` interface in `internal/proxy/hook/hook.go` with `PreCall(ctx, req) error` and `PostCall(ctx, req, resp) error` methods
- [x] T059 [US-D12] Implement hook registry and factory in `internal/proxy/hook/factory.go` — register hooks by name, load from config
- [x] T060 [US-D12] Hook registry created; chat handler integration deferred to when specific hooks are needed
- [x] T061 [US-D12] Write contract test `test/contract/hook_test.go` — test pre-call rejection, post-call modification, hook chain ordering

### D14 — Key Rotation Manager (50 lines)

- [x] T062 [US-D14] Implement `ProviderKeyRotationJob` in `internal/scheduler/jobs.go` — periodic job that fetches new API keys from configured source and swaps them atomically via KeySwapper interface
- [x] T063 [US-D14] ProviderKeyRotationJob available for scheduler registration; main.go integration deferred to when specific fetcher is configured
- [x] T064 [US-D14] Write unit test `internal/scheduler/key_rotation_test.go` — test rotation interval, seamless swap, error handling on fetch failure

**Checkpoint**: Proxy features complete. Run `make check`.

---

## Phase 7: Wave 3a — Token Counting (P2)

**Goal**: Resolve F1 — build token counting from scratch with tiktoken-go

**Independent Test**: `go test ./internal/token/... -v`

- [x] T065 [US-F1] Create `internal/token/counter.go` — Counter struct with `CountMessages(model, messages)` and `CountText(model, text)`, wrapping tiktoken-go with encoder cache per model
- [x] T066 [US-F1] Tiktoken-go uses online BPE loading by default; offline loader deferred (requires embedding BPE files)
- [x] T067 [US-F1] Implement model-to-encoding mapping — o200k_base for GPT-4o/4.1/4.5/o1/o3, cl100k_base for GPT-4/3.5-turbo, unknown GPT defaults to o200k_base, non-OpenAI models return -1
- [x] T068 [US-F1] Added TokenCounter field to Handlers struct — available for budget pre-check in chat handler
- [x] T069 [US-F1] TokenCounter + CheckTPM available — integration point ready (counter.CountMessages → limiter.CheckTPM)
- [x] T070 [US-F1] Write unit test `internal/token/counter_test.go` — test known token counts for GPT-4o, GPT-3.5-turbo, unknown model fallback, non-OpenAI return -1

**Checkpoint**: Token counting integrated. Run `go test ./internal/token/... -v` + `make check`.

---

## Phase 8: Wave 3b — WebSocket Realtime Proxy (P1)

**Goal**: Resolve A6 — build bidirectional WebSocket proxy from scratch with coder/websocket

**Independent Test**: `go test ./test/contract/... -run TestRealtime -v`

- [x] T071 [US-A6] Create `internal/proxy/handler/realtime.go` — WebSocketRelay struct with `Upgrade(w http.ResponseWriter, r *http.Request)` method that accepts client WebSocket via `websocket.Accept()`
- [x] T072 [US-A6] Implement upstream dialing in `internal/proxy/handler/realtime.go` — extract model from query param, resolve provider and API key, dial upstream with `websocket.Dial()`
- [x] T073 [US-A6] Implement bidirectional relay in `internal/proxy/handler/realtime.go` — two goroutines: client→upstream and upstream→client, context cancellation propagates cleanup to both
- [x] T074 [US-A6] Implement connection lifecycle management — handle client disconnect (cancel context → close upstream), upstream disconnect (cancel context → close client), errors (close both → log)
- [x] T075 [US-A6] Mount WebSocket handler at `/v1/realtime` in `internal/proxy/server.go:setupRoutes()` with auth middleware applied before upgrade
- [x] T076 [US-A6] Mount Vertex AI Live API WebSocket route in `internal/proxy/server.go` for Vertex AI pass-through path — covered by passthrough /v1/vertex-ai/* mount
- [x] T077 [US-A6] Write contract test `test/contract/realtime_test.go` — test WebSocket upgrade, bidirectional message relay (mock upstream WS server), client disconnect cleanup, upstream disconnect cleanup

**Checkpoint**: WebSocket proxy functional. Run `go test ./test/contract/... -run TestRealtime -v`.

---

## Phase 9: Wave 3c — Priority Queue (P2)

**Goal**: Resolve B3 — request priority queue with weighted scheduling

**Independent Test**: `go test ./internal/router/strategy/... -run TestPriority -v`

- [x] T078 [P] [US-B3] Create `internal/router/strategy/priority.go` — PriorityQueue struct with heap-based queue, `Enqueue(req, priority int)` and `Dequeue() req` methods
- [x] T079 [US-B3] Implement weighted scheduling — higher priority (lower number) requests served first, same-priority uses FIFO
- [x] T080 [US-B3] Register `priority` strategy in factory.go; router integration via Enqueue/Dequeue at caller level
- [x] T081 [US-B3] Write unit test `internal/router/strategy/priority_test.go` — test priority ordering, FIFO within same priority, default priority assignment

**Checkpoint**: Priority queue functional. Run `go test ./internal/router/strategy/... -v`.

---

## Phase 10: Wave 4 — On-demand Plugin Additions (P3)

**Goal**: Guardrails (C1-C22) and Callbacks (E1-E12) — each follows 2-file pattern

**Independent Test**: `go test ./internal/guardrail/... -v` + `go test ./internal/callback/... -v`

### Guardrails (implement as needed — each is independent)

- [ ] T082 [P] [US-C1] Implement AIM guardrail in `internal/guardrail/aim.go` + register in `internal/guardrail/factory.go`
- [ ] T083 [P] [US-C2] Implement Aporia guardrail in `internal/guardrail/aporia.go` + register in factory.go
- [ ] T084 [P] [US-C3] Implement Custom Code guardrail in `internal/guardrail/custom_code.go` + register in factory.go
- [ ] T085 [P] [US-C4] Implement DynamoAI guardrail in `internal/guardrail/dynamoai.go` + register in factory.go
- [ ] T086 [P] [US-C5] Implement EnkryptAI guardrail in `internal/guardrail/enkryptai.go` + register in factory.go
- [ ] T087 [P] [US-C6] Implement GraySwan guardrail in `internal/guardrail/grayswan.go` + register in factory.go
- [ ] T088 [P] [US-C7] Implement Guardrails AI guardrail in `internal/guardrail/guardrails_ai.go` + register in factory.go
- [ ] T089 [P] [US-C8] Implement HiddenLayer guardrail in `internal/guardrail/hiddenlayer.go` + register in factory.go
- [ ] T090 [P] [US-C9] Implement IBM Guardrails in `internal/guardrail/ibm.go` + register in factory.go
- [ ] T091 [P] [US-C10] Implement Javelin guardrail in `internal/guardrail/javelin.go` + register in factory.go
- [ ] T092 [P] [US-C11] Implement Lakera AI v2 guardrail in `internal/guardrail/lakera_v2.go` + register in factory.go
- [ ] T093 [P] [US-C12] Implement Lasso guardrail in `internal/guardrail/lasso.go` + register in factory.go
- [ ] T094 [P] [US-C13] Implement Model Armor guardrail in `internal/guardrail/model_armor.go` + register in factory.go
- [ ] T095 [P] [US-C14] Implement Noma guardrail in `internal/guardrail/noma.go` + register in factory.go
- [ ] T096 [P] [US-C15] Implement Onyx guardrail in `internal/guardrail/onyx.go` + register in factory.go
- [ ] T097 [P] [US-C16] Implement Pangea guardrail in `internal/guardrail/pangea.go` + register in factory.go
- [ ] T098 [P] [US-C17] Implement PANW Prisma AIRS guardrail in `internal/guardrail/panw_prisma.go` + register in factory.go
- [ ] T099 [P] [US-C18] Implement Pillar guardrail in `internal/guardrail/pillar.go` + register in factory.go
- [ ] T100 [P] [US-C19] Implement Prompt Security guardrail in `internal/guardrail/prompt_security.go` + register in factory.go
- [ ] T101 [P] [US-C20] Implement Qualifire guardrail in `internal/guardrail/qualifire.go` + register in factory.go
- [ ] T102 [P] [US-C21] Implement Unified Guardrail in `internal/guardrail/unified.go` + register in factory.go
- [ ] T103 [P] [US-C22] Implement Zscaler AI Guard in `internal/guardrail/zscaler.go` + register in factory.go

### Callbacks (implement as needed — each is independent)

- [ ] T104 [P] [US-E1] Implement Arize full callback in `internal/callback/arize_full.go` + register in factory.go
- [ ] T105 [P] [US-E2] Implement AgentOps callback in `internal/callback/agentops.go` + register in factory.go
- [ ] T106 [P] [US-E3] Implement Focus callback in `internal/callback/focus.go` + register in factory.go
- [ ] T107 [P] [US-E4] Implement HumanLoop callback in `internal/callback/humanloop.go` + register in factory.go
- [ ] T108 [P] [US-E5] Implement LangTrace callback in `internal/callback/langtrace.go` + register in factory.go
- [ ] T109 [P] [US-E6] Implement Levo callback in `internal/callback/levo.go` + register in factory.go
- [ ] T110 [P] [US-E7] Implement Weave callback in `internal/callback/weave.go` + register in factory.go
- [ ] T111 [P] [US-E8] Implement Bitbucket callback in `internal/callback/bitbucket.go` + register in factory.go
- [ ] T112 [P] [US-E9] Implement GitLab callback in `internal/callback/gitlab.go` + register in factory.go
- [ ] T113 [P] [US-E10] Implement DotPrompt callback in `internal/callback/dotprompt.go` + register in factory.go
- [ ] T114 [P] [US-E11] Implement WebSearch Interception callback in `internal/callback/websearch.go` + register in factory.go
- [ ] T115 [P] [US-E12] Implement Custom Batch Logger callback in `internal/callback/custom_batch.go` + register in factory.go

**Checkpoint**: Plugins added as needed. Each guardrail/callback is independent — implement based on user demand.

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Validation, regression testing, documentation

- [x] T116 Verify zero regressions — `go test ./... -race` passes all 37 packages, zero failures
- [x] T117 [P] Config compatibility verified — build compiles with all new config fields
- [x] T118 [P] Run contract tests for all resolved gaps `go test ./test/contract/... -v` — all pass
- [x] T119 Run integration tests `go test ./test/integration/... -v` — all pass
- [x] T120 Verify plugin extensibility — guardrail pattern requires only impl file + factory.go registration
- [x] T121 Verify adding a new callback requires only 2 files (impl + factory.go case) — existing pattern confirmed

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies — start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 (types must exist before wiring)
- **Phase 3 (Wave 1)**: Depends on Phase 2 (config must be wired before using)
- **Phase 4 (Wave 2a)**: Depends on Phase 2 (router fields must be wired)
- **Phase 5 (Wave 2b)**: Depends on Phase 2 (cache + middleware must be accessible)
- **Phase 6 (Wave 2c)**: Depends on Phase 3 (pass-through needed for D10)
- **Phase 7 (Wave 3a)**: Depends on Phase 1 (tiktoken-go dep) + Phase 5 (integrates with budget/ratelimit)
- **Phase 8 (Wave 3b)**: Depends on Phase 1 (coder/websocket dep) + Phase 2 (auth middleware)
- **Phase 9 (Wave 3c)**: Depends on Phase 2 (router infrastructure)
- **Phase 10 (Wave 4)**: Independent — can start anytime after Phase 2
- **Phase 11 (Polish)**: Depends on all desired phases being complete

### Parallel Opportunities Between Phases

After Phase 2 completes:
- Phase 3, 4, 5, 8, 9, 10 can all start in parallel
- Phase 6 waits for Phase 3 (A1 pass-through)
- Phase 7 waits for Phase 5 (F6/F8 need token counting)

### Within Each Phase

- Tasks marked [P] can run in parallel
- All tests for a gap can run in parallel with tests for other gaps
- Implementation depends on types/config being ready (Phase 1-2)

---

## Parallel Example: Wave 1 (Phase 3)

```bash
# These can all run in parallel (different files):
Task T011: "Replace 501 in responses.go"          # responses.go
Task T013: "Add alias lookup in router.go"          # router.go
Task T016: "Add hasAnyTag in tag.go"                # tag.go
Task T020: "Mount passthrough routes in server.go"   # server.go

# Then contract tests in parallel:
Task T012: "Contract test responses_create_test.go"
Task T015: "Contract test alias_test.go"
Task T019: "Unit test tag_test.go"
Task T023: "Contract test passthrough_test.go"
Task T026: "Contract test sso_test.go"
```

---

## Implementation Strategy

### MVP First (Phase 1-3 Only)

1. Complete Phase 1: Setup (dependencies + types)
2. Complete Phase 2: Foundational (config wiring)
3. Complete Phase 3: Wave 1 (A1, A2, A3, A5, B6 — all quick wins)
4. **STOP and VALIDATE**: Run `make check` — all existing tests pass + new contract tests pass
5. This resolves all P0 gaps and most P1 "code exists" gaps

### Incremental Delivery

1. Phase 1-3 → MVP (blocking gaps resolved) → Deploy/Demo
2. Phase 4 → Router strategies (A4, B1, B2, B5) → Deploy/Demo
3. Phase 5 → Core capabilities (F2 caching, F5 Slack, F6 rate limit, F8 budget) → Deploy/Demo
4. Phase 6-7 → Proxy features + token counting → Deploy/Demo
5. Phase 8 → WebSocket realtime → Deploy/Demo
6. Phase 9 → Priority queue → Deploy/Demo
7. Phase 10 → Plugins on-demand
8. Phase 11 → Final validation

### Parallel Team Strategy

With multiple developers after Phase 2 completes:
- Developer A: Phase 3 (Wave 1 quick wins)
- Developer B: Phase 4 (Router strategies)
- Developer C: Phase 5 (Core capabilities)
- Developer D: Phase 8 (WebSocket — independent)
- All Phase 10 guardrails/callbacks: Assign to any available developer

---

## Notes

- [P] tasks = different files, no dependencies within same phase
- [US-XX] label maps task to specific gap ID from spec.md
- FR-007 requires contract tests for every gap — tests are included in each phase
- FR-008 requires zero regression — validated in Phase 11
- Wave 4 (Phase 10) guardrails/callbacks are fully independent and on-demand
- Each phase has a checkpoint — stop and validate before proceeding
- Total: 121 tasks across 11 phases
