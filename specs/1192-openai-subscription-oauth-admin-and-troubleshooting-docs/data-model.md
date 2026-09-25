# Data Model: HO-1192 docs contract

This is a documentation data model. No database schema changes are planned.

## OpenAI OAuth Config Section

- `enabled`: boolean, whether OpenAI OAuth subscription connect flow is enabled.
- `issuer_url`: optional issuer base URL, defaults to OpenAI auth issuer.
- `authorize_url`: optional override for authorization endpoint.
- `token_url`: optional override for token endpoint.
- `redirect_uri`: callback URL; local HTTP callback must use `/auth/callback`.
- `client_id`: public OpenAI OAuth client id.
- `scopes`: OAuth scopes.
- `originator`: OAuth originator metadata.

Validation in docs:

- callback URL must be absolute.
- HTTP callback must be localhost.
- local HTTP path must be `/auth/callback`.
- no client secret is configured.

## Subscription Model Config Section

- `model_name`: TianjiLLM model alias.
- `tianji_params.model`: upstream OpenAI model id.
- `tianji_params.openai_subscription_credential_ids`: ordered credential ids for routing.

Validation in docs:

- IDs are non-empty and unique.
- custom `api_base` is not used with subscription credentials.
- provider is official OpenAI.

## Credential Lifecycle Section

- `Connect`: creates OAuth state and sends admin through OpenAI authorization.
- `Test`: validates credential usability through a safe probe.
- `Refresh`: refreshes expired or near-expired credential material.
- `Disable`: prevents routing while preserving record/audit trail.
- `Delete`: removes credential record or routeable credential access.

Expected docs fields:

- purpose.
- when to use.
- expected success result.
- expected failure result.
- security note.

## Troubleshooting Row

- `symptom`: operator-visible failure.
- `likely_cause`: most probable root cause category.
- `checks`: concrete verification steps.
- `operator_action`: remediation.
- `safe_to_retry`: whether retry is safe after action.

Required rows:

- `401` from TianjiLLM.
- `401` from upstream OpenAI after refresh.
- `refresh_failed`.
- all credentials disabled.
- no credential configured.
- credential missing or wrong type.
- callback state expired/replayed/mismatched.
- redirect URI rejected.
- DB/cache unavailable.
- OpenClaw API key invalid.
- OpenClaw surface not OpenAI-compatible.

## OpenClaw Gateway Example

- `models.providers.openai.baseUrl`: TianjiLLM `/v1`.
- `models.providers.openai.apiKey`: `${TIANJI_OPENCLAW_API_KEY}`.
- `models.providers.openai.models[]`: OpenAI-compatible model ids exposed by TianjiLLM.
- `agents.defaults.model.primary`: chat/reasoning model selection.
- `agents.defaults.imageGenerationModel.primary`: image model selection.
- `messages.tts.providers.openai.baseUrl`: TianjiLLM `/v1`.
- `tools.media.audio.models[]`: STT model entry with TianjiLLM `/v1`.
- `agents.defaults.memorySearch.remote.baseUrl`: TianjiLLM `/v1`.

Validation in docs:

- All secrets are placeholders.
- The placeholder key is explicitly a TianjiLLM API key.
- No OpenAI subscription token appears in OpenClaw config.
