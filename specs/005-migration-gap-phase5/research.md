# Research: Phase 5 Migration Gap Closure

**Date**: 2026-02-18
**Branch**: `005-migration-gap-phase5`

---

## R1: MCP Server — Go SDK Selection

### Decision
Use `github.com/modelcontextprotocol/go-sdk/mcp` v1.3.0 (official MCP Go SDK).

### Rationale
- **Official**: Maintained by MCP organization + Google. Spec updates land with zero delay.
- **Idiomatic Go**: Generic `mcp.AddTool[Input, Output]()` auto-derives JSON Schema from struct tags. No manual schema construction.
- **Standard `http.Handler`**: `mcp.NewSSEHandler()` and `mcp.NewStreamableHTTPHandler()` both implement `http.Handler`, integrating directly with chi router.
- **Battle-tested**: GitHub MCP Server, Google ADK, Jaeger, Envoy AI Gateway, DataDog Agent, Docker MCP Gateway, Bytebase all use it in production.
- **Three transports**: Stdio, SSE (MCP 2024-11-05), Streamable HTTP (MCP 2025-06-18).

### Alternatives Considered
| Library | Rejected Because |
|---------|------------------|
| `mark3labs/mcp-go` (community) | Requires manual builder-pattern schema (`mcp.WithString()`, `mcp.WithNumber()`). Less maintainable. Official SDK supersedes it. |
| `golang.org/x/tools/internal/mcp` | Internal package, not importable. gopls itself is migrating to the official SDK. |
| `trpc-group/trpc-mcp-go` | Narrow ecosystem (tRPC-only). Smaller community. |

### Key API Surface
```go
// Server creation
server := mcp.NewServer(&mcp.Implementation{Name: "tianjiLLM", Version: "v1.0.0"}, &mcp.ServerOptions{HasTools: true})

// Tool registration (generic — auto schema from struct tags)
mcp.AddTool(server, &mcp.Tool{Name: "search", Description: "Web search"}, handler)

// SSE transport (mounts as http.Handler on chi)
sseHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server { return server }, nil)

// Streamable HTTP transport
httpHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server { return server }, nil)
```

### Sources
- https://github.com/modelcontextprotocol/go-sdk (v1.3.0)
- https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
- https://modelcontextprotocol.io/specification/2025-11-25

---

## R2: MCP Server — Python Architecture Analysis

### Decision
Replicate Python's two-tier architecture: MCP Server Manager (upstream aggregation) + local tool registry, with SSE + Streamable HTTP transports.

### Rationale
Python TianjiLLM's MCP server is a **proxy-of-proxies**: it aggregates tools from multiple upstream MCP servers and exposes them through a unified namespace to MCP clients. Key components:

| Component | Responsibility |
|-----------|---------------|
| `MCPServerManager` | Registry of upstream MCP servers. Creates MCP clients per server. Handles tool name prefixing (`{server}-{tool}`), tool-to-server mapping, `call_tool()` dispatch. |
| `MCPToolRegistry` | Local tool registry for config-defined tools. Each tool has name, description, input_schema, and a callable handler. |
| `server.py` | MCP protocol handler. Registers `tools/list`, `tools/call` via the MCP SDK. Mounts SSE + Streamable HTTP transports. |
| `rest_endpoints.py` | REST wrappers: `GET /mcp-rest/tools/list`, `POST /mcp-rest/tools/call` for non-MCP clients. |
| `mcp_management_endpoints.py` | CRUD for MCP servers in DB: `GET/POST/PUT/DELETE /v1/mcp/server`. |
| `semantic_tool_filter.py` | Embedding-based tool filtering using `semantic-router`. |

### Config Format (proxy_config.yaml)
```yaml
mcp_servers:
  wikipedia:
    transport: "stdio"     # stdio | sse | http
    command: "uvx"
    args: ["mcp-server-fetch"]
  zapier:
    transport: "sse"
    url: "https://actions.zapier.com/mcp/<key>/sse"
    auth_type: "api_key"
    authentication_token: "<token>"
```

### Tool Name Prefixing
Tools are prefixed with server name using separator (default `-`, configurable via `MCP_TOOL_PREFIX_SEPARATOR`): e.g., `zapier-gmail_send_email`. When only one server is allowed for a user, prefixing is skipped.

### Error Model
All errors wrapped in `CallToolResult` with `isError=True` and `TextContent` message. HTTP errors, guardrail violations, and generic exceptions all follow this pattern.

### Go Implementation Notes
- Use official Go SDK's `mcp.NewServer()` + `mcp.AddTool()` for the MCP protocol layer.
- `MCPServerManager` equivalent: Go struct with `map[string]*MCPServer` for upstream server registry.
- Upstream connections: Go SDK's `mcp.NewClient()` for stdio/SSE/HTTP transports to upstream servers.
- Tool prefixing: simple `strings.SplitN(name, separator, 2)`.
- Mount as `http.Handler` on chi: `/mcp/sse` for SSE, `/mcp` for Streamable HTTP.
- REST endpoints: standard chi handlers wrapping the MCP server's list/call logic.

