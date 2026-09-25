# Tasks: Org detail OpenAI credential panel

**Input**: Design documents from `/specs/1189-org-detail-openai-credential-panel/`
**Prerequisites**: Linear state must move to `In Progress` before production implementation。

## Phase 1: RED UI E2E first

- [x] T001 [P] Add failing E2E `TestOrgDetail_OpenAICredentialsPanelShowsOrgScopedCredentials` in `test/e2e/orgs_detail_test.go`。
- [x] T002 [P] Seed org A/org B credentials so the test proves current org isolation and other-org exclusion。
- [x] T003 [P] Seed a same-org generic `api_key` credential and assert it is excluded from the OpenAI credential panel。
- [x] T004 [P] Add failing E2E `TestOrgDetail_OpenAICredentialsEmptyState` for org without OpenAI subscription credentials。
- [x] T005 [P] Add failing E2E `TestOrgDetail_OpenAICredentialLinkNavigatesToDetail` from panel row to `/ui/credentials/{credential_id}`。
- [x] T006 [P] Add failing E2E `TestOrgDetail_OpenAICredentialsSecretMaterialNotRendered` with token-looking metadata/value fixtures。
- [x] T007 Run targeted RED command and confirm failures are missing panel implementation, not fixture/environment failures。

## Phase 2: Shared credential row adapter

- [x] T008 Inspect `internal/ui/handler_credentials.go` current helper visibility after latest main；reuse or extract `buildCredentialRow` style logic without creating a second parser。
- [x] T009 Add an org-detail credential loading helper that calls `h.DB.ListCredentialsByOrg(ctx, &orgID)`。
- [x] T010 Filter loaded rows to `credential_type = "openai_subscription"`。
- [x] T011 Convert rows into the existing safe `pages.CredentialRow` shape or a shared equivalent。
- [x] T012 Ensure adapter never reads/decrypts `CredentialValue` and never renders arbitrary metadata map values。

## Phase 3: Org detail data model

- [x] T013 Extend `pages.OrgDetailData` with `OpenAICredentials []CredentialRow` or a narrow shared panel row type。
- [x] T014 Update `loadOrgDetailData` to populate org-scoped OpenAI credential rows alongside org/members/teams/metadata。
- [x] T015 Keep DB failure behavior safe and consistent with existing org detail patterns。

## Phase 4: Templ UI

- [x] T016 Add `OpenAI credentials` card/section in `internal/ui/pages/orgs.templ` after overview cards and before Members。
- [x] T017 Render populated table with Name, Email, Credential status, Quota, Last refresh, Last error, and Detail link。
- [x] T018 Render explicit empty state: `No OpenAI subscription credentials connected to this organization`。
- [x] T019 Use existing `badge`, `button`, `card`, `table` components and `overflow-x-auto` responsive table pattern。
- [x] T020 Do not add lifecycle action buttons on Org detail; detail/actions stay on `/ui/credentials/{credential_id}`。
- [x] T021 Match the owner-approved desktop/mobile mockscreen for placement, card/table hierarchy, copy tone, status/quota badge treatment, empty state, detail-link affordance, and responsive mobile behavior。
- [x] T022 Regenerate templ output for changed `.templ` files using repo command。

## Phase 5: Security and compatibility regression

- [x] T023 Assert Org detail DOM omits `credential_value`, access token, refresh token, `id_token`, JWT, bearer string, encrypted blob, fallback API key。
- [x] T024 Assert unknown quota/status renders as unknown/not recorded and never as healthy/zero usage。
- [x] T025 Assert disabled/error credential metadata remains visible even if quota state is stale/allowed。
- [x] T026 Assert existing Org detail tests still pass。
- [x] T027 Assert existing Credentials list/detail/action tests still pass。

## Phase 6: Verification and PR update

- [x] T028 Capture desktop and mobile result-screen screenshots after implementation and compare them against the approved mockscreen before `Waiting Merge`。
- [x] T029 Run `go test ./test/e2e -tags e2e -run 'TestOrgDetail_OpenAICredential|TestCredentials' -count=1`。
- [x] T030 Run `go test ./internal/ui/... -count=1`。
- [x] T031 Run `go test ./internal/proxy/handler/... ./internal/callback/... -count=1`。
- [x] T032 Run `go tool golangci-lint run`。
- [x] T033 Run `git diff --check origin/main...HEAD`。
- [x] T034 Verify PR diff contains only HO-1189 implementation files plus `specs/1189-org-detail-openai-credential-panel/`。
- [x] T035 Update PR body with UI behavior summary, E2E coverage map, safe metadata guarantee, mockscreen/result-screen evidence, and remaining CI state。

## Scope Stop

Stop after Todo planning PR moves Linear HO-1189 to `Waiting`. Production implementation starts only after Linear state is moved to `In Progress`.
