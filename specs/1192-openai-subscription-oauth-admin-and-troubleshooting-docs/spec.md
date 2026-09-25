# Feature Specification: HO-1192 OpenAI subscription OAuth admin and troubleshooting docs

**Feature Branch**: `HO-1192-openai-subscription-oauth-admin-and`
**Created**: 2026-05-09
**Status**: Draft for Todo scope review
**Input**: Linear HO-1192 `[Docs] OpenAI subscription OAuth admin and troubleshooting docs`

## Summary

建立一組 operator-facing 文件，說明 TianjiLLM 的 OpenAI subscription OAuth 如何設定、連線、管理 credential lifecycle、排除常見錯誤，並補上 OpenClaw 透過 TianjiLLM `/v1` 作為 OpenAI-compatible gateway 的設定範例。

本 issue 是 docs-only scope。Todo 階段只產生 SpecKit planning artifacts；README 或 `docs/` 的實際文件修改需等 Linear 進入 `In Progress`。

## User Stories and Testing

### User Story 1 - 管理員啟用 OpenAI subscription OAuth

身為 TianjiLLM operator，我要照文件設定 `general_settings.openai_oauth`、確認 callback URL、完成 OpenAI Connect，讓 subscription credential 被安全保存並可用於模型路由。

**Why**: OAuth setup 是後續所有 subscription routing 的入口，錯誤 callback 或未啟用設定會直接阻斷連線。

**Acceptance Criteria**:

1. 文件列出 `general_settings.openai_oauth.enabled`、`redirect_uri`、default issuer/client/scopes，以及 local callback `http://localhost:1455/auth/callback` 的限制。
2. 文件說明 UI Connect 需要 organization context，並說明 callback paste fallback 的使用時機。
3. 文件明確指出不需要 client secret，不使用 scraping，不把 OpenAI subscription access/refresh token 寫入 OpenClaw config。

### User Story 2 - 管理員維護 credential lifecycle

身為 TianjiLLM admin，我要能理解 `/ui/credentials` 的 Connect、Test、Refresh、Disable、Delete 行為，讓 credential 狀態可被安全操作與排錯。

**Why**: Subscription credential 會失效、refresh fail、被 disable 或被刪除；operator 需要知道每個動作的目的和安全邊界。

**Acceptance Criteria**:

1. 文件包含 `/ui/credentials` list/detail 入口與每個 lifecycle action 的使用目的。
2. 文件說明 Test 只驗證 credential 是否可用；Refresh 更新 token；Disable 停用但保留紀錄；Delete 移除 credential 關聯。
3. 文件列出 audit、安全遮罩、錯誤訊息不洩漏 token 的期望。

### User Story 3 - Operator 排除 subscription routing 錯誤

身為 operator，我要看到每個常見錯誤的原因、症狀、查證方法與修復動作，讓 401、refresh failed、all credentials disabled、no credential configured 可快速定位。

**Why**: 這些錯誤可能來自 OAuth config、credential lifecycle、model config、OpenClaw proxy key 或 endpoint surface mismatch。

**Acceptance Criteria**:

1. Troubleshooting matrix 包含 `401`、`refresh_failed`、all credentials disabled、no credential configured、missing org、callback expired、redirect mismatch、DB/cache unavailable。
2. 每列包含 symptom、likely cause、checks、operator action。
3. 文件區分 TianjiLLM API key authentication failure 與 OpenAI subscription OAuth failure。

### User Story 4 - OpenClaw 使用 TianjiLLM 作為 OpenAI-compatible gateway

身為 OpenClaw operator，我要能用 config examples 把 chat/reasoning、image generation、transparent image、TTS、STT、embeddings/memory 指向 TianjiLLM `/v1`，但不把 OpenAI subscription token 放進 OpenClaw。

**Why**: OpenClaw 只應持有 TianjiLLM/OpenClaw provider API key；OpenAI subscription token 必須留在 TianjiLLM credential store。

**Acceptance Criteria**:

1. OpenClaw examples 覆蓋 `models.providers.*` + `agents.defaults.model`、`agents.defaults.imageGenerationModel`、`messages.tts.providers.openai.baseUrl`、`tools.media.audio.models`、`agents.defaults.memorySearch`。
2. Examples 使用 `https://tianji.example.com/v1` 和 `${TIANJI_OPENCLAW_API_KEY}` placeholders，不包含任何 OpenAI subscription access token、refresh token 或 real secret。
3. 文件說明可直接使用 TianjiLLM `/v1` 的 surface：chat completions、responses、embeddings、images、audio transcriptions、audio speech。
4. 文件說明仍需 native OpenClaw auth/config 的 surface：OpenClaw plugins/connectors、browser/GitHub/Discord providers、ChatGPT backend/Codex profile 類非 OpenAI-compatible `/v1` protocol。

