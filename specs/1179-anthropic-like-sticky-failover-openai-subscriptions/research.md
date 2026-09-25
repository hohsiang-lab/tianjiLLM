# Research: Anthropic-like Sticky/Failover for OpenAI Subscriptions

## Repo Reality

### Current OpenAI subscription behavior

- `internal/proxy/handler/openai_subscription_resolution.go` currently resolves subscription credentials by reading `params.OpenAISubscriptionCredentialIDs` and choosing `ids[0]`.
- `resolveOpenAIAPIKeyForParams` already enforces the no-fallback shape: when subscription IDs are present, it resolves subscription credentials instead of returning configured `api_key`.
- `resolveUsableOpenAISubscriptionBundle` and `loadOpenAISubscriptionCredential` already classify credential failures into typed codes: missing, wrong type, disabled, malformed, expired, lookup failed, and refresh failed.
- `internal/config/validate.go` already rejects empty/duplicate subscription credential IDs, rejects custom `api_base`, and limits subscription IDs to official OpenAI model configs.

**Decision**: Add multi-candidate routing on top of existing OpenAI subscription resolution/refresh code. Do not bypass typed credential errors and do not re-open `api_key` fallback.

### Current Anthropic native upstream strategy

- `internal/proxy/handler/native_upstream.go` has reusable strategy concepts: per-provider round-robin, disabled-token exclusion, sticky selection, utilization-aware selection, and throttle gating.
- Anthropic gating uses `callback.RateLimitStore`, disabled token hashes, and Anthropic-specific 5h/7d/7d_sonnet utilization fields.
- Current `stickyTrackKey(providerName, modelName)` is provider/model-class scoped, not org scoped.

**Decision**: Reuse the strategy concepts, not the Anthropic data model. OpenAI subscription routing needs OpenAI-specific rate-limit state and an org-scoped sticky key.

### Org context

- `internal/proxy/middleware/auth.go` defines `ContextKeyOrgID`.
- Handler/router code already reads org ID from request context.

**Decision**: Use `ContextKeyOrgID` as the sticky scope source. Missing org ID must map to a deterministic master/no-org track.

## Official OpenAI Docs

### Rate-limit headers

Context7 against `/websites/developers_openai_api` and official OpenAI docs search both confirm the HTTP response rate-limit headers:

- `x-ratelimit-limit-requests`
- `x-ratelimit-limit-tokens`
- `x-ratelimit-remaining-requests`
- `x-ratelimit-remaining-tokens`
- `x-ratelimit-reset-requests`
- `x-ratelimit-reset-tokens`

Sources checked:

- https://developers.openai.com/api/docs/guides/rate-limits/usage-tiers
- https://platform.openai.com/docs/guides/rate-limits/retrying-with-exponential-backoff%20.eot
- https://platform.openai.com/docs/api-reference/debugging-requests

**Decision**: Parse request/token remaining and reset values. Gate only when a parsed remaining value is exhausted and a reset time is known or inferable. Do not treat absent headers as exhausted.

## GitHub / grep-app Evidence

`grep-app-cli` searches for Go usage of OpenAI-style headers found public examples documenting or forwarding these headers:

- `labring/aiproxy` documents `X-RateLimit-Limit-Requests`, `X-RateLimit-Limit-Tokens`, `X-RateLimit-Remaining-Requests`, `X-RateLimit-Remaining-Tokens`, `X-RateLimit-Reset-Requests`, and `X-RateLimit-Reset-Tokens` on OpenAI-compatible endpoints.
- Previous searches also found Go projects parsing or recording reset/remaining rate-limit headers rather than applying provider-specific quota windows.

**Decision**: Keep OpenAI header parsing straightforward and provider-specific. Do not introduce Anthropic quota scoring into OpenAI.

## Reference Repo Search

Reference repo grep across available local sibling projects did not find an existing OpenAI subscription credential routing implementation. The relevant implementation reality is inside TianjiLLM.

**Decision**: Follow TianjiLLM handler patterns instead of importing a reference-repo abstraction.

## Alternatives Considered

### Alternative 1: Reuse Anthropic `RateLimitStore` directly

Rejected. Anthropic state tracks `Unified5h`, `Unified7d`, and Sonnet-specific utilization. OpenAI official headers expose request/token remaining and reset timing. Reusing the Anthropic store would either lose OpenAI semantics or encourage fake 5h/7d data.

### Alternative 2: Add DB-backed sticky/rate-limit persistence now

Rejected for this slice. Current native sticky state is in-memory. HO-1179 asks for sticky/failover behavior, not cross-process durability or a schema migration. In-memory state plus tests is the smaller compatible change.

### Alternative 3: Fail over every upstream error including mid-stream failures

Rejected. Once a streaming response has emitted bytes, retrying with another credential would produce mixed/duplicated output for the client. Failover must stop at the response-commit boundary.

### Alternative 4: Treat missing OpenAI rate-limit headers as unavailable

Rejected. The issue says quota/rate-limit gate where available. Missing headers are not proof of quota exhaustion, and treating them as exhausted would break endpoints or transports that do not expose headers.

## Final Decisions

- Route all configured OpenAI subscription credential IDs.
- Key routing and sticky state by credential ID/account metadata, never raw tokens.
- Use org-scoped sticky state from request context.
- Parse OpenAI `x-ratelimit-*` headers into an OpenAI-specific in-memory state.
- Gate only from explicit credential failures or parsed exhausted rate-limit dimensions.
- Fail over before response commitment; do not retry after streaming bytes are relayed.
- Preserve no-silent-`api_key`-fallback when subscription IDs are configured.
