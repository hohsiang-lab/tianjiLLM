# Codex Usage Window Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task with checkpoints.

**Goal:** Preserve global Codex quota windows separately from model-specific buckets so Tianji renders the same usage scopes as OpenAI Codex.

**Architecture:** Keep `UsageSnapshot.RateLimit`, `PrimaryWindow`, and `WeeklyWindow` scoped to the top-level upstream `rate_limit`. Parse each `additional_rate_limits` entry into a bucket with its own primary and weekly windows, then expose those windows through the existing UI, Discord, cache, and reset consumers without changing routing policy or credential resolution.

**Tech Stack:** Go 1.26, `chatgptcodex` normalized provider types, HTMX/templ UI, pgx-backed runtime cache, Go tests, `make ui`, GitHub Actions.

**Spec:** `/Users/norman/.codex/worktrees/06f6/tianjiLLM/docs/superpowers/specs/2026-08-20-codex-usage-window-mapping-design.md`

## Global Constraints

- Do not add a database migration or persist raw upstream usage JSON.
- Do not change credential storage, OAuth refresh, token resolution, proxy routing, or model payload behavior.
- Keep `UsageSnapshot.RateLimit` unchanged for routing score calculations.
- Never log, render, persist, or return tokens, authorization headers, encrypted credential values, or raw upstream bodies.
- Preserve support for existing flat additional-bucket response fixtures.
- Keep the diff minimal; do not add a generic usage abstraction.

---

### Task 1: Make provider normalization scope-aware

**Files:**
- Modify: `internal/provider/chatgptcodex/usage.go:45-81,233-336`
- Test: `internal/provider/chatgptcodex/usage_test.go`

**Interfaces:**
- Consumes: decoded `map[string]any` from `normalizeUsageSnapshot`.
- Produces: `UsageBucket{Name, PrimaryWindow, WeeklyWindow}` and global-only `UsageSnapshot.PrimaryWindow`/`WeeklyWindow`.

- [ ] **Step 1: Add the failing live-shape parser test**

Add a test in `internal/provider/chatgptcodex/usage_test.go` using this fixture shape:

```json
{
  "plan_type": "pro",
  "rate_limit": {
    "allowed": true,
    "limit_reached": false,
    "primary_window": {
      "used_percent": 4,
      "limit_window_seconds": 604800,
      "reset_after_seconds": 592249,
      "reset_at": 1787801789
    }
  },
  "additional_rate_limits": [
    {
      "limit_name": "GPT-5.3-Codex-Spark",
      "rate_limit": {
        "allowed": true,
        "limit_reached": false,
        "primary_window": {
          "used_percent": 0,
          "limit_window_seconds": 18000,
          "reset_after_seconds": 18000,
          "reset_at": 1787227541
        },
        "secondary_window": {
          "used_percent": 0,
          "limit_window_seconds": 604800,
          "reset_after_seconds": 604800,
          "reset_at": 1787814341
        }
      }
    }
  ]
}
```

Assert:

```go
require.NotNil(t, snapshot.PrimaryWindow.UsedPercent)
assert.InDelta(t, 0.04, *snapshot.PrimaryWindow.UsedPercent, 0.0001)
assert.Equal(t, int64(604800), *snapshot.PrimaryWindow.LimitWindowSeconds)
assert.Nil(t, snapshot.WeeklyWindow.UsedPercent)
require.Len(t, snapshot.AdditionalBuckets, 1)
assert.Equal(t, "GPT-5.3-Codex-Spark", snapshot.AdditionalBuckets[0].Name)
assert.InDelta(t, 0, *snapshot.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
assert.Equal(t, int64(18000), *snapshot.AdditionalBuckets[0].PrimaryWindow.LimitWindowSeconds)
assert.InDelta(t, 0, *snapshot.AdditionalBuckets[0].WeeklyWindow.UsedPercent, 0.0001)
assert.Equal(t, int64(604800), *snapshot.AdditionalBuckets[0].WeeklyWindow.LimitWindowSeconds)
```

