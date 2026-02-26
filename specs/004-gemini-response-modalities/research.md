# Research: Gemini Response Modalities

**Feature**: 004-gemini-response-modalities
**Date**: 2026-02-26

## Summary

This feature is a pure request/response transformation layer change — no new dependencies,
no DB schema changes. All changes are confined to `internal/provider/gemini/` and
`internal/model/request.go`.

---

## Decision 1: `modalities` as First-Class Field vs ExtraParams

**Decision**: Add `Modalities []string` as a first-class field on `ChatCompletionRequest`.

**Rationale**: Currently, `modalities` is not in `knownFields` in `request.go`, so it falls
into `ExtraParams`. Promoting it to a first-class field:
- Makes the intent explicit in the type
- Allows the Gemini provider to read it with a type-safe accessor
- Follows the pattern of other request params like `ResponseFormat`, `ToolChoice`

**Alternatives considered**:
- Read from `ExtraParams["modalities"]` → works but brittle; requires type casting at call site
- Provider-level hook to intercept unknown params → unnecessary abstraction

**Sources**: Existing pattern in `internal/model/request.go` (lines 14-31, `knownFields` map)

---

## Decision 2: `responseModalities` in `generationConfig`

**Decision**: When `Modalities` contains `"image"`, set `generationConfig.responseModalities`
to `["TEXT", "IMAGE"]`. When `modalities` contains only `"text"` (no image), set
`["TEXT"]`. When `modalities` is nil/empty, do NOT set `responseModalities` at all
(preserves existing behavior per FR-003).

**Rationale**: The Gemini API uses uppercase `"TEXT"` / `"IMAGE"` for `responseModalities`
(per spec assumption). Only injecting `responseModalities` when image output is requested
ensures zero change for existing text-only flows (SC-005: no latency overhead on common path).

**Gemini API reference** (from spec, assumption): `generationConfig.responseModalities: ["TEXT", "IMAGE"]`

---

## Decision 3: `inlineData` Struct in `geminiPart`

**Decision**: Add `InlineData *geminiInlineData` field to `geminiPart` struct.

```go
type geminiInlineData struct {
    MimeType string `json:"mimeType"`
    Data     string `json:"data"`
}

type geminiPart struct {
    Text         string            `json:"text,omitempty"`
    FunctionCall *geminiFuncCall   `json:"functionCall,omitempty"`
    InlineData   *geminiInlineData `json:"inlineData,omitempty"`
}
```

**Rationale**: Strongly typed struct for JSON marshaling/unmarshaling is more robust than
`map[string]any`. The `omitempty` tags ensure inlineData is omitted from requests that
don't include images (no wire overhead).

---

## Decision 4: Mixed Content Response Strategy

**Decision**: When any candidate part has `InlineData`, return `message.content` as
`[]model.ContentPart` (array). When all parts are text-only, return plain `string`.

**Data URL format**: `data:<mimeType>;base64,<data>` — matches the format that
`transformContentPart` already uses for input image parsing, and matches OpenAI's
convention for inline images.

**Implementation**: In `transformToOpenAI`, change the accumulation loop from
`strings.Builder` to tracking whether any `inlineData` part was seen, then decide
return type at the end.

```go
// When hasImage is true:
msg.Content = []model.ContentPart{...}   // typed array
// When hasImage is false (all text):
msg.Content = textBuilder.String()        // plain string — no regression
```

**Alternatives considered**:
- Always return array regardless of image → breaks SC-002 (existing text tests), FR-007
- Return array only for multi-part → same issue for single text responses

---

## Decision 5: Streaming `inlineData` Handling

**Decision**: Apply the same mixed-content logic to `ParseStreamChunk`. When a chunk
contains `inlineData`, set `delta.Content` to a JSON-encoded `image_url` content part.

**Concern**: The OpenAI streaming spec uses `delta.content` as a string. For image parts
in streaming, we emit a JSON-encoded content part array element as the delta content.
This is consistent with how GPT-4o handles image streaming.

**Note from spec edge case**: "Image data in streaming responses (if Gemini returns
`inlineData` in stream chunks) MUST be forwarded correctly." — Gemini may or may not
stream images as `inlineData` chunks; we handle it if it appears.

---

## Decision 6: Input `image_url` Bug Fix (bonus)

**Finding**: Existing `transformContentPart` for `image_url` input passes the raw URL
string as `inlineData.data`. For data URLs (`data:image/jpeg;base64,...`), this is
wrong — the `data` field should contain only the base64 portion, and `mimeType` should
be extracted from the data URL prefix.

**Decision**: Fix this as part of the same PR since it's in the same code path we're
touching. Extract MIME type and base64 data from data URLs.

**Status**: Minor bug fix bundled with this feature.

---

## Constitution Check Notes

- **Principle I (Python-First)**: Python TianjiLLM not available locally; Gemini API
  behavior confirmed from spec assertions and existing Go code patterns.
- **Principle VII (sqlc-First)**: No DB queries involved — N/A.
- **Principle IV (Test-Driven)**: Unit tests will cover all 3 mapping paths (text-only,
  image-only, mixed) as required by SC-004.

---

## Files to Modify

| File | Change |
|------|--------|
| `internal/model/request.go` | Add `Modalities []string` field + update `knownFields` |
| `internal/provider/gemini/gemini.go` | 1) Add `InlineData` to `geminiPart`; 2) `transformRequestBody` → inject `responseModalities`; 3) `transformToOpenAI` → handle mixed content; 4) Fix `transformContentPart` for data URLs |
| `internal/provider/gemini/stream.go` | Handle `inlineData` in `ParseStreamChunk` |
| `internal/provider/gemini/gemini_test.go` | Add tests for all new paths |

**No new files required.**
