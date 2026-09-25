# Quickstart: HO-1186 Credentials UI

## Preconditions

- Linear HO-1186 is `In Progress` before implementation starts.
- Worktree is `/Users/n0rmanc/.openclaw/workspace-sima/worktrees/tianjiLLM/HO-1186-credentials-sidebar-list-detail-quota-ui`.
- No production code is written during Todo planning.

## Implementation Flow

1. Start from failing E2E:

```bash
go test ./test/e2e -tags e2e -run 'TestCredentials' -count=1
```

Expected first result: fails because `/ui/credentials` route/sidebar/pages do not exist yet.

2. Add route/sidebar/handler/page implementation using existing UI patterns:

- `internal/ui/routes.go`
- `internal/ui/pages/layout.templ`
- `internal/ui/handler_credentials.go`
- `internal/ui/pages/credentials.templ`
- generated `*_templ.go` files
- E2E helper/test files under `test/e2e`

3. Regenerate templ output with the repo's existing generation command.

4. Run targeted gates:

```bash
go test ./test/e2e -tags e2e -run 'TestCredentials' -count=1
go test ./internal/ui/... -count=1
go test ./internal/proxy/handler/... ./internal/callback/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

5. Inspect rendered DOM assertions:

- List page does not show generic `api_key` credentials.
- Detail page shows email/status/quota fields.
- Unknown quota does not show fake `0% used`.
- No token-looking fixture string appears in body/HTML.

## Manual UI Smoke

If local services are available:

1. Start app using the repo's normal local dev flow.
2. Login to `/ui/login` with master key.
3. Open `/ui/credentials`.
4. Confirm sidebar active state and list table.
5. Open a credential detail page.
6. Confirm cards/table/badges fit desktop and narrow viewport.

## Done Criteria for Implementation Phase

- All tasks in `tasks.md` are marked complete only after code and tests are actually done.
- PR includes E2E coverage map for list/detail/quota/secret DOM assertions.
- Linear moves to `In Review` only after local gates pass and PR is updated.
