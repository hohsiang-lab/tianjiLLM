# OpenClaw via TianjiLLM Gateway

This guide shows how to point OpenClaw model, image, speech, transcription, and memory embedding calls at TianjiLLM as an OpenAI-compatible `/v1` gateway.

Use a TianjiLLM API key in OpenClaw. Do not put OpenAI subscription tokens in OpenClaw config. TianjiLLM resolves the OpenAI subscription credential from its own `model_list` entry and credential store.

## TianjiLLM Model Config

Expose OpenAI models through TianjiLLM model aliases that use subscription credential IDs:

```yaml
model_list:
  - model_name: "gpt-5.2-subscription"
    tianji_params:
      model: "openai/gpt-5.2"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

  - model_name: "gpt-image-2"
    tianji_params:
      model: "openai/gpt-image-2"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

  - model_name: "gpt-image-1.5"
    tianji_params:
      model: "openai/gpt-image-1.5"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

  - model_name: "gpt-4o-transcribe"
    tianji_params:
      model: "openai/gpt-4o-transcribe"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

  - model_name: "gpt-4o-mini-tts"
    tianji_params:
      model: "openai/gpt-4o-mini-tts"
      openai_subscription_credential_ids:
        - "cred-openai-primary"

  - model_name: "text-embedding-3-small"
    tianji_params:
      model: "openai/text-embedding-3-small"
      openai_subscription_credential_ids:
        - "cred-openai-primary"
```

Issue a TianjiLLM virtual key or use a deployment key for OpenClaw. Store that key in OpenClaw as `${TIANJI_OPENCLAW_API_KEY}` or another secret-managed placeholder.

## OpenClaw Provider Config

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
            "id": "gpt-5.2-subscription",
            "name": "GPT-5.2 via TianjiLLM",
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
        "primary": "openai/gpt-5.2-subscription"
      },
      "imageGenerationModel": {
        "primary": "openai/gpt-image-2"
      }
    }
  }
}
```

`models.providers.openai.baseUrl` points at TianjiLLM `/v1`, not directly at OpenAI. `models.providers.openai.apiKey` is the TianjiLLM key that OpenClaw uses to call TianjiLLM.

## Image Generation

Use `openai/gpt-image-2` as the standard image generation default:

```json
{
  "agents": {
    "defaults": {
      "imageGenerationModel": {
        "primary": "openai/gpt-image-2"
      }
    }
  }
}
```

OpenAI's current image generation docs state that `gpt-image-2` does not support transparent backgrounds. For transparent PNG/WebP output, route a transparent-capable model such as `openai/gpt-image-1.5` and request a transparent background with PNG or WebP output in the image call.

```json
{
  "agents": {
    "defaults": {
      "imageGenerationModel": {
        "primary": "openai/gpt-image-2",
        "fallbacks": ["openai/gpt-image-1.5"]
      }
    }
  }
}
```

When invoking transparent image generation, set the model override to `openai/gpt-image-1.5` and use `background: "transparent"` with `outputFormat: "png"` or `outputFormat: "webp"`.

### Reference Image Edits

OpenClaw sends reference-image edits either through `/v1/images/edits` multipart uploads or through a Codex Responses payload with `input_image` parts. For Responses payloads, OpenAI expects `image_url` to be a string:

```json
{
  "type": "input_image",
  "image_url": "data:image/png;base64,...",
  "detail": "auto"
}
```

TianjiLLM also accepts legacy Chat Completions-style `image_url` objects such as `{"url":"https://example.test/reference.png","detail":"high"}` at the OpenAI-compatible boundary, but it normalizes forwarded Codex Responses payloads to the string-shaped `image_url` form.

The same validation boundary also covers image references carried forward in Responses conversation payloads, including prior output items under `input[].output[]` and WebSocket `previous_response_id` expansions. Malformed output image data URLs, such as values missing the `;base64,` separator, fail locally with an actionable 400 before Tianji calls the ChatGPT Codex backend.

When an OpenAI subscription model is configured to use the ChatGPT Codex backend transport, TianjiLLM also removes Responses parameters that the Codex backend rejects even though they are valid for the public OpenAI Responses API. This Codex-only normalization strips `max_output_tokens`, `max_tokens`, and `temperature` before forwarding the request. Standard OpenAI API-key and direct subscription `/v1/responses` routing are not changed by this compatibility behavior.

## Text To Speech

```json
{
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
  }
}
```

TianjiLLM routes this to `/v1/audio/speech` when the model alias exists in `model_list`.

## Speech To Text

```json
{
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
            },
            "capabilities": ["transcription"]
          }
        ]
      }
    }
  }
}
```

TianjiLLM routes this to `/v1/audio/transcriptions`. `whisper-1` can be used as a compatibility fallback when configured as a TianjiLLM model alias.

## Embeddings And Memory Search

```json
{
  "agents": {
    "defaults": {
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
  }
}
```

This uses TianjiLLM `/v1/embeddings`. Keep the vector model ID aligned with a TianjiLLM `model_list` alias that uses an OpenAI subscription credential.

## Surface Compatibility

| OpenClaw surface | Can use TianjiLLM `/v1` directly? | Notes |
| --- | --- | --- |
| Chat and reasoning model calls | Yes | Configure `models.providers.openai.baseUrl` and `agents.defaults.model`. |
| Responses API model calls | Yes, when the client uses OpenAI-compatible `/v1/responses` | TianjiLLM proxies `/v1/responses` for configured OpenAI models. |
| Embeddings and memory search | Yes | Configure `agents.defaults.memorySearch.remote.baseUrl`. |
| Image generation/edit endpoints | Yes | Configure image models in TianjiLLM and OpenClaw model provider defaults. |
| Transparent image output | Yes, with a transparent-capable image model | Use `gpt-image-1.5` rather than `gpt-image-2` for transparent background requests. |
| Text to speech | Yes | Configure `messages.tts.providers.openai.baseUrl`. |
| Speech to text | Yes | Configure `tools.media.audio.models[].baseUrl` and bearer header. |
| OpenClaw plugins/connectors | No | Use the native plugin or connector auth/config for each integration. |
| Browser, GitHub, Discord, Linear, calendar, email, and device tools | No | These are not OpenAI-compatible `/v1` model endpoints. |
| ChatGPT backend or Codex profile integrations using non-`/v1` protocols | No | Configure those through their native OpenClaw/provider settings. |

## Troubleshooting OpenClaw Calls

- `401` from TianjiLLM: verify `${TIANJI_OPENCLAW_API_KEY}` and TianjiLLM virtual key permissions first.
- `401` or refresh failure from upstream OpenAI: test and refresh the TianjiLLM credential in `/ui/credentials`.
- `no credential configured`: add `openai_subscription_credential_ids` to the TianjiLLM model alias that OpenClaw is calling.
- `all credentials disabled`: reconnect or select a healthy credential in TianjiLLM, then update the model alias.
- OpenClaw surface has no `baseUrl` or provider model setting: it cannot be routed through TianjiLLM by OpenAI-compatible config alone; use native OpenClaw config for that surface.
