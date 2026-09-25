# Docs Contract: HO-1192

## Scope Contract

The implementation PR must update operator docs only:

- `README.md`
- `docs/openai-subscription-oauth.md`
- `docs/openclaw-tianjillm-gateway.md`
- optional small link updates in existing docs

No Go, SQL, generated template, OpenAPI generator, or runtime behavior files are in scope unless the docs verification test requires a small docs-only test harness.

## Required Sections

### README

- Short feature pointer.
- Minimal OAuth config snippet.
- Link to detailed docs.

### OpenAI subscription OAuth admin doc

- Prerequisites: TianjiLLM admin access, DB, cache, OpenAI account, TianjiLLM API key for downstream clients.
- OAuth config fields and defaults.
- Callback URL rules.
- Connect flow.
- Callback paste fallback.
- Credential lifecycle: Test, Refresh, Disable, Delete.
- Model config with `openai_subscription_credential_ids`.
- Security boundary and redaction expectations.
- Troubleshooting matrix.

### OpenClaw gateway doc

- `models.providers.openai.baseUrl` pointing to TianjiLLM `/v1`.
- `agents.defaults.model` chat/reasoning example.
- `agents.defaults.imageGenerationModel` standard image example with `openai/gpt-image-2`.
- Transparent image guidance with `openai/gpt-image-1.5`.
- `messages.tts.providers.openai.baseUrl` TTS example.
- `tools.media.audio.models` STT example.
- `agents.defaults.memorySearch.remote.baseUrl` embeddings/memory example.
- Surface compatibility matrix.

## Snippet Contract

Every config snippet must satisfy:

- No real secret values.
- Use `${TIANJI_OPENCLAW_API_KEY}` for OpenClaw-to-TianjiLLM auth.
- Do not define OpenAI subscription access/refresh/id tokens.
- Do not set custom `api_base` with `openai_subscription_credential_ids`.
- Include enough surrounding keys to be copyable into the relevant config file after replacing placeholders.

## Troubleshooting Contract

The troubleshooting matrix must include:

| Symptom | Required action |
| --- | --- |
| TianjiLLM 401 | Check downstream TianjiLLM API key before debugging OpenAI OAuth |
| Upstream 401 after refresh | Refresh/Test credential, then reconnect when refresh fails |
| `refresh_failed` | Disable failed credential, reconnect, verify Test |
| All credentials disabled | Enable healthy credential or connect a new credential |
| No credential configured | Add `openai_subscription_credential_ids` to the model config |
| Credential missing/wrong type | Remove invalid id from model config and select a subscription credential |
| Redirect URI rejected | Fix absolute URL, localhost HTTP rule, or `/auth/callback` path |
| Callback expired/replayed | Restart Connect from `/ui/credentials` |
| DB/cache unavailable | Restore dependency, then retry Connect |
| OpenClaw surface unsupported | Use native OpenClaw provider config or an OpenAI-compatible surface |

## Verification Contract

Minimum verification after implementation:

```bash
git diff --check origin/main...HEAD
git diff --name-only origin/main...HEAD
rg -n --glob '!specs/**/contracts/docs-contract.md' "OPENAI_SUBSCRIPTION_(ACCESS|REFRESH|ID)_TOKEN|sk-ant-oat|credential_value\\s*:" README.md docs/openai-subscription-oauth.md docs/openclaw-tianjillm-gateway.md specs/1192-openai-subscription-oauth-admin-and-troubleshooting-docs
rg -n "openai_subscription_credential_ids|messages\\.tts\\.providers\\.openai|tools\\.media\\.audio\\.models|agents\\.defaults\\.memorySearch|gpt-image-2|gpt-image-1\\.5" README.md docs/openai-subscription-oauth.md docs/openclaw-tianjillm-gateway.md specs/1192-openai-subscription-oauth-admin-and-troubleshooting-docs
```

When running the forbidden-secret scan from the implementation branch, scan only HO-1192 implementation docs (`README.md`, `docs/openai-subscription-oauth.md`, and `docs/openclaw-tianjillm-gateway.md`) plus HO-1192 artifacts, and pass `--glob '!specs/**/contracts/docs-contract.md'` so the contract's own sample command does not self-match. The forbidden-secret scan must return no matches. The coverage scan must return matches for each required topic.
