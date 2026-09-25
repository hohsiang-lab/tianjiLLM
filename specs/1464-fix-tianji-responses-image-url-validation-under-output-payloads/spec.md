# Specification: HO-1464 Fix Tianji Responses image_url validation under output payloads

## Scope

Fix TianjiLLM's ChatGPT Codex backend Responses payload normalization so image references nested under Responses output items are never forwarded upstream in a malformed image URL shape.

This issue owns validation and normalization for image payloads reachable from `/v1/responses` request bodies, including previous-response conversation state that may place prior assistant output items back into the next request input. It is separate from HO-1462, which fixed `input[].content[].image_url` object-vs-string normalization for reference-image edit inputs.

It does not change image generation model selection, subscription credential selection, WebSocket protocol framing, pricing, OpenClaw image generation config, or OpenAI subscription OAuth lifecycle.

## Problem

Remote Tianji pod logs showed repeated upstream 400s on `/v1/responses`:

```text
Invalid 'input[63].output[1].image_url'. Expected a base64-encoded data URL with an image MIME type (e.g. 'data:image/png;base64,aW1nIGJ5dGVzIGhlcmU='), but got a value without the ';base64' separator.
type=invalid_request_error
param=input[63].output[1].image_url
code=invalid_value
```

Observed request IDs included `tianji-bdbccbb97-2dlfv/5RiF7w93Gt-000353`, `000403`, `000405`, `000406`, and `000410` around 2026-05-16 17:22-17:24 Asia/Taipei. The pod was healthy during the error window, so this is a request-shape bug, not a Tianji restart.

Repo evidence:

- `internal/provider/chatgptcodex/transport.go` normalizes image parts only when they are under `input[].content[]`.
- `normalizeResponsesMessageMap` only looks for `content`; it does not traverse `output` arrays inside message/output items.
- `responseConversationPayload` merges completed response `output` items into later `input` when WebSocket `previous_response_id` is used.
- Existing HO-1462 tests cover `input[].content[].image_url` and image edit multipart rebuilds, but there is no regression for `input[63].output[1].image_url`.

Official OpenAI docs evidence:

- The Images and vision guide documents Responses image inputs as content parts with string `image_url`, for example `{"type":"input_image","image_url":"https://..."}`.
- The same guide says image inputs may be fully qualified URLs, Base64 data URLs, or file IDs.
- The guide shows `detail` as a sibling field beside `image_url`, not nested inside `image_url`.

Root cause chain: A Responses conversation payload can carry prior assistant output items forward under `input[].output[]`; Tianji normalizes only `input[].content[]`; a malformed output image item with `image_url` missing the `;base64` data URL separator survives local validation; ChatGPT Codex backend rejects `input[63].output[1].image_url` before model execution.

## User Stories

### US1 - Output image references are blocked before upstream when malformed (P1)

As a Tianji operator, I want malformed image references under Responses output items to fail locally, so Tianji does not proxy avoidable upstream 400s.

**Independent Test**: A handler or transport regression builds a payload whose effective upstream request contains `input[63].output[1].image_url` without `;base64`; Tianji returns a safe 400 before upstream is called.

### US2 - Valid output image data URLs remain accepted (P1)

As an OpenClaw/Codex caller, I want prior image outputs with legal Base64 data URLs to remain usable in later Responses turns.

**Independent Test**: A regression includes `input[].output[].image_url = "data:image/png;base64,..."` and proves the captured upstream payload preserves the string value byte-for-byte.

### US3 - Previous-response WebSocket state is validated after expansion (P1)

As a Tianji maintainer, I want WebSocket `previous_response_id` expansion to be validated on the expanded payload, so stored output items cannot bypass request validation.

**Independent Test**: A WebSocket test stores a previous response containing an output image item, then sends a `previous_response_id` turn; malformed output image URLs fail locally and valid ones are forwarded in normalized shape.

### US4 - HO-1462 input image normalization does not regress (P2)

As a maintainer, I want the new traversal to reuse the existing image normalization rules, so `input[].content[]`, `/v1/images/edits`, file IDs, and legacy object-shaped client inputs keep working.

**Independent Test**: Existing HO-1462 tests stay green, and a focused regression proves `input[].content[].image_url` still normalizes as before.

## Functional Requirements

- **FR-001**: Tianji MUST inspect Responses image payloads under `input[].output[]` before forwarding to the ChatGPT Codex backend.
- **FR-002**: Tianji MUST reject malformed `input[].output[].image_url` values locally with a safe `invalid_request_error`/client-validation 400 before any upstream request is made.
- **FR-003**: Tianji MUST accept and preserve valid `data:image/<mime>;base64,<bytes>` values under output image items.
- **FR-004**: Tianji MUST preserve fully qualified HTTPS image URLs and file IDs for `input[].content[]` image parts where the upstream item type supports them; carried-forward `input[].output[]` image items are limited to valid `data:image/<mime>;base64,<bytes>` URLs for this incident scope.
- **FR-005**: Tianji MUST keep `detail` as a sibling field when present; it MUST NOT nest `detail` inside `image_url`.
- **FR-006**: Tianji MUST keep existing `input[].content[]` normalization behavior from HO-1462 unchanged.
- **FR-007**: Tianji MUST validate the final expanded WebSocket payload after `previous_response_id` merge, not only the incremental frame.
- **FR-008**: Tianji MUST produce regression coverage that names or asserts the exact failure path `input[63].output[1].image_url`.
- **FR-009**: Tianji MUST NOT introduce logging of raw bearer tokens, subscription credentials, or full image data URL payloads.
- **FR-010**: Tianji MUST keep non-image Responses items, tool calls, function call outputs, text output items, and regular chat completions behavior unchanged.

## Non-Goals

- Do not implement production code while HO-1464 is in Todo.
- Do not change OpenClaw client payload generation.
- Do not change OpenAI subscription OAuth, credential refresh, failover ordering, attribution, or account selection.
- Do not add image URL fetching, MIME sniffing, transcoding, file upload, or image hosting.
- Do not change `/v1/images/generations`, `/v1/images/edits`, or `/v1/images/variations` behavior except insofar as shared tests must stay green.
- Do not broaden this into a full Responses schema validator.

## Success Criteria

- **SC-001**: A payload reproducing `input[63].output[1].image_url` without `;base64` fails locally with 400 and zero upstream calls.
- **SC-002**: A valid output image data URL under `input[].output[]` is forwarded as a string without mutation.
- **SC-003**: WebSocket previous-response expansion cannot bypass output image validation.
- **SC-004**: Existing HO-1462 `input[].content[]` image URL normalization tests remain green.
- **SC-005**: Focused handler/provider tests for Responses/Codex/image payloads pass.
- **SC-006**: Docs identify `input[].output[]` as covered by the same image URL validation boundary.
- **SC-007**: E2E route coverage proves the DB-managed `openai/*` Codex Responses HTTP/WebSocket paths enforce the output-image validation boundary.

## Open Questions

None for implementation scope. The issue description, remote error path, repo traversal gap, and OpenAI docs are sufficient to proceed after owner scope confirmation.
