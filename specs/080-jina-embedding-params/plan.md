# Implementation Plan: Tianji Proxy Jina Embedding Extra Params + Base64 Decode

**Branch**: `080-jina-embedding-params` | **Date**: 2026-04-04 | **Spec**: [spec.md](./spec.md)
**Linear**: HO-489

## Summary

兩個根因：
1. `EmbeddingRequest` 缺 `Normalized` / `Task` 欄位
2. `EmbeddingData.Embedding []float64` 寫死，任何 provider 回 base64 都爆

**採用大專案標準做法**（如 LiteLLM、OpenAI SDK）：
- Response wire struct 用 `json.RawMessage`
- Provider 層的 `TransformEmbeddingResponse` 負責 decode + normalize 成 `[]float64`
- 職責清晰：每個 provider 自己處理自己的 response format

## Technical Context

**Language/Version**: Go 1.24.x  
**Primary Dependencies**: stdlib `encoding/json`, `encoding/base64`, `encoding/binary`, `math`  
**Storage**: N/A  
**Testing**: `go test` + testify  
**Target Platform**: Linux server  
**Project Type**: single Go module  
**Constraints**: no regression on existing embedding tests, public `EmbeddingData.Embedding []float64` interface 不變  

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Python-First Reference | ✅ skip | Proxy fix |
| II. Feature Parity | ✅ N/A | Proxy behavior fix |
| III. Research Before Build | ✅ | research.md + 大專案架構確認 |
| IV. Failing-Tests-First | ✅ | 所有測試列在 Failing Tests section |
| V. Go Best Practices | ✅ | wire struct + provider-layer normalize，正確分層 |
| VI. No Stale Knowledge | ✅ | base64 規格已查證 |
| VII. sqlc-First | ✅ N/A | 無 DB 改動 |

## Architecture Decision

**Wire struct pattern**（大專案做法）：

```
upstream response JSON
        ↓
embeddingDataWire { Embedding json.RawMessage }   ← private, parse-only
        ↓
provider.TransformEmbeddingResponse decode + normalize
        ↓
model.EmbeddingData { Embedding []float64 }        ← public, caller 看到
```

`openai/embedding.go` 新增 `decodeEmbeddingField(raw json.RawMessage) ([]float64, error)` helper：
- 若 raw 是 JSON array → 直接 unmarshal 成 `[]float64`
- 若 raw 是 JSON string（base64） → base64 decode → float32 LE bytes → `[]float64`
- 其他格式 → error

所有繼承 openai `TransformEmbeddingResponse` 的 provider 自動受益。
Jina 不需要 override response（request strip 已夠）。

## Project Structure

### Source Code (affected files)

```text
internal/model/response.go              # EmbeddingRequest 加 Task/Normalized
internal/provider/openai/embedding.go   # 加 wire struct + decodeEmbeddingField helper，更新 TransformEmbeddingResponse
internal/provider/openai/embedding_test.go  # 新增 base64 decode tests（或加入 openai_test.go）
internal/provider/jina/jina.go          # override TransformEmbeddingRequest
internal/provider/jina/jina_test.go     # 新增 jina-specific tests
```

## Failing Tests

