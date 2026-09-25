# Requirements Checklist: Subscription wildcard routing regression verification

## Scope

- [x] Subscription-backed `openai/*` wildcard route is explicitly covered.
- [x] ChatGPT Codex backend route is explicitly covered.
- [x] Wrong Platform `/v1/chat/completions` route is explicitly prohibited for subscription wildcard traffic.
- [x] API-key-backed `openai/*` wildcard route is explicitly preserved.
- [x] 401/403/429 diagnostics are covered through full wildcard routing.
- [x] Deployment verification notes are included and secret-safe.

## Governance

- [x] Todo phase contains docs only.
- [x] Tasks require Linear `In Progress` before test/code edits.
- [x] RED tests are first implementation tasks.
- [x] No real OpenAI, ChatGPT, or production OpenClaw calls are required.

## Secret Safety

- [x] Access tokens must not be printed.
- [x] Refresh tokens must not be printed.
- [x] API keys must not be printed.
- [x] Authorization header values must not be printed.
- [x] Encrypted credential values and raw credential JSON must not be printed.
