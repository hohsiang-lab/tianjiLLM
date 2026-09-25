# Quickstart: HO-1178 Verification

## Preconditions

- Linear HO-1178 is moved to `In Progress` before implementation starts.
- Local tests must use mock OpenAI upstream servers only.
- No real OpenAI credentials are required.

## Targeted Test Commands

```bash
go test ./internal/proxy/handler/... -run 'OpenAI|Subscription|Chat|Completion|Response|Embedding|Image|Audio|Model' -count=1
go test ./internal/provider/openai/... ./internal/proxy/handler/... ./internal/proxy/... -count=1
```

## Manual Mock Verification Shape

1. Configure an official OpenAI model with `openai_subscription_credential_ids: ["cred_a"]`.
2. Seed mock credential `cred_a` with encrypted access token `subscription-access`.
3. Point official OpenAI base URL to an `httptest` upstream through test-only provider construction or existing endpoint override helpers.
4. Call each in-scope endpoint.
5. Assert upstream receives `Authorization: Bearer subscription-access`.
6. Repeat with no subscription IDs and assert `Authorization: Bearer sk-existing`.
7. Repeat with custom `api_base` and assert subscription auth is not used.

## In-Scope Endpoint Checklist

- `/v1/chat/completions`
- `/v1/completions`
- `/v1/responses`
- `/v1/embeddings`
- `/v1/images/generations`
- `/v1/images/edits`
- `/v1/images/variations`
- `/v1/audio/transcriptions`
- `/v1/audio/speech`
- `/v1/models` or repo's concrete model-test upstream path
