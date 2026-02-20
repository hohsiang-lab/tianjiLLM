# Quickstart: TianjiLLM-Go Migration Development

**Date**: 2026-02-16

## Prerequisites

- Go 1.22+ (verify: `go version`)
- PostgreSQL 15+ running locally or via Docker
- Redis 7+ running locally or via Docker
- Python TianjiLLM repo cloned at `../tianji` (for reference)

## Dev Setup

```bash
# Clone and build
git clone https://github.com/praxisllmlab/tianjiLLM.git
cd tianjiLLM
make build

# Run with config
export DATABASE_URL="postgresql://localhost:5432/tianji?sslmode=disable"
export REDIS_HOST="localhost"
export REDIS_PORT="6379"
make run
```

## Adding a New Provider (Phase 1 pattern)

1. Create directory: `internal/provider/{name}/`
2. Create `{name}.go` with struct embedding `openai.BaseProvider`
3. Override methods as needed (typically `GetSupportedParams`, `MapParams`)
4. Add `init()` with `provider.Register("{name}", &Provider{})`
5. Add test with real request/response fixtures from Python test data
6. Reference: `internal/provider/anthropic/` for non-OpenAI provider

## Adding a Callback Integration (Phase 3 pattern)

1. Create directory: `internal/callback/{name}/`
2. Implement `Callback` interface (Name, PreCall, PostCallSuccess, PostCallFailure, StreamEvent)
3. Register in config loader when `callbacks` YAML field matches name
4. Add test verifying callback fires with correct CallbackData

## Adding a Guardrail (Phase 4 pattern)

1. Create directory: `internal/guardrail/{name}/`
2. Implement `Guardrail` interface (embeds Callback + ShouldRun, Apply)
3. Register in config loader when `guardrails` YAML field matches name
4. Add test verifying PII/moderation detection and blocking

## Test Commands

```bash
make test                                    # all tests
go test ./internal/provider/... -v           # all providers
go test ./internal/provider/anthropic/... -v # single provider
go test ./test/contract/... -v               # contract tests
go test ./test/integration/... -v            # integration tests
```

## Migration Verification

For each migrated feature, run the same request against both proxies:

```bash
# Python proxy (port 4000)
curl http://localhost:4000/v1/chat/completions -H "Authorization: Bearer sk-test" \
  -d '{"model": "together_ai/meta-llama/Meta-Llama-3.1-8B-Instruct", "messages": [{"role": "user", "content": "hi"}]}'

# Go proxy (port 8000)
curl http://localhost:8000/v1/chat/completions -H "Authorization: Bearer sk-test" \
  -d '{"model": "together_ai/meta-llama/Meta-Llama-3.1-8B-Instruct", "messages": [{"role": "user", "content": "hi"}]}'

# Compare response format (should be identical structure)
```
