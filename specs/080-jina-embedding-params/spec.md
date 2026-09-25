# Feature Specification: Tianji Proxy Jina Embedding Extra Params + Base64 Response Decode

**Feature Branch**: `080-jina-embedding-params`  
**Created**: 2026-04-04  
**Updated**: 2026-04-04  
**Status**: Draft  
**Linear**: HO-489

## Background

`memory-lancedb-pro` plugin 在使用 `jina-embeddings-v5-text-small` 時，帶上 `normalized: true` 與 `encoding_format: "base64"`，proxy 回 502。有兩個根因：

1. `EmbeddingRequest` struct 沒有 `Normalized` / `Task` 欄位
2. **系統性問題**：`EmbeddingData.Embedding []float64` 寫死，任何 provider（不只 jina）回傳 base64 encoded embedding 時都會 JSON parse error → 502

## User Scenarios & Testing

### User Story 1 - Jina Embedding 請求帶額外參數不再 502 (Priority: P1)

呼叫端（如 memory-lancedb-pro）使用 jina model 時帶上 `normalized` 和 `encoding_format: "base64"`，proxy 應正常回傳 embedding 結果而非 502。

**Why this priority**: 這是 blocker。目前 sima 的 memory 系統直接無法使用 jina embeddings。

**Independent Test**: curl 帶 `normalized: true` + `encoding_format: "base64"` 打 `/v1/embeddings`，回傳 200 且 response 包含 float array embeddings。

**Acceptance Scenarios**:

1. **Given** jina model 設定好，**When** POST `/v1/embeddings` body 含 `normalized: true`，**Then** 回傳 200，response 包含有效 embedding data
2. **Given** jina model 設定好，**When** POST `/v1/embeddings` body 含 `encoding_format: "base64"`，**Then** 回傳 200，response 包含 float array（proxy strip + 強制 float）
3. **Given** jina model 設定好，**When** POST `/v1/embeddings` body 同時含兩個參數，**Then** 回傳 200，功能正常
4. **Given** jina model 設定好，**When** POST `/v1/embeddings` body 不含這兩個參數，**Then** 行為與現在一致（regression-free）

---

### User Story 2 - Proxy Response Layer 支援 base64 Embedding Decode (Priority: P2)

任何 provider（openai、voyage、jina、compat 等）若回傳 `encoding_format: "base64"` response，proxy 應自動 decode 成 float array，讓 response 一律是 `[]float64`。

**Why this priority**: 這是系統性問題，不只 jina。OpenAI 官方 API 支援 base64，其他 compat provider 也可能回 base64。補丁只解 jina，這條解所有 provider。

**Implementation note**: 技術實作順序先於 US1，因為 US1 jina fix 可直接繼承此修實結果。

**Independent Test**: mock upstream 回 base64 encoded embedding，proxy response 含正確 float array。

**Acceptance Scenarios**:

1. **Given** openai provider，**When** upstream 回 base64 encoded embedding，**Then** proxy response 是 float array
2. **Given** 任意 provider，**When** upstream 回 float array response，**Then** proxy response 不變（regression-free）
3. **Given** 任意 provider，**When** upstream 回 base64 string，**Then** decode 正確（float32 little-endian bytes）
4. **Given** 任意 provider，**When** upstream 回無效 base64，**Then** 回 502 + 明確 error message

---

### User Story 3 - Jina 特有 task 欄位透傳 (Priority: P3)

Jina API 支援 `task` 欄位（如 `retrieval.passage`），呼叫端帶上時不應被 proxy strip 掉。

**Why this priority**: `task` 影響 embedding 品質，丟棄會導致 retrieval 效果降級。

**Independent Test**: curl 帶 `task: "retrieval.passage"` 打 proxy，確認請求有帶到 jina upstream。

**Acceptance Scenarios**:

1. **Given** jina model，**When** request 帶 `task` 欄位，**Then** upstream request 也帶 `task`
2. **Given** 其他非 jina provider，**When** request 帶 `task` 欄位，**Then** `task` 不影響現有行為（忽略或透傳依 provider 決定）

---

