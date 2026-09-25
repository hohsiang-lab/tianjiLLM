# Analyze: HO-1192

## Inputs Reviewed

- Linear HO-1192 title, goal, scope, tests, labels, and state.
- Linear comments: none.
- Repo source for OpenAI OAuth config, validation, UI routes, callback handling, subscription endpoint coverage.
- OpenClaw live config schema for provider, TTS, audio media, and memory search fields.
- Official OpenAI model docs for image, audio, transcription, and embedding endpoint/model names.

## Consistency Checks

### Issue scope to spec

- README/config examples: covered by FR-001 and docs contract.
- Security docs: covered by FR-006 and snippet contract.
- Admin guide Connect/Test/Disable/Delete: covered by FR-005 and lifecycle data model.
- Troubleshooting: covered by FR-007 and troubleshooting contract.
- OpenClaw chat/reasoning: covered by FR-008.
- OpenClaw image generation: covered by FR-009.
- Transparent PNG/WebP: covered by FR-010.
- TTS: covered by FR-011.
- STT: covered by FR-012.
- Embeddings/memory: covered by FR-013.
- Surface compatibility: covered by FR-014 and quickstart compatibility section.
- Contract docs/tests, no generator: covered by FR-015 and FR-016.

### State gate

Current Linear state is `Todo`. The legal automatic gate is Todo planning to `Waiting` after SpecKit artifacts, analysis, draft PR, and owner notification. `Waiting Merge` is not reachable from Todo because implementation, review, and CI gates require later Linear states.

### Security

Examples use `${TIANJI_OPENCLAW_API_KEY}` and `https://tianji.example.com/v1`. No OpenAI subscription token value is included. The docs contract requires a forbidden-token scan before implementation PR review.

### UI mockscreen

Not applicable. HO-1192 is docs-only and does not introduce owner-facing UI layout changes.

## Findings

- Fatal: 0
- Critical: 0
- Minor: 0

## Scope Confirmation

The Todo planning scope is complete when these artifacts are committed and pushed in a draft PR. Implementation must stop at `Waiting` until Linear moves to `In Progress`.

## In Progress Implementation Evidence

- Linear state was rechecked as `In Progress` before editing implementation docs.
- Repo source rechecked:
  - `internal/config/openai_oauth.go` for OAuth defaults, PKCE flags, scopes, and callback validation.
  - `internal/ui/handler_credentials.go` and `internal/proxy/handler/openai_subscription_lifecycle.go` for Connect/Test/Refresh/Disable/Delete behavior.
  - `internal/proxy/server.go` and `internal/proxy/handler/openai_subscription_endpoints_test.go` for `/v1` endpoint coverage.
- OpenClaw schema rechecked through `gateway config.schema.lookup` for `models.providers.*`, `agents.defaults.imageGenerationModel`, `messages.tts.providers.*`, `tools.media.audio.models.*`, and `agents.defaults.memorySearch.remote`.
- Official OpenAI docs rechecked on 2026-05-09:
  - `gpt-image-2` is a current image model and is used for normal image generation examples.
  - Current image docs say `gpt-image-2` does not support transparent backgrounds, so transparent PNG/WebP examples route to `gpt-image-1.5`.
  - Speech-to-text docs confirm `/v1/audio/transcriptions` with `gpt-4o-transcribe` / `gpt-4o-mini-transcribe`.
  - Embedding docs confirm `text-embedding-3-small` / `text-embedding-3-large`.
