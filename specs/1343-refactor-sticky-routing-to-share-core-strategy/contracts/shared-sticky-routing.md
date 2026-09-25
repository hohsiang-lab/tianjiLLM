# Contract: Shared sticky routing strategy

## Core invariants

- Core stores selected stable candidate IDs, not provider secrets.
- Core owns locking and track map updates.
- Core never imports Anthropic, OpenAI, Codex, DB, HTTP, or provider packages.
- Core receives a candidate list that has already been filtered for provider availability.
- Core reuses a selected candidate only when the policy says reuse is still valid.
- Core reselects when selected candidate is absent, provider policy forces re-evaluation, or no prior selection exists.
- Core fallback order is deterministic for equal or unknown scores.

## Claude adapter contract

- Track key remains `anthropic:sonnet` for Sonnet requests and `anthropic:all` otherwise.
- Reuse remains valid only while selected token is available and the selected 5h reset timestamp has not changed.
- Candidate score remains based on earliest known 7d reset.
- Unknown 7d reset loses to known reset; all unknown keeps deterministic fallback.
- Current 5h/7d/7d_sonnet throttle gates remain outside or before core selection and must not change.

## OpenAI subscription/Codex adapter contract

- Track key remains `openai-subscription:<org-scope>:<model-or-route-class>`.
- Candidate ID is OpenAI subscription credential ID.
- Existing disabled, lookup, refresh, malformed, expired, and rate-limited candidate failures remain typed and sanitized.
- Fresh Codex usage snapshots may influence scoring/re-evaluation only when cache entries are fresh at routing time.
- Snapshot statuses other than fresh, cache miss, stale cache, or backoff do not block routing by themselves.
- Missing snapshot data must fall back to current deterministic behavior.
- Request routing must not call the Codex usage fetcher.

## No-fetch assertion surface

The following request paths must be covered by fail-on-fetch tests:

- `POST /v1/chat/completions` with ChatGPT Codex backend transport.
- `POST /v1/responses` with ChatGPT Codex backend transport.
- Responses WebSocket generation with ChatGPT Codex backend transport.

Expected result: `CodexUsageFetcher.Fetch` call count stays zero for all three paths.