### Edge Cases

- `encoding_format: "float"` 是 default，不應有任何行為變化
- `normalized: false` 帶入時，proxy 應 strip 掉（不透傳，因為 jina API 行為未定義）
- **Base64 decode 規格**：OpenAI 格式為 float32 little-endian packed bytes，每 4 bytes 一個 float32，decode 後轉 float64 array
- Upstream 回無效 base64（非 4 的倍數 bytes）時，回 502 + error（handler 層已有 `TransformEmbeddingResponse` error → 502 路徑，無需額外改動）
- 非 jina provider 收到 `normalized` 欄位時，應 strip 掉（避免 upstream 報錯）
- 現有 float array response 不受影響（wire struct decode 路徑會 try `[]float64` first）

## Requirements

### Functional Requirements

- **FR-001**: `EmbeddingRequest` 必須加入 `Normalized *bool` 欄位（pointer，omitempty）
- **FR-002**: `EmbeddingRequest` 必須加入 `Task string` 欄位（omitempty）
- **FR-003**: Jina provider `TransformEmbeddingRequest` 必須 strip `encoding_format` 和 `normalized`，透傳 `task` 和 `dimensions`
- **FR-004**: Jina provider `GetSupportedParams` 必須包含 `normalized` 和 `task`
- **FR-005**: 非 jina provider 的 `TransformEmbeddingRequest` 不得透傳 `normalized` 欄位（struct omitempty 即可）
- **FR-006**: 現有 embedding 測試（`TestEmbedding_*`）必須全部 pass
- **FR-007**: `openai.TransformEmbeddingResponse` 必須改用 private wire struct（`embeddingDataWire` 含 `json.RawMessage`），再 normalize 成 `model.EmbeddingData`（`[]float64`）
- **FR-008**: `decodeEmbeddingField` helper：同時支援 `[]float64` JSON array 和 base64 string（float32 LE bytes）；decode 失敗回 error

### Key Entities

- **EmbeddingRequest**: OpenAI-compatible embedding request struct，新增 `Normalized` / `Task` 欄位
- **EmbeddingData**: public struct 維持 `[]float64`，不改（caller interface 不變）
- **embeddingDataWire** (private): openai provider 內部 wire struct，`Embedding json.RawMessage`，僅用於 parse upstream response
- **Jina Provider**: `internal/provider/jina/jina.go`，需 override `TransformEmbeddingRequest`

## Success Criteria

### Measurable Outcomes

- **SC-001**: 帶 `normalized: true` + `encoding_format: "base64"` 的 jina embedding 請求回傳 200
- **SC-002**: Response embedding data 為有效 float array，維度與 model 宣告一致（e.g., 1024 for v5-small）
- **SC-003**: 所有現有 contract tests pass，無 regression
- **SC-004**: Jina provider unit test 覆蓋新 param 處理路徑
- **SC-005**: Base64 encoded embedding response（任何繼承 openai response 的 provider）可被 proxy 正確 decode 成 float array
- **SC-006**: 修完後 memory-lancedb-pro 切回 tianji proxy，實際 embedding 請求（帶 normalized + encoding_format + task）回傳 200 且維度正確（smoke test）

## Assumptions

- Jina API **支援** `encoding_format: "base64"`（官方文件確認）。對 jina 選擇 strip：（1）proxy response struct 問題已由 FR-007/FR-008 systemic fix 解決，（2）memory-lancedb-pro 需要 float array，strip 是最簡單且零風險的做法
- Base64 decode 規格遵循 OpenAI：float32 little-endian packed bytes
- Jina API 對 `normalized` 的支援未確認，保守做法：strip（不透傳）
- `task` 欄位 jina API 支援，應透傳
- `memory-lancedb-pro` 需要的是 float array embedding，不是 base64
- Systemic fix（FR-007/FR-008）讓所有繼承 openai `TransformEmbeddingResponse` 的 provider 自動受益
- **HO-489 完成後**：memory-lancedb-pro 需手動切回指向 tianji proxy（目前因 403 暫時直連 Jina API）；該切換不在本 issue 範圍內，另行處理
