# Tasks: Inject OpenAI Subscription Auth Across Official Endpoints

**Input**: `spec.md`, `plan.md`, `research.md`, `data-model.md`

## Phase 1: Test Baseline

- [x] T001 Identify concrete model/test upstream path for official OpenAI model health/list behavior.
- [x] T002 Add failing subscription-auth test for `/v1/chat/completions` non-streaming and streaming.
- [x] T003 Add failing subscription-auth test for `/v1/completions`.
- [x] T004 Add failing subscription-auth test for `/v1/responses`.
- [x] T005 Add failing subscription-auth test for `/v1/embeddings`.
- [x] T006 Add failing subscription-auth test for `/v1/images/generations`.
- [x] T007 Add failing subscription-auth tests for `/v1/images/edits` and `/v1/images/variations`.
- [x] T008 Add failing subscription-auth tests for `/v1/audio/transcriptions` and `/v1/audio/speech`.
- [x] T009 Add failing subscription-auth/API-key/custom-base tests for the concrete model/test path.
- [x] T010 Add negative assertions that client virtual key and fallback API key do not reach upstream when subscription IDs are configured.

## Phase 2: Shared Auth Resolution

- [x] T011 Add a request-context-aware helper for official OpenAI endpoint auth resolution if existing helpers are insufficient.
- [x] T012 Ensure the helper returns subscription bearer material for official OpenAI configs and API-key material for omitted/empty subscription IDs.
- [x] T013 Ensure subscription resolution errors are returned safely and do not fallback to API key.
- [x] T014 Ensure custom `api_base` / OpenAI-compatible providers are excluded.

## Phase 3: Endpoint Integration

- [x] T015 Wire chat completions to the shared subscription-aware auth path without breaking router fallback.
- [x] T016 Wire legacy completions to the same auth behavior.
- [x] T017 Replace or split `/v1/responses` proxy auth so it is subscription-aware without broad Assistants scope changes.
- [x] T018 Verify embeddings inherits the auth helper or patch it narrowly.
- [x] T019 Verify image generation inherits the auth helper or patch it narrowly.
- [x] T020 Patch image edit/variation native-format auth path if needed.
- [x] T021 Verify audio transcription/speech inherit the auth helper or patch them narrowly.
- [x] T022 Patch model/test upstream auth path.

## Phase 4: Verification

- [x] T023 Run targeted endpoint auth tests.
- [x] T024 Run `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/...`.
- [x] T025 Run `git diff --check origin/main...HEAD`.
- [x] T026 Update PR body with endpoint matrix and test evidence.

## Dependencies

- T001 before T009/T022.
- T002-T010 before T011-T022.
- T011-T014 before endpoint integration unless a test proves an endpoint already uses the correct path.
- T023-T026 after all endpoint tasks.

## Verification Evidence

- `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/... -run 'OpenAI|Subscription|Chat|Completion|Response|Embedding|Image|Audio|Model'` — pass
- `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/...` — pass
- `make lint` — pass
- `git diff --check origin/main...HEAD` — pass
