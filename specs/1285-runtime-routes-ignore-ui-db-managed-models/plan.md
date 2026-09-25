# Implementation Plan: Runtime routes consume UI DB-managed models

**Branch**: `HO-1285-runtime-routes-db-models`
**Spec**: `specs/1285-runtime-routes-ignore-ui-db-managed-models/spec.md`
**Linear**: HO-1285
**Phase**: Todo planning only; production implementation starts only after Linear moves to `In Progress`.

## Technical Context

- Language/runtime: Go module `github.com/praxisllmlab/tianjiLLM`.
- Router: `internal/router.Router` built from `[]config.ModelConfig`.
- Runtime handlers: `internal/proxy/handler`.
- UI/model DB persistence: `internal/ui/handler_models.go`, `internal/proxy/handler/model_mgmt.go`, sqlc `ProxyModelTable`.
- Current model config type: `config.ModelConfig` and `config.TianjiParams`.
- Tests: Go integration/E2E with mocked upstreams under `test/e2e`, `test/integration`, and handler package tests.

## Governance

- Todo state allows SpecKit artifacts, branch, draft PR only.
- `bug-fix-pipeline` has been consulted because HO-1285 is a Bug-labeled runtime regression; implementation tasks therefore require RED tests first.
- No production code may be edited until Linear moves to `In Progress`.
- Draft PR is a planning PR, not implementation/review evidence.

## Architecture

### Runtime model source boundary

Add one handler/server-owned boundary that can:

- load YAML models
- load DB models when DB exists
- convert DB rows to `config.ModelConfig`
- publish an immutable merged model list
- build router from the same merged model list
- support refresh after DB model mutations

The boundary should avoid scattering DB fetches across route handlers.

### Snapshot strategy

Preferred implementation:

- Build immutable `RuntimeModelSnapshot{Models, Router, LoadedAt}`.
- Publish it with `atomic.Value` or guard it with `sync.RWMutex`.
- Reads are lock-light and never mutate `Models`.
- Refresh loads DB rows, builds a new list/router, validates it, then swaps the whole snapshot.

`atomic.Value` is attractive for read-mostly routing. `RWMutex` is also acceptable if implementation needs richer refresh state or error bookkeeping.

### DB to config conversion

Create a converter near the runtime source, not inside UI-only code:

1. Decode `ProxyModelTable.TianjiParams` into `config.TianjiParams`.
2. Decode `ProxyModelTable.ModelInfo` into `config.ModelInfo` and access-control shape if present.
3. Require non-empty `model_name` and `tianji_params.model`.
4. Preserve `api_base`, `api_key`, `openai_subscription_credential_ids`, rate limits, and overflow fields.
5. Return sanitized row-level errors.

### Consumers to update

Replace config-only reads with runtime source reads:

- `ListModels`
- `findModelConfig`
- `resolveProviderFromConfigRouteWithContext`
- `resolveProviderBaseURLWithContext`
- `resolveAssistantsUpstream`
- `resolveOpenAIEndpointRouteForRequest`
- `ModelGroupInfo`
- router initialization and refresh path

Chat routing should use router from merged snapshot. Direct resolver fallback should use the same merged list.

### Management refresh

Add a refresh hook to handlers:

- `/model/new`
- `/model/update`
- `/model/delete`
- UI create/update/delete handlers

Preferred behavior: after successful DB mutation, call refresh and return success only if runtime state refreshes or return success with explicit warning if mutation committed but refresh failed.

If hot refresh conflicts with current router lifecycle, implementation must make restart-required behavior explicit and test it.

## Plan Review Evidence

- Repo reality: `cmd/tianji/main.go` initializes router with `router.New(cfg.ModelList, ...)`.
- Repo reality: `ListModels`, `findModelConfig`, and `resolveProviderBaseURLWithContext` iterate `h.Config.ModelList`.
- Repo reality: `ModelGroupInfo` reads `h.Router.ListModelGroups()` and has a DB exclusion comment.
- Repo reality: UI/model management paths already write `ProxyModelTable`, and key UI already lists DB model names for restrictions.
- Repo reality: `assistants.go` has OpenAI endpoint helpers that fall back to scanning `h.Config.ModelList`, including `resolveAssistantsUpstream` and `resolveOpenAIEndpointRouteForRequest`.
- Repo reality: `server.go` registers the issue-listed files, batches, fine-tuning, assistants, threads, vector stores, responses, image edits/variations, OCR, videos, containers, RAG, Anthropic batch, native Anthropic, and Gemini route families.
- Official Go docs: `sync/atomic.Value` supports atomic `Load`/`Store` of a consistently typed value, suitable for immutable runtime snapshots.
- Official Go docs: `sync.RWMutex` supports many readers or one writer, suitable for guarded cache refresh.
- Context7 `/go-chi/docs`: `chi.URLParam` is the supported way to read `{model}` path params for Azure-compatible route tests.
- grep.app prior art: Kubernetes, OpenTelemetry, and other Go projects use `atomic.Value` for read-mostly runtime state/delegate snapshots.

