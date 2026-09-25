# Codex Catalog Permanent Credential Filter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep permanent, explicitly non-selectable OpenAI subscription credentials out of catalog entitlement aggregation while retaining temporary refresh/quota failures as fail-closed blockers.

**Architecture:** Preserve the existing configured-credential aggregation and static fallback. Enrich a catalog result only when credential resolution identifies a known permanent non-selectable state, then skip those results during exact metadata aggregation. Leave generic refresh failures eligible for the existing all-credentials gate so temporary outages and quota-unavailable states do not produce optimistic metadata.

**Tech Stack:** Go, `testing`, `testify`, existing OpenAI subscription catalog cache and handler tests.

**Spec:** User request in this task; no standalone spec file exists.

## Global Constraints

- Exclude only clearly permanent non-selectable states (`credential_disabled`, `refresh_token_invalidated`, `auth_failed_after_refresh`, `operator_disabled`).
- Do not exclude generic `refresh_failed`, rate-limited, quota-gated, or upstream-unavailable results.
- Preserve the existing runtime routing error-code contract.
- Keep the static fallback metadata unchanged; catalog failure must not be hidden by claiming image support.
- No deployment or restart. Commit, push, and PR delivery are authorized by the user after validation.

### Task 1: Add the regression coverage

**Files:**
- Modify: `internal/proxy/handler/openai_subscription_codex_catalog_test.go`
- Modify: `internal/proxy/handler/list_models_codex_test.go`

**Interfaces:**
- Consume the existing `OpenAISubscriptionCodexCatalogResult` lookup path and configured credential IDs.
- Produce tests proving permanent credentials are skipped and temporary failures still fail closed.

- [x] **Step 1: Write the failing permanent-state test**

Add a catalog aggregation test with one active catalog result and one result marked as a permanent non-selectable credential. Assert that `buildListModelsResponse` uses the active catalog’s `InputModalities` (`[]string{"text", "image"}`) instead of the static `[]string{"text"}` fallback.

- [x] **Step 2: Write the failing temporary-state test**

Add a paired test with one active catalog result and one degraded result whose `LastErrorReason` is `refresh_failed`. Assert that the response remains on static metadata (`[]string{"text"}`), preserving fail-closed behavior.

- [x] **Step 3: Run the focused tests to verify RED**

Run:

```bash
go test ./internal/proxy/handler -run 'TestListModelsSubscriptionCatalog(ExcludesPermanent|KeepsTemporary)' -count=1
```

Expected: the permanent-state test fails because the current aggregation still requires every configured credential.

### Task 2: Implement the smallest catalog eligibility fix

**Files:**
- Modify: `internal/proxy/handler/openai_subscription_codex_catalog.go`

**Interfaces:**
- Add a persisted result marker for permanent non-selectability.
- Set the marker only from known permanent credential-resolution errors.
- Filter marked results before exact catalog metadata comparison.

- [x] **Step 1: Add the result/cache marker**

Add `PermanentlyUnavailable bool` to `OpenAISubscriptionCodexCatalogResult` and `openAISubscriptionCodexCatalogCacheValue`, preserving JSON compatibility with `omitempty`.

- [x] **Step 2: Set the marker at the resolution boundary**

When catalog credential resolution fails, classify only `credential_disabled`, `refresh_token_invalidated`, `auth_failed_after_refresh`, and explicit `operator_disabled` metadata as permanent. Keep generic refresh errors as `refresh_failed`. Persist the marker on degraded results and restore it from shared cache.

- [x] **Step 3: Exclude marked results from aggregation**

Before the existing candidate loop compares catalog models, build a credential list containing only results that are not marked permanent. If all configured credentials are permanent, return the existing static-fallback result. Do not alter wildcard or runtime Responses Lite behavior beyond the same catalog lookup result semantics.

### Task 3: Verify the complete change

**Files:**
- No additional files.

- [x] **Step 1: Run focused catalog and model-list tests**

```bash
go test ./internal/proxy/handler -run 'Test(OpenAISubscriptionCodexCatalog|ListModels|CodexModelInfo)' -count=1
```

- [x] **Step 2: Run the full handler package tests**

```bash
go test ./internal/proxy/handler -count=1
```

- [x] **Step 3: Run repository checks**

```bash
make lint
git diff --check
git status --short
```

- [x] **Step 4: Re-read the diff against the constraints**

Confirm no fallback metadata changed, no generic temporary failure is filtered, and no routing error-code tests require updates.

## Verification evidence

- `go test ./internal/proxy/handler -run 'Test(OpenAISubscriptionCodexCatalog|ListModels|CodexModelInfo)' -count=1` — PASS (69 tests).
- `go test ./internal/proxy/handler -count=1` — PASS (1134 tests).
- `go test ./internal/proxy/... -count=1` — PASS (1278 tests).
- `make lint` — PASS (0 issues).
- `git diff --check` — PASS.
- `go test ./... -count=1` — repository packages passed, but integration tests under `test/integration` failed because local PostgreSQL at `127.0.0.1:5433` refused connections; no code failure was observed.

Oracle fix-round additions:

- Usage-snapshot refresh preserves structured `refresh_token_invalidated` metadata while retaining `refresh_failed` as the runtime error code.
- Temporary degraded results, including cached last-known-good catalogs, fail closed in exact and wildcard catalog aggregation.
- Whitespace-padded `disabled` status is classified as `credential_disabled` in both credential loaders.
