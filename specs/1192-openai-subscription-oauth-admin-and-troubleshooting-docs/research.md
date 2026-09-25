# Research: HO-1192 OpenAI subscription OAuth admin and troubleshooting docs

## Repository Evidence

### OpenAI-compatible API surface

`README.md` already describes TianjiLLM as exposing `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`, and streaming via SSE. Existing endpoint tests also cover subscription routing for:

- `/v1/chat/completions`
- `/v1/responses`
- `/v1/embeddings`
- `/v1/images/generations`
- `/v1/images/edits`
- `/v1/images/variations`
- `/v1/audio/transcriptions`
- `/v1/audio/speech`

### OAuth config defaults

`internal/config/openai_oauth.go` defines:

- issuer: `https://auth.openai.com`
- authorize URL: `https://auth.openai.com/oauth/authorize`
- token URL: `https://auth.openai.com/oauth/token`
- redirect URI: `http://localhost:1455/auth/callback`
- client ID: `app_EMoamEEZ73f0CkXaXp7hrann`
- originator: `tianjillm`
- scopes: `openid`, `profile`, `email`, `offline_access`, `api.connectors.read`, `api.connectors.invoke`

Redirect validation requires an absolute `http` or `https` URL. HTTP redirects are limited to localhost hosts and path `/auth/callback`.

### Model config constraints

`internal/config/config.go` defines `tianji_params.openai_subscription_credential_ids`. `internal/config/validate.go` enforces:

- no empty credential ID.
- no duplicate credential ID.
- no custom `api_base` when subscription credentials are configured.
- official OpenAI model provider only.

### Admin lifecycle routes

`internal/ui/routes.go` exposes OpenAI Connect and callback paste routes under the protected UI. `internal/ui/handler_openai.go` requires an org id, creates a state record, and redirects to the OpenAI authorize URL. Credential detail routes provide lifecycle actions through the credentials UI.

### Refresh and routing failure codes

Subscription routing and refresh code uses safe reason codes including:

- `credential_missing`
- `credential_wrong_type`
- `credential_disabled`
- `credential_malformed`
- `credential_expired`
- `credential_lookup_failed`
- `refresh_failed`
- `auth_failed_after_refresh`

These should appear in troubleshooting docs as operator-facing diagnostic categories, not as leaked credential data.

## OpenClaw Schema Evidence

OpenClaw live config schema confirms:

- `models.providers.*` requires `baseUrl` and `models`; supports sensitive `apiKey`, adapter `api`, headers, and request overrides.
- `models.providers.*.models.*` requires `id` and `name`; supports `api`, `baseUrl`, `reasoning`, `input`, `headers`, and compatibility metadata.
- `messages.tts.providers.openai` supports sensitive `apiKey` and wildcard provider fields such as `baseUrl`, model, and voice.
- `tools.media.audio.models.*` supports `provider`, `model`, `baseUrl`, `headers`, and `request`.
- `agents.defaults.memorySearch` supports provider/model config and `remote.baseUrl`, sensitive `remote.apiKey`, and headers.

Docs examples should use only placeholders and should not reproduce local runtime config values.

## Official OpenAI Docs Evidence

Official OpenAI docs consulted via OpenAI-domain search:

- `https://developers.openai.com/api/docs/models/gpt-image-2`
- `https://developers.openai.com/api/docs/models/gpt-image-1.5`
- `https://developers.openai.com/api/docs/models/gpt-4o-mini-tts`
- `https://developers.openai.com/api/docs/models/gpt-4o-transcribe`
- `https://developers.openai.com/api/docs/models/whisper-1`
- `https://developers.openai.com/api/docs/models/text-embedding-3-small`
- `https://developers.openai.com/api/docs/models/text-embedding-3-large`

Relevant endpoint mapping:

- Image generation and edits use `/v1/images/generations` and `/v1/images/edits`.
- Speech generation uses `/v1/audio/speech`.
- Speech-to-text uses `/v1/audio/transcriptions`.
- Embeddings use `/v1/embeddings`.

## Decisions

### Use TianjiLLM `/v1` for OpenAI-compatible surfaces only

OpenClaw can route OpenAI-compatible HTTP surfaces through TianjiLLM by setting the provider base URL to TianjiLLM `/v1`. Non-OpenAI-compatible OpenClaw integrations still need their native provider auth/config.

### Prefer env placeholders in docs examples

Use `${TIANJI_OPENCLAW_API_KEY}` and `https://tianji.example.com/v1`. Do not show fake OpenAI subscription token strings, because copied examples can become unsafe operational patterns.

### Keep troubleshooting action-oriented

Every troubleshooting row must map symptom to likely cause, check, and operator action. Avoid vague “check logs” only rows.

## Open Questions

None for Todo planning. Implementation should verify final OpenClaw config snippets against the installed OpenClaw version before publishing docs.