---

## R3: Search Providers — Architecture

### Decision
Implement 6 search providers from scratch using `net/http` + `encoding/json`. No Go client libraries exist for any of these APIs. Define a `SearchProvider` interface mirroring Python's `BaseSearchConfig`.

### Rationale
All 6 search APIs are simple REST endpoints. Every real-world Go implementation found (30+ repos) uses direct HTTP clients. The APIs are too simple to justify third-party dependencies.

### Interface Design (from Python `BaseSearchConfig`)
```
SearchProvider interface:
  Name() string
  HTTPMethod() string                    // "GET" or "POST"
  ValidateEnvironment(apiKey, apiBase) headers
  GetCompleteURL(apiBase, params) string
  TransformRequest(query, params) body
  TransformResponse(raw) SearchResponse
```

### Per-Provider Details

| Provider | Endpoint | Method | Auth | Query Field | Max Results | Domain Filter |
|----------|----------|--------|------|-------------|-------------|---------------|
| Brave | `api.search.brave.com/res/v1/web/search` | GET | Header: `X-Subscription-Token` | `q` | 20 | `site:` appended to query |
| Tavily | `api.tavily.com/search` | POST | Body: `api_key` field | `query` | 20 | `include_domains` array |
| SearXNG | `{user_base}/search` | GET | Optional Bearer | `q` | ~20 (uncontrolled) | Not supported |
| Exa AI | `api.exa.ai/search` | POST | Header: `x-api-key` | `query` | 100 | `includeDomains` array |
| Google PSE | `googleapis.com/customsearch/v1` | GET | Query param: `key=` + `cx=` | `q` | 10 | `siteSearch` (single) |
| DataForSEO | `api.dataforseo.com/v3/serp/google/organic/live/advanced` | POST | HTTP Basic Auth | `keyword` | 700 | `domain` |

### Unified Response Model
```
SearchResult { Title, URL, Snippet, Date?, LastUpdated? }
SearchResponse { Results []SearchResult, Object: "search" }
```

### Integration with Tool-Calling
Python exposes search via `POST /v1/search/{tool_name}` endpoint. Search tools are configured in `search_tools:` YAML section and dispatched via `tianji.asearch()`. Go should follow the same pattern — a chi handler at `/v1/search/{tool_name}` that looks up the provider and dispatches.

### Sources
- Brave: `SamSaffron/term-llm`, `tingly-dev/tingly-box` (MIT)
- Tavily: `vxcontrol/pentagi`, `adrianliechti/wingman` (MIT)
- SearXNG: `cloudwego/eino-ext` (Apache-2.0)
- Exa: `adrianliechti/wingman`, `SamSaffron/term-llm` (MIT)

---

## R4: Discovery Endpoints — Python Architecture Analysis

### Decision
Implement discovery as a focused set of endpoints: `/discovery/models`, `/discovery/providers`, and `/public/model_hub`. Skip OAuth well-known endpoints (handled by MCP auth) and UI-specific endpoints (out of scope per "UI Dashboard" exclusion).

### Rationale
Python TianjiLLM's "discovery" is spread across 15+ endpoints in three modules. For Go Phase 5, we focus on the high-value endpoints that serve API consumers and admin tooling:

**Priority endpoints for Phase 5:**

| Endpoint | Source | Returns |
|----------|--------|---------|
| `GET /discovery/models` | Config + model_cost JSON | All available models with capabilities |
| `GET /discovery/providers` | Provider registry | Configured provider status |
| `GET /model_group/info` | Router + model_cost JSON | Aggregated model group capabilities |
| `GET /public/providers` | Static enum | All supported provider names |

**Deferred (UI Dashboard scope):**
- `/.well-known/tianji-ui-config` — UI-only
- `/public/model_hub` — requires health check DB integration
- `/public/providers/fields` — requires static JSON (provider_create_fields.json)
- `/v2/model/info` — paginated, complex sorting/filtering

### Capability Determination Logic
Model capabilities are merged from multiple sources (highest priority first):
1. Config/DB `model_info` overrides
2. `model_prices_and_context_window.json` lookup by model name
3. Provider metadata via `GetSupportedParams()`

For model group aggregation: OR logic for boolean capabilities, max for numeric values (tokens, costs), sum for TPM/RPM.

### Key Types
```
DiscoveryModel {
  ModelGroup, Providers[], MaxInputTokens, MaxOutputTokens,
  InputCostPerToken, OutputCostPerToken, Mode,
  SupportsVision, SupportsFunctionCalling, SupportsStreaming,
  SupportedParams[]
}
```

---

## R5: AutoRouter — Semantic Routing Analysis