- [ ] **Step 2: Run the focused test and confirm the current bug**

Run:

```bash
go test ./internal/provider/chatgptcodex -run 'TestUsageClientFetch_NormalizesGlobalAndAdditionalWindows' -count=1
```

Expected: FAIL because `UsageBucket` has no window fields and the nested
secondary window currently fills the global weekly window.

- [ ] **Step 3: Change the normalized bucket type**

Replace the flat fields in `UsageBucket` with:

```go
type UsageBucket struct {
    Name          string      `json:"name,omitempty"`
    PrimaryWindow UsageWindow `json:"primary_window,omitempty"`
    WeeklyWindow  UsageWindow `json:"weekly_window,omitempty"`
}
```

- [ ] **Step 4: Parse additional buckets explicitly**

Add a helper near `usageRateLimitFromMap`, with a concrete signature:

```go
func usageAdditionalBucketsFromMap(root map[string]any) []UsageBucket
```

The helper must:

1. read `root["additional_rate_limits"]` as `[]any`;
2. derive a sanitized bucket name from `limit_name`, `name`, `model`, or
   `bucket`;
3. parse nested `rate_limit.primary_window` into `PrimaryWindow`;
4. parse nested `rate_limit.secondary_window` into `WeeklyWindow`;
5. map a flat bucket-level usage field into `PrimaryWindow`;
6. skip entries without a usable name and usage window;
7. preserve deterministic input order.

Call it from `normalizeUsageSnapshot` before the generic walk, and stop the
generic walk from promoting windows under `additional_rate_limits` into the
global `PrimaryWindow` or `WeeklyWindow`.

- [ ] **Step 5: Preserve existing parser fixtures**

Update existing tests that assert `AdditionalBuckets[0].UsedPercent`,
`ResetAt`, or `Status` to assert the equivalent
`AdditionalBuckets[0].PrimaryWindow` fields. Keep the flat
`additional_rate_limits` fixture and prove it still produces one bucket.

- [ ] **Step 6: Run provider tests**

Run:

```bash
go test ./internal/provider/chatgptcodex -run 'Usage|Wham' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the provider change**

```bash
git add internal/provider/chatgptcodex/usage.go internal/provider/chatgptcodex/usage_test.go
git commit -m "fix: preserve global and bucket Codex usage windows"
```

### Task 2: Update runtime consumers without changing routing

**Files:**
- Modify: `internal/proxy/handler/openai_subscription_codex_usage.go:510-530,660-700`
- Modify: `internal/proxy/handler/openai_subscription_discord_usage_report.go:160-185`
- Test: `internal/proxy/handler/openai_subscription_codex_usage_test.go`
- Test: `internal/proxy/handler/openai_subscription_discord_usage_report_test.go`

**Interfaces:**
- Consumes: `UsageBucket.PrimaryWindow` and `UsageBucket.WeeklyWindow`.
- Produces: correct snapshot-presence checks, reset expiry checks, and unambiguous Discord output.

- [ ] **Step 1: Add failing runtime-consumer assertions**

Add a bucket fixture with both windows and assert:

```go
assert.True(t, codexUsageResultHasSnapshot(result))
assert.True(t, hasExpectedEarliestReset(snapshot, now))
```

Add Discord report assertions for distinct bucket fields, for example:

```text
additional_GPT-5.3-Codex-Spark_primary_5h=0.0%
additional_GPT-5.3-Codex-Spark_weekly_7d=0.0%
```

Use the existing redaction/sanitization helpers for bucket names.

- [ ] **Step 2: Run the focused tests and confirm failures**

Run:

```bash
go test ./internal/proxy/handler -run 'CodexUsage|DiscordUsageReport' -count=1
```

Expected: FAIL at references to the removed flat bucket fields or missing
bucket-window output.

- [ ] **Step 3: Update snapshot presence and reset calculations**

Change `codexUsageResultHasSnapshot` and `codexUsageEarliestReset` to inspect
both bucket windows:

```go
for _, bucket := range snapshot.AdditionalBuckets {
    considerWindow(bucket.PrimaryWindow)
    considerWindow(bucket.WeeklyWindow)
}
```

Retain the existing global `RateLimit`-based selection score logic unchanged.

- [ ] **Step 4: Update Discord rendering**

Render each present bucket window with a stable scope suffix. Do not emit a
bucket's value as the global `weekly_7d` field. Keep deterministic
credential-id and bucket ordering.

- [ ] **Step 5: Run runtime tests**

Run:

```bash
go test ./internal/proxy/handler -run 'OpenAISubscription.*Usage|CodexUsage|DiscordUsageReport' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit runtime consumer changes**

