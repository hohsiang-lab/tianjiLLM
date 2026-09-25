# Tasks: HO-2370 Mirror Codex subscription model catalog

**Input**: Design documents from `specs/HO-2370-mirror-codex-subscription-catalog/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/codex-subscription-catalog.md`, `quickstart.md`

**Tests**: Failing tests are MANDATORY. Write tests first, run them to confirm they compile and fail, then implement.

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Create upstream Sol/Terra/Luna catalog fixtures in `internal/provider/chatgptcodex/testdata/codex_catalog_sol_terra_luna.json`
- [X] T002 [P] Add catalog test helper structs/builders in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T003 [P] Add provider catalog parser test harness in `internal/provider/chatgptcodex/catalog_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T004 Define sanitized upstream catalog structs and parser contract in `internal/provider/chatgptcodex/catalog.go`
- [X] T005 Define handler catalog cache/result interfaces in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T006 Use the existing `cache.Cache` credential-keyed entry for persistent last-known-good catalog storage; no SQL required

---

## Phase 3: User Story 1 - Subscription aliases use upstream Codex metadata (Priority: P1)

**Goal**: Routable subscription aliases expose upstream-derived Codex metadata.

**Independent Test**: Fixture-backed Sol/Terra/Luna aliases return upstream context, truncation, reasoning, modality, and tier fields.

### Tests for User Story 1

- [X] T007 [P] [US1] Write failing `TestCodexCatalogClientParsesReasoningModalitiesAndTiers` in `internal/provider/chatgptcodex/catalog_test.go`
- [X] T008 [P] [US1] Write failing `TestListModelsSubscriptionCatalogEnrichesSolTerraLuna` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T009 [P] [US1] Write failing `TestListModelsSubscriptionCatalogKeepsRuntimeRouteBoundary` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T010 [P] [US1] Write failing `TestCodexModelInfoFromConfigPreservesExplicitUpstreamNullAutoCompact` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T011 [US1] Run `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|SubscriptionCatalog|AutoCompact' -count=1` and record expected failures

### Implementation for User Story 1

- [X] T012 [US1] Implement upstream catalog parsing in `internal/provider/chatgptcodex/catalog.go`
- [X] T013 [US1] Implement catalog-to-alias enrichment in `internal/proxy/handler/list_models_catalog.go`
- [X] T014 [US1] Ensure explicit upstream null auto-compact is preserved in `internal/proxy/handler/list_models_catalog.go`
- [X] T015 [US1] Ensure future reasoning efforts and text/image modalities pass through in `internal/proxy/handler/list_models_catalog.go`

---

## Phase 4: User Story 2 - Catalog refresh is cached and credential-aware (Priority: P1)

**Goal**: Catalog refreshes are scoped, cached, and invalidated safely.

**Independent Test**: Multiple calls and multiple credentials prove no per-request fetch and no entitlement leakage.

### Tests for User Story 2

- [X] T016 [P] [US2] Write failing `TestOpenAISubscriptionCodexCatalogUsesFreshCacheWithoutRefetch` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T017 [P] [US2] Write failing `TestOpenAISubscriptionCodexCatalogSeparatesCredentialEntitlements` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T018 [P] [US2] Write failing `TestOpenAISubscriptionCodexCatalogInvalidatesOnCredentialRefresh` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T019 [P] [US2] Write failing `TestOpenAISubscriptionCodexCatalogCoalescesConcurrentRefreshes` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T020 [US2] Run `go test ./internal/proxy/handler -run 'OpenAISubscriptionCodexCatalog' -count=1` (red before implementation; green after)

### Implementation for User Story 2

- [X] T021 [US2] Implement credential-aware catalog cache and TTL logic in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T022 [US2] Implement upstream refresh using resolved subscription credentials in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T023 [US2] Implement ordered multi-credential selection without cross-credential cache sharing in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T024 [US2] Wire credential refresh/disable/replacement invalidation in the existing credential update paths
- [X] T024a [US2] Regression-test successful credential refresh/update invalidates in-memory and persistent catalog cache in `internal/proxy/handler/credential_test.go`

