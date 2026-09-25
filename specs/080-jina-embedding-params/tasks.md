# Tasks: Tianji Proxy Jina Embedding Extra Params + Base64 Decode

**Input**: Design documents from `/specs/080-jina-embedding-params/`
**Linear**: HO-489

**Architecture**: Wire struct pattern — `json.RawMessage` in wire struct, provider layer normalize to `[]float64`

---

## Phase 1: Foundational

**Purpose**: `EmbeddingRequest` 加欄位（unblock 所有 story）

- [x] T001 Add `Task string` and `Normalized *bool` (omitempty) to `EmbeddingRequest` in `internal/model/response.go`
  - `EmbeddingData` **不改**（保持 `[]float64`，public interface 不變）

**Checkpoint**: `go build ./...` pass

---

## Phase 2: User Story 2 — Base64 Decode in openai provider (Priority: P2, 但先做，unblock P1)

> 先做 systemic fix，讓 jina 可以直接繼承，不用自己 override response

**Goal**: `openai.TransformEmbeddingResponse` 改用 wire struct，支援 base64 decode

**Independent Test**: `go test ./internal/provider/openai/... -v -run "TestDecodeEmbeddingField|TestTransformEmbeddingResponse_Base64"`

### Tests for User Story 2 (MANDATORY) 🔴

> ⚠️ T002 依賴 T001 完成（需要 `Normalized *bool` / `Task` struct fields）

- [x] T002 [US2] Write failing tests in `internal/provider/openai/embedding_test.go`:
  - `TestDecodeEmbeddingField_FloatArray` — float array JSON → `[]float64`（regression）
  - `TestDecodeEmbeddingField_Base64` — base64 string → 正確 float array
  - `TestDecodeEmbeddingField_InvalidBase64` — 無效 base64 → error
  - `TestDecodeEmbeddingField_InvalidByteLength` — 非 4 倍數 bytes → error
  - `TestDecodeEmbeddingField_EmptyBase64` — 空 base64 → empty `[]float64`
  - `TestTransformEmbeddingResponse_Base64` — 完整 response E2E
  - `TestEmbeddingRequest_NormalizedField` — Normalized *bool marshal
  - `TestEmbeddingRequest_TaskField` — Task string marshal

> Run: `go test ./internal/provider/openai/... -v` → compile 但 fail

### Implementation for User Story 2

- [x] T003 [US2] Add private `embeddingDataWire` + `embeddingResponseWire` structs in `internal/provider/openai/embedding.go`
- [x] T004 [US2] Add `decodeEmbeddingField(raw json.RawMessage) ([]float64, error)` helper in `internal/provider/openai/embedding.go`
  - base64 decode 規格：float32 little-endian packed bytes，每 4 bytes → float32 → float64
- [x] T005 [US2] Update `TransformEmbeddingResponse` in `internal/provider/openai/embedding.go` to use wire struct + `decodeEmbeddingField`

**Checkpoint**: `go test ./internal/provider/openai/... -v` 全 pass（含既有 tests）

---

## Phase 3: User Story 1 — Jina strip params (Priority: P1) 🎯

**Goal**: Jina override `TransformEmbeddingRequest`，strip `encoding_format` + `normalized`，透傳 `task`

**Independent Test**: `go test ./internal/provider/jina/... -v`

### Tests for User Story 1 (MANDATORY) 🔴

> ⚠️ T006 依賴 T001 完成（需要 `Normalized *bool` / `Task` struct fields）

- [x] T006 [US1] Write failing tests in `internal/provider/jina/jina_test.go`:
  - `TestJinaTransformEmbeddingRequest_StripsEncodingFormat`
  - `TestJinaTransformEmbeddingRequest_StripsNormalized`
  - `TestJinaTransformEmbeddingRequest_FloatFallback`
  - `TestJinaTransformEmbeddingRequest_NormalizedFalse`
  - `TestJinaTransformEmbeddingRequest_TransparentTask`
  - `TestJinaGetSupportedParams_IncludesNormalizedAndTask`

> Run: `go test ./internal/provider/jina/... -v` → compile 但 fail

### Implementation for User Story 1

- [x] T007 [US1] Add local `jinaEmbeddingRequest` struct in `internal/provider/jina/jina.go`
- [x] T008 [US1] Override `TransformEmbeddingRequest` in `internal/provider/jina/jina.go`
- [x] T009 [US1] Update `GetSupportedParams` in `internal/provider/jina/jina.go` → 加 `"normalized"`, `"task"`

**Checkpoint**: `go test ./internal/provider/jina/... -v` 全 pass

---

## Phase 4: Polish & Regression

- [x] T010 [P] Run contract tests: `go test ./test/contract/... -run "TestEmbedding" -v`
- [x] T011 [P] Run all openai tests: `go test ./internal/provider/openai/... -v`
- [x] T012 [P] Run all jina tests: `go test ./internal/provider/jina/... -v`
- [x] T013 `go build ./...` pass
- [x] T014 Commit: `feat: wire struct base64 decode + jina strip params (HO-489)`

---

## Phase 5: Smoke Test (SC-006)

> 分離任務，不在本 branch 執行；不隸 CI 。HO-489 merge 後由 owner 手動執行。

- [ ] T015 **[owner: Norman]** Deploy HO-489 fix to staging / local tianji proxy
- [ ] T016 **[owner: Norman]** Update memory-lancedb-pro config 指向 tianji proxy（而非直連 Jina API）
- [ ] T017 **[owner: Norman]** 發送一條測試 memory store/recall，確認 embedding 度數正確（1024）且回傳 200

---

## Dependencies

- T001 先完成，unblock T002 + T006
- T002 + T006 可並行（T001 完成後，不同檔案）
- T003-T005 依賴 T002 tests pass
- T007-T009 依賴 T001（struct fields）；T005 systemic fix 不影響 jina 繼承關係，但建議 T005 後再做以確保 regression base 穩定
- T010-T014 全部 impl 完成後

---

## Implementation Strategy

Phase 2（systemic fix）先做，再做 Phase 3（jina）。
Jina 直接繼承 openai `TransformEmbeddingResponse`，不用自己 decode，只負責 request strip。
