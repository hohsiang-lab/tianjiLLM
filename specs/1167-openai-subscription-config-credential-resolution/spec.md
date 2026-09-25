# Feature Specification: OpenAI Subscription Config and Credential Resolution

**Feature Branch**: HO-1167-openai-subscription-config-credential-resolution
**Created**: 2026-05-07
**Input**: Linear HO-1167 - `[BE] OpenAI subscription config + credential resolution`

## Summary

Add explicit per-model OpenAI subscription credential configuration and resolve those credentials without changing existing API-key behavior. Credential application must split by runtime transport: direct OpenAI HTTP paths receive a resolved bearer credential boundary for later injection work, while Codex app-server paths hand account credentials to app-server through its account login JSON-RPC surface.

## Scope

- Add `model_list[].tianji_params.openai_subscription_credential_ids: []`.
- Require explicit subscription credential IDs; never auto-consume every organization credential.
- Preserve existing `api_key` and `$OPENAI_API_KEY` interpolation behavior when no subscription credential IDs are configured.
- Reject configs that combine subscription credential IDs with custom `api_base`.
- Apply subscription credentials only to the official OpenAI provider with the default OpenAI base URL.
- Return explicit errors when configured subscription credential IDs are missing, invalid, disabled, expired without refresh, or unusable.
- Do not silently fallback to `api_key` after subscription credential IDs are configured.
- Split credential application by transport:
  - Direct OpenAI HTTP paths resolve subscription credentials as bearer-auth material for later route/injection tickets.
  - Codex app-server paths do not use HTTP bearer injection. They resolve and refresh the selected OpenAI subscription/Codex profile, then deliver account credentials through `account/login/start` with `chatgptAuthTokens`.
  - Local stdio Codex app-server launches strip `CODEX_API_KEY` and `OPENAI_API_KEY` from the spawned app-server environment when a subscription-style profile is selected.
  - WebSocket Codex app-server connection auth remains `appServer.authToken`/headers only; OpenAI account credentials still go through app-server login RPC.

## Out of Scope

- Production implementation while Linear is Todo.
- Starting production implementation while Linear is not In Progress.
- UI work for selecting subscription credentials.
- Adding new credential storage tables. HO-1176 already owns encrypted `openai_subscription` persistence.
- Redaction of token/JWT sinks beyond the HO-1183 shared redaction behavior.
- Real OpenAI network calls in tests.
- Direct Codex app-server vendoring or protocol upgrades beyond what implementation needs to support the selected checked version.

## User Stories

### US1 - Configure explicit OpenAI subscription credentials

As an operator, I can bind a model deployment to specific OpenAI subscription credential IDs so the proxy never guesses which account to use.

**Acceptance Scenarios**

1. Given a model config has `openai_subscription_credential_ids: ["cred_a"]`, when config loads, then the field is parsed as a typed list on `TianjiParams`.
2. Given a model config has no subscription IDs, when config loads, then existing `api_key` and `$OPENAI_API_KEY` behavior remains unchanged.
3. Given subscription IDs are configured with `api_base`, when config loads, then validation fails with a clear config error.
4. Given subscription IDs are configured on a non-OpenAI or custom OpenAI-compatible provider, when config loads or resolves, then validation rejects the configuration.

### US2 - Resolve credentials explicitly at runtime

As a request router, I can resolve the selected model's configured OpenAI subscription credential and report unusable credentials without falling back to API keys.

**Acceptance Scenarios**

1. Given a selected deployment has configured subscription credential IDs, when resolution runs, then only those IDs are considered.
2. Given a configured ID does not exist or is not `openai_subscription`, when resolution runs, then the request fails with an explicit credential-resolution error.
3. Given configured subscription credentials are unusable, when resolution fails, then no `api_key` fallback is attempted.
4. Given no subscription IDs are configured, when resolution runs, then existing API-key provider behavior remains unchanged.

### US3 - Apply credentials by transport

As an app-server integration, I can use OpenAI subscription credentials without corrupting Codex auth mode or letting environment API keys override the selected account.