---

## Phase 5: User Story 3 - Last-known-good fallback is safe (Priority: P2)

**Goal**: Transient failures degrade visibly and safely.

**Independent Test**: Refresh failure serves last-known-good; empty cache falls back static; no secrets leak.

### Tests for User Story 3

- [X] T025 [P] [US3] Write failing `TestOpenAISubscriptionCodexCatalogServesLastKnownGoodOnRefreshFailure` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T026 [P] [US3] Write failing `TestListModelsSubscriptionCatalogFallsBackToStaticMetadataWithoutCache` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T027 [P] [US3] Write failing `TestOpenAISubscriptionCodexCatalogRedactsSecretsFromCacheAndLogs` in `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- [X] T028 [US3] Run `go test ./internal/proxy/handler -run 'LastKnownGood|StaticMetadata|RedactsSecrets' -count=1` (covered by scoped catalog/static test run)

### Implementation for User Story 3

- [X] T029 [US3] Implement last-known-good hydration/persistence in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T030 [US3] Implement degraded refresh result/log fields without secret material in `internal/proxy/handler/openai_subscription_codex_catalog.go`
- [X] T031 [US3] Preserve conservative static fallback for no-upstream/no-cache states in `internal/proxy/handler/list_models_catalog.go`

---

## Phase 6: User Story 4 - HO-1331 response contract remains intact (Priority: P2)

**Goal**: Preserve OpenAI-compatible `data[]` and Codex-compatible `models[]` from one runtime source.

**Independent Test**: Existing and new list-model tests pass with enriched metadata.

### Tests for User Story 4

- [X] T032 [P] [US4] Keep/update `TestListModelsPreservesOpenAICompatibleShape` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T033 [P] [US4] Keep/update `TestListModelsHiddenAliasesExcludedFromBothShapes` in `internal/proxy/handler/list_models_codex_test.go`
- [X] T034 [P] [US4] Keep/update `TestRuntimeModelSource_ListModelsIncludesDBManagedModel` in `internal/proxy/handler/runtime_model_source_test.go`

### Implementation for User Story 4

- [X] T035 [US4] Refactor list-model assembly so `data[]` and `models[]` share the same filtered runtime model slice in `internal/proxy/handler/list_models_catalog.go`
- [X] T036 [US4] Run `go test ./internal/proxy/handler -run 'ListModels|RuntimeModelSource' -count=1`

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T037 [P] Update `quickstart.md` with actual test commands and live verification notes after implementation
- [X] T038 Run `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|ListModels|RuntimeModelSource' -count=1`
- [X] T039 Run `go test ./internal/... -count=1` or document any unrelated failures
- [X] T040 Run `git diff --check`

## Review Repair Checkpoint