## Functional Requirements

- **FR-001**: 文件必須新增或更新 README/config examples，提供 OpenAI subscription OAuth minimal config 和 model routing config。
- **FR-002**: 文件必須明確說明 `general_settings.openai_oauth` 的 defaults：issuer、authorize URL、token URL、redirect URI、client ID、scopes、originator。
- **FR-003**: 文件必須說明 callback URL validation：absolute `http`/`https`、localhost HTTP only、localhost path `/auth/callback`。
- **FR-004**: 文件必須說明 `openai_subscription_credential_ids` 的 model config constraint：credential ID 不可空白、不可重複、不可與 custom `api_base` 並用、只支援 official OpenAI models。
- **FR-005**: 文件必須說明 credential lifecycle：Connect、Test、Refresh、Disable、Delete。
- **FR-006**: 文件必須說明 security boundary：OpenAI subscription tokens 只存在 TianjiLLM encrypted credential storage；OpenClaw examples 只使用 TianjiLLM provider API key placeholder。
- **FR-007**: 文件必須列出 troubleshooting matrix，並對每個 expected failure mode 對應 operator action。
- **FR-008**: 文件必須包含 OpenClaw chat/reasoning config using `models.providers.*` + `agents.defaults.model`。
- **FR-009**: 文件必須包含 OpenClaw image generation config using `agents.defaults.imageGenerationModel` with `openai/gpt-image-2`。
- **FR-010**: 文件必須說明 transparent PNG/WebP 應使用 `openai/gpt-image-1.5` model override 並搭配 transparent background capable image call。
- **FR-011**: 文件必須包含 OpenClaw TTS config using `messages.tts.providers.openai.baseUrl`。
- **FR-012**: 文件必須包含 OpenClaw STT config using `tools.media.audio.models` and TianjiLLM `/v1/audio/transcriptions` routing。
- **FR-013**: 文件必須包含 OpenClaw embeddings/memory config using `agents.defaults.memorySearch.remote.baseUrl` and provider key placeholder。
- **FR-014**: 文件必須說明哪些 OpenClaw surfaces 可直接使用 TianjiLLM `/v1`，哪些仍需 native OpenClaw provider auth/config。
- **FR-015**: 文件必須說明 contract docs/tests，包含 docs snippet validation、forbidden secret scan、troubleshooting coverage。
- **FR-016**: 不新增 OpenAPI generator，不改 production code，不新增 OAuth implementation behavior。

## Key Entities

- **OpenAI OAuth deployment config**: `general_settings.openai_oauth` fields and defaults。
- **OpenAI subscription credential**: TianjiLLM managed credential record holding subscription token material and lifecycle state。
- **Subscription routed model**: `model_list[].tianji_params` entry that references `openai_subscription_credential_ids`。
- **OpenClaw provider config**: OpenClaw model/media/memory config that points to TianjiLLM `/v1` using a TianjiLLM API key。
- **Troubleshooting row**: A documented failure mode with symptom, cause, checks, and operator action。

## Edge Cases

- OAuth enabled but cache/DB unavailable, state cannot be created or credential cannot be saved。
- Callback state is expired, replayed, mismatched, or missing code。
- Redirect URI is HTTP non-localhost or localhost path is not `/auth/callback`。
- Model has `openai_subscription_credential_ids` but also custom `api_base`。
- All configured subscription credentials are disabled, malformed, expired, missing, or refresh failed。
- OpenClaw points to TianjiLLM `/v1` but uses a native OpenAI subscription token instead of TianjiLLM API key。
- OpenClaw surface does not use OpenAI-compatible `/v1` and therefore cannot be routed through TianjiLLM by `baseUrl` alone。

## Non-Goals

- No production code changes in Todo planning.
- No OpenAPI generator changes.
- No new OAuth provider implementation.
- No storage or display of real token values in docs, examples, PR body, or comments.
- No UI mockscreen; this is a docs issue, not owner-facing layout work.

## Success Criteria

- **SC-001**: A new operator can follow docs to enable OAuth, connect a credential, and configure a model with `openai_subscription_credential_ids`.
- **SC-002**: A TianjiLLM admin can map each listed failure mode to a concrete operator action.
- **SC-003**: OpenClaw examples cover chat, image, transparent image, TTS, STT, and embeddings/memory with TianjiLLM `/v1`.
- **SC-004**: Secret scan finds no OpenAI subscription token examples or real-looking credential payloads in docs.
- **SC-005**: The PR diff is limited to HO-1192 SpecKit artifacts in Todo, and later docs implementation files in `README.md` / `docs/**` after `In Progress`.
