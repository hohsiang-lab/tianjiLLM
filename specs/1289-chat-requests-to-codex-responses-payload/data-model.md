# Data Model: HO-1289 chat to Codex Responses payload

## `model.ChatCompletionRequest`

Existing input model from `internal/model/request.go`。

### HO-1289 usage

- Read `Model` for backend model normalization。
- Read `Messages` for instructions and conversation `input`。
- Read compatible options through explicit allowlist。
- Read `ExtraParams` only to produce unsupported-field diagnostics。

## `CodexResponsesPayload`

Internal output body for HO-1288 backend transport。

### Proposed fields

```go
type CodexResponsesPayload struct {
    Model        string         `json:"model"`
    Instructions string         `json:"instructions,omitempty"`
    Input        []CodexInputItem `json:"input"`
    Stream       *bool          `json:"stream,omitempty"`
    MaxTokens    *int           `json:"max_output_tokens,omitempty"`
    Temperature  *float64       `json:"temperature,omitempty"`
    TopP         *float64       `json:"top_p,omitempty"`
    Metadata     map[string]any `json:"metadata,omitempty"`
}
```

Exact field names may change if implementation verifies a different backend key；behavior must be equivalent and tested。

## `CodexInputItem`

Responses-compatible input item。

### Proposed message shape

```go
type CodexInputItem struct {
    Type    string             `json:"type"` // "message"
    Role    string             `json:"role"`
    Content []CodexContentPart `json:"content"`
}
```

Supported roles:

- `user`
- `assistant`
- `developer` or `system` only for tested non-leading instruction behavior

## `CodexContentPart`

Typed content part。

### Proposed shapes

```json
{ "type": "input_text", "text": "hello" }
{ "type": "input_image", "image_url": "data:image/png;base64,...", "detail": "auto" }
```

Rules:

- Do not stringify arbitrary JSON objects。
- Reject unsupported content with diagnostics。
- Do not include secret/auth material。

## `CodexPayloadDiagnostic`

Structured diagnostic for unsupported or omitted fields。

### Proposed fields

```go
type CodexPayloadDiagnostic struct {
    Field   string
    Code    string
    Message string
    Fatal   bool
}
```

Codes:

- `unsupported_field`
- `unsupported_message_role`
- `unsupported_content_type`
- `unsupported_multi_completion`
- `unsupported_response_format`
- `unsupported_streaming`
- `ignored_field`

Fatal diagnostics block the upstream request。Non-fatal diagnostics may be logged or surfaced safely depending on handler integration。

## Security Rules

- Payload and diagnostics must not include `api_key`、access token、refresh token、credential ID、raw credential JSON、or Authorization header values。
- Mapper should not receive credential objects at all。