**Acceptance Scenarios**

1. Given the route is a direct OpenAI HTTP path, when a selected deployment resolves subscription credentials, then the output is bearer-auth material for the later HTTP injection boundary.
2. Given the route is a Codex app-server path, when a selected deployment resolves subscription credentials, then the app-server login handoff uses `account/login/start` with `chatgptAuthTokens`, not HTTP bearer injection.
3. Given local stdio app-server is launched for a subscription-style profile, when the process environment is built, then `CODEX_API_KEY` and `OPENAI_API_KEY` are omitted.
4. Given WebSocket app-server is used, when the socket connects, then `appServer.authToken`/headers authenticate only the app-server connection and OpenAI account credentials are still delivered by login RPC.

## Functional Requirements

- **FR-001**: `config.TianjiParams` MUST add `OpenAISubscriptionCredentialIDs []string` with YAML key `openai_subscription_credential_ids`.
- **FR-002**: The config loader MUST parse an omitted field as empty and an explicit YAML list as the exact configured IDs.
- **FR-003**: Validation MUST reject empty strings and duplicate IDs in `openai_subscription_credential_ids`.
- **FR-004**: Validation MUST reject `openai_subscription_credential_ids` when `api_base` is non-empty or points away from the official OpenAI default base URL.
- **FR-005**: Validation MUST reject subscription IDs for provider prefixes other than `openai` or bare OpenAI-default model names.
- **FR-006**: Existing `api_key`, `api_base`, `api_version`, and environment interpolation MUST stay unchanged when subscription IDs are empty.
- **FR-007**: Runtime resolution MUST look up only the configured credential IDs and MUST NOT scan all organization credentials.
- **FR-008**: Runtime resolution MUST require `credential_type = "openai_subscription"` and safe status metadata compatible with HO-1176/HO-1183.
- **FR-009**: Runtime resolution MUST refresh or reject expired credentials according to the available OpenAI subscription credential helper contract; it MUST NOT use stale access tokens silently.
- **FR-010**: Runtime resolution MUST return an explicit sanitized error when a configured credential is missing, disabled, expired, malformed, or has no usable account token bundle.
- **FR-011**: If subscription IDs are configured, failure MUST NOT fallback to `api_key` or `$OPENAI_API_KEY`.
- **FR-012**: Direct OpenAI HTTP resolution MUST expose a typed resolved credential object that later injection code can turn into `Authorization: Bearer ...`.
- **FR-013**: Codex app-server resolution MUST create a login RPC payload using `account/login/start` and `chatgptAuthTokens` for the selected account credential.
- **FR-014**: Codex app-server resolution MUST NOT apply subscription credentials as raw HTTP bearer headers to the app-server connection.
- **FR-015**: Local stdio app-server env construction MUST remove `CODEX_API_KEY` and `OPENAI_API_KEY` when the selected profile uses subscription credentials.
- **FR-016**: WebSocket app-server connection auth MUST keep app-server auth separate from OpenAI account auth.
- **FR-017**: Tests MUST cover config load, validation failures, API-key regression, direct HTTP resolution, Codex app-server login RPC selection, local env stripping, and WebSocket auth separation.

## Edge Cases

- `openai_subscription_credential_ids: []` behaves the same as omitted.
- `api_base: ""` is treated as default OpenAI base URL; any non-empty custom base is rejected with subscription IDs.
- Bare model names without provider prefix follow current OpenAI-default parsing behavior.
- Multiple configured IDs are preserved in configured order; selection strategy belongs to the resolver implementation, not YAML parsing.
- Subscription metadata may mark a credential disabled after refresh failure; resolution must surface that state instead of using API-key fallback.
- App-server external-auth support is version-sensitive; implementation must pin behavior to the checked Codex app-server protocol version.

## Success Criteria

- SpecKit artifacts explain the config field, validation gates, runtime resolver boundary, and app-server transport split.
- Analyze reports zero fatal and zero critical artifact issues.
- No production files are changed during Todo planning; draft PR and Waiting transition are allowed after owner authorization.
