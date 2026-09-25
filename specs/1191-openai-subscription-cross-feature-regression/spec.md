# Feature Specification: OpenAI Subscription Cross-Feature Regression Matrix

**Feature Branch**: `HO-1191-openai-subscription-cross-feature-regression`
**Created**: 2026-05-09
**Input**: Linear HO-1191 - `[QA] OpenAI subscription cross-feature regression matrix`

## Summary

建立一份可在 CI 中被測試實作代表的 OpenAI subscription regression matrix，覆蓋 HO-1165 / HO-1173 指定的 backend、API、UI E2E acceptance paths。這張票不是新增產品行為；它 owns coverage audit、缺口補測、測試組織與 CI 證據，確保 OpenAI subscription OAuth、credential lifecycle、routing/failover、quota/redaction、Models page selection 與既有 `api_key` path 不會在跨功能整合後回歸。

## Scope

- 盤點現有 unit、integration、UI E2E 測試，將每個 HO-1165 / HO-1173 acceptance path 對到具體測試檔、測試名稱、CI command。
- 補上缺少或過弱的 regression coverage；優先使用現有 Go test、`httptest.NewServer()`、`internal/testutil/openaitest`、現有 Playwright E2E fixture。
- 覆蓋 PKCE/state/code_verifier、callback success/failure、refresh rotation、per-credential refresh lock、401 refresh retry then failover、disabled credential exclusion、quota headers、redaction、spend/audit attribution、credential API lifecycle、UI actions、Models page multi-credential selection。
- 明確保護 existing `api_key` behavior：沒有 subscription IDs 時維持 `api_key` / `$OPENAI_API_KEY` / custom OpenAI-compatible `api_base` 路徑。
- 明確保護 config validation：`openai_subscription_credential_ids` 不能與 custom `api_base` 組合。
- CI 不得呼叫 real OpenAI；所有 OAuth/token/upstream coverage 必須走 local mock / guarded client。

## Out of Scope

- 新產品行為、UI redesign、credential lifecycle semantics 變更。
- 真實 OpenAI 帳號、真實 OpenAI network、WireMock、Dockerized mock service。
- OpenAPI generator 或新 contract framework。
- HO-1192 文件內容；本票只可在 quickstart / matrix 中引用 docs follow-up，不實作文檔主體。

## User Stories

### US1 - Maintainer Sees One Cross-Feature Matrix

As a maintainer, I can inspect one matrix and know which CI-backed test protects each OpenAI subscription acceptance path.

**Acceptance Scenarios**

1. Given a HO-1165 / HO-1173 acceptance path, when I check the matrix, then it names the exact test file, test case, layer, command, and current status.
2. Given an acceptance path is not covered strongly enough, when the matrix is updated, then it marks the gap and maps it to an implementation task before the issue can reach Waiting Merge.
3. Given a future PR changes OpenAI subscription behavior, when review checks coverage, then the matrix shows whether unit, integration, or UI E2E must be rerun.

### US2 - Offline Backend and API Regression Coverage

As CI, I can run OpenAI subscription backend/API tests without OpenAI credentials or real network.

**Acceptance Scenarios**

1. Given OAuth exchange or refresh logic runs in tests, when it needs an OAuth/token server, then it uses `httptest.NewServer()` / `internal/testutil/openaitest` and guarded clients.
2. Given upstream OpenAI calls are tested, when tests target `/v1/models` or representative endpoints, then they target the local upstream mock.
3. Given code accidentally targets `auth.openai.com` or `api.openai.com` in covered test paths, when tests run, then a guard test fails with the forbidden host.

### US3 - UI E2E Covers Operator Workflows

As an operator, I can rely on UI E2E to cover connect, callback, credential actions, quota rendering, and Models page selection.

**Acceptance Scenarios**

