# Technical Review: 004-gemini-response-modalities Plan

**Reviewer**: 魯班 (Eng)  
**Date**: 2026-02-26  
**Scope**: plan.md technical feasibility against existing codebase

---

## ✅ Agreed / Looks Good

1. **Overall approach** — Confining changes to 4 files is correct. The scope is minimal and well-contained.

2. **`Modalities` field on `ChatCompletionRequest`** — Adding `[]string` with `json:"modalities,omitempty"` + `knownFields` entry is straightforward and correct.

3. **`geminiInlineData` struct + `InlineData` on `geminiPart`** — Clean design, matches Gemini API shape. Using a pointer (`*geminiInlineData`) with `omitempty` is the right call.

4. **`transformRequestBody` modalities injection** — The `strings.ToUpper` loop to convert `["text", "image"]` → `["TEXT", "IMAGE"]` is simple and correct. Adding to `genConfig` map means `len(genConfig) > 0` will be true, so the map gets attached to `body`. No issue.

5. **`transformToOpenAI` branching on `hasImage`** — The approach of accumulating both `textParts` and `contentParts`, then branching on `hasImage` to decide string vs `[]ContentPart`, is sound and preserves backward compatibility (FR-007). `Message.Content` is typed `any`, so both forms marshal correctly.

6. **`transformContentPart` data URL fix** — The current code is genuinely broken for base64 data URLs (passes the full `data:image/jpeg;base64,...` as `inlineData.data` instead of just the base64 payload). The plan's fix with prefix stripping is correct and necessary. Good catch.

7. **Test plan** — Covering all 3 content paths (text-only, image-only, mixed) plus request transformation is the right set. Backward compat regression test is essential.

---

## ⚠️ Suggestions

### S1. FR-003 contradiction — `modalities: ["text"]` should NOT set `responseModalities`

The plan's code:
```go
if len(req.Modalities) > 0 {
    // sets responseModalities
}
```

This fires for `modalities: ["text"]`, which would set `responseModalities: ["TEXT"]` upstream. But **FR-003** explicitly says:

> When `modalities` contains only `"text"` (or is absent), the proxy MUST NOT set `responseModalities`.

**Suggestion**: Guard on whether `"image"` (or any non-text modality) is present:
```go
if hasNonTextModality(req.Modalities) {
    modalities := make([]string, 0, len(req.Modalities))
    for _, m := range req.Modalities {
        modalities = append(modalities, strings.ToUpper(m))
    }
    genConfig["responseModalities"] = modalities
}
```
Or simply check `slices.Contains(req.Modalities, "image")`.

### S2. Streaming: `Delta.Content` is `*string` — can't hold structured image data

The plan proposes:
```go
imgPart := model.ContentPart{...}
b, _ := json.Marshal(imgPart)
imgStr := string(b)
delta.Content = &imgStr  // JSON object stuffed into a string field
```

**Problem**: `Delta.Content` is `*string` and clients expect it to be a plain text fragment per the OpenAI streaming contract. Stuffing a JSON-encoded `ContentPart` object into it will confuse any standard OpenAI SDK consumer — they'll get a raw JSON string like `{"type":"image_url","image_url":{"url":"data:..."}}` as if it were text content.

**Options**:
1. **Extend `Delta` to support `any` content** (like `Message.Content` already does) — cleanest but bigger change.
2. **Don't stream images through `delta.content`** — accumulate image parts and emit them only in the final chunk or as a separate field. Check how Gemini actually delivers image data in SSE (it may come as a single final chunk, not incremental).
3. **At minimum**, document this as a known limitation and decide whether streaming + image output is actually a required path for the initial implementation.

This is the biggest design gap in the plan.

### S3. Streaming loop overwrites `delta.Content` — last part wins

In the streaming handler, the plan loops over parts:
```go
for _, part := range candidate.Content.Parts {
    if part.InlineData != nil {
        delta.Content = &imgStr  // overwrites
    }
    if part.Text != "" {
        delta.Content = &part.Text  // overwrites
    }
}
```

If a single chunk contains multiple parts (text + image, or multiple images), only the last one survives. The existing code has the same issue for text (but Gemini typically sends one text part per chunk). For mixed content this becomes a real problem.

**Suggestion**: Either accumulate parts into a slice, or acknowledge that Gemini sends one part per streaming chunk and add a comment + test to validate that assumption.

### S4. `transformContentPart` fallback for non-data-URL images

The plan's fix handles `data:` URLs but falls through to using the raw URL as `data` for `https://` image URLs. The current code has the same issue. While not in scope, consider at minimum adding a comment like `// TODO: support fetching remote URLs` so it doesn't get forgotten.

### S5. Add `"modalities"` to `GetSupportedParams()`

The plan doesn't mention updating `GetSupportedParams()`. If this list is used for validation or documentation, `"modalities"` should be added.

---

## ❌ Must Fix

### M1. Streaming image design is not viable as written

As described in S2 above, the streaming approach of JSON-encoding a `ContentPart` into `Delta.Content *string` **breaks the OpenAI streaming contract**. This must be redesigned before implementation.

Concrete recommendation: **Check whether Gemini actually streams image data incrementally or sends it as a single response.** If Gemini sends image data only in the final chunk (which is likely — you can't incrementally decode a base64 image), then the simplest approach is:
- For streaming + image modality: buffer the response and return it as a non-streaming response, OR
- Extend `Delta` to have a `ContentParts` field for structured content

Either way, the current plan's streaming approach cannot ship as-is.

### M2. `genConfig` not always attached to body when modalities is the only config

Looking at the existing code:
```go
if len(genConfig) > 0 {
    body["generationConfig"] = genConfig
}
```

The plan adds modalities to `genConfig`, so this works. **However**, looking more carefully: the plan says "after the existing `generationConfig` block" — if the implementation adds the modalities code *after* the `if len(genConfig) > 0` check, the modalities won't be included. 

**Must ensure** the modalities injection happens *before* the `if len(genConfig) > 0` body attachment. The plan's pseudocode placement is ambiguous — make it explicit.

---

## Summary

| Category | Count |
|----------|-------|
| ✅ Agreed | 7 |
| ⚠️ Suggestions | 5 |
| ❌ Must Fix | 2 |

**Overall**: The non-streaming path is solid and well-designed. The main blocker is the streaming + image design (M1) which needs rethinking before implementation. M2 is a minor placement issue but could cause a subtle bug. The FR-003 contradiction (S1) should also be resolved in the plan before coding.
