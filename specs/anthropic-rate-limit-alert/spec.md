# Feature Specification: Anthropic Rate Limit Monitoring + Discord Alert

**Feature Branch**: `feat/anthropic-rate-limit-alert`
**Created**: 2026-03-01
**Updated**: 2026-03-01 (fix: use precise per-type input/output headers; store raw values without conversion)
**Status**: Draft
**Linear Issue**: HO-69

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Alert When Input Tokens Are Low (Priority: P1)

As an operator, I want a Discord alert when Anthropic input token quota drops below threshold,
so I can act before requests get rate limited.

**Independent Test**: Set `discord_webhook_url` + `ratelimit_alert_threshold: 0.2`. Send a
request that returns `anthropic-ratelimit-input-tokens-remaining` < 20% of
`anthropic-ratelimit-input-tokens-limit`. Verify Discord POST contains exact header values.

**Acceptance Scenarios**:

1. **Given** threshold 0.2, **When** `input-tokens-limit=10000`, `input-tokens-remaining=1000` (10%),
   **Then** Discord POST fires with exact values: limit=10000, remaining=1000, reset=<raw RFC3339 string>.

2. **Given** threshold 0.2, **When** `input-tokens-remaining=4000`, limit=10000 (40%),
   **Then** no alert.

3. **Given** `discord_webhook_url` not set, **Then** no alert, no error.

---

### User Story 2 - Alert When Output Tokens Are Low (Priority: P1)

Output tokens are a separate limit and can be exhausted independently from input tokens.

**Independent Test**: Trigger response with `output-tokens-remaining` below threshold.
Verify Discord alert fires with exact output token values.

**Acceptance Scenarios**:

1. **Given** threshold 0.2, **When** `output-tokens-limit=8000`, `output-tokens-remaining=1500` (18.75%),
   **Then** Discord alert fires with exact values: output limit=8000, remaining=1500.

2. **Given** input tokens above threshold, output tokens below threshold,
   **Then** alert fires for output tokens only (checks are independent).

---

### User Story 3 - Cooldown Prevents Spam (Priority: P2)

Separate cooldown per alert type to avoid flooding Discord.

**Acceptance Scenarios**:

1. **Given** cooldown 1h, input alert fired, **When** another input-below-threshold response arrives within 1h,
   **Then** second alert suppressed.

2. **Given** input alert on cooldown, **When** output tokens drop below threshold,
   **Then** output alert fires (separate cooldown key).

3. **Given** 61 minutes after last alert, **When** new triggering response arrives,
   **Then** new alert sent.

---

### User Story 4 - Configurable Threshold (Priority: P3)

**Acceptance Scenarios**:

1. `ratelimit_alert_threshold: 0.3` + remaining=25% → alert fires.
2. `ratelimit_alert_threshold: 0.1` + remaining=25% → no alert.

---

### Edge Cases

- `anthropic-ratelimit-input-tokens-remaining` missing → skip input check silently (debug log).
- `anthropic-ratelimit-input-tokens-limit` missing or parses as 0 → skip (warn log with raw value).
- `anthropic-ratelimit-output-tokens-*` missing → skip output check silently.
- Discord webhook returns non-2xx → warn log with status code + response body. Proxy unaffected.
- Streaming response → headers are on initial HTTP response; `ModifyResponse` fires before body is consumed. Correct.
- `ratelimit_alert_threshold` not set → default 0.2.

---

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: In `native_format.go` `ModifyResponse`, when provider is Anthropic, read ALL of:
  - `anthropic-ratelimit-input-tokens-limit`
  - `anthropic-ratelimit-input-tokens-remaining`
  - `anthropic-ratelimit-input-tokens-reset`
  - `anthropic-ratelimit-output-tokens-limit`
  - `anthropic-ratelimit-output-tokens-remaining`
  - `anthropic-ratelimit-output-tokens-reset`
  - `anthropic-ratelimit-requests-limit`
  - `anthropic-ratelimit-requests-remaining`
  - `anthropic-ratelimit-requests-reset`

