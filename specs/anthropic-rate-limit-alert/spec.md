# Feature Specification: Anthropic Rate Limit Monitoring + Discord Alert

**Feature Branch**: `feat/anthropic-rate-limit-alert`
**Created**: 2026-03-01
**Updated**: 2026-03-01 (C-04: sentinel -1 for missing/unparseable headers; C-03: threshold default 0.2)
**Status**: Draft
**Linear Issue**: HO-69

## Design Principles

- **No derived fields.** All values stored exactly as received from headers.
- **No silent fallbacks.** Missing or unparseable headers are logged as errors, not swallowed.
  Errors must be observable. The alert check is skipped only because the data is unavailable,
  and that unavailability is always recorded in the log.
- **No combined/aggregated tokens.** Input and output are tracked independently.

---

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Alert When Input Tokens Are Low (Priority: P1)

As an operator, I want a Discord alert when Anthropic input token quota drops below threshold.

**Independent Test**: Set `discord_webhook_url` + `ratelimit_alert_threshold: 0.2`. Send a
request that returns `anthropic-ratelimit-input-tokens-remaining` < 20% of
`anthropic-ratelimit-input-tokens-limit`. Verify Discord POST contains exact header values.

**Acceptance Scenarios**:

1. **Given** threshold 0.2, **When** `input-tokens-limit=10000`, `input-tokens-remaining=1000` (10%),
   **Then** Discord POST fires with exact values: limit=10000, remaining=1000, reset=<raw RFC3339>.

2. **Given** threshold 0.2, **When** `input-tokens-remaining=4000`, limit=10000 (40%),
   **Then** no alert.

3. **Given** `discord_webhook_url` not set, **Then** no alert, no error log.

---

### User Story 2 - Alert When Output Tokens Are Low (Priority: P1)

Output tokens are a separate limit exhausted independently from input tokens.

**Acceptance Scenarios**:

1. **Given** threshold 0.2, **When** `output-tokens-limit=8000`, `output-tokens-remaining=1500` (18.75%),
   **Then** Discord alert fires with exact values: output limit=8000, remaining=1500.

2. **Given** input above threshold, output below threshold,
   **Then** alert fires for output only (checks are fully independent).

---

### User Story 3 - Cooldown Prevents Spam (Priority: P2)

**Acceptance Scenarios**:

1. **Given** cooldown 1h, key `"ratelimit:anthropic:input"`, **When** another triggering response
   arrives within 1h, **Then** second alert suppressed.

2. **Given** input on cooldown, **When** output drops below threshold,
   **Then** output alert fires (separate key `"ratelimit:anthropic:output"`).

3. **Given** 61 minutes after last alert, new triggering response → new alert sent.

---

### User Story 4 - Configurable Threshold (Priority: P3)

1. `ratelimit_alert_threshold: 0.3` + remaining=25% → alert fires.
2. `ratelimit_alert_threshold: 0.1` + remaining=25% → no alert.

---

## Implementation Clarifications

### C-01: Provider Detection
The rate limit header check runs only when `providerName == "anthropic"`.
This matches the existing pattern in `native_format.go`:
```go
switch providerName {
case "anthropic":
    // existing OAuth header logic
    // ADD: read rate limit headers here
}
```
No other provider detection mechanism is needed.

### C-02: Goroutine Ownership
`CheckAndAlert` is responsible for spawning its own goroutine internally:
```go
func (d *DiscordRateLimitAlerter) CheckAndAlert(state AnthropicRateLimitState) {
    go func() {
        // threshold check + cooldown + Discord POST
    }()
}
```
The caller (`ModifyResponse`) calls `CheckAndAlert(state)` synchronously — it does not wrap it in `go`.
This keeps async logic encapsulated inside the alerter, not leaked to the caller.

### C-03: Zero Value Threshold
Go's zero value for `float64` is `0.0`. If `RatelimitAlertThreshold` is not set in config,
it will be `0.0`, which would cause every request to trigger an alert.

Resolution: `NewDiscordRateLimitAlerter` applies the default internally:
```go
func NewDiscordRateLimitAlerter(webhookURL string, threshold float64, ...) *DiscordRateLimitAlerter {
    if threshold == 0 {
        threshold = 0.2
    }
    ...
}
```
`ProxyConfig.Validate()` is not modified. The default is applied at construction time.



### C-04: Sentinel Value for Missing / Unparseable Headers

`AnthropicRateLimitState` uses `-1` as sentinel value to indicate a header was absent or could not be parsed.

```go
type AnthropicRateLimitState struct {
    InputTokensLimit      int64  // -1 if missing or unparseable
    InputTokensRemaining  int64  // -1 if missing or unparseable
    InputTokensReset      string // "" if missing
    OutputTokensLimit     int64  // -1 if missing or unparseable
    OutputTokensRemaining int64  // -1 if missing or unparseable
    OutputTokensReset     string // "" if missing
    RequestsLimit         int64  // -1 if missing or unparseable
    RequestsRemaining     int64  // -1 if missing or unparseable
    RequestsReset         string // "" if missing
}
```

**Why -1**: `0` is a theoretically valid header value. `-1` is never a valid token count, making it unambiguous.
This pattern is used by `liushuangls/go-anthropic` (the most widely-used Anthropic Go SDK community library).

**Guard in CheckAndAlert**:
```go
// Only check input tokens if both limit and remaining are valid (not -1)
if state.InputTokensLimit > 0 && state.InputTokensRemaining >= 0 {
    ratio := float64(state.InputTokensRemaining) / float64(state.InputTokensLimit)
    if ratio < d.threshold {
        // trigger input alert
    }
}
```

