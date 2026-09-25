# Scope Confirmation: HO-1464

## Decision

Implemented after owner confirmation/state transition to In Progress with a narrow TianjiLLM Responses/Codex payload-validation fix for image references nested under `input[].output[]`.

## Confirmed Scope

- Add validation/normalization coverage for `input[].output[].image_url`.
- Reproduce the observed path `input[63].output[1].image_url`.
- Convert malformed upstream 400s into local actionable 400s before backend calls.
- Preserve valid `data:image/<mime>;base64,<bytes>` values.
- Preserve HO-1462 `input[].content[].image_url` behavior.
- Validate WebSocket `previous_response_id` expanded payloads after merge.
- Keep docs updated for the expanded Responses image validation boundary.

## Explicitly Out Of Scope

- Production code while Linear is Todo.
- OpenClaw client changes.
- OpenAI subscription OAuth/credential/failover changes.
- Model/pricing/config changes.
- Image fetching, transcoding, MIME sniffing, upload, or hosting.
- Full Responses schema validation beyond the image URL boundary needed here.

## Gate Analysis

- Fatal: 0
- Critical: 0
- Owner input required: 0

## Evidence Used

- Linear HO-1464 description and remote upstream error path.
- Repo `internal/provider/chatgptcodex/transport.go`: content-only image normalization and `MarshalResponsesPayload` validation path.
- Repo `internal/proxy/handler/responses.go`: WebSocket previous-response expansion stores completed output back into later input.
- HO-1462 SpecKit and tests: existing coverage for `input[].content[].image_url`.
- OpenAI Images and vision docs: Responses image inputs use string `image_url`, support Base64 data URLs/file IDs/URLs, and put `detail` beside `image_url`.

## Scope Statement

HO-1464 should implement a path-aware extension of the existing HO-1462 image URL normalization boundary, covering carried-forward Responses output items without changing unrelated Responses, credential, or image-generation behavior.

## Implementation Evidence

- `internal/provider/chatgptcodex/transport.go` now traverses Responses `output[]` items and direct carried-forward image items in addition to `content[]`.
- Malformed output image data URLs fail during `MarshalResponsesPayload(..., validate=true)` before upstream request construction.
- HTTP regression covers the exact observed `input[63].output[1].image_url` path.
- WebSocket regression covers `previous_response_id` expansion; the existing conversation merge flattens prior completed `output[]` into later `input[]`, so the final local validation path is `input[2].image_url`.
