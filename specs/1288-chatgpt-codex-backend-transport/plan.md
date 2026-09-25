# Implementation Plan: ChatGPT Codex backend transport

**Branch**: `HO-1288-chatgpt-codex-backend-transport`
**Linear**: HO-1288
**Spec**: [spec.md](spec.md)
**Phase**: Todo planning only; implementation starts only after Linear moves to `In Progress`.

## Summary

Add a transport boundary separate from `internal/provider/openai` that turns selected OpenAI subscription credentials into ChatGPT Codex backend `/codex/responses` requests. Keep `internal/provider/openai` and API-key routing on Platform `/v1` unchanged. The implementation should be RED-first with mocked upstreams proving URL/header/body behavior before any production code changes.

## Technical Context

- Language/runtime: Go module in TianjiLLM.
- Existing direct OpenAI provider: `internal/provider/openai/openai.go` uses default `https://api.openai.com/v1` and appends `/chat/completions`.
- Existing OpenAI endpoint proxy: `internal/proxy/handler/openaiEndpointProxy` route family already handles direct `/v1/responses` and other Platform endpoints.
- Existing subscription resolver: `internal/proxy/handler/openai_subscription_resolution.go` resolves `openAISubscriptionTransportDirectHTTP` bearer tokens and `openAISubscriptionTransportCodexAppServer` login payloads.
- Existing config: `config.OpenAIOAuthConfig.Originator` currently defaults to `tianjillm` for OAuth authorize flow; HO-1288 should not reuse that default for backend requests because the backend transport default must be `codex_cli_rs`.
- Testing: Go unit/handler/provider tests with `httptest.Server`; no real ChatGPT/OpenAI calls.

## Constitution Check

| Principle | Status | Notes |
|---|---:|---|
| Spec-first | PASS | Todo phase changes only this specs directory. |
| Repo reality first | PASS | Plan is based on current provider, proxy handler, subscription resolver, and codexapp helpers. |
| Research before build | PASS | research.md records official OpenAI API docs limits plus local OpenClaw/Codex wire evidence. |
| Failing-tests-first | PASS | tasks.md starts with RED URL/header/no-Platform tests. |
| Security | PASS | Token/header errors must use existing redaction boundary. |
| No broad refactor | PASS | Keep normal OpenAI provider untouched and add a separate transport. |

## Architecture Decision

### D1 - Separate provider/transport, not `openai.Provider` mutation

Add a dedicated ChatGPT Codex backend transport package or handler-local boundary, for example:

```text
internal/provider/chatgptcodex/
  transport.go
  transport_test.go
```

Do not change `internal/provider/openai.Provider.GetRequestURL()` for this feature. That provider owns Platform `/v1/chat/completions`; mutating it would risk API-key regressions.

### D2 - Explicit route selection

Use an explicit config signal to select the backend transport. Preferred implementation choice:

```yaml
model_list:
  - model_name: chatgpt/gpt-5.2-codex
    tianji_params:
      model: chatgpt/gpt-5.2-codex
      openai_subscription_credential_ids: ["cred-a"]
      openai_subscription_transport: chatgpt_codex_backend
```

If implementation finds an existing transport enum or provider-registration pattern that is cleaner, use it, but tests must prove selection is explicit and API-key routes stay unchanged.

### D3 - Separate backend config from OAuth authorize config

Add backend request config without changing the OAuth authorize `originator` default:

```yaml
general_settings:
  openai_oauth:
    codex_backend_base_url: https://chatgpt.com/backend-api
    codex_backend_originator: codex_cli_rs
```

The exact field names may change to match repo conventions. The behavior must remain: default URL is ChatGPT backend, default originator is `codex_cli_rs`, override is testable.

### D4 - Reuse credential resolver and selection gates

Extend existing resolver enum with a third transport if useful:

```go
openAISubscriptionTransportChatGPTCodexBackend
```

It should return access token + account ID, not `codexapp.LoginStartParams`. Candidate ordering, disabled credential handling, refresh, sticky/failover, and rate-limit gating should reuse the current `resolveOpenAISubscriptionAttemptOrder` path.

### D5 - Mocked backend first

