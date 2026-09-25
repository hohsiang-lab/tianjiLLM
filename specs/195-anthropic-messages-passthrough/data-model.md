# Data Model: Anthropic /v1/messages Passthrough

**Date**: 2026-03-25

## Entities

This feature is stateless — no new database entities or persistent state.

### Request Body (peek only)

The handler reads only the `model` field from the request body to resolve the upstream API key. The rest of the body is forwarded unchanged.

```
{ "model": string }  // only field extracted; rest forwarded as-is
```

### Header Manipulation

| Header | Source | Behavior |
|--------|--------|----------|
| `Authorization` | Client | **Replaced** with upstream OAuth token (`Bearer sk-ant-oat01-...`) or removed if using `x-api-key` |
| `x-api-key` | System | **Set** to upstream API key (non-OAuth only) |
| `anthropic-beta` | Client | **Merged** with OAuth-required betas (e.g., `oauth-2025-04-20`), deduped |
| `anthropic-version` | Client | **Forwarded** as-is, or defaulted to `2023-06-01` |
| `anthropic-dangerous-direct-browser-access` | System | **Set to `true`** when OAuth token detected |
| `Host`, `Content-Length` | System | **Rewritten** by ReverseProxy (hop-by-hop) |
| All other headers | Client | **Forwarded** unchanged (`User-Agent`, `x-app`, `X-Stainless-*`, etc.) |

### Query Parameters

All query parameters from the client request (e.g., `?beta=true`) are forwarded verbatim to the upstream URL.