**No `InputParsed` / `OutputParsed` bool fields.** Sentinel value replaces them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: In `native_format.go` `ModifyResponse`, when provider is Anthropic, attempt to read
  ALL of the following headers:
  - `anthropic-ratelimit-input-tokens-limit`
  - `anthropic-ratelimit-input-tokens-remaining`
  - `anthropic-ratelimit-input-tokens-reset`
  - `anthropic-ratelimit-output-tokens-limit`
  - `anthropic-ratelimit-output-tokens-remaining`
  - `anthropic-ratelimit-output-tokens-reset`
  - `anthropic-ratelimit-requests-limit`
  - `anthropic-ratelimit-requests-remaining`
  - `anthropic-ratelimit-requests-reset`

- **FR-002**: Header values stored as raw types: integers as `int64`, reset timestamps as `string`
  (RFC3339 exactly as received). **No rounding, no aggregation, no conversion, no derived fields.**

- **FR-003**: If a header is **missing** (empty string from `resp.Header.Get`):
  → Log at **error** level: `log.Errorf("ratelimit: Anthropic response missing header %q", headerName)`
  → Skip the threshold check for that token type.
  This is not a silent skip — the error must appear in logs.

- **FR-004**: If a header is **present but fails to parse** (e.g. non-integer for a token count):
  → Log at **error** level: `log.Errorf("ratelimit: cannot parse header %q value %q: %v", headerName, rawValue, err)`
  → Skip the threshold check for that token type.
  Again, not silent — error must appear in logs.

- **FR-005**: Two independent threshold checks:
  - `float64(InputTokensRemaining) / float64(InputTokensLimit) < threshold` → trigger input alert
  - `float64(OutputTokensRemaining) / float64(OutputTokensLimit) < threshold` → trigger output alert
  Each check only runs if both its limit and remaining headers were successfully parsed.

- **FR-006**: Implement `DiscordRateLimitAlerter` in `internal/callback/discord_ratelimit.go`:
  ```go
  type DiscordRateLimitAlerter struct {
      webhookURL string
      threshold  float64
      cooldown   time.Duration
      mu         sync.Mutex
      alerted    map[string]time.Time
      client     *http.Client
  }
  ```
  Method: `CheckAndAlert(state AnthropicRateLimitState)`

- **FR-007**: Discord alert payload (`{"content": "..."}`) MUST include:
  - Exact `InputTokensLimit`, `InputTokensRemaining`, `InputTokensReset` as parsed from headers
  - Exact `OutputTokensLimit`, `OutputTokensRemaining`, `OutputTokensReset` as parsed from headers
  - Exact `RequestsLimit`, `RequestsRemaining`, `RequestsReset` as parsed from headers
  - Which type triggered the alert (input / output / both)
  Reset timestamps are the raw header string, not reformatted.

- **FR-008**: Cooldown keys: `"ratelimit:anthropic:input"`, `"ratelimit:anthropic:output"`.
  Default cooldown: 1 hour. Per-key, independent.

- **FR-009**: `ProxyConfig` adds:
  - `DiscordWebhookURL string` yaml:`discord_webhook_url`
  - `RatelimitAlertThreshold float64` yaml:`ratelimit_alert_threshold` (default: 0.2)

- **FR-010**: `DiscordRateLimitAlerter` instantiated only if `discord_webhook_url` non-empty.

- **FR-011**: Alert sending is non-blocking (goroutine). Discord non-2xx response:
  → Log at **error** level: `log.Errorf("ratelimit: Discord webhook returned %d: %s", statusCode, body)`

- **FR-012**: `ModifyResponse` MUST NOT return an error due to rate limit logic. Rate limit errors
  are logged but do not affect the proxy response to the client.

### Key Entities

```go
// AnthropicRateLimitState holds raw parsed header values.
// Uses -1 as sentinel for missing or unparseable integer headers (never a valid token count).
// No derived or computed fields. No bool flags.
type AnthropicRateLimitState struct {
    InputTokensLimit      int64  // -1 if header missing or unparseable
    InputTokensRemaining  int64  // -1 if header missing or unparseable
    InputTokensReset      string // "" if header missing
    OutputTokensLimit     int64  // -1 if header missing or unparseable
    OutputTokensRemaining int64  // -1 if header missing or unparseable
    OutputTokensReset     string // "" if header missing
    RequestsLimit         int64  // -1 if header missing or unparseable
    RequestsRemaining     int64  // -1 if header missing or unparseable
    RequestsReset         string // "" if header missing
}
```

---

## Success Criteria *(mandatory)*

- **SC-001**: Input tokens below threshold → Discord POST within 500ms, message contains exact
  `InputTokensLimit`, `InputTokensRemaining`, `InputTokensReset` values.

- **SC-002**: Output tokens below threshold → Discord POST within 500ms, message contains exact
  `OutputTokensLimit`, `OutputTokensRemaining`, `OutputTokensReset` values.

- **SC-003**: 100 triggering responses within 1 hour → exactly 1 alert per type.

- **SC-004**: Proxy P99 latency unaffected (alert in goroutine).

- **SC-005**: `discord_webhook_url` absent → no alerter instantiated.

- **SC-006**: All existing tests pass.

- **SC-007**: Missing header → error log entry with header name. No panic. No client error response.

- **SC-008**: Unparseable header → error log entry with header name and raw value. No panic. No client error response.

- **SC-009**: Discord non-2xx → error log entry with status code and body. Proxy response unaffected.