```bash
git add internal/proxy/handler/openai_subscription_codex_usage.go \
  internal/proxy/handler/openai_subscription_codex_usage_test.go \
  internal/proxy/handler/openai_subscription_discord_usage_report.go \
  internal/proxy/handler/openai_subscription_discord_usage_report_test.go
git commit -m "fix: keep Codex bucket scopes in runtime consumers"
```

### Task 3: Render global and model-specific windows in the UI

**Files:**
- Modify: `internal/ui/handler_credentials.go:425-436`
- Modify: `internal/ui/pages/usage_codex.templ:20-33,135-165`
- Modify: generated `internal/ui/pages/usage_codex_templ.go` via `make ui`
- Test: `internal/ui/handler_codex_usage_test.go`
- Test: `internal/ui/pages/usage_codex_test.go`

**Interfaces:**
- Consumes: global snapshot windows and bucket windows from Task 1.
- Produces: `CodexUsageBucketView` with explicit primary/weekly UI windows.

- [ ] **Step 1: Add failing UI mapping and render tests**

Add a handler test that builds a snapshot with:

```go
globalPrimary := UsageWindow{UsedPercent: float64Ptr(0.04), LimitWindowSeconds: int64Ptr(604800)}
sparkPrimary := UsageWindow{UsedPercent: float64Ptr(0), LimitWindowSeconds: int64Ptr(18000)}
sparkWeekly := UsageWindow{UsedPercent: float64Ptr(0), LimitWindowSeconds: int64Ptr(604800)}
```

Assert:

```go
assert.Nil(t, card.PrimaryWindow.UsedPercent)
require.NotNil(t, card.WeeklyWindow.UsedPercent)
assert.InDelta(t, 0.04, *card.WeeklyWindow.UsedPercent, 0.0001)
require.Len(t, card.AdditionalBuckets, 1)
assert.InDelta(t, 0, *card.AdditionalBuckets[0].PrimaryWindow.UsedPercent, 0.0001)
assert.InDelta(t, 0, *card.AdditionalBuckets[0].WeeklyWindow.UsedPercent, 0.0001)
```

Add a template assertion for:

```text
All models
Additional model limits
GPT-5.3-Codex-Spark
5-hour limit
Weekly limit
4% used
0% used
```

Also assert the bucket is not rendered with the `All models` label.

- [ ] **Step 2: Run the focused UI tests and confirm failures**

Run:

```bash
go test ./internal/ui ./internal/ui/pages -run 'CodexUsage|Usage' -count=1
```

Expected: FAIL because the current card and template use
`[]CodexUsageWindowView` for flat buckets.

- [ ] **Step 3: Add the UI bucket view model**

Define a small view type beside the existing Codex usage view types:

```go
type CodexUsageBucketView struct {
    Name          string
    PrimaryWindow CodexUsageWindowView
    WeeklyWindow  CodexUsageWindowView
}
```

Change `CodexUsageCredentialCard.AdditionalBuckets` to
`[]CodexUsageBucketView`. Map both provider windows in
`codexUsageCardForCredential`.

- [ ] **Step 4: Update the template**