- **FR-002**: All header values stored as raw parsed types — integers as `int64`, reset timestamps
  as raw `string` (RFC3339, as-is from header). **No rounding, no aggregation, no conversion.**

- **FR-003**: Two independent threshold checks per response:
  - `float64(input-tokens-remaining) / float64(input-tokens-limit) < threshold` → trigger input alert
  - `float64(output-tokens-remaining) / float64(output-tokens-limit) < threshold` → trigger output alert
  Each check has its own cooldown key. Both checks run on every Anthropic response.

- **FR-004**: Implement `DiscordRateLimitAlerter` in `internal/callback/discord_ratelimit.go`:
  - Fields: `webhookURL string`, `threshold float64`, `cooldown time.Duration`,
    `mu sync.Mutex`, `alerted map[string]time.Time`, `client *http.Client`
  - Method: `CheckAndAlert(state AnthropicRateLimitState)` (non-blocking, runs in goroutine)

- **FR-005**: Discord alert payload:
  ```json
  {"content": "⚠️ Anthropic Rate Limit\nInput: {remaining}/{limit} remaining (resets {reset})\nOutput: {remaining}/{limit} remaining (resets {reset})\nRequests: {remaining}/{limit} remaining (resets {reset})\nAlert triggered by: input | output | both"}
  ```
  All values are **exact integers / strings from headers**. Reset timestamps are the raw header
  string, not reformatted. The percentage shown in message is `remaining * 100 / limit` (integer,
  display only). Threshold comparison always uses `float64` on raw values.

- **FR-006**: Cooldown keys:
  - `"ratelimit:anthropic:input"` for input token alert
  - `"ratelimit:anthropic:output"` for output token alert
  Default cooldown: 1 hour. Per-key, independent.

- **FR-007**: `ProxyConfig` (`internal/config/config.go`) adds:
  - `DiscordWebhookURL string` yaml:`discord_webhook_url`
  - `RatelimitAlertThreshold float64` yaml:`ratelimit_alert_threshold` (default: 0.2)

- **FR-008**: `DiscordRateLimitAlerter` instantiated only if `discord_webhook_url` non-empty.
  Wired into `Handlers` struct, called from `ModifyResponse`.

- **FR-009**: Alert goroutine is fire-and-forget. No blocking in request path.

- **FR-010**: `ModifyResponse` must NOT return error from rate limit logic. All errors logged at
  warn level with raw header value, then swallowed. Proxy response continues normally.

- **FR-011**: Parse errors log raw header value:
  `log.Warnf("ratelimit: cannot parse %q=%q: %v", headerName, rawValue, err)`

### Key Entities

```go
// AnthropicRateLimitState holds raw parsed header values.
// No derived or computed fields. All values as received from Anthropic.
type AnthropicRateLimitState struct {
    InputTokensLimit      int64
    InputTokensRemaining  int64
    InputTokensReset      string // raw RFC3339 from header, not reformatted
    OutputTokensLimit     int64
    OutputTokensRemaining int64
    OutputTokensReset     string
    RequestsLimit         int64
    RequestsRemaining     int64
    RequestsReset         string
}
```

---

## Success Criteria *(mandatory)*

- **SC-001**: Input tokens below threshold → Discord POST within 500ms, message contains exact
  `InputTokensLimit`, `InputTokensRemaining`, `InputTokensReset` values from headers.

- **SC-002**: Output tokens below threshold → Discord POST within 500ms, message contains exact
  `OutputTokensLimit`, `OutputTokensRemaining`, `OutputTokensReset` values from headers.

- **SC-003**: 100 triggering responses within 1 hour → exactly 1 alert per type (input, output).

- **SC-004**: Proxy P99 latency unaffected (alert is goroutine, not in request path).

- **SC-005**: `discord_webhook_url` absent → no `DiscordRateLimitAlerter` instantiated, zero alert code executed.

- **SC-006**: All existing `internal/callback/` and `internal/proxy/handler/` tests pass.

- **SC-007**: Missing or unparseable headers → proxy completes normally, warn log, no panic, no client error.
