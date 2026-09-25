# Contract: Codex session-aware subscription routing

This is an internal routing contract for `internal/proxy/handler`.

## Inputs

### Supported Codex Entry Points

- HTTP `/v1/responses`: decoded request payload map.
- WebSocket `/v1/responses`: decoded `response.create` frame payload map.
- HTTP `/v1/responses/compact`: decoded request payload map.
- Chat completion and chat streaming: `model.ChatCompletionRequest.Metadata`.

### Session Metadata Shape

```json
{
  "client_metadata": {
    "session_id": "sess_123",
    "x-codex-turn-metadata": "{\"session_id\":\"sess_123\"}"
  }
}
```

Extraction priority:

1. `client_metadata.session_id`
2. JSON string at `client_metadata["x-codex-turn-metadata"]`, then nested `session_id`

## Outputs

### With Session ID

```text
track_key = openai-subscription:<org>:codex-session:<session_id>:<route_class>
first candidate = rendezvous_max(session_id, selectable_credential_ids)
```

### Missing or Invalid Session ID

```text
track_key = openai-subscription:<org>:<route_class>
selection = existing fallback behavior
```

## Safety Rules

- Credential id may be used for routing score and safe logs.
- Raw bearer token, credential value, full request body, full metadata payload, and unredacted session payload must not be logged.
- Image generation/edit requests remain unchanged and do not accept new metadata fields in this issue.
