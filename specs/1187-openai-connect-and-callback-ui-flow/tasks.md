# Tasks: OpenAI connect and callback UI flow

**Input**: `spec.md`, `plan.md`, Linear HO-1187
**State**: Todo planning complete; tasks are intentionally unchecked until Linear moves to `In Progress`.

## Phase 1 - Tests First

- [x] T001 Add UI E2E file `test/e2e/openai_connect_flow_test.go` for authenticated connect success with mock OpenAI OAuth upstream.
- [x] T002 Extend E2E setup or helpers to configure `OpenAIOAuth.AuthorizeURL` / `TokenURL` to the local mock server for this test only.
- [x] T003 Add E2E assertion that `/ui/credentials` renders `Connect OpenAI` for logged-in admin.
- [x] T004 Add E2E assertion that clicking connect reaches mock authorize URL with `state`, `code_challenge`, `code_challenge_method=S256`, and `/oauth/openai/callback` redirect URI.
- [x] T005 Add E2E callback success case using mock token fixture and assert success page plus `/ui/credentials` action.
- [x] T006 Add E2E assertion that after success, returning to `/ui/credentials` shows the saved OpenAI subscription credential safe metadata.
- [x] T007 Add E2E callback provider-denial failure case with valid state and `error=access_denied`.
- [x] T008 Add E2E invalid/missing/expired state failure case.
- [x] T009 Add E2E token-exchange failure case using mock token non-2xx fixture.
- [x] T010 Add unauthenticated connect test proving `/ui/openai/connect?org_id=...` redirects/blocks through existing UI auth behavior.
- [x] T011 Add DOM secret assertions for success/failure pages: no `access_token`, `refresh_token`, `id_token`, authorization code, code verifier, bearer string, JWT, encrypted credential blob, or raw upstream response.

## Phase 2 - Connect Entry Point

- [x] T012 Add OpenAI Connect CTA view-model field to the Credentials page data adapter in `internal/ui/handler_credentials.go`.
- [x] T013 Render `Connect OpenAI` action in `internal/ui/pages/credentials.templ` using existing button/icon patterns.
- [x] T014 Ensure CTA URL passes explicit `org_id` from the current UI/session/default organization source.
- [x] T015 Keep `/ui/openai/connect` inside existing `sessionAuth`; do not move route or loosen auth.
- [x] T016 Keep existing OpenAI OAuth backend state/PKCE helpers; do not duplicate primitive generation in UI page code.

## Phase 3 - Callback Result UI

- [x] T017 Replace minimal `writeOpenAIOAuthPage` output with a reusable safe result page renderer.
- [x] T018 Style callback success and failure pages with existing Tianji UI assets/classes or reusable `templ` component.
- [x] T019 Add success result content: clear title, safe message, and link/button to `/ui/credentials`.
- [x] T020 Add failure result content for invalid/expired state, provider denial, missing code, token exchange failure, and credential save failure.
- [x] T021 Escape and redact provider-supplied `error` / `error_description` before rendering.
- [x] T022 Ensure callback result renderer does not require `/ui` session cookie.
- [x] T023 Preserve HO-1175 callback behavior: consume state once, trust stored `org_id`, ignore query `org_id` / `organization_id` / `credential_id`.

## Phase 4 - Verification

- [x] T024 Compile E2E package with `go test -c -tags e2e ./test/e2e`; full browser E2E remains CI-run because local execution requires `E2E_DATABASE_URL`.
- [x] T025 Run `go test ./internal/ui/... -run 'TestHandleOpenAIConnect' -count=1`.
- [x] T026 Run `go test ./internal/proxy/handler/... -run 'TestOpenAIOAuthCallback' -count=1`.
- [x] T027 Run `go test ./internal/proxy/... -run 'TestOpenAIOAuthRoutes' -count=1`.
- [x] T028 Run `git diff --check origin/main...HEAD`.
- [x] T029 Review changed UI for no new frontend framework/package and no duplicated OAuth primitive logic.
- [x] T030 Confirm E2E and handler tests do not call live `auth.openai.com` / `api.openai.com`.

## Scope Confirmation

- [x] T031 Confirm with owner through the draft PR/Linear Waiting gate that scope is limited to Credentials CTA + safe callback result UI + offline UI E2E.
