# Quickstart: Fix API Key Authentication

**Feature**: `001-fix-api-key-auth` | **Phase**: 1 | **Date**: 2026-02-25

## What was broken

A virtual API key created via `/key/generate` always returned 401 Unauthorized,
because `cmd/tianji/main.go` never passed the database connection to the auth
middleware. Only the master key worked.

## What the fix does (3 changes)

| # | File | Change |
|---|------|--------|
| 1 | `internal/proxy/middleware/db_validator.go` | New `DBValidator` adapter: wraps `*db.Queries`, implements `TokenValidator` + `GuardrailProvider`, maps pgx errors to sentinel errors |
| 2 | `internal/proxy/middleware/auth.go` | Handle `ErrDBUnavailable` → 503; add INFO/WARN/ERROR log entries |
| 3 | `cmd/tianji/main.go` | Pass `DBQueries: &middleware.DBValidator{DB: queries}` to `proxy.ServerConfig` |

## Verifying the fix

### Prerequisites

```bash
# PostgreSQL running and configured
export DATABASE_URL=postgres://tianji:tianji@localhost:5432/tianji

# Start the proxy
make run
```

### Step 1: Create a virtual API key (master key required)

```bash
curl -s -X POST http://localhost:4000/key/generate \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"key_name": "test-key"}' | jq .
```

Expected response:
```json
{
  "key": "sk-...",
  "key_name": "test-key",
  "token": "..."
}
```

Copy the `"key"` value.

### Step 2: Use the virtual key (was broken, now works)

```bash
export VIRTUAL_KEY="sk-..."  # from Step 1

curl -s -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $VIRTUAL_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Before fix**: `{"error":{"message":"invalid API key","type":"authentication_error"}}` (401)
**After fix**: Upstream response forwarded (200)

### Step 3: Verify master key still works (regression check)

```bash
curl -s -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

Expected: 200 OK (same as before the fix).

### Step 4: Verify blocked key returns 403

```bash
# Block the key
curl -s -X POST http://localhost:4000/key/block \
  -H "Authorization: Bearer $MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{"key": "$VIRTUAL_KEY"}'

# Try to use it
curl -s -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $VIRTUAL_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

Expected: `{"error":{"message":"API key is blocked","type":"authentication_error"}}` (403)

### Step 5: Verify DB outage returns 503

```bash
# Stop PostgreSQL, then try a virtual key
curl -s -X POST http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $VIRTUAL_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4o", "messages": [{"role": "user", "content": "Hello"}]}'
```

Expected: 503 Service Unavailable (not 401 or 500)

---

## Running the tests

```bash
# Unit + contract tests (no DB required)
go test ./internal/proxy/middleware/... -v -run TestAuth
go test ./test/contract/... -v

# Integration test (virtual key auth — requires test mock)
go test ./test/integration/... -v -run TestVirtualKeyAuth

# Full suite
make test
```

---

## Key files changed

```text
internal/proxy/middleware/
├── auth.go           # +structured logging, +503 for ErrDBUnavailable
└── db_validator.go   # NEW

cmd/tianji/
└── main.go           # +DBQueries wiring (~3 lines)

test/
├── integration/virtual_key_auth_test.go  # NEW integration test
└── contract/auth_wiring_test.go          # NEW unit test (or auth_test.go)
```
