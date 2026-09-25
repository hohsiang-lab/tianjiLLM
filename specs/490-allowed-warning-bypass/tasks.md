# Tasks: HO-490 — allowed_warning bypass

**Branch**: `490-allowed-warning-bypass`
**Plan**: [plan.md](./plan.md) | **Spec**: [spec.md](./spec.md)

---

## Phase 1: Setup

- [X] T001 Verify branch `490-plan-review` checked out (plan-review base)

---

## Phase 2: Foundational — Replace consts

- [X] T002 Replace `UnifiedStatusRateLimited`/`UnifiedStatusOverage` with `UnifiedStatusRejected` + add `UnifiedStatusAllowedWarning` in `internal/callback/ratelimit_store.go`

---

## Phase 3: US-1 — allowed_warning bypass

**Goal**: Tokens with `allowed_warning` status bypass utilization gate and remain available.

**Independent test criteria**: `TestSelectUpstreamThrottle_AllowedWarning_BypassesUtilizationGate` passes.

- [X] T003 [US1] Write failing test `TestSelectUpstreamThrottle_AllowedWarning_BypassesUtilizationGate` in `internal/proxy/handler/native_upstream_test.go` — assert token with `allowed_warning`+5h=0.92 is selected ≥1 time in 20 trials
- [X] T004 [US1] Write failing test `TestSelectUpstreamThrottle_Allowed_HighUtil_StillThrottled_NoRegression` in `internal/proxy/handler/native_upstream_test.go` — assert `allowed`+5h=0.92 never selected
- [X] T005 [US1] Run tests to confirm T003/T004 compile and fail: `go test ./internal/proxy/handler/... -run "TestSelectUpstreamThrottle_AllowedWarning|TestSelectUpstreamThrottle_Allowed_HighUtil" -v`
- [X] T006 [US1] Implement `allowed_warning` bypass in `selectUpstreamWithThrottle` in `internal/proxy/handler/native_upstream.go`
- [X] T007 [US1] Run T003/T004 — confirm both pass

---

## Phase 4: US-2 — rejected token skip

**Goal**: Tokens with `rejected` status are skipped. Remove stale `rate_limited`/`overage` test references.

**Independent test criteria**: `TestSelectUpstreamThrottle_SkipsRejectedStatus` passes.

- [X] T008 [US2] Write failing test `TestSelectUpstreamThrottle_SkipsRejectedStatus` in `internal/proxy/handler/native_upstream_test.go` — assert `rejected` token never selected
- [X] T008b [US2] Write failing test `TestSelectUpstreamThrottle_RejectedStatusIgnoredAfterReset` in `internal/proxy/handler/native_upstream_test.go` — assert stale `rejected` token (UnifiedReset in past) is included after status pruned
- [X] T009 [US2] Run T008/T008b to confirm they compile and fail
- [X] T010 [US2] Implement `rejected` skip in `selectUpstreamWithThrottle` in `internal/proxy/handler/native_upstream.go`
- [X] T011 [US2] Replace stale test `TestSelectUpstreamThrottle_SkipsRateLimitedStatus` with `TestSelectUpstreamThrottle_SkipsRejectedStatus` in `internal/proxy/handler/native_upstream_test.go`
- [X] T011b [US2] Replace stale test `TestSelectUpstreamThrottle_OverageStatusIgnoredAfterReset` with `TestSelectUpstreamThrottle_RejectedStatusIgnoredAfterReset` in `internal/proxy/handler/native_upstream_test.go`
- [X] T012 [US2] Update `ratelimit_store_test.go`, `ratelimit_flusher_test.go` — replace `UnifiedStatusRateLimited`/`UnifiedStatusOverage` with `UnifiedStatusRejected`
- [X] T013 [US2] Run `go test ./internal/callback/... ./internal/proxy/handler/...` — confirm all pass

---

## Phase 5: US-3 — Discord alerts

**Goal**: Discord alerts fire for `rejected` (blocking) and `allowed_warning` (non-blocking).

**Independent test criteria**: `TestCheckAndAlertOAuth_StatusRejected_TriggersAlert` and `TestCheckAndAlertOAuth_AllowedWarning_TriggersAlert` pass.

- [X] T015 [P] [US3] Write failing test `TestCheckAndAlertOAuth_StatusRejected_TriggersAlert` in `internal/callback/discord_ratelimit_oauth_test.go` — assert 1 alert fires, message contains "rejected"
- [X] T016 [P] [US3] Write failing test `TestCheckAndAlertOAuth_AllowedWarning_TriggersAlert` in `internal/callback/discord_ratelimit_oauth_test.go` — assert ≥1 alert fires, message contains "allowed_warning"
- [X] T017 [US3] Run T015/T016 to confirm compile and fail
- [X] T018 [US3] Implement `rejected` alert (early-return) and `allowed_warning` alert (non-blocking) in `internal/callback/discord_ratelimit.go`
- [X] T019 [US3] Rename `TestCheckAndAlert_OAuth_RateLimited_AlertFiresAndReturns` → `TestCheckAndAlert_OAuth_Rejected_AlertFiresAndReturns` in `internal/callback/discord_ratelimit_test.go`; update raw string + assertion to `rejected`
- [X] T019b [US3] Update `TestCheckAndAlert_OAuth_Overage_AlertFiresAndReturns` → replace with `rejected` variant (duplicate of T019 scenario; merge or remove)
- [X] T020 [US3] Run `go test ./internal/callback/... -run "TestCheckAndAlert" -v` — confirm all pass

---

## Phase 6: Polish & Final Verification

- [X] T021 `go build ./...` — confirm no compile errors
- [X] T022 `go test ./internal/...` — confirm all tests pass
- [X] T023 `go vet ./...` — confirm no vet issues
- [ ] T024 Commit with message `feat: add allowed_warning bypass + rejected const (HO-490)`
- [ ] T025 Push and open PR targeting `main`
- [ ] T026 Update HO-490 Linear state → `In Review`

---

## Dependencies

```
T001 → T002 → T003..T007 (US-1)
              T008..T014 (US-2, can start after T002)
              T015..T020 (US-3, can start after T002)
T007 + T014 + T020 → T021..T026
```

US-1, US-2, US-3 are **parallel** after T002 (separate files, no shared state).
