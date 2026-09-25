# Tasks: HO-1464 Fix Tianji Responses image_url validation under output payloads

**Input**: Design documents from `/specs/1464-fix-tianji-responses-image-url-validation-under-output-payloads/`
**Prerequisites**: Linear state must move out of Todo before production implementation.

## Phase 1 - RED Coverage

- [x] T001 Re-read Linear HO-1464, this SpecKit package, HO-1462 artifacts, OpenAI image input docs, and affected Tianji Responses code.
- [x] T002 Add failing provider-level regression for valid `input[].output[].image_url = data:image/png;base64,...` preserving the string value.
- [x] T003 Add failing provider-level regression for malformed exact path `input[63].output[1].image_url` returning a safe validation error.
- [x] T004 Add failing provider-level regression proving non-image output items remain unchanged.
- [x] T005 Add failing HTTP handler regression proving malformed output image URL returns 400 before backend call.
- [x] T006 Add failing HTTP handler regression proving valid output image data URL reaches captured backend request.
- [x] T007 Add failing WebSocket regression proving `previous_response_id` expanded output image items are validated before upstream.
- [x] T007A Add E2E HTTP route regressions for malformed exact-path rejection and valid output image data URL forwarding.
- [x] T007B Add E2E WebSocket route regression proving malformed `previous_response_id` output image state is rejected before a second backend request.
- [x] T008 Add regression assertion that error/log payloads do not include raw data URL bytes or subscription secrets.

## Phase 2 - Output Traversal And Validation

- [x] T009 Refactor Responses normalization to traverse image-bearing output item arrays in addition to message content arrays.
- [x] T010 Reuse existing image URL string/object/file-id normalization semantics where valid.
- [x] T011 Reject malformed output `image_url` locally when `MarshalResponsesPayload(..., validate=true)` prepares an upstream request.
- [x] T012 Preserve valid Base64 image data URLs byte-for-byte.
- [x] T013 Preserve supported sibling `detail` fields.
- [x] T014 Leave function calls, custom tool calls, text outputs, and non-image output items unchanged.
- [x] T015 Ensure WebSocket `previous_response_id` expansion uses the same final validation path.

## Phase 3 - Regression Safety

- [x] T016 Verify HO-1462 `input[].content[]` image URL normalization tests still pass.
- [x] T017 Verify `/v1/images/edits` multipart image rebuild behavior stays string-shaped.
- [x] T018 Verify OpenAI subscription credential resolution/failover behavior is not changed.
- [x] T019 Verify direct non-Codex OpenAI proxy routes remain pass-through.

## Phase 4 - Docs

- [x] T020 Update `docs/openclaw-tianjillm-gateway.md` to state Tianji validates image references under Responses `content[]` and carried-forward `output[]` items.
- [x] T021 Document that malformed output image data URLs fail locally with an actionable 400 before upstream.

## Phase 5 - Verification

- [x] T022 Run `go test ./internal/provider/chatgptcodex -run 'Responses|Image|Output|Payload' -count=1`.
- [x] T023 Run `go test ./internal/proxy/handler -run 'CreateResponse.*Codex|CreateResponseWebSocket.*Previous|Image' -count=1`.
- [x] T024 Run `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -count=1`.
- [x] T025 Add `test/e2e/codex_subscription_route_test.go` coverage for the issue-owned HTTP and WebSocket routes.
- [x] T026 Run `git diff --check origin/main...HEAD`.
- [x] T027 Confirm PR diff contains implementation/tests/docs plus this SpecKit package only.
- [x] T028 Record verification evidence in PR, Linear, and Discord thread without secrets.

## Scope Stop

Implementation is complete after Linear moved out of Todo. Final PR/Linear evidence is recorded after CI passes.
