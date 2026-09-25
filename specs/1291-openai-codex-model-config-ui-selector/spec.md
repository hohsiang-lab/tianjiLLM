# Feature Specification: OpenAI subscription Codex transport model config/UI selector

**Branch**: `HO-1291-openai-codex-model-config-ui`
**Linear**: HO-1291
**Created**: 2026-05-10
**Status**: Todo planning only; no production code in this phase

## Summary

TianjiLLM 目前 main 已有 `openai_subscription_transport` 的部分 backend/config baseline，但 Models UI 仍只會把 `openai_subscription_credential_ids` 寫入 `tianji_params`，沒有欄位讓 operator 明確選擇 subscription-backed model 要走一般 OpenAI Platform transport，還是 ChatGPT Codex backend transport。這會讓 `openai/*` wildcard 綁 subscription credentials 後仍容易被理解成一般 Platform `/v1/chat/completions` route。

HO-1291 要完成明確 model config/UI selector：當 Models UI 選到 OpenAI subscription credentials 時，表單必須顯示並保存 transport choice；Codex transport 與 Platform API transport 不能靠隱含規則或 credential presence 猜測。

## Scope

In scope:

- 完成 model `tianji_params.openai_subscription_transport` 的 UI 建立/編輯/顯示路徑；main 已有 typed config/runtime baseline 時不得重複新增。
- 支援 `direct_openai_http` 與 `chatgpt_codex_backend` 兩種可測模式。
- 更新 config validation，讓有 subscription credentials 的 model 必須明確選 transport，且只允許已知值。
- 更新 Models Create/Edit UI，在選到 OpenAI subscription credentials 時顯示 transport selector。
- Edit Model 必須 prefill 既有 transport；清空 subscription credentials 時不得留下 stale Codex transport。
- UI/server validation 必須阻止 subscription credentials 搭配錯誤或缺失 transport。
- 保留現有 API-key-backed `openai/*` route 與 custom `api_base` 行為。
- 加入 UI/config/E2E tests 覆蓋 create/edit subscription wildcard + Codex transport，以及 API-key regression。

Out of scope:

- 實作 HO-1288 的 ChatGPT Codex backend HTTP transport。
- 實作 HO-1290 的 Codex response/error normalization。
- 改 OpenAI OAuth connect/refresh/credential lifecycle。
- 改 pricing sync、credential CRUD、team/access group UI。
- 真實呼叫 OpenAI 或 ChatGPT endpoint。
- 讓所有 OpenAI subscription models 自動改走 Codex backend。

## User Stories and Tests

### User Story 1 - Admin 建立 subscription-backed Codex wildcard model (P1)

Proxy admin 在 Models UI 建立 `openai/*` 或 `chatgpt/*` wildcard model 時，可以選 OpenAI subscription credentials，並明確選 `ChatGPT Codex backend` transport。

**Independent Test**: E2E seed active `openai_subscription` credential，Create Model 填 `model_name=openai/*`、`model=openai/*`，選 credential 與 Codex transport，submit 後 DB `tianji_params` 包含 `openai_subscription_credential_ids` 與 `openai_subscription_transport="chatgpt_codex_backend"`。

**Acceptance Scenarios**:

1. Given subscription credentials exist, when Create Model opens, then transport selector is available near the subscription credential selector.
2. Given admin selects subscription credential and Codex transport, when saving, then DB stores the explicit transport field.
3. Given saved model renders in table, then UI shows safe transport summary without token material.

### User Story 2 - Edit Model prefill/change/removal is explicit (P1)

Proxy admin 編輯既有 subscription-backed model 時，能看到目前 transport，切換後保存，或清空 subscription credentials 時一併移除 transport field。

**Independent Test**: E2E seed model with `openai_subscription_credential_ids=["cred-a"]` and `openai_subscription_transport="chatgpt_codex_backend"`，Edit dialog preselects Codex transport；切換 direct/save 後 DB 更新；再清空 credentials/save 後 DB 不保留 stale transport。

**Acceptance Scenarios**:

1. Given model has Codex transport, when Edit opens, then Codex option is selected.
2. Given admin switches to direct transport, when saving, then DB stores `direct_openai_http`.
3. Given admin removes all subscription credentials, when saving, then `openai_subscription_transport` is omitted.

