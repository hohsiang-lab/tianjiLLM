# Quickstart: HO-2370 validation

## Local test validation

```bash
go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Catalog|ListModels|RuntimeModelSource' -count=1
go test ./internal/proxy/handler -run 'CodexUsage|CodexCatalog' -count=1
git diff --check
```

## Fixture validation

1. Add a test fixture representing upstream Sol/Terra/Luna catalog metadata with:
   - `context_window: 372000`
   - `max_context_window: 372000`
   - `effective_context_window_percent: 95`
   - `auto_compact_token_limit: null`
   - `truncation_policy: {"mode":"tokens","limit":10000}`
   - text/image modalities
   - model-specific default/supported reasoning levels including `max` or `ultra` where present
   - speed/service tiers where present
2. Configure matching Tianji runtime aliases.
3. Assert `/models` exposes upstream-derived metadata only for routable aliases.

## Live validation after implementation/deploy

1. Configure Codex/OpenClaw to use Tianji as the model provider.
2. Trigger a model refresh against Tianji.
3. Confirm Sol/Terra/Luna no longer report fabricated `128000` context or synthesized 90% auto-compaction values.
4. Confirm refresh logs and Tianji logs contain no access tokens, refresh tokens, bearer headers, or raw credential bundles.

## Expected fallback checks

- With upstream failure and last-known-good cache: response uses cached upstream metadata and emits degraded refresh evidence.
- With no upstream and no cache: response uses conservative static fallback metadata.
- With hidden aliases: neither `data[]` nor `models[]` contains the hidden alias.
