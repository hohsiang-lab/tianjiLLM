# Quickstart: Virtual Key OAuth Prefix

## What Changed

Virtual key 格式從 `sk-<uuid>` / `sk-<hex>` 改為 `sk-ant-oat01-tianji-<60 hex chars>`，以通過 OpenClaw 的 OAuth token 檢測。

## Files to Modify

1. `internal/proxy/handler/key.go` — API key generation
2. `internal/proxy/handler/key_ext.go` — API key regeneration
3. `internal/ui/handler_keys.go` — UI key generation

## Implementation Steps

1. Write failing tests (see plan.md Failing Tests section)
2. Update `generateVirtualKey()` in `key.go` (or rename existing):
   ```go
   func generateVirtualKey() string {
       b := make([]byte, 30)
       _, _ = rand.Read(b)
       return "sk-ant-oat01-tianji-" + hex.EncodeToString(b)
   }
   ```
3. Replace `"sk-" + uuid.New().String()` in `key.go` line 47 and `key_ext.go` line 32
4. Update `generateAPIKey()` in `handler_keys.go` to match
5. Run tests: `go test ./internal/proxy/handler/... ./internal/ui/... -v`
6. Run full suite: `make test`

## Verification

```bash
# Generate a key and verify format
curl -X POST http://localhost:4000/key/generate \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"key_name":"test"}' | jq -r '.key'
# Expected: sk-ant-oat01-tianji-<60 hex chars>
```
