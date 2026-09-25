# Tasks: HO-1462 Fix Tianji image reference edit payload image_url type

**Input**: Design documents from `/specs/1462-fix-tianji-image-reference-edit-payload-image-url/`
**Prerequisites**: Linear state moved out of Todo before production implementation.

## Phase 1 - RED Coverage

- [x] T001 Re-read Linear HO-1462, this SpecKit package, OpenAI image input docs, OpenClaw image generator code, and affected Tianji payload code before implementation.
- [x] T002 Add failing `chatgptcodex` payload unit test proving official `input_image.image_url` string input remains string-shaped after mapping.
- [x] T003 Add failing `chatgptcodex` payload unit test proving legacy `image_url.url` object input normalizes to string-shaped `image_url`.
- [x] T004 Add failing `chatgptcodex` payload unit test proving `detail` is preserved beside `image_url`, not nested inside it.
- [x] T005 Add failing data URL regression test proving `data:image/...;base64,...` remains byte-for-byte in string `image_url`.
- [x] T006 Add failing malformed image part test proving non-string/missing URL returns a safe unsupported-payload or invalid-request error before upstream forwarding.
- [x] T007 Add failing handler/transport regression test capturing outbound OpenAI subscription Responses body for reference URL edit and asserting `image_url` is a JSON string.
- [x] T008 Add failing handler/transport regression test capturing outbound body for data URL reference edit.
- [x] T009 Add negative regression test asserting outbound body does not contain `"image_url":{"url":` for Responses `input_image` parts.

## Phase 2 - Payload Normalization

- [x] T010 Replace outbound Responses image content representation so `image_url` serializes as string.
- [x] T011 Map official string-shaped `input_image.image_url` content parts.
- [x] T012 Map legacy object-shaped `image_url.url` content parts.
- [x] T013 Preserve supported `detail` beside `image_url`.
- [x] T014 Keep text content mapping unchanged.
- [x] T015 Keep non-image Responses fields such as model, stream, temperature, metadata, and max output tokens unchanged.
- [x] T016 Return safe local errors for malformed image references when the payload is rebuilt.

## Phase 3 - Endpoint Safety

- [x] T017 Verify raw OpenAI endpoint proxy paths that do not rebuild payloads remain pass-through.
- [x] T018 Verify `/v1/images/generations` behavior remains unchanged.
- [x] T019 Verify `/v1/images/edits` uses string `image_url`; `/v1/images/variations` routing remains unchanged.
- [x] T020 Verify OpenAI subscription credential failover still retries with the same normalized request body.
- [x] T021 Verify no token, bearer value, or image data URL is newly logged by the normalization path.

## Phase 4 - Docs

- [x] T022 Update `docs/openclaw-tianjillm-gateway.md` with reference-image payload compatibility notes.
- [x] T023 Cite OpenAI Responses image input string `image_url` shape in test names/comments or docs.

## Phase 5 - Verification

- [x] T024 Run `go test ./internal/provider/chatgptcodex -run 'Payload|Responses|Image' -count=1`.
- [x] T025 Run `go test ./internal/proxy/handler -run 'OpenAISubscriptionRouting.*Image|OpenAISubscriptionRouting.*Responses|CreateResponse|ChatGPTCodexBackendImage' -count=1`.
- [x] T026 Run `go test ./internal/proxy/handler -run 'OpenAISubscription' -count=1`.
- [x] T027 Run `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -count=1`.
- [x] T028 Run `git diff --check`.
- [x] T029 Confirm PR diff contains production/test/docs files plus this SpecKit package only.
- [x] T030 Record verification evidence in PR and Linear/thread without secrets.

## Scope Stop

Production implementation is complete after Linear moved out of Todo. Final PR/Linear evidence is recorded after CI passes.
