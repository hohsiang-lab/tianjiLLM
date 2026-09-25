# Implementation Plan: HO-1464 Fix Tianji Responses image_url validation under output payloads

## Technical Context

- Repository: `hohsiang-lab/tianjiLLM`
- Language/runtime: Go
- Current base: `origin/main` at `05ae92c` (`fix(HO-1462): normalize image reference payloads`)
- Affected surfaces:
  - `internal/provider/chatgptcodex/transport.go`
  - `internal/provider/chatgptcodex/transport_test.go`
  - `internal/proxy/handler/responses.go`
  - `internal/proxy/handler/responses_codex_test.go`
  - `internal/proxy/handler/chatgpt_codex_wildcard_test.go` only if shared regression coverage requires it
  - `docs/openclaw-tianjillm-gateway.md`

## Current Repo Findings

- `NormalizeResponsesPayload` calls `normalizeResponsesPayload(..., validate=false)`; `MarshalResponsesPayload` calls the same normalization with `validate=true`.
- `normalizeResponsesPayload` handles `input` string conversion and `input` array traversal.
- `normalizeResponsesMessageMap` only normalizes `content`; it leaves any `output` key unchanged.
- `normalizeResponsesContentPartMap` already knows how to normalize `input_image` / `image_url` parts with string `image_url`, legacy object `image_url.url`, `file_id`, and sibling `detail`.
- `responseConversationPayload` can merge completed response `output` items into later `input` for WebSocket conversation state, so output items are on the live upstream request path.
- Existing HO-1462 coverage proves input content image parts are normalized, but it does not cover output image items.

## External Contract

OpenAI's Images and vision guide documents Responses image input as:

```json
{
  "type": "input_image",
  "image_url": "https://api.example/image.jpg",
  "detail": "original"
}
```

The same page says image inputs can be fully qualified URLs, Base64 data URLs, or file IDs. It shows `detail` as a sibling of `image_url`.

For this incident, the observed upstream validator required a Base64 image data URL at `input[63].output[1].image_url` and rejected a value without the `;base64` separator. Tianji should stop that request locally when it is malformed.

## Proposed Design

1. Generalize Responses image normalization so it can traverse image-bearing item collections, not only message `content`.
2. Reuse the existing `imageURLStringAndDetail` / `normalizeResponsesContentPartMap` behavior where possible, but report a path-aware safe error for output items.
3. Add explicit traversal for `output` arrays in Responses input items:
   - inspect each `output[]` item;
   - normalize or validate `image_url` when an item has image semantics;
   - leave non-image output items unchanged.
4. Validate after WebSocket `previous_response_id` expansion because stored prior output is merged into the next upstream payload.
5. Keep `NormalizeResponsesPayload(..., validate=false)` best-effort behavior for audit/log preparation, but ensure the `MarshalResponsesPayload(..., validate=true)` path fails before upstream.
6. Keep data URL redaction behavior unchanged in audit payloads.

## Test Strategy

- Provider-level regression in `internal/provider/chatgptcodex/transport_test.go`:
  - valid `input[].output[].image_url = data:image/png;base64,...` is preserved;
  - malformed `input[63].output[1].image_url` fails with a safe client-validation error;
  - non-image output items are unchanged.
- Handler-level HTTP regression in `internal/proxy/handler/responses_codex_test.go`:
  - malformed output image URL returns 400 and backend call count stays zero;
  - valid output image data URL reaches captured backend request.
- WebSocket regression in `internal/proxy/handler/responses_codex_test.go`:
  - previous-response expansion with output image items is validated on the expanded payload.
- E2E regression in `test/e2e/codex_subscription_route_test.go`:
  - DB-managed `openai/*` Codex Responses route rejects malformed `input[63].output[1].image_url` before any backend request;
  - valid output image data URLs reach the Codex backend route;
  - WebSocket `previous_response_id` expansion rejects malformed carried-forward output image URLs before a second backend request.
- Existing HO-1462 tests remain green.

## Verification Plan

```bash
go test ./internal/provider/chatgptcodex -run 'Responses|Image|Output|Payload' -count=1
go test ./internal/proxy/handler -run 'CreateResponse.*Codex|CreateResponseWebSocket.*Previous|Image' -count=1
go test ./internal/provider/chatgptcodex ./internal/proxy/handler -count=1
E2E_DATABASE_URL="postgres://tianji:tianji@localhost:5433/tianji_e2e?sslmode=disable" go test -tags e2e -count=1 -run 'CodexResponses.*OutputImage|CodexResponsesWebSocket.*OutputImage' ./test/e2e/...
git diff --check origin/main...HEAD
```

## Rollout Notes

- Expected blast radius is narrow: only ChatGPT Codex backend Responses payload preparation changes.
- The fix should turn a remote upstream 400 into a local 400 for malformed output image references.
- Valid previous-response image state should continue to work.
- No migration or config change is required.

## Out of Scope

- OpenClaw payload changes.
- Broad JSON schema validation for every Responses object.
- New provider config, pricing, or credential behavior.
- Direct Platform OpenAI proxy behavior that does not rebuild/normalize payloads.
