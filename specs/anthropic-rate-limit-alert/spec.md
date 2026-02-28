# Feature Specification: Anthropic Rate Limit Monitoring + Discord Alert

**Feature Branch**: `feat/anthropic-rate-limit-alert`  
**Created**: 2026-03-01  
**Status**: Draft  
**Linear Issue**: HO-69

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Rate Limit Alert When Tokens Are Low (Priority: P1)

As an operator running TianjiLLM with Anthropic as a provider, I want to receive a Discord alert
when the Anthropic API's remaining token quota drops below a configured threshold, so I can take
action before hitting rate limits that would degrade service.

**Why this priority**: Core alert function — without this, the feature has no value.

**Independent Test**: Configure `discord_webhook_url` + `ratelimit_alert_threshold: 0.2` in
`proxy_config.yaml`. Send an Anthropic request that returns `anthropic-ratelimit-tokens-remaining`
header with value below 20% of limit. Verify Discord webhook receives a POST with a meaningful message.

**Acceptance Scenarios**:

1. **Given** `discord_webhook_url` and `ratelimit_alert_threshold: 0.2` are set in config,
   **When** an Anthropic response includes `anthropic-ratelimit-tokens-remaining` = 1000 and
   `anthropic-ratelimit-tokens-limit` = 10000 (10% remaining),
   **Then** TianjiLLM POSTs an alert message to the Discord webhook URL within the same request cycle.

2. **Given** same config,
   **When** an Anthropic response includes `anthropic-ratelimit-tokens-remaining` = 4000 and
   `anthropic-ratelimit-tokens-limit` = 10000 (40% remaining — above threshold),
   **Then** no Discord alert is sent.

3. **Given** `discord_webhook_url` is not set in config,
   **When** any Anthropic response is received,
   **Then** no Discord alert is attempted and no error is logged.

---

### User Story 2 - Cooldown Prevents Alert Spam (Priority: P2)

As an operator, I want the alert system to avoid flooding Discord with repeated messages
every request, so I can focus on actionable signals rather than noise.

**Why this priority**: Without cooldown, a busy API period will spam Discord and operators
will start ignoring the channel.

**Independent Test**: Send 20 Anthropic requests in rapid succession, all returning tokens below
threshold. Verify Discord receives only 1 alert within the cooldown window (default 1 hour).

**Acceptance Scenarios**:

1. **Given** cooldown is 1 hour (default),
   **When** a rate limit alert fires and another triggering response arrives within 1 hour,
   **Then** the second alert is suppressed.

2. **Given** cooldown is 1 hour,
   **When** a rate limit alert fires and 61 minutes pass, and another triggering response arrives,
   **Then** the new alert is sent.

---

### User Story 3 - Configurable Alert Threshold (Priority: P3)

As an operator, I want to tune the alerting threshold per deployment (e.g., alert at 30% for
critical production, 10% for dev environments), so the system fits different risk tolerances.

**Why this priority**: Flexibility is needed but the feature works at the default 20%.

**Independent Test**: Set `ratelimit_alert_threshold: 0.3`. Send a response with 25% tokens
remaining. Verify alert fires. Set threshold to `0.1`. Send same response. Verify no alert fires.

**Acceptance Scenarios**:

1. **Given** `ratelimit_alert_threshold: 0.3`,
   **When** remaining tokens = 25% of limit,
   **Then** alert fires.

2. **Given** `ratelimit_alert_threshold: 0.1`,
   **When** remaining tokens = 25% of limit,
   **Then** no alert fires.

---

### Edge Cases

- What if `anthropic-ratelimit-tokens-remaining` header is missing from response?
  → Skip rate limit check silently; do not error.
- What if `anthropic-ratelimit-tokens-limit` is 0 or missing?
  → Skip rate limit check to avoid division-by-zero; log a warning.
- What if the Discord webhook URL returns non-2xx?
  → Log a warning, continue proxy response unaffected.
- What happens with streaming responses?
  → Headers are available on the initial HTTP response; check them there regardless of body streaming.
- What if `ratelimit_alert_threshold` is not set?
  → Default to 0.2 (20%).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST read `anthropic-ratelimit-tokens-remaining` and
  `anthropic-ratelimit-tokens-limit` response headers in `native_format.go`'s `ModifyResponse`
  handler for Anthropic provider responses.

- **FR-002**: System MUST calculate remaining token ratio:
  `ratio = remaining / limit`. If `ratio < ratelimit_alert_threshold`, trigger Discord alert.

- **FR-003**: System MUST implement `DiscordRateLimitAlerter` in
  `internal/callback/discord_ratelimit.go`, following the `SlackCallback` struct pattern
  (webhook URL, cooldown, `alerted` map, mutex, `sendThrottledToWebhook`).

- **FR-004**: `DiscordRateLimitAlerter` MUST send HTTP POST to Discord Incoming Webhook URL
  with JSON body `{"content": "<alert message>"}` (Discord Incoming Webhook format).

- **FR-005**: Alert message MUST include: remaining tokens, limit, percentage remaining,
  and current timestamp.

- **FR-006**: System MUST support cooldown per alert key to suppress duplicate alerts.
  Default cooldown: 1 hour. Alert key: `"ratelimit:anthropic"`.

- **FR-007**: Config struct MUST add two new optional fields:
  - `discord_webhook_url` (string, YAML tag `discord_webhook_url`)
  - `ratelimit_alert_threshold` (float64, YAML tag `ratelimit_alert_threshold`, default 0.2)

- **FR-008**: `DiscordRateLimitAlerter` MUST be initialized only when `discord_webhook_url`
  is non-empty. It MUST be wired into the handler/proxy flow that processes Anthropic responses.

- **FR-009**: Alert sending MUST be non-blocking (use `go` goroutine) to not affect proxy latency.

- **FR-010**: The `ModifyResponse` hook MUST NOT return an error due to rate limit header
  parsing failures; all errors are logged and swallowed.

### Key Entities

- **DiscordRateLimitAlerter**: Struct in `internal/callback/discord_ratelimit.go`. Fields:
  `webhookURL string`, `threshold float64`, `cooldown time.Duration`, `mu sync.Mutex`,
  `alerted map[string]time.Time`, `client *http.Client`.

- **Config extension**: Two new fields in `internal/config/config.go` `ProxyConfig` struct:
  `DiscordWebhookURL string` and `RatelimitAlertThreshold float64`.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: When Anthropic response contains tokens-remaining < threshold x tokens-limit,
  Discord webhook receives a POST within 500ms of the response being processed (excluding network).

- **SC-002**: In a sequence of 100 triggering responses within 1 hour, Discord receives exactly 1
  alert (cooldown works).

- **SC-003**: Proxy response latency is unaffected by alert sending (alert is async goroutine).

- **SC-004**: All existing tests in `internal/callback/` and `internal/proxy/handler/` continue
  to pass with no regression.

- **SC-005**: When `discord_webhook_url` is absent from config, no `DiscordRateLimitAlerter` is
  instantiated and no alert code path is exercised.
