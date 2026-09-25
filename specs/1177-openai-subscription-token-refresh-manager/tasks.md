# Tasks: OpenAI Subscription Token Refresh Manager

**Input**: SpecKit artifacts under `specs/1177-openai-subscription-token-refresh-manager/`

## Phase 1: Provider Refresh Helper Tests

- [x] T001 [P] Add failing `TestOpenAIRefreshToken_PublicClientRequestShape` in `internal/provider/openai/oauth_test.go`.
- [x] T002 [P] Add failing `TestOpenAIRefreshToken_ParsesRotatedRefreshToken` in `internal/provider/openai/oauth_test.go`.
- [x] T003 [P] Add failing `TestOpenAIRefreshToken_ErrorRedactsRequestSecrets` in `internal/provider/openai/oauth_test.go`.

## Phase 2: Handler Refresh Manager Tests

- [x] T004 Add failing `TestResolveOpenAISubscriptionCredential_FreshTokenSkipsRefresh` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T005 Add failing `TestResolveOpenAISubscriptionCredential_RefreshesBeforeExpiryBuffer` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T006 Add failing `TestResolveOpenAISubscriptionCredential_ConcurrentRefreshSingleflight` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T007 Add failing `TestResolveOpenAISubscriptionCredential_RefreshPersistsRotatedToken` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T008 Add failing `TestResolveOpenAISubscriptionCredential_RefreshFailurePersistsRedactedMetadata` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T009 Add failing `TestResolveOpenAISubscriptionCredential_TypedFailureCodes` in `internal/proxy/handler/openai_subscription_refresh_test.go`.
- [x] T010 Add failing `TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken` in `internal/proxy/handler/openai_subscription_resolution_test.go`.

## Phase 3: Provider Refresh Helper Implementation

- [x] T011 Implement `openai.RefreshToken(ctx, client, cfg, refreshToken)` beside `ExchangeCode`.
- [x] T012 Ensure refresh helper posts `grant_type=refresh_token`, `refresh_token`, and `client_id`.
- [x] T013 Ensure refresh helper rejects/omits `client_secret` and Basic auth.
- [x] T014 Ensure refresh helper parses access token, optional refresh token, token type, expiry, scope, and raw metadata.
- [x] T015 Ensure refresh helper redacts token-shaped data in returned errors.

## Phase 4: Refresh Manager Implementation

- [x] T016 Add handler-local refresh buffer constant defaulting to `5 * time.Minute`.
- [x] T017 Add testable clock/dependency hook for expiry-buffer tests.
- [x] T018 Add typed `OpenAISubscriptionCredentialError` with stable code constants.
- [x] T019 Add per-handler `singleflight.Group` keyed by `credential_id`.
- [x] T020 Implement fresh-token short-circuit with no upstream call.
- [x] T021 Implement refresh-needed path that reloads the credential inside the singleflight function.
- [x] T022 Implement missing/wrong-type/disabled/malformed/expired typed error mapping.

## Phase 5: Persistence and Metadata

- [x] T023 Persist successful refresh with `UpdateOpenAISubscriptionCredential`.
- [x] T024 Preserve existing refresh token when refresh response omits `refresh_token`.
- [x] T025 Persist rotated refresh token when refresh response includes `refresh_token`.
- [x] T026 Persist safe success metadata with `status=active` and `last_refresh_at`.
- [x] T027 Persist redacted failure metadata with `status=refresh_failed`, `last_error`, and safe reason.
- [x] T028 Ensure refresh failure does not rewrite encrypted `credential_value`.

## Phase 6: Resolver Integration

- [x] T029 Replace current expired-token hard failure with refresh manager call.
- [x] T030 Ensure direct HTTP resolution returns refreshed bearer material.
- [x] T031 Ensure Codex app-server resolution returns refreshed `chatgptAuthTokens`.
- [x] T032 Ensure no-subscription/API-key behavior remains unchanged.
- [x] T033 Ensure no API-key fallback occurs after subscription IDs are configured.

## Phase 7: Verification

- [x] T034 Run `go test ./internal/provider/openai/... -run 'TestOpenAIRefreshToken' -v`.
- [x] T035 Run `go test ./internal/proxy/handler/... -run 'TestResolveOpenAISubscriptionCredential_.*Refresh|TestResolveOpenAISubscriptionCredential_Typed|TestResolveOpenAISubscriptionCredential_CodexUsesRefreshedToken' -v`.
- [x] T036 Run `go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/testutil/openaitest/... -v`.
- [x] T037 Run `git diff --check origin/main...HEAD`.
- [x] T038 Confirm no test performs real `auth.openai.com` or `api.openai.com` calls.