1. Given an admin uses `Connect OpenAI`, when UI E2E runs, then success, localhost paste callback, unauthenticated block, and readable failure pages are covered with mocked OAuth.
2. Given an admin manages credentials, when UI E2E runs, then Test, Refresh, Disable, Delete, confirm-cancel, error redaction, list/detail metadata, and quota states are covered.
3. Given an admin configures a model, when UI E2E runs, then create/edit multi-select persists selected `openai_subscription_credential_ids`, filters non-subscription credentials, hides token material, and blocks custom `api_base`.
4. Given no subscription credentials are selected, when model create/edit runs, then existing `api_key` behavior remains covered.

## Functional Requirements

- **FR-001**: Repository MUST contain a committed coverage matrix artifact for OpenAI subscription regression coverage.
- **FR-002**: The matrix MUST include columns for acceptance path, owning issue/group, layer, test file, test name, command, current coverage status, and gap action.
- **FR-003**: Matrix coverage MUST include PKCE/state/code_verifier validation and callback state cleanup.
- **FR-004**: Matrix coverage MUST include callback success, provider denial, invalid/expired state, token exchange failure, and secret-safe success/error DOM.
- **FR-005**: Matrix coverage MUST include refresh before expiry, forced refresh, refresh token rotation, per-credential refresh lock, refresh failure metadata, and idempotent refresh behavior.
- **FR-006**: Matrix coverage MUST include 401 refresh retry then failover, disabled credential exclusion, all-unusable explicit errors, and no `api_key` fallback when subscription IDs are configured.
- **FR-007**: Matrix coverage MUST include official OpenAI endpoint bearer injection for chat/completions, responses, embeddings, images, audio, and models/test endpoint.
- **FR-008**: Matrix coverage MUST include OpenAI quota/rate-limit header parsing, quota store behavior, sticky/lowest-utilization routing consumption, and credential UI quota rendering.
- **FR-009**: Matrix coverage MUST include spend/audit attribution and redaction of token, JWT, bearer, encrypted credential, code, and raw upstream response material.
- **FR-010**: Matrix coverage MUST include credential CRUD/API lifecycle actions: list safe metadata, test, refresh, disable, delete, idempotency, and failure redaction.
- **FR-011**: Matrix coverage MUST include UI E2E for Connect/Test/Refresh/Disable/Delete and Models page multi-credential create/edit selection.
- **FR-012**: Matrix coverage MUST include existing `api_key` path regression when subscription IDs are omitted.
- **FR-013**: Matrix coverage MUST include config validation that rejects subscription IDs with custom `api_base`.
- **FR-014**: All OAuth/token/upstream tests in this scope MUST run offline and MUST NOT require real OpenAI credentials or live OpenAI hosts.
- **FR-015**: Implementation MUST add or update tests only where the matrix identifies a concrete gap; it MUST NOT rewrite unrelated test architecture.

## Edge Cases

- Existing tests may cover a behavior indirectly; the matrix must mark indirect coverage as insufficient if it does not assert the issue acceptance path.
- UI E2E must verify persisted DB/config state for critical create/edit flows, not only visible toast text.
- Error redaction assertions must inspect response/DOM/audit metadata depending on layer; checking only HTTP status is insufficient.
- `api_base` custom-provider behavior must stay API-key based and must not be tested through subscription credential shortcuts.
- Multiple selected credentials can be stored in rendered order or DB order; tests should assert set equality when order is not product behavior.
- Tests that install global HTTP clients/transports must restore them with `t.Cleanup`.

## Success Criteria

- **SC-001**: Coverage matrix maps every HO-1191 / HO-1173 listed acceptance path to CI-represented tests or an explicit fixed gap.
- **SC-002**: Backend/API regression tests pass with `go test ./internal/... ./test/integration/...` or the narrower CI-equivalent command chosen in tasks.
- **SC-003**: UI E2E OpenAI subscription tests pass with `go test -tags e2e -count=1 ./test/e2e/...` or a documented targeted subset plus full CI evidence.
- **SC-004**: CI evidence shows lint, unit/integration, E2E, and build jobs pass on the PR head.
- **SC-005**: No production behavior changes are introduced unless required solely to make an already-specified acceptance path testable; any such change must be tied to a matrix gap.
