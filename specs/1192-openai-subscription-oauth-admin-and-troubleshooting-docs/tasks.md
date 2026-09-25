# Tasks: HO-1192 OpenAI subscription OAuth admin and troubleshooting docs

**Input**: `spec.md`, `plan.md`, `research.md`, `data-model.md`, `quickstart.md`, `contracts/docs-contract.md`
**State Gate**: Tasks are implementation work and remain unchecked until Linear moves to `In Progress`.

## Phase 1: Documentation Structure

- [x] T001 Create `docs/openai-subscription-oauth.md` with admin setup, lifecycle, security, and troubleshooting sections.
- [x] T002 Create `docs/openclaw-tianjillm-gateway.md` with OpenClaw config examples and surface compatibility matrix.
- [x] T003 Update `README.md` with a short OpenAI subscription OAuth pointer, minimal config, and docs links.

## Phase 2: TianjiLLM OAuth Admin Guide

- [x] T004 Document `general_settings.openai_oauth` fields, defaults, callback URL rules, and no-client-secret boundary.
- [x] T005 Document Connect flow, organization requirement, state/callback behavior, and callback paste fallback.
- [x] T006 Document credential lifecycle actions: Test, Refresh, Disable, Delete.
- [x] T007 Document model config using `openai_subscription_credential_ids`, including no custom `api_base` and official OpenAI model constraints.
- [x] T008 Document token storage and redaction boundary: subscription tokens stay in TianjiLLM credential storage.

## Phase 3: Troubleshooting

- [x] T009 Add troubleshooting matrix for TianjiLLM 401, upstream 401, `refresh_failed`, all credentials disabled, no credential configured, missing/wrong type credential, redirect URI rejection, callback expiry/replay, DB/cache dependency failure, and OpenClaw unsupported surface.
- [x] T010 Map each troubleshooting row to a concrete operator action and safe retry guidance.

## Phase 4: OpenClaw Gateway Examples

- [x] T011 Add chat/reasoning example using `models.providers.openai.baseUrl` and `agents.defaults.model`.
- [x] T012 Add image generation example using `agents.defaults.imageGenerationModel` and `openai/gpt-image-2`.
- [x] T013 Add transparent PNG/WebP guidance using `openai/gpt-image-1.5` model override.
- [x] T014 Add TTS example using `messages.tts.providers.openai.baseUrl`.
- [x] T015 Add STT example using `tools.media.audio.models` and TianjiLLM `/v1/audio/transcriptions`.
- [x] T016 Add embeddings/memory example using `agents.defaults.memorySearch.remote.baseUrl`.
- [x] T017 Add surface compatibility section distinguishing OpenAI-compatible `/v1` surfaces from native OpenClaw provider auth/config.

## Phase 5: Verification

- [x] T018 Run markdown/diff sanity checks for touched docs.
- [x] T019 Run forbidden-secret scan over README, docs, and HO-1192 SpecKit artifacts.
- [x] T020 Run coverage scan for required keywords and config surfaces.
- [x] T021 Update PR body with docs summary, verification commands, and no-real-token note.

## Verification Evidence

- `git diff --check`
- Forbidden-secret scan from the docs contract returned no matches over README, the two HO-1192 docs, and HO-1192 artifacts.
- Coverage scan confirmed `openai_subscription_credential_ids`, `models.providers.openai.baseUrl`, `agents.defaults.model`, `agents.defaults.imageGenerationModel`, `messages.tts.providers.openai`, `tools.media.audio.models`, `agents.defaults.memorySearch`, `gpt-image-2`, `gpt-image-1.5`, `refresh_failed`, disabled credentials, and no credential configured docs coverage.
