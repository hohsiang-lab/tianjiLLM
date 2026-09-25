# Research: OpenAI subscription Codex transport model config/UI selector

## Repo Reality

### Current config model

After `origin/main` advanced to `3fa617a` (HO-1290), `internal/config/config.go` already defines `TianjiParams.OpenAISubscriptionTransport` with YAML key `openai_subscription_transport`. HO-1291 must reuse that field rather than re-adding it.

### Current validation

`internal/config/validate.go` rejects blank/duplicate subscription credential IDs, custom non-default `api_base`, and non-OpenAI provider names. It also rejects `openai_subscription_transport=chatgpt_codex_backend` when no subscription credential IDs are present.

Remaining HO-1291 gap: validation still does not require a transport when subscription credential IDs are present, does not accept/reject `direct_openai_http` through a shared config-level enum, and does not reject unknown transport strings for subscription-backed rows.

### Current Models UI

`internal/ui/handler_models.go` parses repeated `openai_subscription_credential_ids` fields and persists them into `tianji_params`. `internal/ui/pages/models.templ` renders the OpenAI subscription credential selector and custom API base conflict warning. There is no selector for Platform direct HTTP vs ChatGPT Codex backend.

### Current runtime decode

`internal/proxy/handler/runtime_model_source.go` now maps DB `tianji_params.openai_subscription_transport` into typed `config.TianjiParams.OpenAISubscriptionTransport`. HO-1291 should keep a regression test for this boundary, not implement the decoder from scratch.

### Current backend routing baseline

`internal/proxy/handler/openai_subscription_resolution.go` now has transport values for `direct_openai_http`, legacy `codex_app_server`, and `chatgpt_codex_backend`. Chat completion routing checks `isChatGPTCodexBackendTransport(...)` and can route the backend path when the config field is already present. HO-1291 still owns the model config/UI path that lets operators create and edit that field explicitly.

### Prior specs

HO-1172 established Models UI credential selection and safe metadata display. HO-1288 and HO-1290 have since landed backend/config-adjacent pieces on `origin/main`; HO-1291 remains the UI selector and config validation hardening slice.

## Official OpenAI Evidence

- OpenAI Help Center says Codex is included with ChatGPT Plus/Pro/Business/Edu/Enterprise plans and can be connected through a ChatGPT account.
- OpenAI Codex CLI docs say Codex CLI can authenticate with a ChatGPT account or an API key.
- OpenAI API model docs for GPT-5.2-Codex list public API endpoints such as `/v1/chat/completions` and `/v1/responses`, while the private ChatGPT backend endpoint from HO-1288 is not documented as a public Platform endpoint.

Implication: Tianji should not infer private ChatGPT backend routing from model name alone. Subscription-backed Codex routing must be explicit, and normal API-key OpenAI routing must remain valid.

## Design Conclusion

Use the existing per-model enum-like config field:

```yaml
tianji_params:
  model: openai/*
  openai_subscription_credential_ids:
    - cred-a
  openai_subscription_transport: chatgpt_codex_backend
```

Accepted values:

- `direct_openai_http`: existing subscription bearer path to official OpenAI Platform API.
- `chatgpt_codex_backend`: HO-1288 ChatGPT Codex backend HTTP transport.

Because main now already contains the field, runtime decode, and backend routing hook, implementation should focus on:

- config validation for missing/unknown transport on subscription-backed rows;
- Create/Edit Models UI parsing, persistence, prefill, table summary, and stale-field removal;
- tests proving existing API-key OpenAI and custom `api_base` behavior remain valid.

## Open Questions

No owner input is required for Todo scope. Implementation should still decide one compatibility detail before coding: whether existing DB rows with subscription credentials but no transport should be grandfathered until edited, or require a one-time admin update. New UI/config saves must always be explicit.
