# Specification: HO-1462 Fix Tianji image reference edit payload image_url type

## Scope

Fix TianjiLLM OpenAI subscription routing so reference-image edit requests sent through the OpenAI-compatible Responses/Image path use an upstream-compatible image reference shape.

This issue owns request-shape normalization for image content parts that Tianji forwards or rebuilds for OpenAI subscription routes, plus regression coverage proving reference edit payloads no longer send `image_url` as an object when the upstream OpenAI API expects a string.

It does not change image model selection, credential selection, pricing, background/transparency fallback behavior, or OpenClaw client-side image generation configuration.

## Problem

Reference image edit through Tianji fails against OpenAI `gpt-image-2` with:

```text
OpenAI image edit failed (HTTP 400): Invalid type for 'input[0].content[1].image_url': expected an image URL, but got an object instead. [type=invalid_request_error, code=invalid_type]
```

Text-only image generation succeeds on the same provider/model route, so the failure is isolated to the image-reference payload shape.

Repo evidence:

- `internal/proxy/handler/responses.go` routes `/v1/responses` either through raw OpenAI endpoint proxying or the ChatGPT Codex backend transport when the subscription route selects that transport.
- `internal/provider/chatgptcodex/payload.go` maps image content parts to `ContentPart{Type: "input_image", ImageURL: *model.ImageURL}`, which serializes `image_url` as an object like `{"url":"...","detail":"..."}`.
- The same mapper only accepts incoming object-shaped `image_url`; it does not accept or preserve official Responses-style `image_url` strings.
- `internal/proxy/handler/responses_codex_test.go` and `internal/proxy/handler/chatgpt_codex_wildcard_test.go` are the right handler-level locations for captured outbound request body assertions on `/v1/responses` and `/v1/images/edits`.

Official OpenAI docs evidence:

- OpenAI Images/Vision docs show Responses image input as `{"type":"input_image","image_url":"https://..."}`.
- The same docs state image inputs can be fully qualified URLs, Base64 data URLs, or file IDs, and multiple images may appear in the content array.

Root cause chain: OpenClaw sends a reference image edit request through Tianji; Tianji's OpenAI subscription/Codex response payload path rebuilds image content using a Chat Completions-style `image_url.url` object; OpenAI Responses validates `input[].content[].image_url` as a string for `input_image`; upstream rejects the object before model execution.

## User Stories

### US1 - Reference URL edit reaches OpenAI with string image_url (P1)

As an OpenClaw operator, I want Tianji to forward a reference image URL edit with `image_url` as a string, so `openai/gpt-image-2` reference edits do not fail schema validation.

**Independent Test**: A handler/transport test sends a Responses image-generation edit payload containing a text part and an `input_image` part with a URL; the captured upstream request body contains `"image_url":"https://..."`, not `"image_url":{"url":"..."}`.

### US2 - Data URL references remain supported (P1)

As an image generation caller, I want Base64 data URL references to keep working, so local/reference image edits do not regress when images are inlined.

**Independent Test**: A regression test sends an `input_image` data URL and proves upstream receives the same data URL as a string.

### US3 - Legacy object-shaped client payloads are tolerated at Tianji boundary (P2)

As a Tianji maintainer, I want existing object-shaped `image_url.url` inputs to be normalized instead of rejected, so callers using Chat Completions-style image parts do not break during migration.

**Independent Test**: A unit test sends `{"type":"input_image","image_url":{"url":"https://...","detail":"high"}}` and proves the upstream payload uses string `image_url` while preserving supported `detail` separately when the upstream schema allows it.

### US4 - Unsupported image references fail before upstream with a safe error (P2)

As a Tianji maintainer, I want malformed image parts to fail locally with a clear non-secret error, so upstream 400s do not hide client payload bugs.

**Independent Test**: A unit test sends `input_image` without a string URL/file ID/data URL and asserts Tianji returns a redacted `invalid_request_error` without forwarding the request.

## Functional Requirements

- **FR-001**: Tianji MUST normalize OpenAI Responses `input_image.image_url` to a string before forwarding/rebuilding payloads for OpenAI subscription routes.
- **FR-002**: Tianji MUST accept official string-shaped `image_url` inputs.
- **FR-003**: Tianji SHOULD continue accepting legacy object-shaped `image_url.url` inputs at its client boundary and normalize them to the official upstream string shape.
- **FR-004**: Tianji MUST preserve valid image URLs and Base64 data URLs byte-for-byte except for JSON shape normalization.
- **FR-005**: Tianji MUST NOT send `image_url` as a JSON object for OpenAI Responses `input_image` content.
- **FR-006**: Tianji MUST preserve `detail` when present and supported by the target upstream payload schema; it MUST NOT bury `detail` under `image_url` when upstream expects it beside `image_url`.
- **FR-007**: Tianji MUST keep text-only image generation and non-image Responses requests behavior unchanged.
- **FR-008**: Tianji MUST keep `/v1/images/generations` and `/v1/images/variations` endpoint routing behavior unchanged; `/v1/images/edits` MUST use string-shaped `input_image.image_url` when it rebuilds multipart image references into the Codex Responses payload.
- **FR-009**: Tianji MUST keep OpenAI subscription credential resolution, failover, refresh, attribution, and redaction behavior unchanged.
- **FR-010**: Malformed image references MUST fail with a safe `invalid_request_error` before upstream forwarding when Tianji is rebuilding the payload.
- **FR-011**: Regression coverage MUST capture the actual outbound JSON body sent to upstream for URL and data URL reference-image edits.
- **FR-012**: Documentation MUST clarify Tianji accepts both legacy object-shaped client input and official string-shaped Responses input, but forwards official string-shaped `image_url` upstream.

## Non-Goals

- Do not change the default image model from `gpt-image-2`.
- Do not add transparent background fallback logic.
- Do not change OpenClaw config or provider selection.
- Do not alter OpenAI subscription OAuth lifecycle, refresh, or account selection.
- Do not implement file upload/download, URL fetching, image MIME validation, or image transcoding.
- Do not convert this into a broader multimodal payload rewrite beyond image reference content parts.
- Do not change OpenClaw client code; Tianji must accept OpenClaw's current Codex Responses image request shape.

## Success Criteria

- **SC-001**: A reference image URL edit through Tianji reaches upstream with `input[].content[].image_url` as a JSON string.
- **SC-002**: A reference image data URL edit reaches upstream with the data URL as a JSON string.
- **SC-003**: Legacy `image_url.url` object input is normalized locally; it is never forwarded upstream as an object.
- **SC-004**: Existing text-only image generation and Responses routing tests remain green.
- **SC-005**: OpenAI subscription credential/failover tests remain green.
- **SC-006**: Docs and tests cite the OpenAI Responses image input shape so future changes do not reintroduce the Chat Completions object shape.

## Open Questions

None. The issue statement, OpenClaw client code, repo evidence, and OpenAI docs evidence are sufficient to scope implementation.
