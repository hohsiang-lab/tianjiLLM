# Quickstart: HO-1335 `/v1/responses` request-log pipeline

## Local Regression

1. Run the issue-owned handler tests:

```bash
go test ./internal/proxy/handler -run 'Responses|Codex|RequestLog|Log' -count=1
```

2. Run spend pipeline tests:

```bash
go test ./internal/spend -count=1
```

3. Run diff hygiene:

```bash
git diff --check
```

## Expected Local Results After Implementation

- HTTP `POST /v1/responses` success test records exactly one success log.
- HTTP `POST /v1/responses` failure test records exactly one failure/error log and zero success logs.
- WebSocket generation success test records exactly one success log after `response.completed`.
- WebSocket `generate:false` prewarm test records zero logs and makes zero upstream backend calls.
- Existing `/v1/chat/completions` Codex logging tests remain green.

## Live Verification

1. Send a true Codex request through Tianji:

```bash
codex exec --model openai/gpt-5.5 "say OK"
```

Use the operator's Tianji custom-provider config for `https://tianji.hohsiang.com.tw/v1`.

2. Open:

```text
https://tianji.hohsiang.com.tw/ui/logs?page=1&time_range=24h&live_tail=true
```

3. Verify the newest request appears.

Expected visible fields when available:

- model `openai/gpt-5.5`
- success or failed status
- request id
- duration
- upstream token key/subscription attribution
- token usage and cost when derivable

## Prewarm Guard

Synthetic WebSocket `response.create` with `generate:false` is not a real model call. A prewarm-only probe must not create a UI request-log row.
