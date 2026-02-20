# TianjiLLM

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.24-00ADD8.svg)](https://go.dev/)

An OpenAI-compatible LLM proxy written in Go. Translates requests to 55+ LLM providers through a unified API — drop-in replacement for OpenAI's API.

## Features

- **55+ Providers** — OpenAI, Anthropic, Azure, Gemini, Bedrock, Groq, DeepSeek, Mistral, Cohere, and [many more](internal/provider/)
- **OpenAI-Compatible API** — `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`, streaming via SSE
- **Wildcard Model Routing** — `claude-*` routes to `anthropic/claude-*` automatically
- **Load Balancing** — multi-deployment routing with retry, fallback chains, health tracking
- **Virtual Keys** — create API keys with per-key budgets, rate limits, and model access controls
- **Spend Tracking** — per-key, per-model, per-user cost tracking with PostgreSQL
- **Web UI** — dashboard for key management, model config, request logs, and usage analytics
- **Config-Driven** — YAML config with env var interpolation and secret resolver support
- **Provider Extensibility** — add a provider by implementing one interface, zero changes to existing code

## Quick Start

### Prerequisites

- Go 1.24+
- PostgreSQL (optional, for key management and spend tracking)
- Redis (optional, for caching)

### Build & Run

```bash
# Install UI tooling
make tools

# Build (generates templ + tailwind + Go binary)
make build

# Run with config
./bin/tianji --config proxy_config.yaml
```

### Minimal Config

```yaml
model_list:
  - model_name: "gpt-4o"
    tianji_params:
      model: "openai/gpt-4o"
      api_key: "$OPENAI_API_KEY"

  - model_name: "claude-*"
    tianji_params:
      model: "anthropic/claude-*"
      api_key: "$ANTHROPIC_API_KEY"

general_settings:
  master_key: "$PROXY_MASTER_KEY"
  port: 4000
```

### Usage

```bash
# Chat completion (OpenAI-compatible)
curl http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $PROXY_MASTER_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# Use any configured model
curl http://localhost:4000/v1/chat/completions \
  -H "Authorization: Bearer $PROXY_MASTER_KEY" \
  -d '{"model": "claude-sonnet-4-5", "messages": [{"role": "user", "content": "Hello"}]}'
```

## Architecture

```
Client → chi router → auth middleware → handler.resolveProvider()
  → provider.TransformRequest() → HTTP to upstream
  → provider.TransformResponse() → OpenAI-format JSON back to client
```

Model names use `"provider/model"` format (e.g. `"anthropic/claude-sonnet-4-5"`). Bare names default to `"openai"`.

### Provider System

Every provider implements the `Provider` interface (7 methods) and self-registers via `init()`. Adding a provider requires zero changes to existing code. OpenAI-compatible providers reuse the `openaicompat` package.

### Router

Multi-deployment load balancing with pluggable strategies (shuffle/latency/cost). Deployment health tracked via failure count + cooldown + EMA latency. Fallback chains handle context window and content policy errors.

## Development

```bash
make dev          # Hot-reload: watches .go/.templ/.css
make test         # go test -race -cover ./...
make lint         # golangci-lint run
make check        # lint + test + build
make e2e          # Playwright E2E tests
make generate     # sqlc codegen after editing .sql files
```

## Tech Stack

Go 1.24, chi/v5, pgx/v5, go-redis/v9, templ + HTMX + Tailwind CSS v4, Playwright, sqlc, OpenTelemetry, Prometheus.

## License

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE).
