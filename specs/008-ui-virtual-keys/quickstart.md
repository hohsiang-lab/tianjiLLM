# Quickstart: UI Virtual Keys Management

**Feature**: 008-ui-virtual-keys
**Date**: 2026-02-20

## Prerequisites

```bash
# Ensure you're on the feature branch
git checkout 008-ui-virtual-keys

# Verify tools
go version          # Go 1.22+
make lint           # golangci-lint
sqlc version        # sqlc for DB query codegen
```

## Development Workflow

### 1. Database Queries (sqlc)

New queries go in `internal/db/queries/verification_token.sql` and `internal/db/queries/team.sql`.

**重要**：可選過濾參數必須用 `sqlc.narg()` 而非裸 `$N`，否則生成 `string` 而非 `*string`，導致 `IS NULL` 判斷失效。

```bash
# After editing .sql files:
make generate       # sqlc generate → updates internal/db/*.sql.go

# Verify generated code compiles:
go build ./internal/db/...
```

### 2. templ Templates

All UI templates in `internal/ui/pages/`.

```bash
# After editing .templ files:
templ generate      # generates *_templ.go files

# Or use watch mode during development:
templ generate --watch
```

### 3. Tailwind CSS

```bash
# After changing CSS classes in .templ files:
cd internal/ui/assets
npx @tailwindcss/cli -i input.css -o css/output.css
# Or watch mode:
npx @tailwindcss/cli -i input.css -o css/output.css --watch
```

### 4. Run & Test

```bash
# Run full check (lint + test + build):
make check

# Run only key-related tests:
go test ./internal/ui/... -run TestKey -v
go test ./internal/proxy/handler/... -run TestKeyList -v
go test ./test/contract/... -run TestKey -v

# Run the server locally:
make run
# Then visit http://localhost:4000/ui/keys
```

## Key Files to Modify

| File | What to do |
|------|------------|
| `internal/db/queries/verification_token.sql` | Add `ListVerificationTokensFiltered` (sqlc.narg), `CountVerificationTokensFiltered`, `GetVerificationTokenByAlias`, `RegenerateVerificationTokenWithParams` |
| `internal/db/queries/team.sql` | Add `ListTeamAliases` (team_id → alias mapping) |
| `internal/proxy/handler/key.go` | Enhance `KeyList` with filter/page params |
| `internal/ui/routes.go` | Add detail/edit/update/delete/regenerate routes (分離路由模式) |
| `internal/ui/handler_keys.go` | Major rewrite: detail handler, enhanced create (return key), edit, regenerate, toast responses |
| `internal/ui/pages/keys.templ` | Enhanced table columns, filter UI, pagination, create dialog with key reveal |
| `internal/ui/pages/key_detail.templ` | NEW: detail page (overview + settings tabs, edit form, delete confirm, regenerate dialog) |

## Architecture Notes

- **HTMX partial vs full page**: 分離路由模式（沿用現有 `/keys` + `/keys/table`），不使用 HX-Request header 檢測
- **Toast notifications**: Toast 組件自帶 `fixed` 定位，通過 OOB swap `afterbegin:body` 追加，不需容器元素
- **Detail page URL**: `/ui/keys/{token}` where token is the SHA256 hash (from DB)
- **Create key flow**: Handler generates raw key → hashes → stores hash in DB → returns table partial + OOB dialog with raw key
- **Team alias**: 分離查詢 `ListTeamAliases`，handler 構建 map[string]string 注入 KeyRow.TeamAlias
- **No client-side JS framework**: All interactivity via HTMX attributes + minimal inline JS (copy-to-clipboard, delete confirmation input validation)
