# Data Model: Gemini Response Modalities

**Feature**: 004-gemini-response-modalities

## Overview

No new DB tables or migrations. All changes are Go struct fields used for
in-memory request/response transformation.

---

## 1. Modified: `model.ChatCompletionRequest`

**File**: `internal/model/request.go`

```go
type ChatCompletionRequest struct {
    // ... existing fields ...
    Modalities []string `json:"modalities,omitempty"` // NEW: ["text", "image"]
}
```

**Validation rules**:
- `nil` / absent → text-only mode (no responseModalities sent to Gemini)
- `[]` (empty) → same as nil; ignored per spec edge case
- `["text"]` → only TEXT modality
- `["text", "image"]` → TEXT + IMAGE modalities
- `["image"]` → only IMAGE modality (valid; Gemini may handle)

**knownFields update**: Add `"modalities": true` to the `knownFields` map in
`UnmarshalJSON` so it is not captured into `ExtraParams`.

---

## 2. Modified: `geminiPart` (internal to gemini package)

**File**: `internal/provider/gemini/gemini.go`

```go
// New type added:
type geminiInlineData struct {
    MimeType string `json:"mimeType"`
    Data     string `json:"data"`
}

// Modified type:
type geminiPart struct {
    Text         string            `json:"text,omitempty"`
    FunctionCall *geminiFuncCall   `json:"functionCall,omitempty"`
    InlineData   *geminiInlineData `json:"inlineData,omitempty"` // NEW
}
```

**Invariants**:
- Exactly one of `Text`, `FunctionCall`, `InlineData` is non-zero per part
- `InlineData.Data` contains raw base64 (no data: URL prefix)
- `InlineData.MimeType` is the MIME type string (e.g., `"image/png"`)

---

## 3. Mapping: OpenAI `modalities` → Gemini `responseModalities`

```
OpenAI request           Gemini generationConfig
─────────────────────    ──────────────────────────────
nil / []                 (field absent — no change)
["text"]                 responseModalities: ["TEXT"]
["image"]                responseModalities: ["IMAGE"]
["text", "image"]        responseModalities: ["TEXT", "IMAGE"]
["image", "text"]        responseModalities: ["TEXT", "IMAGE"]  (order normalized)
```

**Normalization**: For safety, always output TEXT before IMAGE when both present.
This avoids potential Gemini API sensitivity to ordering.

---

## 4. Mapping: Gemini `inlineData` → OpenAI `image_url` ContentPart

```
Gemini response part     OpenAI ContentPart
────────────────────     ──────────────────────────────────────────
{text: "..."}            {type: "text", text: "..."}
{inlineData: {           {type: "image_url", image_url: {
  mimeType: "image/png",   url: "data:image/png;base64,<data>"
  data: "<base64>"       }}
}}
```

---

## 5. Mapping: Response `message.content` Type Decision

```
Gemini candidate parts       OpenAI message.content type
──────────────────────────   ───────────────────────────
All parts are text           string  (plain, no regression)
Any part is inlineData       []ContentPart  (array)
Mix of text + inlineData     []ContentPart  (array, order preserved)
```

---

## 6. `model.ContentPart` (existing, unchanged)

**File**: `internal/model/request.go` (lines 123-132)

```go
type ContentPart struct {
    Type     string    `json:"type"`
    Text     string    `json:"text,omitempty"`
    ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
    URL    string  `json:"url"`
    Detail *string `json:"detail,omitempty"`
}
```

No changes needed — existing type is sufficient.

---

## State Transitions

Not applicable. This feature is stateless request/response transformation only.

---

## No DB Schema Changes

This feature has zero database impact. No migrations, no sqlc queries, no schema files
to add.
