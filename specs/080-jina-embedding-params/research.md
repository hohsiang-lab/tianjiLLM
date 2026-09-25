# Research: Jina Embedding Extra Params

**Date**: 2026-04-04

## Decision 1: encoding_format base64 handling

**Research**: Jina API 原生支援 `encoding_format: "base64"`（官網：「Choose output format: float (default), binary (compact storage), or base64 (efficient transmission)」）。

**Root Cause**: 問題不是 jina 拒絕 base64，而是 tianji proxy `TransformEmbeddingResponse` 嘗試把 jina 回傳的 base64 string unmarshal 進 `EmbeddingData.Embedding []float64`，導致 JSON parse error → 502。

**Decision**: 在 jina `TransformEmbeddingRequest` 裡 strip `encoding_format`（不送給 upstream），讓 jina 預設回傳 float array。這是最簡單且 risk-free 的修法，也符合 issue 描述的「最簡單修法」。

**Alternatives considered**:
- 支援 base64 decode：需要新增 `EmbeddingData.Embedding` 成 `any` 或增加 base64→float64 decode，更複雜，收益不明確（memory-lancedb-pro 最終需要的是 float array）
- 完整透傳：需要改 response struct + base64 decode，不值得

## Decision 2: normalized 欄位

**Research**: `normalized` 是 jina-specific param（L2-normalize embeddings），不在 OpenAI spec。當前 `EmbeddingRequest` struct 沒有這欄位，JSON marshal 時直接丟失，upstream 收不到。

**Decision**: 在 jina `TransformEmbeddingRequest` 裡也 strip `normalized`（不透傳）。原因：
1. 移除後 200 說明 jina 也不一定需要它（或預設行為已足夠）
2. memory-lancedb-pro 傳 `normalized: true` 主要是 hint，strip 後實際效果差異不大
3. 避免改動 `EmbeddingRequest` struct 影響其他 provider

**Alternatives considered**:
- 加 `Normalized *bool` 到 `EmbeddingRequest` + jina 透傳：加欄位後非 jina provider 也會看到它（靠 omitempty 不送），可行但改動面較大
- 用 jina-specific request struct override：乾淨但需要更多 code

## Decision 3: task 欄位

**Research**: Jina API 支援 `task` 欄位（`retrieval.passage`, `retrieval.query`, `text-matching` 等），影響 embedding 品質。目前 `EmbeddingRequest` 沒有此欄位。

**Decision**: 加 `Task string` 到 `EmbeddingRequest`（omitempty），非 jina provider 透過 omitempty 不送，jina 透過 openai base 直接透傳。需更新 `GetSupportedParams`。

## Implementation Strategy

jina provider 需要 override `TransformEmbeddingRequest`（目前繼承 openai，沒有 override）。在 override 中：
1. 複製 req，strip `EncodingFormat`（強制 float）
2. Strip 或忽略 `Normalized`（改在 jina-specific struct 或在 openai base 加欄位後 strip）
3. 透傳 `Task`

最簡方案：在 jina `TransformEmbeddingRequest` 裡建 local struct，只包含 jina 接受的欄位，直接 marshal。
