# Research: HO-1340 Codex usage snapshot

## Decision 1 - Treat `wham/usage` as best-effort, not billing truth

Decision: Tianji may fetch and display `GET https://chatgpt.com/backend-api/wham/usage`, but the UI must label it as an operational Codex usage snapshot rather than billing truth.

Rationale:

- The Linear issue records a successful live Tianji-side probe using a refreshed OpenAI subscription credential.
- Official OpenAI docs do not document this endpoint as a stable public API.
- The endpoint shape may change without public API compatibility guarantees.

Alternatives considered:

- Treat as billing source of truth: rejected because endpoint is unofficial.
- Do not implement: rejected because operators need visibility and the endpoint has live value when isolated behind cache/backoff.

## Decision 2 - Reuse existing OpenAI subscription resolution/refresh

Decision: The usage service must call existing credential resolution/refresh helpers and must not decrypt token bundles independently.

Rationale:

- `resolveOpenAISubscriptionCredentialByID` already calls `resolveUsableOpenAISubscriptionBundle`.
- ChatGPT Codex backend transport already receives refreshed bearer token and account id through this path.
- `credentials.go` already centralizes encrypted token-bundle storage and safe metadata handling.

Alternatives considered:

- Add a direct DB decrypt path in the usage service: rejected because it duplicates sensitive credential handling and risks refresh-token semantics drifting.

## Decision 3 - In-memory cache first

Decision: Implement per-process in-memory cache/backoff first, storing only normalized safe snapshot data.

Rationale:

- The feature is UI/status visibility, not billing/audit truth.
- A 60s+ cache satisfies the issue scope and avoids unnecessary calls to an unofficial endpoint.
- Existing Claude Code tab uses in-memory/runtime status patterns for usage-like data, with DB only for metadata/state that must survive restarts.

Alternatives considered:

- Persist snapshots to DB immediately: rejected for Todo scope because it increases migration and retention surface for data from an unofficial endpoint.

## Decision 4 - Preserve last successful snapshot on transient failures

Decision: On 429/5xx, return last successful normalized snapshot with stale/backoff metadata when available.

Rationale:

- Live investigation found no useful `Retry-After` or `X-RateLimit-*` headers, so Tianji must control local backoff.
- Operators benefit more from a stale-but-labeled last success than from an empty card during transient endpoint failure.

Alternatives considered:

- Drop snapshot on failure: rejected because it loses useful operator signal.
- Retry aggressively: rejected because endpoint is unofficial and may rate-limit.

## Decision 5 - UI mirrors Claude Code tab patterns without reusing Anthropic data model

Decision: Add a separate Codex usage tab and view model instead of forcing Codex buckets into Claude Code `ClaudeCodeTokenCard`.

Rationale:

- Existing Claude Code tab is Anthropic OAuth-token-specific and includes Sonnet-only/overage semantics.
- Codex usage buckets are OpenAI subscription credential-specific and include primary/weekly/additional model buckets.
- Separate view models keep semantics explicit and reduce accidental coupling.

Alternatives considered:

- Reuse Claude Code tab/card structs: rejected because labels and fields do not map cleanly.

## Evidence

- Repo evidence:
  - `internal/provider/chatgptcodex/transport.go` owns ChatGPT Codex backend headers and base URL defaults.
  - `internal/proxy/handler/openai_subscription_resolution.go` resolves refreshed subscription bearer tokens and account ids.
  - `internal/ui/handler_credentials.go` and `internal/ui/pages/credentials.templ` own credential detail.
  - `internal/ui/handler_usage.go` and `internal/ui/pages/usage.templ` own usage tabs.
  - `internal/ui/handler_claude_code.go` and `internal/ui/pages/usage_claude_code.templ` are the closest existing usage-tab pattern.
- External evidence:
  - Official OpenAI docs cover Codex and standard bearer-auth API semantics but do not publish `wham/usage` as a stable API.
  - HO-1340 investigation recorded a live HTTP 200 response and absence of useful upstream rate-limit headers.
