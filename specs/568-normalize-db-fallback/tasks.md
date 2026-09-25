# Tasks: HO-568 Normalize Expired Windows in DB Fallback

## Status: COMPLETE

- [X] T001: Systematic audit of all DBRowToState/dbRowToState call sites
- [X] T002: Fix handler_claude_code.go:74 — wrap DBRowToState with NormalizeExpiredWindows
- [X] T003: Add TestLoadClaudeCodeData_DBFallback_ExpiredWindowZerosUsage
- [X] T004: go vet ./... PASS
- [X] T005: go test ./internal/ui/... PASS
- [X] T006: Push branch + create draft PR
- [X] T007: Move HO-568 → Waiting, notify owner
