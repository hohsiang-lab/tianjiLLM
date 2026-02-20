# Quickstart: Phase 5 Migration Gap Closure

**Branch**: `005-migration-gap-phase5`

## Prerequisites

- Go 1.22+ installed
- `make check` passes on current `main` branch
- PostgreSQL running (for prompt template and MCP server DB storage)
- Redis running (for existing cache/rate-limit features)

## New Dependency

```bash
go get github.com/modelcontextprotocol/go-sdk@v1.3.0
```

This is the only new external dependency in Phase 5. All search providers, guardrails, callbacks, and discovery endpoints use `net/http` + `encoding/json` only.

## Feature Verification

### 1. MCP Server

```yaml
# proxy_config.yaml
mcp_servers:
  test_server:
    transport: "http"
    url: "https://mcp.deepwiki.com/mcp"
```

```bash
# Start proxy
make run

# Test MCP tools listing (REST endpoint)
curl -H "Authorization: Bearer sk-master-key" \
  http://localhost:4000/mcp-rest/tools/list

# Test MCP tools call
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -H "Content-Type: application/json" \
  -d '{"name":"test_server-read_wiki_structure","arguments":{"repoName":"upstream Python TianjiLLM"}}' \
  http://localhost:4000/mcp-rest/tools/call
```

### 2. Search Provider

```yaml
# proxy_config.yaml
search_tools:
  - search_tool_name: "brave-search"
    tianji_params:
      search_provider: "brave"
      api_key: "$BRAVE_API_KEY"
```

```bash
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -H "Content-Type: application/json" \
  -d '{"query":"latest Go 1.23 features","max_results":5}' \
  http://localhost:4000/v1/search/brave-search
```

### 3. Image Variations

```bash
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -F "image=@test.png" \
  -F "model=dall-e-2" \
  -F "n=1" \
  http://localhost:4000/v1/images/variations
```

### 4. Discovery

```bash
# Model group info (with capabilities)
curl -H "Authorization: Bearer sk-master-key" \
  http://localhost:4000/model_group/info

# All supported providers (public, no auth)
curl http://localhost:4000/public/providers

# Full model cost map (public, no auth)
curl http://localhost:4000/public/tianji_model_cost_map
```

### 5. New Provider (e.g., ElevenLabs)

```yaml
# proxy_config.yaml
model_list:
  - model_name: "tts"
    tianji_params:
      model: "elevenlabs/eleven_multilingual_v2"
      api_key: "$ELEVENLABS_API_KEY"
```

```bash
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"elevenlabs/eleven_multilingual_v2","input":"Hello world"}' \
  http://localhost:4000/v1/audio/speech
```

### 6. AutoRouter

```yaml
# proxy_config.yaml
model_list:
  - model_name: "smart-router"
    tianji_params:
      model: "auto_router/smart-router"
      auto_router_config: '{"routes":[{"name":"gpt-4o","utterances":["complex analysis","write code"],"score_threshold":0.5},{"name":"gpt-4o-mini","utterances":["quick question","translate"],"score_threshold":0.5}]}'
      auto_router_default_model: "gpt-4o-mini"
      auto_router_embedding_model: "openai/text-embedding-3-small"
  - model_name: "gpt-4o"
    tianji_params:
      model: "openai/gpt-4o"
      api_key: "$OPENAI_API_KEY"
  - model_name: "gpt-4o-mini"
    tianji_params:
      model: "openai/gpt-4o-mini"
      api_key: "$OPENAI_API_KEY"
```

```bash
# Complex prompt → should route to gpt-4o
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-router","messages":[{"role":"user","content":"Write a production-grade Go HTTP server with graceful shutdown"}]}' \
  http://localhost:4000/v1/chat/completions

# Simple prompt → should route to gpt-4o-mini
curl -X POST -H "Authorization: Bearer sk-master-key" \
  -H "Content-Type: application/json" \
  -d '{"model":"smart-router","messages":[{"role":"user","content":"What is 2+2?"}]}' \
  http://localhost:4000/v1/chat/completions
```

## Running Tests

```bash
# Full suite
make check

# MCP only
go test ./internal/mcp/... -v

# Search providers only
go test ./internal/search/... -v

# Discovery only
go test ./internal/proxy/handler/... -run TestDiscovery -v

# New providers only
go test ./internal/provider/elevenlabs/... -v
go test ./internal/provider/deepgram/... -v

# Guardrails only
go test ./internal/guardrail/... -v

# Callbacks only
go test ./internal/callback/... -v

# AutoRouter only
go test ./internal/router/strategy/auto/... -v
```
