# Data Model: Phase 4 — Full Migration Gap Closure

**Branch**: `004-migration-gap-phase4` | **Date**: 2026-02-17

## Overview

Phase 4 primarily extends existing data structures rather than creating new entities. Most gaps involve wiring existing config fields to runtime logic, or adding fields to existing structs. Only 2 genuinely new entities are introduced: `TokenCounter` and `WebSocketRelay`.

---

## Extended Entities (Modify Existing)

### RouterSettings (internal/router/router.go)

Current struct is missing fields that config already parses.

| Field | Type | Source | Purpose |
|-------|------|--------|---------|
| `ModelGroupAlias` | `map[string]ModelGroupAliasItem` | `config.RouterSettings.ModelGroupAlias` | Alias resolution before deployment lookup + model visibility control |
| `Fallbacks` | `map[string][]string` | `config.TianjiLLMSettings.Fallbacks` | Per-model error-based fallback chains |
| `DefaultFallbacks` | `[]string` | `config.TianjiLLMSettings.DefaultFallbacks` | Default fallback when no model-specific fallback configured |
| `ContentPolicyFallbacks` | `map[string][]string` | `config.RouterSettings.ContentPolicyFallbacks` | Fallback on content policy errors (HTTP 400) |
| `ModelGroupRetryPolicy` | `map[string]RetryPolicy` | `config.RouterSettings.ModelGroupRetryPolicy` | Per-model-group retry/timeout config |
| `EnableTagFiltering` | `bool` | Already exists in config | Enable tag-based deployment filtering |
| `TagFilteringMatchAny` | `bool` | Already exists in config | Match any tag vs match all tags |

### RetryPolicy (NEW type in internal/router/)

| Field | Type | Purpose |
|-------|------|---------|
| `NumRetries` | `int` | Max retry attempts for this model group |
| `TimeoutSeconds` | `int` | Per-attempt timeout |
| `RetryAfterSeconds` | `int` | Backoff between retries |

### ModelGroupAliasItem (NEW type in internal/router/)

Matches Python's `RouterModelGroupAliasItem(TypedDict)`.

| Field | Type | Purpose |
|-------|------|---------|
| `Model` | `string` | Target model group name to resolve to |
| `Hidden` | `bool` | If true, alias is not returned in `/v1/models` list response |

### Deployment (internal/router/router.go)

| Field | Type | Source | Purpose |
|-------|------|--------|---------|
| `Region` | `string` | `config.TianjiLLMParams.Region` | Geographic region for region-based routing |

### GeneralSettings / SSOSettings (internal/config/config.go)

| Field | Type | Purpose |
|-------|------|---------|
| `SSOClientID` | `string` | OIDC client ID |
| `SSOClientSecret` | `string` | OIDC client secret (env var resolved) |
| `SSOIssuerURL` | `string` | OIDC issuer URL for discovery |
| `SSORedirectURI` | `string` | Callback URL for auth code flow |
| `SSOScopes` | `[]string` | Requested OIDC scopes |
| `SSORoleMapping` | `map[string]string` | IDP group → TianjiLLM role mapping |

### PassThroughEndpoint (internal/config/config.go)

Already defined. No changes needed.

```
Path    string            — route path pattern
Target  string            — upstream base URL
Headers map[string]string — additional headers to forward
```

---

## New Entities

### TokenCounter (internal/token/counter.go)

Wraps `tiktoken-go` for pre-request token estimation.

| Field/Method | Type | Purpose |
|-------------|------|---------|
| `CountMessages(model string, messages []Message) int` | method | Count tokens for chat messages |
| `CountText(model string, text string) int` | method | Count tokens for raw text |
| `encoderCache` | `map[string]*tiktoken.Tiktoken` | Cache encoders by model name |

**Relationships**:
- Used by: chat handler (budget pre-check), rate limiter (TPM check), router (TPM-based selection)
- Depends on: `tiktoken-go` library

**Validation**: Unknown model names fall back to `o200k_base` encoding. Non-OpenAI models return -1 (caller uses API-returned usage instead).

### WebSocketRelay (internal/proxy/handler/realtime.go)

Bidirectional WebSocket proxy between client and upstream provider.

| Field/Method | Type | Purpose |
|-------------|------|---------|
| `Upgrade(w, r)` | method | Accept client WebSocket, dial upstream, start relay |
| `upstreamURL` | `string` | Provider's realtime endpoint URL |
| `authHeader` | `string` | Authorization header for upstream |

**Relationships**:
- Mounted at: `/v1/realtime` in server.go
- Depends on: `coder/websocket`, auth middleware (extracts API key before upgrade)

**State transitions**:
```
Client connects → Upgrade → Dial upstream → Relay active
                                           ↓
                    Client disconnects → Cancel context → Close upstream
                    Upstream disconnects → Cancel context → Close client
                    Error → Close both → Log
```

---

## Cache Key Model (for F2 — LLM Response Caching)

Not a new entity, but a critical data structure for cache integration.

| Component | Value |
|-----------|-------|
| Key format | `tianji:cache:{sha256(model + sorted_messages_json)}` |
| Value format | Serialized `model.ChatCompletionResponse` JSON |
| TTL | From `config.CacheParams.TTL` (default 60s) |
| Namespace | From `config.CacheParams.Namespace` (optional prefix) |

**Cache flow in chat handler**:
1. Pre-call: compute key → `Cache.Get(key)` → if hit, return cached response
2. Post-call: `Cache.Set(key, response, ttl)` → store for future hits
3. Streaming: assemble full response from stream chunks → cache assembled result

---

## Fallback Chain Model (for A4 — General Fallback)

| Field | Type | Purpose |
|-------|------|---------|
| `Fallbacks` | `map[string][]string` | `"gpt-4" → ["claude-3", "gemini-pro"]` |
| `DefaultFallbacks` | `[]string` | Applied when no model-specific fallback exists |

**Resolution order**:
1. Try all deployments of requested model (existing Route() behavior)
2. If all fail → check `Fallbacks[modelName]` → try each fallback model in order
3. If no model-specific fallback or all fail → try `DefaultFallbacks` in order
4. If all fail → return original error

---

## Entity Relationship Summary

```
Config (YAML)
├── PassThroughEndpoints[] → server.go route registration → forwardToProvider()
├── SSOSettings → auth.NewSSOHandler() → handler.SSOLogin/SSOCallback
├── RouterSettings
│   ├── ModelGroupAlias → router.Route() alias lookup
│   ├── Fallbacks → router.GeneralFallback()
│   ├── DefaultFallbacks → router.GeneralFallback()
│   ├── ModelGroupRetryPolicy → router.Route() per-group retry
│   ├── EnableTagFiltering → router.Route() → strategy.PickWithTags()
│   └── TagFilteringMatchAny → strategy.hasAnyTag()
├── CacheParams → cache.New() → chat handler Get/Set
└── TianjiLLMSettings
    └── Callbacks → callback.Registry (existing)

New Dependencies
├── coder/websocket → handler/realtime.go → WebSocketRelay
└── tiktoken-go → internal/token/counter.go → TokenCounter
```
