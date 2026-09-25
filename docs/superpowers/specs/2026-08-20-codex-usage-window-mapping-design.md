# Codex Usage Window Mapping Design

**Date:** 2026-08-20
**Status:** Approved for implementation planning
**Branch:** `codex/fix-codex-usage-window-mapping`

## Problem

The OpenAI Codex usage response can contain multiple scopes at once:

- a global `rate_limit` window for all models;
- one or more `additional_rate_limits` buckets such as
  `GPT-5.3-Codex-Spark`, each with its own primary and weekly windows.

The current normalized model has one global `PrimaryWindow`, one global
`WeeklyWindow`, and a flat additional bucket. During recursive parsing,
an additional bucket's `secondary_window` can populate the global
`WeeklyWindow`. The UI then hides a seven-day global primary window and
renders the bucket's weekly value as `All models`.

A live query using a Tianji database credential demonstrated the failure
shape:

```text
global rate_limit.primary_window:       7d / 4% used
Spark additional primary_window:        5h / 0% used
Spark additional secondary_window:      7d / 0% used
Tianji UI result:                       All models weekly / 0% used
```

The UI therefore does not faithfully represent the upstream scopes.

## Goal

Make Tianji display the same quota scopes as the official Codex usage
surface:

```text
Global / All models
  Weekly: 4% used

GPT-5.3-Codex-Spark
  5-hour limit: 0% used
  Weekly limit: 0% used
```

The fix must preserve raw global fields for routing and expose model-specific
windows without conflating them with global usage.

## Non-goals

- No database migration or persisted raw usage response.
- No changes to OpenAI subscription credential storage, OAuth refresh, or
  token resolution.
- No changes to `/v1` request routing policy or model payload behavior.
- No change to the unofficial endpoint's billing/status semantics.
- No new generic usage framework or provider abstraction.

## Design

### 1. Normalized provider model

Keep `UsageSnapshot.PrimaryWindow`, `UsageSnapshot.WeeklyWindow`, and
`UsageSnapshot.RateLimit` as the global scope. Change `UsageBucket` so each
bucket can retain both windows:

```text
UsageBucket
  Name
  PrimaryWindow
  WeeklyWindow
```

Flat legacy bucket shapes that contain only `used_percent`, `reset_at`, or
`status` map to the bucket primary window. Nested
`rate_limit.primary_window` and `rate_limit.secondary_window` map to the
bucket primary and weekly windows respectively.

The parser must not promote nested additional-bucket windows into the global
snapshot windows. Global seven-day data exposed by OpenAI as
`rate_limit.primary_window` remains global data. Existing handling that
displays a seven-day global primary window as the global weekly window remains
valid only when the global weekly window is otherwise absent.

### 2. Parser boundaries

Parse the top-level `rate_limit` first. Parse
`additional_rate_limits` as explicit bucket records rather than discovering
their windows through the global recursive walker.

The parser must:

- preserve top-level global primary and secondary windows;
- preserve every additional bucket name;
- preserve both bucket windows when present;
- tolerate missing optional bucket windows;
- retain support for the existing flat bucket fixture;
- avoid duplicate bucket entries caused by recursive traversal.

The normalized `RateLimit` pointer remains unchanged so existing selection
scoring continues to use the upstream rate-limit fields.

### 3. UI view model

Replace the UI's flat additional-window representation with a bucket view
containing two optional windows:

```text
CodexUsageBucketView
  Name
  PrimaryWindow
  WeeklyWindow
```

The credential card renders:

1. global plan usage, labelled `All models`;
2. each additional bucket under `Additional model limits`;
3. a 5-hour label for a bucket primary window;
4. a weekly label for a bucket weekly window.

The card must not label a bucket window `All models`. Missing windows are
omitted without hiding other known windows.

### 4. Runtime helpers and reports

Update the non-UI consumers of `UsageBucket`:

- reset/expiry calculation considers both bucket windows;
- snapshot-presence checks consider both bucket windows;
- Discord usage reporting emits bucket primary and weekly values with
  unambiguous labels;
- cache serialization remains normalized and backward-compatible when older
  entries lack the new fields.

Global routing score calculations continue to read `Snapshot.RateLimit`; this
change does not alter routing thresholds or credential selection behavior.

### 5. Safety

The existing credential-resolution path remains the only token access path.
The change stores and renders only normalized safe fields. It must not add
logging or persistence of access tokens, refresh tokens, authorization
headers, encrypted credential values, or raw upstream response bodies.

## Test plan

### Provider parser

Add a fixture matching the live response shape and assert:

- global seven-day usage remains `4%`;
- global weekly remains absent when the upstream global secondary window is
  absent;
- Spark primary is `0%` with a five-hour window;
- Spark weekly is `0%` with a seven-day window;
- flat legacy buckets still normalize;
- missing bucket windows do not drop global data.

### UI and handler

Assert that a card renders:

- global `All models` weekly usage;
- Spark five-hour usage;
- Spark weekly usage;
- no duplicate or misleading `All models` bucket label.

### Runtime consumers

Cover bucket-window reset calculation, snapshot-presence detection, and
Discord output. Preserve existing routing-score and credential lifecycle
tests.

### Verification commands

```bash
make ui
go test ./internal/provider/chatgptcodex ./internal/proxy/handler ./internal/ui ./internal/ui/pages
git diff --check
```

After implementation, run the focused usage tests first, then the broader
repository test gates required by CI.

## Rollout and acceptance

Acceptance requires:

1. the live raw response shape above is covered by a deterministic fixture;
2. Tianji renders global and model-specific windows separately;
3. existing proxy routing behavior remains unchanged;
4. no secret or raw response material appears in tests, logs, JSON, or HTML;
5. `ponytail-review` finds no avoidable abstraction or dead compatibility code;
6. the branch is pushed and the PR has a verified remote ref and CI state.
