# Contract: Chat to Codex Responses payload

## Valid Minimal Request

Input:

```json
{
  "model": "chatgpt/gpt-5.2-codex",
  "messages": [{ "role": "user", "content": "hello" }]
}
```

Output:

```json
{
  "model": "gpt-5.2-codex",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [{ "type": "input_text", "text": "hello" }]
    }
  ]
}
```

## Instruction Mapping

Leading `system` and `developer` messages are transformed into top-level `instructions`。

Rules:

- Preserve order。
- Prefix each segment with original role。
- Do not merge instruction text into user message content。
- Test non-leading `system` / `developer` behavior explicitly。

## Compatible Options

Mapper may preserve:

- `max_tokens` / `max_completion_tokens`
- `temperature`
- `top_p`
- `stream=true` is currently unsupported for the HO-1288 route and must return deterministic no-upstream diagnostic
- `metadata`
- `user`, only as safe metadata/client correlation

## Unsupported Fields

Fatal by default:

- `tools`
- `tool_choice`
- assistant `tool_calls`
- `tool` role messages
- `n > 1`
- `logprobs`
- `top_logprobs`
- unsupported `response_format`
- unknown `ExtraParams`

Implementation may downgrade a field to non-fatal ignore only when tests prove this cannot corrupt backend semantics and diagnostics record the omission。

## Non-Goals

- No HTTP request creation。
- No credential lookup。
- No Authorization header handling。
- No real network tests。
- No normal OpenAI provider behavior changes。
- No streaming response adapter。
