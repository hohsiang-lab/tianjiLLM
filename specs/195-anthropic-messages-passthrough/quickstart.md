# Quickstart: Anthropic /v1/messages Passthrough

## Setup

1. Configure `proxy_config.yaml` with an Anthropic OAuth token:

```yaml
model_list:
  - model_name: "claude-*"
    tianji_params:
      model: "anthropic/claude-*"
      api_key: os.environ/CLAUDE_CODE_OAUTH_TOKEN
```

2. Set the OAuth token environment variable:

```bash
export CLAUDE_CODE_OAUTH_TOKEN="sk-ant-oat01-..."
```

3. Start TianjiLLM:

```bash
make run
```

## Usage with Claude Code CLI

```bash
export ANTHROPIC_BASE_URL=https://your-tianji-instance.com
export ANTHROPIC_AUTH_TOKEN=sk-your-tianji-virtual-key
claude
```

## Direct curl test

```bash
# Non-streaming
curl -X POST "http://localhost:4000/v1/messages?beta=true" \
  -H "Authorization: Bearer sk-your-virtual-key" \
  -H "Content-Type: application/json" \
  -H "anthropic-beta: claude-code-20250219,oauth-2025-04-20" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-6",
    "messages": [{"role": "user", "content": "hi"}],
    "max_tokens": 10
  }'

# Streaming
curl -X POST "http://localhost:4000/v1/messages?beta=true" \
  -H "Authorization: Bearer sk-your-virtual-key" \
  -H "Content-Type: application/json" \
  -H "anthropic-beta: claude-code-20250219,oauth-2025-04-20" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-sonnet-4-6",
    "messages": [{"role": "user", "content": "hi"}],
    "max_tokens": 10,
    "stream": true
  }'
```

## Verification

```bash
# Run tests
go test ./internal/proxy/handler/... -run "TestAnthropicMessages" -v
go test ./internal/provider/anthropic/... -run "TestMergeBetaHeaders" -v
```