### Decision
Implement embedding-based semantic routing using TianjiLLM's own embedding endpoint. Use a simple in-process cosine similarity matcher instead of the `semantic-router` Python library.

### Rationale
Python's AutoRouter is embedding-based (not LLM-based or rule-based):
1. Each route defines utterances (example phrases) and a score threshold
2. On request, the last user message is embedded via the configured embedding model
3. Cosine similarity is computed against all route utterance vectors
4. The highest-scoring route above threshold becomes the target model group
5. If no match, falls back to `default_model`

### Go Implementation Approach
The `semantic-router` Python library is thin — it just does embedding + cosine similarity. In Go:
- **Embedding**: Call tianjiLLM's own embedding endpoint (internal function call, not HTTP)
- **Cosine similarity**: 10 lines of Go code with `math` package — no ML library needed
- **Route vectors**: Pre-computed at startup, cached in memory
- **Per-request overhead**: 1 embedding API call (~50-200ms) + negligible vector math

### Config Format (matches Python)
```yaml
model_list:
  - model_name: "my-auto-router"
    tianji_params:
      model: "auto_router/my-auto-router"
      auto_router_config: '{"routes": [...]}'        # or auto_router_config_path
      auto_router_default_model: "gpt-4o-mini"
      auto_router_embedding_model: "openai/text-embedding-3-small"
```

### Fallback Behavior
- No matching route → use `default_model`
- Messages is nil (embedding request) → skip routing, use original model
- Embedding API fails → **Python has no graceful degradation here** (bug). Go should fall back to `default_model` with a logged warning.

### Dependencies
None beyond tianjiLLM's own embedding capability. No external ML libraries needed.

---

## R6: Provider Patterns — Go Codebase Analysis

### Decision
All ~20 new providers follow the established self-register pattern. OpenAI-compatible providers embed `*openai.Provider`; custom-format providers implement all 7 interface methods.

### Rationale
The Go codebase has a clean, proven pattern:

**OpenAI-compatible (minimal — 1 file, ~15 lines):**
```go
package myprovider

import (
    "github.com/praxisllmlab/tianjiLLM/internal/provider"
    "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

type Provider struct{ *openai.Provider }

func init() {
    provider.Register("myprovider", &Provider{openai.NewWithBaseURL("https://api.example.com/v1")})
}

func (p *Provider) GetSupportedParams() []string { return []string{...} }
```

**Custom format (2 files — provider.go + stream.go):**
- `provider.go`: Full `Provider` interface (7 methods) + `init()` registration
- `stream.go`: `ParseStreamEvent(data) (*model.StreamChunk, bool, error)`

### Provider Classification for Phase 5

**OpenAI-compatible (embed `*openai.Provider`, ~15 lines each):**
baseten, hosted_vllm, codestral, friendliai, jina_ai, voyage, infinity, nebius, ovhcloud, lambda_ai, nscale, gigachat

**Custom format (need full TransformRequest/Response):**
elevenlabs (TTS — unique audio format), deepgram (STT — unique audio format), aws_polly (TTS — AWS Signature V4), stability (image — `/v1/generation/{engine}/text-to-image` custom format), fal_ai (async job pattern), recraft (image — unique format)

---

## R7: Guardrail & Callback Plugin Patterns

### Decision
All 34 plugins follow the established 2-file pattern: implementation file + factory.go case addition.

### Rationale

**Guardrail interface (3 methods):**
```go
type Guardrail interface {
    Name() string
    SupportedHooks() []Hook          // pre_call, post_call
    Run(ctx, hook, req, resp) (Result, error)
}
```
Registration: Add `case "mode_string"` in `factory.go`'s `NewFromConfig()` switch.

**Callback interface (2 methods):**
```go
type CustomLogger interface {
    LogSuccess(data LogData)
    LogFailure(data LogData)
}
```
Registration: Add `case "type_string"` in `factory.go`'s `NewFromConfig()` switch.

### Pattern
Each guardrail/callback is an HTTP client that:
1. Extracts relevant content from `req`/`resp` (using `extractContent()` helper for guardrails)
2. Sends it to an external API
3. Returns pass/fail (guardrails) or silently logs (callbacks)

No new interfaces or abstractions needed. The plugin system is designed for exactly this expansion.

---

## R8: Image Variations Endpoint

### Decision
Add `POST /v1/images/variations` using the same pass-through pattern as `ImagesEdit`.

### Rationale
`ImagesEdit` (server.go:154) is already implemented as a pass-through proxy via `assistantsProxy()`. `ImageVariations` should be identical — register the route, create a handler that calls `assistantsProxy()`. This is a 5-line change.

### Source
- Go: `internal/proxy/handler/native_format.go:121` — `ImagesEdit` calls `h.assistantsProxy(w, r)`
- Route: `server.go:154` — `r.Post("/images/edits", s.Handlers.ImagesEdit)`
