# Feature Specification: Models page multi OpenAI subscription credential selection

**Feature Branch**: `HO-1172-models-page-multi-openai-subscription-credential`
**Created**: 2026-05-09
**Input**: Linear HO-1172 - `[FE] Models page multi OpenAI subscription credential selection`

## 摘要

在 TianjiLLM 管理 UI 的 Models page，讓 proxy admin 在新增或編輯 model config 時，可以從既有 OpenAI subscription credentials 中選取一個或多個 credential ID，並把選取結果保存到 `model_list[].tianji_params.openai_subscription_credential_ids`。UI 必須保留既有 `api_key` / `$OPENAI_API_KEY` 行為，並阻止 subscription credential IDs 與 custom `api_base` 同時保存。

## Scope

- Models page 新增 OpenAI subscription credentials selector，支援多選、移除、預填既有選取值。
- Create model 與 Edit model 表單都支援 `openai_subscription_credential_ids`。
- 保存時將 selected credential IDs 寫入 `tianji_params.openai_subscription_credential_ids`。
- Models table 顯示 selected subscription credential 的 safe metadata 摘要，例如 credential name、safe email、status，不顯示 token material。
- UI 在有 selected subscription credentials 時阻止 custom `api_base`，並提供可修正的錯誤訊息。
- 若未選 subscription credentials，既有 `api_key` / `$OPENAI_API_KEY` / custom `api_base` 行為不變。
- UI E2E 覆蓋 multi-select add/remove/save、custom `api_base` 阻擋、既有 API-key create/edit regression、DB round-trip。
- 2026-05-09 Waiting Merge 後 owner 追加 scope：PR #158 的 GitHub Actions CI jobs 必須改用 staging GitHub runner `staging-monster-ci`。

## Out of Scope

- Todo 階段 production code implementation。
- 新增 OpenAI subscription credential CRUD、connect、test、refresh、disable lifecycle。
- 修改 OpenAI token resolution、routing、quota parser、refresh manager。
- 新增 credential DB schema 或改變 `CredentialTable` lifecycle API contract。
- 解密或顯示 `credential_value`、access token、refresh token、`id_token`、JWT、bearer string。
- 讓 UI 自動選取所有 organization credentials；本 issue 只允許 explicit credential IDs。
- 支援 subscription credentials 搭配 Azure/custom OpenAI-compatible `api_base`。

## User Stories and Tests

### User Story 1 - Admin 在 Create Model 選取多個 subscription credentials (P1)

Proxy admin 新增 OpenAI model deployment 時，可以在表單中看到可用的 OpenAI subscription credentials，選取多個 account 作為該 model 的 explicit credential pool。

**Independent Test**: UI E2E seed 兩筆 `openai_subscription` credentials 和一筆 generic `api_key` credential，開啟 Add Model，選取兩筆 subscription credentials，送出後檢查 DB `tianji_params.openai_subscription_credential_ids` 等於選取的兩個 ID，且 generic `api_key` 不出現在 selector。

**Acceptance Scenarios**:

1. Given OpenAI subscription credentials exist，when admin opens Add Model，then selector lists only `credential_type="openai_subscription"` credentials。
2. Given admin selects two credentials，when model is created，then `openai_subscription_credential_ids` persists exactly those IDs in selected order。
3. Given admin deselects one credential before submit，when model is created，then removed ID is not persisted。
4. Given no subscription credentials exist，when admin opens Add Model，then selector shows an empty state and model can still be created through existing API-key path。

### User Story 2 - Admin 在 Edit Model 預填、移除、保存 selected credentials (P1)

Proxy admin 編輯既有 model 時，可以看到目前已綁定的 OpenAI subscription credentials，移除或新增 credentials 後保存。

**Independent Test**: E2E seed model with `openai_subscription_credential_ids=["cred-a"]`，開啟 Edit Model，確認 `cred-a` checked，再選 `cred-b` / 移除 `cred-a`，保存後 DB round-trip 只剩 `["cred-b"]`。

**Acceptance Scenarios**:

1. Given model has existing IDs，when Edit Model opens，then matching credentials are preselected。
2. Given selected credential metadata exists，when Edit Model opens，then UI displays safe name/email/status summary。
3. Given admin removes all selected credentials and saves，then `openai_subscription_credential_ids` is omitted or saved as empty and existing API-key behavior remains available。
4. Given existing selected ID no longer exists in `CredentialTable`，when Edit Model opens，then UI shows a missing credential badge and preserves/removes it only through explicit admin action。

### User Story 3 - UI blocks invalid subscription + custom API base combination (P1)

Proxy admin cannot accidentally save a model that uses OpenAI subscription credentials with custom `api_base`，because backend validation already rejects that combination and UI should prevent the bad submit earlier.

**Independent Test**: E2E select one OpenAI subscription credential, fill `api_base=https://custom.api.com`，submit，assert dialog stays open, error toast mentions subscription credentials cannot be combined with custom API base, and DB row is not created/updated.

**Acceptance Scenarios**:

1. Given subscription credentials are selected，when admin fills non-empty `api_base`，then submit is blocked or server returns a field-level/toast error without closing dialog。
2. Given custom `api_base` is set，when admin selects subscription credentials，then UI makes the conflict visible before submit。
3. Given admin clears `api_base`，when submit happens，then subscription credential IDs save normally。
4. Given no subscription credentials are selected，when admin fills custom `api_base` and `api_key`，then existing OpenAI-compatible provider behavior remains unchanged。

### User Story 4 - Existing API-key model create/edit regression stays unchanged (P1)

Existing operators who use `api_key` or `$OPENAI_API_KEY` can continue creating and editing models without touching subscription credentials。

