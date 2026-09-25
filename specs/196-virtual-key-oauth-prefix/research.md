# Research: Virtual Key OAuth Prefix

**Date**: 2026-03-25

## Decision Log

### 1. Key Generation Method

- **Decision**: Use `crypto/rand.Read()` + `encoding/hex.EncodeToString()`
- **Rationale**: 密碼學安全的隨機數產生器，Go stdlib，零外部依賴。比 UUID 更靈活（可精確控制長度），且 UI handler 已經使用此模式。
- **Alternatives considered**:
  - `github.com/google/uuid`: 目前 API handler 使用，但 UUID 格式包含 `-` 分隔符（36 chars for 16 bytes），浪費空間且不必要
  - `crypto/rand` + `base64`: 更密集的編碼，但包含 `+/=` 等特殊字元，不適合作為 API key

### 2. Random Part Length

- **Decision**: 30 bytes → 60 hex chars
- **Rationale**: Prefix `sk-ant-oat01-tianji-` = 20 chars + 60 hex chars = 80 chars（恰好滿足 >= 80 requirement）。30 bytes = 240 bits 的熵，遠超安全需求。
- **Alternatives considered**:
  - 32 bytes (64 hex chars, total 84): 稍長但無額外安全價值
  - 24 bytes (48 hex chars, total 68): 不滿足 >= 80 requirement

### 3. Code Organization

- **Decision**: 在 `handler` 和 `ui` 兩個 package 各自更新生成函式，保持相同邏輯
- **Rationale**: 兩個 package 已有 `hashKey()` 重複（Go package 隔離的正常代價）。引入共用 package 會增加依賴圖複雜度，不值得為一個 3 行函式創建新 package。
- **Alternatives considered**:
  - 新建 `internal/keyutil/` package: 過度設計，一個函式不值得一個 package
  - 放在 `internal/model/`: 職責不符，model 是 data types 不是 utility functions
  - 放在 `internal/auth/`: 概念上合理，但會改變現有依賴方向

## No NEEDS CLARIFICATION Items

所有技術決策已確定，無待解決項目。
