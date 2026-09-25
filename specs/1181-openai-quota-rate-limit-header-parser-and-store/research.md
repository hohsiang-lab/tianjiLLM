# Research: OpenAI Quota/Rate-limit Header Parser and Store Integration

## Repo Reality

### Existing OpenAI parser and routing

- `internal/proxy/handler/openai_subscription_ratelimit.go` parses official OpenAI `x-ratelimit-*` headers into handler-local `openAIRateLimitState`.
- `parseOpenAIRateLimitReset` already accepts Go duration strings such as `60s` and `6m0s`, plus RFC3339 timestamps.
- `recordOpenAISubscriptionRateLimit` stores parsed state under `Handlers.openAISubscriptionRateLimit`.
- `openAISubscriptionRateLimitState` is read by candidate gating and lowest-utilization selection.

Decision: HO-1181 should not re-invent the parser from scratch; it should formalize parser output and store consumption behind a normalized state/store contract.

### Existing Anthropic store pattern

- `internal/callback/ratelimit_store.go` has explicit unknown sentinels, reset normalization, in-memory store, dirty tracking, and DB preload/flush support.
- Anthropic `NormalizeExpiredWindows` clears expired utilization/status after reset.
- Anthropic state is keyed by hashed token, but OpenAI subscription routing has stable credential IDs and account IDs available.

Decision: Reuse the store principles, not the Anthropic field shape. OpenAI needs request/token/reset state and optional mock status/utilization, not Anthropic 5h/7d/sonnet windows.

### Existing OpenAI subscription routing

- `internal/proxy/handler/openai_subscription_routing.go` filters gated candidates before sticky selection.
- Sticky state is org/model scoped and points to credential ID.
- Lowest-utilization currently scores `RequestRemaining + TokenRemaining` from handler-local state.

Decision: Gate, sticky re-evaluation, and lowest-utilization should read one normalized OpenAI quota store state.

### Mock harness

- `internal/testutil/openaitest/upstream_server.go` provides mock official endpoints and bearer recording.
- Current grep did not find rich OpenAI quota fixture helpers in `openaitest`.

Decision: If implementation needs allowed/rejected/reset/utilization cases from mocks, add explicit test fixture support under `internal/testutil/openaitest` and keep it no-real-OpenAI guarded.

## Official OpenAI Docs

Source: `https://developers.openai.com/api/docs/guides/rate-limits`, fetched 2026-05-07.

Findings:

- OpenAI rate limits are measured across RPM, RPD, TPM, TPD, and IPM.
- Rate limits are organization/project/model scoped.
- Usage limits/spend caps are separate from request/token rate limits.
- Official response headers include:
  - `x-ratelimit-limit-requests`
  - `x-ratelimit-limit-tokens`
  - `x-ratelimit-remaining-requests`
  - `x-ratelimit-remaining-tokens`
  - `x-ratelimit-reset-requests`
  - `x-ratelimit-reset-tokens`
- Reset samples are duration strings: `1s` and `6m0s`.
- Unsuccessful requests still contribute to per-minute limits.

Decision: Official parser scope is limited to those headers. Do not infer billing quota or monthly subscription allowance.

## Context7

Command attempts:

```bash
ctx7 library openai-go 'rate limit headers response headers request id retries'
ctx7 library net/http 'http header get case insensitive response headers Go'
```

Result: both returned `Monthly quota exceeded`.

Decision: Record the outage and rely on official OpenAI docs plus repo reality. Do not claim Context7 validated this plan.

## GitHub / grep-app

Command:

```bash
/Users/n0rmanc/.cargo/bin/grep-app-cli --json 'x-ratelimit-reset-requests' --language Go
```

Relevant examples:

- `sashabaranov/go-openai/ratelimit.go` defines `RateLimitHeaders` with the same six OpenAI header names and stores reset values as a `ResetTime` string.
- `weaviate/weaviate/usecases/modulecomponents/clients/openai/openai.go` parses `x-ratelimit-reset-requests` and `x-ratelimit-reset-tokens` with `time.ParseDuration`.
- `PullRequestInc/go-gpt3/models.go` parses reset headers into `time.Duration`.
- `newrelic/go-agent/v3/integrations/nropenai/nropenai.go` records the same six rate-limit response headers for observability.

Decision: Go ecosystem examples support treating reset headers as duration-like strings and preserving the six official header names.

## Decisions

1. Add or adapt one OpenAI quota/rate-limit state model with explicit known/unknown flags.
2. Keep official OpenAI header parsing separate from mock-only quota status parsing.
3. Use credential ID/account ID keys, not raw token hashes, for OpenAI subscription quota state.
4. Gate only when an exhausted/rejected state has a future reset deadline.
5. Derive utilization only from positive limit plus known remaining, or from mock utilization when explicitly present.
6. Store updates must happen before response discard/failover for 401/429/5xx attempts.