**Independent Test**: Existing E2E for create/edit optional `api_base`、`api_key`、API-key preservation、unknown field preservation still passes; add targeted regression for model with no selected subscription credentials。

**Acceptance Scenarios**:

1. Given no subscription credentials are selected，when admin creates model with `api_key`，then `tianji_params.api_key` persists exactly as before。
2. Given Edit Model opens and `api_key` input is left empty，when admin saves unrelated fields，then existing API key is preserved。
3. Given unknown `tianji_params` fields exist，when admin edits credential selection or unrelated fields，then unknown fields remain preserved。
4. Given `$OPENAI_API_KEY` is used in `api_key`，when no subscription credentials are selected，then env reference remains unchanged。

### User Story 5 - Safe credential metadata display (P1)

Security reviewer can inspect Models UI and confirm it only displays safe metadata for selected OpenAI subscription credentials。

**Independent Test**: E2E seed credentials with safe email/status plus token-looking metadata/value; assert Models table, Add/Edit dialog, and rendered HTML do not contain token material。

**Acceptance Scenarios**:

1. Given credential has safe email/status/name，when rendered in selector/table，then safe fields are visible。
2. Given credential metadata contains token-looking values，when rendered，then raw token strings are redacted or omitted。
3. Given credential value exists，when rendered，then UI never includes `credential_value` or encrypted blob。
4. Given generic `api_key` credential exists，when rendered，then it never appears as an OpenAI subscription selectable option。

## Functional Requirements

- **FR-001**: Models page Create form MUST support selecting zero or more OpenAI subscription credential IDs。
- **FR-002**: Models page Edit form MUST prefill and update zero or more selected OpenAI subscription credential IDs。
- **FR-003**: Selector options MUST come only from existing `CredentialTable` rows with `credential_type = "openai_subscription"`。
- **FR-004**: UI MUST persist selected IDs into `tianji_params.openai_subscription_credential_ids` with exact selected IDs and no generic credential rows。
- **FR-005**: If selected IDs is empty，UI MUST preserve existing `api_key` / `$OPENAI_API_KEY` / `api_base` behavior。
- **FR-006**: UI MUST reject or block saving selected subscription IDs together with a non-empty custom `api_base`。
- **FR-007**: Create/update handlers MUST preserve existing unknown `tianji_params` fields while adding/updating/removing `openai_subscription_credential_ids`。
- **FR-008**: Models table MUST show safe subscription credential summary for selected IDs。
- **FR-009**: Selector and summary MUST display safe metadata only: credential name, ID, redacted/safe email, status。
- **FR-010**: Rendered HTML MUST NOT include access token、refresh token、`id_token`、JWT、bearer string、encrypted credential value、fallback API key、or raw OpenAI account payload。
- **FR-011**: UI MUST represent missing/deleted selected credential IDs without silently dropping them before admin saves。
- **FR-012**: UI MUST remain offline-testable with seeded DB rows and must not call real OpenAI endpoints。
- **FR-013**: E2E MUST cover create multi-select add/remove/save、edit prefill/update、invalid custom `api_base` block、API-key regression、DB round-trip。
- **FR-014**: UI implementation MUST reuse existing `templ`、HTMX、`AppLayout`、dialog、table、badge、input/button components and must not add a frontend framework。
- **FR-015**: GitHub Actions CI workflow MUST route TianjiLLM `lint`、`test`、`e2e`、`build`、`docker` jobs to staging runner `staging-monster-ci` instead of GitHub-hosted `ubuntu-latest`。

## Key Entities

- **Proxy Model**: Existing `ProxyModelTable` row whose `tianji_params` JSON stores provider model config。
- **OpenAI Subscription Credential Selection**: The explicit list stored at `tianji_params.openai_subscription_credential_ids`。
- **OpenAI Subscription Credential Option**: Safe UI option derived from `CredentialTable` row with `credential_type="openai_subscription"`。
- **Safe Credential Metadata**: Narrow display fields from `credential_name` and redacted `credential_info` (`email`, `status`)。
- **API-key fallback path**: Existing model behavior when no subscription credential IDs are selected。

## Edge Cases

- No OpenAI subscription credentials exist。
- Existing model has selected ID that no longer exists。
- Existing model has selected ID for wrong credential type。
- Existing model has duplicate or blank IDs from old/manual config。
- Admin selects subscription credential then fills custom `api_base`。
- Admin clears selected credentials while an old `api_key` exists。
- `credential_info` is malformed JSON。
- Credential metadata contains token-looking strings。
- Create/update fails validation and dialog must remain open with user input intact。
- Pagination/search table reload must still show selected credential summary after successful save。

## Success Criteria

- **SC-001**: E2E proves Add Model can select two OpenAI subscription credentials and DB stores exact IDs。
- **SC-002**: E2E proves Edit Model preselects existing IDs and persists add/remove changes。
- **SC-003**: E2E proves selected IDs + custom `api_base` is blocked and does not create/update DB state。
- **SC-004**: E2E proves existing API-key create/edit flows still pass with no subscription IDs。
- **SC-005**: Security test proves rendered Models UI contains no token material or encrypted credential value。
- **SC-006**: `go test ./internal/ui/...` and targeted model/credential E2E pass before Waiting CI。
- **SC-007**: PR CI dispatch after the runner follow-up is picked up by `staging-monster-ci` and reaches workflow steps for all non-skipped required jobs。

## Dependencies

- HO-1167: `config.TianjiParams.OpenAISubscriptionCredentialIDs` and backend validation contract。
- HO-1176: encrypted `openai_subscription` credential persistence。
- HO-1184: safe OpenAI subscription credential CRUD response/redaction contract。
- HO-1186: Credentials UI safe metadata patterns and E2E fixtures。
- Existing Models page in `internal/ui/handler_models.go` and `internal/ui/pages/models.templ`。
