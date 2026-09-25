# Quickstart Draft: OpenAI subscription OAuth and OpenClaw gateway

This quickstart is a planning artifact. Final docs will be implemented after Linear moves to `In Progress`.

## TianjiLLM OAuth setup

```yaml
general_settings:
  openai_oauth:
    enabled: true
    redirect_uri: http://localhost:1455/auth/callback

model_list:
  - model_name: gpt-5.5-subscription
    tianji_params:
      model: openai/gpt-5.5
      openai_subscription_credential_ids:
        - cred-openai-subscription-primary
```

Rules:

- Do not set `tianji_params.api_base` on a model that uses `openai_subscription_credential_ids`.
- Credential ids must be non-empty and unique.
- The subscription token is stored by TianjiLLM credential storage, not in this YAML.

## Admin lifecycle

1. Sign in to TianjiLLM admin UI.
2. Open `/ui/credentials`.
3. Use OpenAI Connect for the target organization.
4. Complete OpenAI authorization. For localhost callback flows, paste the callback URL back into TianjiLLM when the browser cannot directly reach the service.
5. Use credential detail actions:
   - Test: verify routeability.
   - Refresh: renew expired or near-expired token material.
   - Disable: stop routing through a credential without deleting history.
   - Delete: remove the credential from future use.

## OpenClaw gateway config

Use TianjiLLM as an OpenAI-compatible `/v1` gateway. `${TIANJI_OPENCLAW_API_KEY}` is a TianjiLLM API key issued for OpenClaw, not an OpenAI subscription token.

```json
{
  "models": {
    "providers": {
      "openai": {
        "api": "openai-completions",
        "baseUrl": "https://tianji.example.com/v1",
        "apiKey": "${TIANJI_OPENCLAW_API_KEY}",
        "models": [
          {
            "id": "gpt-5.5",
            "name": "GPT-5.5 via TianjiLLM",
            "reasoning": true
          },
          {
            "id": "gpt-image-2",
            "name": "GPT Image 2 via TianjiLLM"
          },
          {
            "id": "gpt-image-1.5",
            "name": "GPT Image 1.5 via TianjiLLM"
          }
        ]
      }
    }
  },
  "agents": {
    "defaults": {
      "model": {
        "primary": "openai/gpt-5.5"
      },
      "imageGenerationModel": {
        "primary": "openai/gpt-image-2"
      },
      "memorySearch": {
        "enabled": true,
        "provider": "openai",
        "model": "text-embedding-3-small",
        "remote": {
          "baseUrl": "https://tianji.example.com/v1",
          "apiKey": "${TIANJI_OPENCLAW_API_KEY}"
        }
      }
    }
  },
  "messages": {
    "tts": {
      "providers": {
        "openai": {
          "baseUrl": "https://tianji.example.com/v1",
          "apiKey": "${TIANJI_OPENCLAW_API_KEY}",
          "model": "gpt-4o-mini-tts",
          "voice": "alloy"
        }
      }
    }
  },
  "tools": {
    "media": {
      "audio": {
        "models": [
          {
            "type": "provider",
            "provider": "openai",
            "model": "gpt-4o-transcribe",
            "baseUrl": "https://tianji.example.com/v1",
            "headers": {
              "Authorization": "Bearer ${TIANJI_OPENCLAW_API_KEY}"
            }
          }
        ]
      }
    }
  }
}
```

Transparent PNG/WebP generation should use model override `openai/gpt-image-1.5` with transparent background output. Standard image generation can default to `openai/gpt-image-2`.

## Surface compatibility

Can use TianjiLLM `/v1` directly:

- Chat and reasoning model calls through OpenAI-compatible chat completions.
- Responses and embeddings when TianjiLLM exposes the matching endpoint.
- Image generation/edit/variation through OpenAI-compatible image endpoints.
- TTS through `/v1/audio/speech`.
- STT through `/v1/audio/transcriptions`.

Still requires native OpenClaw provider auth/config:

- OpenClaw plugins/connectors that do not speak OpenAI-compatible `/v1`.
- Browser, Discord, GitHub, Linear, calendar, email, and other non-model integrations.
- ChatGPT backend or Codex profile integrations that use non-`/v1` protocols.

## Troubleshooting quick checks

- TianjiLLM returns 401 before reaching OpenAI: check OpenClaw/TianjiLLM API key.
- Upstream OpenAI auth fails after refresh: check credential status and run Refresh/Test from `/ui/credentials`.
- No credential configured: add `openai_subscription_credential_ids` to the model config.
- All credentials disabled: re-enable a healthy credential or connect a new one.
- Refresh failed: reconnect the OpenAI account and disable the failed credential until verified.
