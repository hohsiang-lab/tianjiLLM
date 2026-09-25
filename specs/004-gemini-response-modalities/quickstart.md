# Quickstart: Gemini Response Modalities

**Feature**: 004-gemini-response-modalities

## Prerequisites

- Gemini API key set in `.env` or `proxy_config.yaml`
- A model configured that supports image generation (e.g., `gemini-2.0-flash-exp`)

## 1. Run the tests (no external calls required)

```bash
# Unit tests for the Gemini provider (fast, no API key needed)
go test ./internal/provider/gemini/... -v -run TestTransformResponse

# Run the entire test suite including the new tests for this feature
go test ./internal/provider/gemini/... -v

# All tests
make test
```

## 2. Manual smoke test (requires Gemini API key)

Start the proxy:
```bash
make run
```

### Test: Text-to-Image Generation (User Story 1 — P1)

```bash
curl http://localhost:4000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-master-key>" \
  -d '{
    "model": "gemini/gemini-2.0-flash-exp",
    "messages": [{"role": "user", "content": "Draw a simple red circle."}],
    "modalities": ["text", "image"]
  }'
```

**Expected response** (abridged):
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": [
        {"type": "text", "text": "Here is the red circle:"},
        {"type": "image_url", "image_url": {"url": "data:image/png;base64,..."}}
      ]
    },
    "finish_reason": "stop"
  }]
}
```

### Test: Text-only request still works (User Story 3 — P3, backward compatibility)

```bash
curl http://localhost:4000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-master-key>" \
  -d '{
    "model": "gemini/gemini-2.0-flash",
    "messages": [{"role": "user", "content": "Say hello."}]
  }'
```

**Expected response** — `content` is a plain string, NOT an array:
```json
{
  "choices": [{
    "message": {
      "role": "assistant",
      "content": "Hello! How can I help you?"
    },
    "finish_reason": "stop"
  }]
}
```

### Test: Image-to-Image (User Story 2 — P2)

```bash
# Encode a small test image
IMG_B64=$(base64 -i /path/to/test.jpg | tr -d '\n')

curl http://localhost:4000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <your-master-key>" \
  -d "{
    \"model\": \"gemini/gemini-2.0-flash-exp\",
    \"messages\": [{
      \"role\": \"user\",
      \"content\": [
        {\"type\": \"image_url\", \"image_url\": {\"url\": \"data:image/jpeg;base64,$IMG_B64\"}},
        {\"type\": \"text\", \"text\": \"Make this a sketch.\"}
      ]
    }],
    \"modalities\": [\"text\", \"image\"]
  }"
```

## 3. Key acceptance criteria to verify

| Criterion | How to verify |
|-----------|--------------|
| SC-001: Standard OpenAI SDK works | Response parses with `openai.ChatCompletion` |
| SC-002: Existing text tests pass | `make test` passes without modification |
| SC-003: Image MIME type preserved | Check `data:image/png;...` vs `data:image/webp;...` in URL |
| SC-004: Unit tests cover 3 paths | `go test ./internal/provider/gemini/... -v` shows all 3 test cases |
| SC-005: Text-only has no overhead | No `responseModalities` in upstream Gemini request |

## 4. Debugging

**Check that `responseModalities` is sent upstream**:
Enable request logging in `proxy_config.yaml`:
```yaml
log_request: true
```
Look for `"responseModalities": ["TEXT", "IMAGE"]` in the logged Gemini upstream request.

**Check that `inlineData` parts are parsed**:
Add `-run TestTransformResponse_ImageOutput` to the test command to isolate the
image output parsing test.