Implementation tests must rewrite or inject the ChatGPT backend URL to `httptest.Server`. No test may hit `chatgpt.com` or `api.openai.com`.

## Target Flow

```text
Client /v1/chat/completions or /v1/responses
  -> runtime model source resolves selected model config
  -> explicit transport == chatgpt_codex_backend
  -> resolveOpenAISubscriptionAttemptOrder(..., chatgpt_codex_backend)
  -> build POST https://chatgpt.com/backend-api/codex/responses
       Authorization: Bearer <access_token>
       ChatGPT-Account-Id: <account_id> when present
       originator: <codex_backend_originator || codex_cli_rs>
       OpenAI-Beta: responses=experimental
       Accept: text/event-stream when streaming
       Content-Type: application/json
  -> send backend Responses-style payload
  -> adapt backend response/SSE to Tianji response
```

## Repo Evidence

- `internal/provider/openai/openai.go` hardcodes default Platform base URL and `/chat/completions`.
- `internal/proxy/handler/openai_subscription_resolution.go` currently has `direct_openai_http` and `codex_app_server` transport enum values, but no ChatGPT backend HTTP transport.
- `internal/proxy/handler/openai_subscription_routing.go` already centralizes candidate selection, disabled/failure handling, sticky/lowest-utilization, and rate-limit gates.
- `internal/config/openai_oauth.go` has OAuth authorize defaults and `Originator`, but its default is `tianjillm`, not HO-1288's required `codex_cli_rs` backend default.
- Existing `internal/codexapp/auth.go` covers app-server login payloads, but HO-1288 is not an app-server login issue.
- `internal/testutil/openaitest/guard_transport.go` already blocks real `api.openai.com` test calls; HO-1288 should add equivalent protection/coverage for ChatGPT backend tests.

## External Evidence

- Official OpenAI API docs for current Codex/API models describe API usage through the Responses API endpoint `/v1/responses`; they do not document `https://chatgpt.com/backend-api/codex/responses` as a public Platform endpoint.
- Official OpenAI docs list Codex models as Responses-capable, reinforcing that the normal public API path remains `/v1/responses` / `/v1/chat/completions`, separate from this ChatGPT subscription backend transport.
- Local OpenClaw provider evidence uses `https://chatgpt.com/backend-api/codex/responses`, `Authorization: Bearer <access_token>`, `ChatGPT-Account-Id`, `originator`, and `OpenAI-Beta: responses=experimental` for ChatGPT-backed Codex traffic.
- Local memory plugin evidence normalizes backend base URL to `/codex/responses` and tests that OAuth mode hits `https://chatgpt.com/backend-api/codex/responses`.

## Risk Register

| Risk | Mitigation |
|---|---|
| Accidentally changes API-key OpenAI provider | Add regression test for API-key `/v1/chat/completions` URL and custom `api_base`. |
| Subscription token still goes to Platform `/v1` | Add mock transport assertion and no-Platform-call guard for selected Codex backend transport. |
| Reusing OAuth `Originator` changes authorize flow | Add separate backend config/default and config tests. |
| Account header casing mismatch | Tests should assert canonical header value via Go's case-insensitive header lookup and include `ChatGPT-Account-Id` in docs. |
| Streaming shape differs from current adapters | Start with RED streaming test; if unsupported in this slice, encode explicit safe unsupported behavior rather than silent broken streaming. |
| Token leakage in errors/logs | Reuse redaction helpers and add sentinel leakage tests. |

## Verification Commands

```bash
go test ./internal/config/... -run 'Test.*Codex.*Backend|Test.*OpenAI.*OAuth' -count=1
go test ./internal/provider/... ./internal/proxy/handler/... -run 'Test.*Codex.*Backend|Test.*OpenAISubscription|Test.*OpenAIProvider' -count=1
go test ./internal/config/... ./internal/provider/... ./internal/proxy/handler/... -count=1
git diff --check origin/main...HEAD
```

## Todo Gate Status

Spec/plan/tasks/analyze are complete when this docs-only branch is pushed and draft PR exists. Implementation remains blocked until Linear moves to `In Progress`.
