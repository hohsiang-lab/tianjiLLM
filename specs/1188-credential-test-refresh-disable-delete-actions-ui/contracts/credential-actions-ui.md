# Contract: Credential actions UI

## UI Routes

All routes are under existing `/ui` session auth.

### `POST /ui/credentials/{credential_id}/test`

Runs OpenAI subscription credential test and returns a credentials list/detail partial plus OOB toast.

Success toast:

- Title: `Credential test succeeded`
- Safe message may include `models_count`.

Failure toast:

- Title: `Credential test failed`
- Safe message uses stable reason code such as `upstream_401` or `credential_missing`.

### `POST /ui/credentials/{credential_id}/refresh`

Runs force refresh and returns refreshed safe metadata partial plus OOB toast.

Success toast:

- Title: `Credential refreshed`
- Safe message may include `last_refresh_at`.

Failure toast:

- Title: `Credential refresh failed`
- Safe message uses stable reason code.

### `POST /ui/credentials/{credential_id}/disable`

Requires confirm. Locally disables credential and returns refreshed safe metadata partial plus OOB toast.

Success toast:

- Title: `Credential disabled`
- Safe message may include safe disabled reason `operator_disabled`.

Failure toast:

- Title: `Credential disable failed`
- Safe message uses stable reason code.

### `POST /ui/credentials/{credential_id}/delete`

Requires destructive confirm. Deletes local credential and returns refreshed list or safe deleted detail state plus OOB toast.

Success toast:

- Title: `Credential deleted`

Failure toast:

- Title: `Credential delete failed`
- Safe message uses stable reason code.

## Backend Contracts Consumed

### `POST /credentials/openai-subscription/{credential_id}/test`

Safe response:

```json
{
  "credential_id": "cred_123",
  "action": "test",
  "status": "ok",
  "models_count": 2
}
```

### `POST /credentials/openai-subscription/{credential_id}/refresh`

Safe response:

```json
{
  "credential_id": "cred_123",
  "action": "refresh",
  "status": "ok",
  "last_refresh_at": "2026-05-08T00:00:00Z"
}
```

### `POST /credentials/openai-subscription/{credential_id}/disable`

Safe response:

```json
{
  "credential_id": "cred_123",
  "action": "disable",
  "status": "ok"
}
```

### `DELETE /credentials/delete/{credential_id}`

Existing credential delete contract. UI must treat success as local deletion only.

## Forbidden UI Output

UI route responses, rendered partials, OOB toast, attributes, links, hidden fields, and error copy must not contain:

- `credential_value`
- `access_token`
- `refresh_token`
- `id_token`
- `Authorization`
- bearer token strings
- JWT-looking strings
- authorization code
- encrypted credential blobs
- fallback API keys
- raw upstream OpenAI token/model response bodies
- raw OpenAI account payload
