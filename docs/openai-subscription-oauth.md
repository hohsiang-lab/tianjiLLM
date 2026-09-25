# OpenAI Subscription OAuth Admin Guide

This guide documents how to operate TianjiLLM OpenAI subscription OAuth credentials. The security boundary is strict: OpenAI subscription token material is created, refreshed, stored, and redacted by TianjiLLM. Clients such as OpenClaw authenticate to TianjiLLM with a TianjiLLM API key and never receive subscription tokens.

## Prerequisites

- TianjiLLM admin UI access.
- `general_settings.master_key` or a virtual key for downstream clients.
- PostgreSQL configured when credentials must be saved.
- Cache configured for OAuth state storage during the Connect flow.
- At least one TianjiLLM organization. `/ui/credentials` disables Connect when no organization exists.
- An OpenAI account that can authorize the subscription flow and access the target models.

## Minimal Config

```yaml
model_list:
  - model_name: "gpt-5.2-subscription"
    tianji_params:
      model: "openai/gpt-5.2"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

general_settings:
  master_key: "$PROXY_MASTER_KEY"
  database_url: "$DATABASE_URL"
  port: 4000
  openai_oauth:
    enabled: true
    redirect_uri: "http://localhost:1455/auth/callback"
```

`openai_subscription_credential_ids` selects credential records saved by TianjiLLM. Do not put an OpenAI access token, refresh token, ID token, or subscription cookie in this file.

## OAuth Config

`general_settings.openai_oauth` supports these fields:

| Field | Default | Notes |
| --- | --- | --- |
| `enabled` | `false` when omitted | Deployment intent flag. Set it to `true` for environments where admins should use OpenAI Connect. |
| `issuer_url` | `https://auth.openai.com` | Base issuer URL used to derive authorize/token URLs when they are not overridden. |
| `authorize_url` | `https://auth.openai.com/oauth/authorize` | Absolute authorization endpoint. |
| `token_url` | `https://auth.openai.com/oauth/token` | Absolute token exchange and refresh endpoint. |
| `redirect_uri` | `http://localhost:1455/auth/callback` | Callback URL registered into the authorization request. |
| `client_id` | TianjiLLM public OpenAI OAuth app ID | Public client ID; no client secret is required. |
| `scopes` | `openid`, `profile`, `email`, `offline_access`, `api.connectors.read`, `api.connectors.invoke` | Space-joined in the authorize URL. |
| `originator` | `tianjillm` | Added to the authorize URL. |

TianjiLLM also sends PKCE `S256`, `id_token_add_organizations=true`, and `codex_cli_simplified_flow=true` in the authorize URL.

## Callback URL Rules

The redirect URI must be an absolute `http` or `https` URL. HTTP is allowed only for localhost-style hosts: `localhost`, `127.0.0.1`, or `::1`. Localhost HTTP callbacks must use `/auth/callback` and must not include query strings or fragments.

Use the default `http://localhost:1455/auth/callback` when the OpenAI authorization tab runs on the same operator machine. Use an HTTPS callback for hosted deployments where the browser can reach TianjiLLM directly.

## Browser Callback Fallback Flow

1. Sign in to TianjiLLM admin UI.
2. Open `/ui/credentials`.
3. Ensure an organization exists; the browser callback fallback needs an organization context.
4. Click **Browser callback fallback**.
5. Complete OpenAI authorization in the new tab.
6. TianjiLLM consumes the one-time OAuth state, exchanges the code with PKCE, stores the credential, and renders a success or failure page.
7. Return to `/ui/credentials` and use **Test** before assigning the credential to production model routes.

The OAuth state is one-time use and expires after 10 minutes. Restart from `/ui/credentials` after expiry, replay, or a mismatched state error.

## Device-Code Connect (Primary)

The admin UI uses OpenAI Codex device authorization before the browser-callback flow:

1. Open `/ui/credentials` and click **Sign in with Device Code**.
2. Click **Get device code** in the dialog to start authorization.
3. Open the displayed verification URL on another device and enter the displayed user code.
4. Keep the TianjiLLM dialog open; it polls the server-side operation status using the provider interval.
5. On completion, TianjiLLM exchanges the provider authorization code with PKCE, saves the encrypted credential, and stops polling.
6. Use **Cancel** or restart after expiry, denial, or a terminal provider error.

The operation ID is opaque and session-bound. Device IDs, authorization codes, PKCE values, access tokens, and refresh tokens are never rendered in the page, URL, or logs. A shared cache with distributed locking is required for every device-code start, including single-instance deployments; memory-only caches are not supported.

If device authorization is unavailable, **Paste callback URL** remains available as the compatibility fallback described below.


For local callback flows, the OpenAI tab may land on `http://localhost:1455/auth/callback?...` while TianjiLLM is not reachable on that operator machine. In that case:

1. Copy the full localhost callback URL from the OpenAI tab.
2. Return to `/ui/credentials`.
3. Click **Paste callback URL**.
4. Paste the full URL and submit.

TianjiLLM accepts only the configured callback scheme, host, and path. The pasted URL must contain the original `state` and either `code` or an OpenAI provider error.

## Credential Lifecycle