- [X] T041 [Review repair] Make lazy `CodexCatalogCache` initialization race-safe with a handler mutex; add `TestOpenAISubscriptionCodexCatalogInitializesNilCacheConcurrently`. Finding: `[code]` critical data race in `openAISubscriptionCodexCatalogCache`; scope: `internal/proxy/handler/openai_subscription_codex_catalog.go`, `internal/proxy/handler/handler.go`, and handler tests; validation: `go test -race ./internal/proxy/handler -run 'OpenAISubscriptionCodexCatalog|ListModels' -count=1` plus `go test ./internal/... -count=1`; review head: `8a7ee157cde6552ac87033795d550bbddad748ce`.
- [X] T042 [Review repair] Invalidate `CodexCatalogCache` and persistent catalog entries when `CredentialUpdate` replaces/disables an OpenAI subscription credential and when `CredentialDelete` removes one. Finding: `[code]` critical stale catalog cache can keep serving old entitlement metadata because `OpenAISubscriptionCodexCatalog` returns fresh cache before DB credential status/readback; scope: `internal/proxy/handler/credentials.go`, `internal/proxy/handler/openai_subscription_codex_catalog.go`, and credential/list-model tests; validation: add regression coverage for update/delete invalidation plus `go test ./internal/proxy/handler -run 'CredentialUpdate|CredentialDelete|OpenAISubscriptionCodexCatalog|ListModels' -count=1`, `go test -race ./internal/proxy/handler -run 'OpenAISubscriptionCodexCatalog|ListModels' -count=1`, and `go test ./internal/... -count=1`; review head: `db1dfe4ac434389e30678e05303a7ce24b3196af`.
- [X] T043 [Review repair] Bound Codex catalog refresh with a timeout and regression coverage so stale/missing catalog refresh cannot hang downstream `/models` indefinitely. Finding: `[errors]` critical unbounded upstream catalog fetch path in `OpenAISubscriptionCodexCatalog`/`CatalogClient` uses the request context and `http.DefaultClient` fallback without a bounded refresh timeout, violating `contracts/codex-subscription-catalog.md` timeout semantics and delaying last-known-good/static fallback. Scope: `internal/proxy/handler/openai_subscription_codex_catalog.go`, `internal/provider/chatgptcodex/catalog.go`, and catalog tests; validation: add timeout/fallback regression plus `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|ListModels|OpenAISubscriptionCodexCatalog' -count=1`, `go test ./internal/... -count=1`, `make lint`, and `git diff --check`; review head: `b900e84f9de9723e7f65b84f733b4c6f21c8ce69`.
- [X] T044 [Review repair] Send the current Codex `client_version` query parameter on upstream catalog refresh and cover the regression so local k8s Tianji enriches Sol/Terra/Luna instead of falling back to static metadata. Finding: `[api/provider-validation]` critical live validation failure on review head `75f9f0d729355f55ba6ca80375bd7ecad7493e10`: current PR head plus local k8s DB subscription credentials still returned `128000` context, `115200` auto-compact, `truncation_policy.limit=128000`, text-only modalities, and default `medium`/`xhigh` reasoning for `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna`; direct upstream probe showed `GET https://chatgpt.com/backend-api/codex/models` returns `400 missing query client_version`, `client_version=0.140.0` returns only older models, and `client_version=0.144.1` returns Sol/Terra/Luna metadata with `context_window=372000`, `auto_compact_token_limit=null`, `truncation_policy.limit=10000`, text/image modalities, and upstream reasoning/tier fields. Scope: `internal/provider/chatgptcodex/catalog.go`, catalog request tests, and config/version plumbing needed to keep the requested client version current. Validation: added request URL/query regression coverage, reran `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|ListModels|OpenAISubscriptionCodexCatalog' -count=1`, reran `go test ./internal/... -count=1`, `make lint`, and `git diff --check`; local live `/v1/models` revalidation was not rerun because the prior local Tianji process had exited and no reusable Tianji runtime config/cluster service was available in this session.

Post-deploy live Codex/OpenClaw model refresh verification is documented in `quickstart.md`; it is not an implementation-owned task because it requires deployed Tianji runtime access.

## Dependencies & Execution Order

- Phase 1 before all implementation phases.
- Phase 2 before user stories.
- US1 and US2 are both P1; US1 parser/enrichment and US2 cache can proceed in parallel after foundational contracts are defined.
- US3 depends on US2 cache structure.
- US4 regression checks should run after each implementation phase and again at the end.

## Parallel Opportunities

- T002 and T003 can run in parallel.
- US1 failing tests T007-T010 can run in parallel.
- US2 failing tests T016-T019 can run in parallel.
- US3 failing tests T025-T027 can run in parallel.
- HO-1331 regression tests T032-T034 can run in parallel.

## Implementation Strategy

1. Write failing tests for US1 and US2 first.
2. Implement catalog parser and cache foundation.
3. Implement alias enrichment from cache.
4. Add last-known-good and degraded fallback.
5. Re-run HO-1331 regression tests and live model refresh verification.
