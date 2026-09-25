# Data Model: Virtual Key OAuth Prefix

**Date**: 2026-03-25

## Entities

### Virtual Key (unchanged schema)

資料庫 schema 不變——`verification_tokens` table 儲存 SHA256 hash，與 key 前綴無關。

| Field | Type | Description |
|-------|------|-------------|
| token | TEXT (PK) | SHA256 hash of the raw key |
| key_name | TEXT | Display name |
| key_alias | TEXT (UNIQUE) | Unique alias |
| ... | ... | Other fields unchanged |

### Key Format (new)

```
sk-ant-oat01-tianji-<60 hex chars>
|____________||_____||____________|
  14 chars    7 chars   60 chars   = 81?
```

Character-by-character count of prefix `sk-ant-oat01-tianji-`:
`s-k---a-n-t---o-a-t-0-1---t-i-a-n-j-i---` = 20 characters

| Component | Value | Length |
|-----------|-------|--------|
| Full prefix | `sk-ant-oat01-tianji-` | 20 chars |
| Random | hex(crypto/rand 30 bytes) | 60 chars |
| **Total** | `sk-ant-oat01-tianji-<60 hex>` | **80 chars** |

## Validation Rules

- `strings.HasPrefix(key, "sk-ant-oat01-tianji-")` — TianjiLLM virtual key
- `strings.Contains(key, "sk-ant-oat")` — passes OpenClaw runtime check
- `strings.HasPrefix(key, "sk-ant-oat01-")` — passes OpenClaw setup check
- `len(key) >= 80` — passes OpenClaw length check
- `len(key) == 80` — exact length (deterministic)

## No Schema Migrations Required

資料庫只存 SHA256 hash，key 前綴變更不影響 schema。