### User Story 3 - Invalid subscription transport combinations are blocked (P1)

Operator 不能保存「有 subscription credentials 但 transport 缺失/未知」或「subscription Codex transport + custom `api_base`」這類錯誤 config。

**Independent Test**: Unit/config tests cover missing transport, unknown transport, duplicate/blank credential IDs, custom `api_base`, non-OpenAI provider, and API-key no-subscription path。

**Acceptance Scenarios**:

1. Given `openai_subscription_credential_ids` is non-empty, when transport is missing, then validation fails with actionable error.
2. Given transport is unknown, when config loads or UI submits, then validation fails.
3. Given no subscription credential is selected, when API-key OpenAI model saves, then no transport is required.

### User Story 4 - Existing API-key `openai/*` remains valid (P1)

Existing operator who uses API key / env API key / custom OpenAI-compatible endpoint should not see routing behavior change.

**Independent Test**: Existing Models create/edit API-key tests remain green; add a targeted regression where `model=openai/*` with API key and no subscription credentials saves without `openai_subscription_transport`。

**Acceptance Scenarios**:

1. Given no subscription credentials, when model uses `api_key`, then save behavior remains unchanged.
2. Given custom `api_base` and no subscription credentials, then save behavior remains unchanged.
3. Given old rows without transport and without subscription credentials, then runtime/config loading remains compatible.

## Functional Requirements

- **FR-001**: System MUST expose and preserve the explicit per-model OpenAI subscription transport field `openai_subscription_transport`.
- **FR-002**: System MUST support `direct_openai_http` for existing Platform direct HTTP subscription behavior.
- **FR-003**: System MUST support `chatgpt_codex_backend` for HO-1288 ChatGPT Codex backend transport selection.
- **FR-004**: Config validation MUST require `openai_subscription_transport` when `openai_subscription_credential_ids` is non-empty.
- **FR-005**: Config validation MUST reject unknown transport values.
- **FR-006**: Config/UI validation MUST keep rejecting subscription credentials with non-default custom `api_base`.
- **FR-007**: Models Create UI MUST expose the transport selector when OpenAI subscription credentials are available/selected.
- **FR-008**: Models Edit UI MUST prefill and update the existing transport.
- **FR-009**: Clearing all subscription credentials MUST remove `openai_subscription_transport` from saved `tianji_params`.
- **FR-010**: Models table MUST show a safe transport summary for subscription-backed rows.
- **FR-011**: Rendered UI MUST NOT include access token, refresh token, bearer string, JWT, encrypted credential value, or raw credential JSON.
- **FR-012**: Runtime model-source decoding MUST continue preserving the field from DB JSON into `config.TianjiParams`.
- **FR-013**: Existing API-key-backed `openai/*` / custom `api_base` model config MUST remain valid without the new transport field.
- **FR-014**: Tests MUST be offline/mocked and MUST NOT call real OpenAI or ChatGPT endpoints.

## Key Entities

- **OpenAI Subscription Transport**: Explicit enum-like model config deciding how selected subscription credentials are used.
- **Subscription-backed Proxy Model**: `ProxyModelTable` row whose `tianji_params` includes `openai_subscription_credential_ids`.
- **Codex Transport Selector**: Models UI control that persists transport choice.
- **API-key OpenAI Model**: Existing model with no subscription credential IDs; remains out of transport selector requirements.

## Success Criteria

- **SC-001**: Config tests reject subscription credentials without transport and accept `chatgpt_codex_backend`.
- **SC-002**: UI/E2E tests prove Create Model saves subscription credentials + Codex transport.
- **SC-003**: UI/E2E tests prove Edit Model prefills and updates transport.
- **SC-004**: Tests prove clearing credentials removes stale transport.
- **SC-005**: API-key `openai/*` create/edit regression remains green.
- **SC-006**: Targeted Go/UI/E2E tests pass offline; `git diff --check` passes.

## Dependencies

- HO-1172 Models page multi OpenAI subscription credential selection.
- HO-1285 runtime DB-managed model source.
- HO-1288 ChatGPT Codex backend transport contract and merged backend routing baseline.
- HO-1290 Codex response/error normalization contract and merged config/runtime baseline.
