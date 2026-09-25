# Quickstart: HO-1289 chat to Codex Responses payload

## Example Input

```json
{
  "model": "chatgpt/gpt-5.2-codex",
  "messages": [
    { "role": "system", "content": "You are concise." },
    { "role": "developer", "content": "Follow repo conventions." },
    { "role": "user", "content": "Summarize this diff." }
  ],
  "max_tokens": 800,
  "temperature": 0.2
}
```

## Expected Mapper Output

```json
{
  "model": "gpt-5.2-codex",
  "instructions": "system: You are concise.\ndeveloper: Follow repo conventions.",
  "input": [
    {
      "type": "message",
      "role": "user",
      "content": [
        { "type": "input_text", "text": "Summarize this diff." }
      ]
    }
  ],
  "max_output_tokens": 800,
  "temperature": 0.2
}
```

## Unsupported Field Example

```json
{
  "model": "chatgpt/gpt-5.2-codex",
  "messages": [{ "role": "user", "content": "hello" }],
  "tools": [{ "type": "function", "function": { "name": "x" } }]
}
```

Expected result: no upstream request；return safe diagnostic，例如：

```json
{
  "field": "tools",
  "code": "unsupported_field",
  "fatal": true
}
```

## Verification Commands

```bash
go test ./internal/provider/chatgptcodex ./internal/proxy/handler -run 'Test.*Codex.*Payload|Test.*ChatGPTCodex.*|Test.*OpenAI.*Chat' -count=1
go test ./internal/model/... ./internal/provider/... ./internal/proxy/handler/... -count=1
go tool golangci-lint run ./internal/provider/... ./internal/proxy/handler/...
git diff --check origin/main...HEAD
```

## Manual Verification Boundary

No manual verification should call real `chatgpt.com` or `api.openai.com`。Acceptance evidence comes from mapper unit tests, merged HO-1288 mocked transport tests, streaming unsupported/no-upstream tests, and normal OpenAI provider regression tests。
