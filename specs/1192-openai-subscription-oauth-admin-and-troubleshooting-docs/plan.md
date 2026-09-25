# Implementation Plan: HO-1192 OpenAI subscription OAuth admin and troubleshooting docs

**Branch**: `HO-1192-openai-subscription-oauth-admin-and`
**Spec**: `specs/1192-openai-subscription-oauth-admin-and-troubleshooting-docs/spec.md`
**Linear**: `HO-1192`
**State Gate**: Todo planning only; implementation waits for Linear `In Progress`

## Technical Context

- Repository: `hohsiang-lab/tianjiLLM`
- Current docs surface: `README.md`, `docs/oauth-token-strategy.md`, `docs/request-flow.md`
- Existing implementation evidence:
  - `internal/config/config.go` defines `general_settings.openai_oauth` and `tianji_params.openai_subscription_credential_ids`.
  - `internal/config/openai_oauth.go` defines OpenAI OAuth defaults and redirect URI validation.
  - `internal/config/validate.go` rejects invalid subscription credential model config.
  - `internal/ui/routes.go`, `internal/ui/handler_openai.go`, and `internal/ui/handler_credentials.go` provide admin routes and lifecycle actions.
  - `internal/proxy/handler/openai_subscription_endpoints_test.go` verifies subscription routing for chat, embeddings, images, audio transcription, and audio speech endpoints.
- External references:
  - OpenAI official model docs for `gpt-image-2`, `gpt-image-1.5`, `gpt-4o-mini-tts`, `gpt-4o-transcribe`, `whisper-1`, `text-embedding-3-small`, `text-embedding-3-large`.
  - OpenClaw config schema for model providers, TTS, audio media models, and memory search remote embeddings.

## Docs Architecture

### Target files after `In Progress`

- `README.md`
  - Add a short OpenAI subscription OAuth setup pointer.
  - Add minimal model config snippet.
  - Link to detailed admin/troubleshooting docs.
- `docs/openai-subscription-oauth.md`
  - Full admin guide and security boundary.
  - OAuth config, callback, credential lifecycle, model routing.
- `docs/openclaw-tianjillm-gateway.md`
  - OpenClaw config examples for TianjiLLM `/v1`.
  - Surface compatibility matrix.
- Optional: `docs/request-flow.md`
  - Add link or small flow reference only if needed for discoverability.

Todo planning artifacts stay under `specs/1192-openai-subscription-oauth-admin-and-troubleshooting-docs/**`.

## Design Decisions

### Decision 1 - Keep OpenAI subscription tokens inside TianjiLLM

OpenClaw examples must use `${TIANJI_OPENCLAW_API_KEY}` as a TianjiLLM/OpenClaw provider key. They must not include OpenAI subscription access tokens, refresh tokens, ID tokens, or credential payload examples.

**Rationale**: TianjiLLM owns OAuth exchange, refresh, credential storage, and routing. OpenClaw is only a downstream client of TianjiLLM `/v1`.

### Decision 2 - Document OpenClaw `openai` provider routed to TianjiLLM `/v1`

Use provider id `openai` in examples where OpenClaw feature selectors expect model ids like `openai/gpt-image-2` or `openai/gpt-image-1.5`. Configure `models.providers.openai.baseUrl` as TianjiLLM `/v1` and `apiKey` as the TianjiLLM API key placeholder.

**Rationale**: The issue explicitly requires OpenClaw examples for `openai/gpt-image-2`, `openai/gpt-image-1.5`, `messages.tts.providers.openai.baseUrl`, and `tools.media.audio.models`.

### Decision 3 - Split TianjiLLM docs from OpenClaw gateway docs

Keep TianjiLLM admin/troubleshooting content separate from OpenClaw config examples, while cross-linking both.

**Rationale**: TianjiLLM operators and OpenClaw operators have different failure modes. Splitting prevents OpenClaw config from being mistaken as the place to store subscription tokens.

### Decision 4 - Use contract-style docs tests, not new generator

Verification should check fenced config snippets, troubleshooting coverage, forbidden secret patterns, and repo link consistency. No OpenAPI generator is needed.

**Rationale**: HO-1192 explicitly requests contract docs/tests and no new OpenAPI generator.

## Verification Strategy

- `git diff --check origin/main...HEAD`
- `git diff --name-only origin/main...HEAD`
- Secret scan over touched docs/spec files:
  - reject real-looking OpenAI subscription token strings.
  - reject env var examples that place subscription access or refresh tokens in docs.
  - allow placeholder `${TIANJI_OPENCLAW_API_KEY}` only.
- Docs contract check:
  - OAuth config snippet includes `general_settings.openai_oauth`.
  - model snippet includes `openai_subscription_credential_ids` and omits custom `api_base`.
  - OpenClaw snippet covers chat, image, transparent image, TTS, STT, embeddings/memory.
  - troubleshooting matrix covers all Linear-listed failure modes.

## Plan Review Notes

- Context7 MCP is not available in this runtime; source verification used repo source, OpenClaw live config schema, and official OpenAI docs search restricted to OpenAI domains.
- GitHub prior-art search is not required for this docs-only issue because implementation is documenting existing local behavior rather than adopting a new library pattern.
- No plan revisions were needed from external references; they confirm the endpoint/model names already required by HO-1192.

## Milestones

1. Todo: Create Traditional Chinese SpecKit artifacts, draft PR, move Linear to `Waiting`.
2. Waiting: Owner reviews scope in draft PR.
3. In Progress: Implement docs under `README.md` and `docs/**`; add docs verification.
4. In Review: Run issue review scene.
5. Waiting CI: Ready PR and wait for checks or docs-only no-CI gate.
6. Waiting Merge: CI green or no-CI verification complete, git status clean.