| Action | What it does | Operator use |
| --- | --- | --- |
| Connect | Creates a new `openai_subscription` credential from the OAuth callback. | Add a credential for an organization. |
| Test | Calls OpenAI `/models` with the credential bearer token and returns a model count. | Verify the credential is usable before routing traffic. |
| Refresh | Forces token refresh through the configured `token_url`. | Recover near-expired or expired token material. |
| Disable | Marks credential metadata as disabled with reason `operator_disabled`. | Stop routing without deleting history. |
| Delete | Removes the credential row through the generic credential delete path. | Remove a credential that should no longer be selectable. |

Lifecycle responses and audit logs use reason codes and redacted metadata. UI pages must not render stored token material, raw authorization codes, bearer strings, or credential payloads.

## Model Routing

Reference connected credentials from `model_list[].tianji_params.openai_subscription_credential_ids`:

```yaml
model_list:
  - model_name: "gpt-5.2-subscription"
    tianji_params:
      model: "openai/gpt-5.2"
      openai_subscription_credential_ids:
        - "cred-openai-primary"
        - "cred-openai-backup"
```

Rules enforced by config and UI validation:

- Credential IDs must be non-empty.
- Duplicate credential IDs are rejected.
- Subscription credentials support official OpenAI models only.
- Do not combine subscription credentials with a custom `api_base`. The default OpenAI API base is the only valid base for subscription routing.
- If all selected credentials are missing, wrong type, disabled, malformed, expired, or refresh-failed, TianjiLLM does not fall back to the static API key for that model.

Subscription routing covers OpenAI-compatible requests that TianjiLLM proxies to OpenAI, including chat completions, responses, embeddings, image generation/edit/variation, audio transcriptions, and audio speech.

For Codex backend routing, enable sticky selection and tune the usage gates at the top level:

```yaml
native_upstream_strategy: sticky
ratelimit_alert_threshold: 0.8
codex_usage_weekly_threshold: 0.85
```

`ratelimit_alert_threshold` is the primary / 5h gate used by existing rate-limit alerting and Codex selection. `codex_usage_weekly_threshold` is the weekly / 7d secondary gate for Codex usage selection. When `codex_usage_weekly_threshold` is omitted or set to `0`, TianjiLLM keeps the historical default of `0.9`.

## Security Boundary

- TianjiLLM owns OAuth exchange, refresh, credential storage, redaction, and routing.
- OpenClaw and other downstream clients authenticate to TianjiLLM with a TianjiLLM API key.
- Do not copy OpenAI subscription token material into OpenClaw, shell env examples, PR comments, docs, or issue threads.
- Do not use browser scraping or a client secret for this flow.
- Use placeholders such as `$PROXY_MASTER_KEY`, `$DATABASE_URL`, and `${TIANJI_OPENCLAW_API_KEY}` in examples.

## Troubleshooting

| Symptom | Likely cause | Checks | Operator action |
| --- | --- | --- | --- |
| TianjiLLM returns `401` before an upstream request | Downstream client is using the wrong TianjiLLM key | Check `Authorization: Bearer ...`, virtual key state, and `general_settings.master_key` | Fix the TianjiLLM API key before debugging OpenAI OAuth. |
| Upstream OpenAI returns `401` after refresh | Credential token is invalid, revoked, or cannot refresh | Open `/ui/credentials/{id}`, run **Refresh**, then **Test** | Reconnect the OpenAI account when refresh/test fails; disable the failed credential until verified. |
| Credential status is `refresh_failed` | Token refresh failed and metadata recorded a redacted error | Credential detail `Last error`, audit logs, refresh action result | Disable the failed credential, reconnect, then verify **Test** succeeds. |
| All selected credentials are disabled | Model route references only disabled credentials | Model config IDs and credential detail status | Reconnect a healthy credential or remove disabled IDs from the model route. |
| No credential configured | Model has no `openai_subscription_credential_ids` | `model_list[].tianji_params` for the model alias | Add at least one connected subscription credential ID to the model config. |
| Credential missing or wrong type | Model references a deleted credential or a non-subscription credential | Credential ID in model config, `/ui/credentials` list | Remove the invalid ID and select a current `openai_subscription` credential. |
| Redirect URI rejected | Callback URL is not absolute, uses HTTP off localhost, has a wrong localhost path, or includes query/fragment | `general_settings.openai_oauth.redirect_uri` and startup/config validation | Use HTTPS for hosted deployments, or `http://localhost:1455/auth/callback` for local paste flow. |
| Callback expired, replayed, or state mismatched | OAuth state expired after 10 minutes or was already consumed | Failure page text and cache availability | Start Connect again from `/ui/credentials`; do not reuse old callback URLs. |
| Connect fails with state or save dependency error | Cache cannot store state or DB cannot save the credential | Cache health, DB connection, server logs | Restore cache/DB, then restart Connect. |
| OpenClaw request fails on one surface but chat works | That OpenClaw surface may not use OpenAI-compatible `/v1` | Compare the surface against `docs/openclaw-tianjillm-gateway.md` | Route only compatible model/media/memory surfaces through TianjiLLM; use native OpenClaw provider config for others. |

## Verification

Use the HO-1192 docs contract to run diff, forbidden-secret, and coverage checks before moving the PR out of review. The forbidden-secret check should scan the implementation docs and HO-1192 artifacts while excluding the contract file that documents the check itself. The coverage check should confirm every required config surface and model keyword is present.