### User Story 1 Tests: Jina strip params

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestJinaTransformEmbeddingRequest_StripsEncodingFormat` | `internal/provider/jina/jina_test.go` | body 不含 `encoding_format` | AS-1.2, AS-1.3 |
| `TestJinaTransformEmbeddingRequest_StripsNormalized` | `internal/provider/jina/jina_test.go` | body 不含 `normalized` | AS-1.1, AS-1.3 |
| `TestJinaTransformEmbeddingRequest_FloatFallback` | `internal/provider/jina/jina_test.go` | 不帶 encoding_format 時 body 也不含 | AS-1.4 regression |
| `TestJinaTransformEmbeddingRequest_NormalizedFalse` | `internal/provider/jina/jina_test.go` | normalized=false 時 strip | Edge Case |
| `TestJinaTransformEmbeddingRequest_TransparentTask` | `internal/provider/jina/jina_test.go` | body 含 `task: "retrieval.passage"` | AS-3.1 |
| `TestJinaGetSupportedParams_IncludesNormalizedAndTask` | `internal/provider/jina/jina_test.go` | params 含 "normalized" 和 "task" | FR-004 |

### User Story 2 Tests: Base64 decode in openai provider

| Test Function | File | Assertion | Covers |
|---------------|------|-----------|--------|
| `TestDecodeEmbeddingField_FloatArray` | `internal/provider/openai/embedding_test.go` | float array JSON → `[]float64` | AS-2.2 regression |
| `TestDecodeEmbeddingField_Base64` | `internal/provider/openai/embedding_test.go` | base64 string → 正確 float array | AS-2.1, AS-2.3 |
| `TestDecodeEmbeddingField_InvalidBase64` | `internal/provider/openai/embedding_test.go` | 無效 base64 → error | AS-2.4 |
| `TestDecodeEmbeddingField_InvalidByteLength` | `internal/provider/openai/embedding_test.go` | 非 4 倍數 bytes → error | Edge Case |
| `TestDecodeEmbeddingField_EmptyBase64` | `internal/provider/openai/embedding_test.go` | 空 base64 → empty slice | Edge Case |
| `TestTransformEmbeddingResponse_Base64` | `internal/provider/openai/embedding_test.go` | 完整 response 含 base64 embedding → `[]float64` | AS-2.1 E2E |
| `TestEmbeddingRequest_NormalizedField` | `internal/provider/openai/embedding_test.go` | Normalized *bool marshal/unmarshal | FR-001 |
| `TestEmbeddingRequest_TaskField` | `internal/provider/openai/embedding_test.go` | Task string marshal/unmarshal | FR-002 |

### Verification Command

```bash
cd ~/projects/tianjiLLM
go test ./internal/provider/jina/... ./internal/provider/openai/... -v
```

## Implementation Notes

### Change 1: `internal/model/response.go` — EmbeddingRequest only

```go
type EmbeddingRequest struct {
    Model          string `json:"model"`
    Input          any    `json:"input"`
    EncodingFormat string `json:"encoding_format,omitempty"`
    Dimensions     *int   `json:"dimensions,omitempty"`
    User           string `json:"user,omitempty"`
    Normalized     *bool  `json:"normalized,omitempty"`   // new
    Task           string `json:"task,omitempty"`          // new
}
// EmbeddingData 維持 []float64，不加 UnmarshalJSON
```

### Change 2: `internal/provider/openai/embedding.go` — wire struct + decode helper

```go
// wire struct，只用於 parse upstream response
type embeddingDataWire struct {
    Object    string          `json:"object"`
    Index     int             `json:"index"`
    Embedding json.RawMessage `json:"embedding"`
}

type embeddingResponseWire struct {
    Object string              `json:"object"`
    Data   []embeddingDataWire `json:"data"`
    Model  string              `json:"model"`
    Usage  model.EmbeddingUsage `json:"usage"`
}

// decodeEmbeddingField: float array or base64 string → []float64
func decodeEmbeddingField(raw json.RawMessage) ([]float64, error) {
    // 1. 先試 []float64：json.Unmarshal(raw, &floats)
    // 2. 若失敗，再試 string：json.Unmarshal(raw, &s)
    //    若連 string 也失敗，回 error（壞 JSON）
    // 3. base64.StdEncoding.DecodeString(s) 得 []byte
    // 4. 若 len(bytes) % 4 != 0 → return error
    // 5. 每 4 bytes：math.Float32frombits(binary.LittleEndian.Uint32(b[i:])) → float32 → float64
}

// TransformEmbeddingResponse: 改為先 parse wire，再 normalize
func (p *Provider) TransformEmbeddingResponse(...) (*model.EmbeddingResponse, error) {
    // parse into embeddingResponseWire
    // for each data item: decodeEmbeddingField(item.Embedding)
    // build model.EmbeddingResponse with []float64
}
```

### Change 3: `internal/provider/jina/jina.go`

新增 local struct + override `TransformEmbeddingRequest`：

```go
type jinaEmbeddingRequest struct {
    Model      string `json:"model"`
    Input      any    `json:"input"`
    Task       string `json:"task,omitempty"`
    Dimensions *int   `json:"dimensions,omitempty"`
    // strip: EncodingFormat, Normalized
}

func (p *Provider) TransformEmbeddingRequest(...) (*http.Request, error) { ... }
```

`GetSupportedParams` 加 `"normalized"`, `"task"`。

## Complexity Tracking

無 Constitution 違規。Wire struct pattern 是 Go idiomatic 做法，符合 Principle V。
