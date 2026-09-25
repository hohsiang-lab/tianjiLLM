# Contract: OpenAI 401 Refresh Retry and Failover

## Applies To

Official OpenAI direct HTTP requests that use `openai_subscription_credential_ids`.

This contract does not apply to:

- API-key deployments without subscription credential IDs
- custom `api_base`
- OpenAI-compatible providers
- Codex app-server transport
- response bodies already committed to the client

## Request Attempt Contract

For each configured credential candidate:

1. Build the upstream request with the candidate bearer token.
2. Send the request.
3. If response status is not 401, preserve existing behavior:
   - 429 and 5xx follow HO-1179 retry/failover behavior.
   - other non-401 4xx responses return without subscription refresh.
4. If response status is 401:
   - close/discard the response body before retrying
   - force-refresh the same credential once
   - rebuild the same upstream request
   - send the retry with the refreshed bearer token
5. If refreshed retry succeeds or returns a non-401 final response, return it.
6. If refreshed retry returns 401:
   - close/discard the response body
   - persist disabled/auth-failed metadata
   - continue with the next credential candidate

## Forced Refresh Contract

Input:

```json
{
  "credential_id": "cred_openai_subscription_a",
  "reason": "upstream_401"
}
```

Behavior:

- Load the latest credential row.
- Reject missing, wrong-type, malformed, disabled, or missing refresh token with typed safe error.
- Call OpenAI token endpoint using refresh-token grant through the existing configured endpoint override.
- Persist successful token bundle through existing update helper.
- Persist refresh failure metadata through existing failure helper.
- Return refreshed bearer material for the same credential.

Output on success:

```json
{
  "credential_id": "cred_openai_subscription_a",
  "bearer_token": "<secret, runtime only>",
  "account_id": "acct_123"
}
```

The bearer token is runtime-only and must not be logged or persisted outside encrypted token storage.

## Metadata Contract

### Refresh Failure

```json
{
  "status": "refresh_failed",
  "last_error": "<redacted safe error>",
  "disabled_reason": "refresh_failed"
}
```

### Retry Still 401

```json
{
  "status": "disabled",
  "last_error": "OpenAI authentication failed after forced refresh",
  "disabled_reason": "auth_failed_after_refresh"
}
```

### Successful Forced Refresh

```json
{
  "status": "active",
  "last_refresh_at": "<RFC3339 timestamp>"
}
```

## HTTP Response Contract

### Success After Same-Credential Retry

Return the successful upstream response unchanged, subject to existing handler transformation.

### Success After Failover

Return the successful response from the next credential unchanged, subject to existing handler transformation.

### All Credentials Failed

Recommended response:

```json
{
  "error": {
    "message": "OpenAI subscription reauthorization required: all configured credentials failed authentication or refresh",
    "type": "authentication_error",
    "code": "openai_subscription_reauthorization_required"
  }
}
```

Recommended status: `401 Unauthorized`.

The response may include sanitized per-credential reason codes only if existing error response shape can safely carry them. It must not include raw upstream body or token material.

## Retry Limits

- One forced refresh per credential per client request.
- One retry after forced refresh per credential per client request.
- Total upstream attempts are capped by `2 * number_of_subscription_candidates` for 401 paths, plus existing HO-1179 429/5xx behavior where applicable.
- No API-key fallback when subscription IDs are configured.

## Redaction Contract

All stored or returned auth failure details must pass through `redact.String` or equivalent HO-1183 helper before leaving the local function boundary.