Keep the global section labelled `All models`. Add a separate
`Additional model limits` section. For each bucket, render only windows with
data:

```templ
if codexUsageWindowHasData(bucket.PrimaryWindow) {
    @codexUsageWindow("5-hour limit", bucket.PrimaryWindow)
}
if codexUsageWindowHasData(bucket.WeeklyWindow) {
    @codexUsageWindow("Weekly limit", bucket.WeeklyWindow)
}
```

Do not add a new component library or generic quota abstraction.

- [ ] **Step 5: Regenerate templ output**

Run:

```bash
make ui
git diff --check
```

- [ ] **Step 6: Run UI tests**

Run:

```bash
go test ./internal/ui ./internal/ui/pages -run 'CodexUsage|Usage' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit the UI change**

```bash
git add internal/ui/handler_credentials.go \
  internal/ui/pages/usage_codex.templ \
  internal/ui/pages/usage_codex_templ.go \
  internal/ui/handler_codex_usage_test.go \
  internal/ui/pages/usage_codex_test.go
git commit -m "fix: render global and model Codex usage scopes"
```

### Task 4: Full verification, ponytail review, and PR delivery

**Files:**
- Review: all branch changes relative to `origin/main`
- No new production files unless required by the preceding tasks

**Interfaces:**
- Consumes: completed provider, runtime, and UI changes.
- Produces: verified branch, PR, green CI, and merged mainline commit.

- [ ] **Step 1: Run focused regression gates**

```bash
go test ./internal/provider/chatgptcodex -run 'Usage|Wham' -count=1
go test ./internal/proxy/handler -run 'OpenAISubscription.*Usage|CodexUsage|DiscordUsageReport' -count=1
go test ./internal/ui ./internal/ui/pages -run 'CodexUsage|Usage' -count=1
```

- [ ] **Step 2: Run repository checks**

```bash
make ui
go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/ui ./internal/ui/pages
git diff --check
```

If a check fails, fix the smallest root-cause change, rerun the failed
command, and record the exact error before continuing.

- [ ] **Step 3: Run ponytail-review on the final diff**

Review only over-engineering. Report each finding in the required format:

```text
<file>:L<line>: <tag> <what to cut>. <replacement>.
```

If no complexity is removable, report:

```text
Lean already. Ship.
net: -0 lines possible.
```

Apply only valid simplifications, rerun affected tests, and do not use the
review to weaken correctness or secret-safety behavior.

- [ ] **Step 4: Verify branch hygiene and commit state**

```bash
git status --short
git diff --check
git log --oneline --decorate -5
```

The worktree must contain only intended changes and the branch must include
the design commit plus implementation commits.

- [ ] **Step 5: Push and create the PR**

```bash
git push -u origin codex/fix-codex-usage-window-mapping
gh pr create \
  --base main \
  --head codex/fix-codex-usage-window-mapping \
  --title "fix: preserve global and model-specific Codex usage" \
  --body-file /tmp/codex-usage-pr-body.md
```

The PR body must state:

- raw `wham/usage` global and bucket scopes were conflated;
- global and model-specific windows are now rendered separately;
- no routing, credential, or database schema behavior changed;
- focused tests and full checks run locally;
- live production verification remains best-effort because `wham/usage` is
  unofficial.

- [ ] **Step 6: Monitor CI and fix failures**

```bash
gh pr checks <PR_NUMBER> --watch
```

For every failure, inspect the failing job and logs, make the smallest fix,
run the corresponding local test, push, and repeat until every required check
is green.

- [ ] **Step 7: Merge only after green gates**

```bash
gh pr view <PR_NUMBER> --json number,state,mergeable,statusCheckRollup,url
gh pr merge <PR_NUMBER> --squash --delete-branch
gh pr view <PR_NUMBER> --json number,state,mergedAt,mergeCommit
git fetch origin main
git status --short --branch
```

Acceptance requires the PR to be merged, required checks green, and the
working tree clean.
