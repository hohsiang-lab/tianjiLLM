# Quickstart: HO-1340 Codex usage snapshot

## Local Regression

Provider helper:

```bash
go test ./internal/provider/chatgptcodex -run 'Usage|Wham|CodexUsage' -count=1
```

Subscription service and credential detail:

```bash
go test ./internal/proxy/handler -run 'OpenAISubscription.*Usage|CodexUsage|CredentialDetail' -count=1
```

UI tab:

```bash
go test ./internal/ui -run 'CodexUsage|CredentialDetail|UsageTab' -count=1
go test ./internal/ui/pages -run 'CodexUsage|Usage' -count=1
```

Diff hygiene:

```bash
git diff --check
```

Templ regeneration when UI templates change:

```bash
make ui
```

## Expected Local Results After Implementation

- Provider helper sends bearer auth and account id when present.
- Parser normalizes email, plan type, primary 5h, weekly, additional buckets, credits/spend-control status, and reset/status fields.
- 401 path refreshes once and retries once.
- 429/5xx path preserves last successful snapshot and activates backoff.
- Cache prevents repeated upstream calls inside TTL.
- Credential detail renders Codex usage without token/raw body leakage.
- `/ui/usage?tab=codex` renders all OpenAI subscription credentials.
- Proxy request tests prove no `wham/usage` call happens during `/v1/chat/completions`, `/v1/responses`, or WebSocket requests.

## Live Verification

Use a live Tianji deployment with an OpenAI subscription credential known to refresh successfully.

1. Open the credential detail page:

```text
https://tianji.hohsiang.com.tw/ui/credentials/<credential_id>
```

2. Trigger manual Codex usage refresh.

3. Verify the UI shows only normalized safe fields:

- email
- plan type
- primary 5h usage/reset/status
- weekly usage/reset/status
- additional buckets such as `GPT-5.3-Codex-Spark`
- last updated / stale / backoff state

4. Open:

```text
https://tianji.hohsiang.com.tw/ui/usage?tab=codex
```

5. Verify the same credential appears in the Codex tab.

6. Trigger repeated refresh inside 60 seconds and verify cached state is reused.

## Secret Safety Check

Search logs, rendered HTML, and any DB snapshot table/cache debug output for token-shaped strings only in a controlled non-secret test fixture. Acceptance requires no real or fixture access token, refresh token, cookie, authorization header, encrypted credential value, or raw upstream body to appear.