## Failing Tests

| Test | File | Initial failure | Covers |
| --- | --- | --- | --- |
| `TestRuntimeModelSource_ListModelsIncludesDBManagedModel` | `test/e2e/runtime_db_models_test.go` or integration equivalent | `/v1/models` omits DB row | FR-001 |
| `TestRuntimeModelSource_ChatCompletionRoutesDBManagedModel` | same | model not found | FR-002, FR-003 |
| `TestRuntimeModelSource_EmbeddingRoutesDBManagedModel` | same | config lookup misses DB model | FR-004 |
| `TestRuntimeModelSource_ModelGroupInfoIncludesDBManagedModel` | `test/integration` or handler test | discovery omits DB row | FR-005 |
| `TestRuntimeModelSource_DuplicateNameDBOverridesYAML` | handler/source unit test | no merge rule | FR-006 |
| `TestRuntimeModelSource_YAMLOnlyStillWorksWithoutDB` | handler/source unit test | regression guard | FR-008 |
| `TestModelManagementRefreshesRuntimeSource` | handler/model management test | runtime source stale | FR-009 |
| `TestRuntimeModelSource_WildcardRule` | source unit test | behavior undefined | FR-007 |
| `TestRuntimeModelSource_OpenAIEndpointFallbackUsesMergedSource` | handler/source unit test | OpenAI endpoint helper scans YAML only | FR-010, FR-015 |
| `TestRuntimeModelSource_AffectedRouteAuditComplete` | docs/review checklist or unit-backed audit | route family unclassified | FR-015, SC-008 |

## Implementation Phases

### Phase 1 - RED tests first

Write tests for list, chat routing, one non-chat route, discovery, duplicate precedence, YAML fallback, refresh, and wildcard rule. Confirm failures are implementation gaps, not fixture setup.

### Phase 2 - Runtime source and conversion

Add DB row conversion and merged snapshot source. Unit-test conversion, duplicate precedence, DB outage fallback, and invalid row handling.

### Phase 3 - Routing/listing consumers

Wire handlers to use runtime source for model list, exact/wildcard lookup, router access, and provider-base resolution.

### Phase 4 - Discovery and native/resource routes

Update `/model_group/info` and at least one representative non-chat/resource route. Audit all listed route families so no config-only runtime lookup remains or each unaffected route is explicitly classified with code evidence.

### Phase 5 - Refresh behavior

Hook create/update/delete paths to refresh runtime source or surface restart-required response. Test the selected behavior.

### Phase 6 - Verification and state progression

Run targeted tests, affected package tests, lint, and diff sanity. Then move through `In Review`, review gate, `Waiting CI`, CI green, and only then `Waiting Merge`.

## Verification Commands

```bash
go test ./test/e2e -tags e2e -run 'TestRuntimeModelSource|TestModelManagementRefreshesRuntimeSource' -count=1
go test ./test/integration -run 'TestRuntimeModelSource|TestModelGroupInfo' -count=1
go test ./internal/proxy/handler/... -run 'TestRuntimeModelSource|TestListModels|TestModelGroupInfo|TestResolveProviderBaseURL' -count=1
go test ./internal/router/... ./internal/config/... ./internal/ui/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

If local E2E infrastructure is unavailable, implementation must still compile targeted E2E tests and run handler/source unit tests locally; CI E2E remains the merge gate.

## Risk Register

| Risk | Mitigation |
| --- | --- |
| Multiple route helpers keep divergent model sources | Create one runtime source and make all lookup paths consume it. |
| Hot refresh races with in-flight requests | Publish immutable snapshot with atomic swap or `RWMutex`. |
| DB outage breaks YAML deployments | Startup and refresh keep last valid/YAML snapshot. |
| DB row JSON malformed | Validate/skip with sanitized errors and tests. |
| Duplicate YAML/DB names behave unpredictably | Encode DB-over-YAML precedence in tests. |
| Wildcard behavior differs between DB and YAML | Reuse existing wildcard specificity or reject DB wildcard names explicitly. |
| OpenAI subscription credential routing regresses | Preserve `TianjiParams.OpenAISubscriptionCredentialIDs` conversion and run affected tests. |

## Todo Gate Status

Spec/plan/tasks/analyze are complete and ready for docs-only draft PR review. Production implementation remains blocked until Linear `In Progress`.
