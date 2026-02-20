# Data Model: Phase 5 Migration Gap Closure

**Date**: 2026-02-18
**Branch**: `005-migration-gap-phase5`

---

## Entity Relationship Overview

```
MCPServerConfig 1───* MCPTool
MCPTool *───1 MCPServerManager (runtime registry)
SearchToolConfig 1───1 SearchProvider
PromptTemplate 1───* PromptVersion (existing DB entity)
DiscoveryModel *───1 ModelGroup
GuardrailConfig 1───1 Guardrail (runtime instance)
CallbackConfig 1───1 CustomLogger (runtime instance)
AutoRouterConfig 1───* Route
Route *───1 ModelGroup
```

---

## US-1: MCP Server Entities

### MCPServerConfig
Represents an upstream MCP server defined in YAML config or stored in DB.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| ServerID | string | Required, unique | Auto-generated or from config key |
| Alias | string | Optional | Short name for tool prefixing |
| Transport | enum | "stdio" \| "sse" \| "http" | How to connect to upstream |
| URL | string | Required for sse/http | Upstream MCP server URL |
| Command | string | Required for stdio | Executable path |
| Args | []string | Optional | Command arguments |
| AuthType | enum | "api_key" \| "bearer_token" \| "basic" \| "oauth2" \| none | Authentication method |
| AuthToken | string | Optional, secret | Static auth token |
| StaticHeaders | map[string]string | Optional | Always-sent headers |
| AllowedTools | []string | Optional | Whitelist (nil = all allowed) |
| DisallowedTools | []string | Optional | Blacklist |

### MCPTool
A callable function exposed to MCP clients. Discovered from upstream servers or defined locally.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| Name | string | Required, unique per server | Unprefixed tool name |
| PrefixedName | string | Derived | `{server_alias}{sep}{name}` |
| Description | string | Required | Human-readable description |
| InputSchema | JSON | Required | JSON Schema for tool input |
| ServerID | string | Required | Which upstream server owns this tool |

### MCPServerManager (runtime)
Singleton managing all upstream server connections and tool-to-server mapping.

| Field | Type | Notes |
|-------|------|-------|
| ConfigServers | map[string]MCPServerConfig | From YAML config |
| DBServers | map[string]MCPServerConfig | From database |
| ToolToServer | map[string]string | prefixed_tool_name → server_id |
| ToolSeparator | string | Default "-", from env `MCP_TOOL_PREFIX_SEPARATOR` |

**State Transitions**: None — MCP servers are either connected or disconnected, managed at transport level.

---

## US-2: Search Provider Entities

### SearchToolConfig
A named search tool configured in YAML, linking to a search provider.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| SearchToolName | string | Required, unique | Route key: `/v1/search/{name}` |
| SearchProvider | enum | brave \| tavily \| searxng \| exa_ai \| google_pse \| dataforseo | Provider implementation |
| APIKey | string | Optional (env fallback) | Provider-specific credential |
| APIBase | string | Optional (default per provider) | Override base URL |
| Description | string | Optional | Human-readable |

### SearchResult
Normalized search result (Perplexity-compatible format).

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| Title | string | Required | Result title |
| URL | string | Required | Result URL |
| Snippet | string | Required | Text excerpt |
| Date | string | Optional | Publication date (ISO 8601) |
| LastUpdated | string | Optional | Last crawl timestamp |

### SearchResponse
Container for search results.

| Field | Type | Notes |
|-------|------|-------|
| Results | []SearchResult | Normalized results |
| Object | string | Always "search" |

---

## US-4: Prompt Template Resolution

Uses existing DB entities — no new entities needed.

### PromptTemplate (existing)
Already defined in `internal/db/` via sqlc. Fields: ID, Name, Version, Template, Variables, Model, Metadata, CreatedAt.

### ChatCompletionRequest Extensions
Two new optional fields on the existing `ChatCompletionRequest`:

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| PromptName | string | Optional | Reference to stored prompt template |
| PromptVariables | map[string]string | Optional | Variable substitutions |
| PromptVersion | int | Optional | Pin specific version (default: latest) |

---

## US-6: Discovery Entities

### DiscoveryModel
Read-only view assembled at query time from config + model pricing data.

| Field | Type | Notes |
|-------|------|-------|
| ModelGroup | string | Model group name from config |
| Providers | []string | Provider names serving this group |
| MaxInputTokens | int | Max across all deployments |
| MaxOutputTokens | int | Max across all deployments |
| InputCostPerToken | float64 | From model_prices JSON |
| OutputCostPerToken | float64 | From model_prices JSON |
| Mode | string | "chat" \| "embedding" \| "image" \| "audio" |
| SupportsVision | bool | OR across deployments |
| SupportsFunctionCalling | bool | OR across deployments |
| SupportsStreaming | bool | OR across deployments |
| SupportsWebSearch | bool | OR across deployments |
| SupportedParams | []string | Union across deployments |

**Aggregation rules for model groups:**
- Boolean fields: OR (any deployment supports → group supports)
- Numeric fields: MAX (highest value across deployments)
- TPM/RPM: SUM (total capacity across deployments)
- String arrays: UNION (deduplicated)

---

## US-8: AutoRouter Entities

### AutoRouterConfig
Configuration for semantic routing on a model deployment.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| Routes | []Route | Required, 1+ | Routing rules |
| DefaultModel | string | Required | Fallback model group |
| EmbeddingModel | string | Required | Model for encoding queries |
| ScoreThreshold | float64 | Default 0.3 | Minimum similarity |

### Route
A single routing rule mapping utterance patterns to a model group.

| Field | Type | Constraints | Notes |
|-------|------|-------------|-------|
| Name | string | Required | Target model group name |
| Utterances | []string | Required, 1+ | Example phrases |
| Description | string | Optional | Human-readable |
| ScoreThreshold | float64 | Optional, overrides global | Per-route threshold |

### RouteVectors (runtime)
Pre-computed embedding vectors for route utterances, cached in memory.

| Field | Type | Notes |
|-------|------|-------|
| RouteIndex | int | Index into Routes array |
| Vectors | [][]float32 | One vector per utterance |

---

## Validation Rules Summary

| Entity | Rule |
|--------|------|
| MCPServerConfig | Transport "stdio" requires Command; "sse"/"http" requires URL |
| MCPTool | PrefixedName must match MCP tool name spec (no spaces, alphanumeric + `-_`) |
| SearchToolConfig | SearchProvider must be one of the 6 supported enum values |
| SearchResult | Title, URL, Snippet all required; Date/LastUpdated optional |
| PromptTemplate | Template must contain at least one `{{variable}}` if Variables is non-empty |
| DiscoveryModel | Mode must be one of: chat, embedding, image, audio, moderation, rerank |
| AutoRouterConfig | DefaultModel and EmbeddingModel must reference existing model groups |
| Route | Utterances must have at least 1 entry; Name must reference existing model group |
