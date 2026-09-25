# Tasks: Codex subscription route E2E and credential lookup recovery

## Todo Phase

- [ ] T001 Confirm Linear state is `In Progress` before editing production or test code.
- [ ] T002 Re-read this spec, plan, research, and the latest Linear comments before implementation.

## Phase 1 - RED Tests

- [ ] T003 Add DB-backed credential lookup fixture for `CredentialTable` and the handler config master key.
- [ ] T004 Add DB-managed `ProxyModelTable` wildcard fixture and runtime model refresh coverage for `openai/*`.
- [ ] T005 Add `TestLoadOpenAISubscriptionCredential_DBBackedExistingRow`.
- [ ] T006 Add `TestLoadOpenAISubscriptionCredential_DistinguishesMissingFromLookupFailure`.
- [ ] T007 Add full-route streaming test `TestCodexSubscriptionRouteE2E_DBBackedCredentialStreams`.
- [ ] T008 Add stale-token full-route test `TestCodexSubscriptionRouteE2E_StaleCredentialRefreshesBeforeStreaming`.
- [ ] T009 Add subscription non-streaming contract test `TestCodexSubscriptionRoute_NonStreamingContractIsDeterministic`.
- [ ] T010 Add all-candidate failure route/resolver test preserving reason codes.
- [ ] T011 Re-run API-key wildcard no-regression or extend `TestOpenAIAPIKeyWildcard_StillUsesChatCompletionsPath`.
- [ ] T012 Run targeted RED command and confirm at least one issue-owned test fails for the intended lookup/runtime integration gap.
- [ ] T013 Commit and push the RED tests before applying the fix.

## Phase 2 - Minimal Fix

- [ ] T014 Fix only the boundary proven by the RED test: generated DB lookup/schema/scan, resolver classification, or runtime model/credential integration.
- [ ] T015 Preserve `credential_missing` vs `credential_lookup_failed` distinction.
- [ ] T016 Preserve wrong-type, disabled, malformed, refresh, and auth-after-refresh reason codes.
- [ ] T017 Ensure explicit subscription credential failure still does not fallback to API key.
- [ ] T018 Ensure route sends subscription bearer only to Codex backend mock and never to Platform mock.

## Phase 3 - Verification

- [ ] T019 Run `go test ./internal/proxy/handler -run 'TestCodexSubscriptionRoute|TestLoadOpenAISubscriptionCredential|TestChatGPTCodexBackendWildcard|TestOpenAIAPIKeyWildcard' -count=1`.
- [ ] T020 Run `go test ./internal/provider/chatgptcodex -run 'Test.*Payload|Test.*Stream' -count=1`.
- [ ] T021 Run `go test ./internal/proxy/handler ./internal/provider/chatgptcodex ./internal/provider/openai -count=1`.
- [ ] T022 If SQL/generated DB code changed, run `sqlc generate` and `go test ./internal/db ./internal/proxy/handler -count=1`.
- [ ] T023 Run `gofmt` on changed Go files.
- [ ] T024 Run `git diff --check origin/main...HEAD`.
- [ ] T025 Audit changed files and commits to ensure HO-1314-only scope.

## Phase 4 - State Progression

- [ ] T026 Commit and push the fix.
- [ ] T027 Move Linear to `In Review` only after RED+fix verification passes.
- [ ] T028 Run the required review gate.
- [ ] T029 After review pass, move through `Waiting CI` and wait for CI webhook.

## Guardrails

- [ ] G001 Do not edit production/test code while Linear remains `Todo`.
- [ ] G002 Do not call real OpenAI or ChatGPT from automated tests.
- [ ] G003 Do not print raw bearer/access/refresh tokens, cookies, decrypted credential JSON, or Authorization headers.
- [ ] G004 Do not change OpenAI API-key wildcard route unless a no-regression test proves behavior is preserved.
