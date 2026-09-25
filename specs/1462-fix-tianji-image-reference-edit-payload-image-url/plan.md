# Implementation Plan: HO-1462 Fix Tianji image reference edit payload image_url type

## Technical Context

- Repository: `hohsiang-lab/tianjiLLM`
- Language/runtime: Go
- Affected surfaces:
  - `internal/provider/chatgptcodex/payload.go`
  - `internal/provider/chatgptcodex/transport.go`
  - `internal/proxy/handler/responses.go`
  - `internal/proxy/handler/images.go`
  - `internal/proxy/handler/responses_codex_test.go`
  - `internal/proxy/handler/chatgpt_codex_wildcard_test.go`
  - `docs/openclaw-tianjillm-gateway.md`
- External contract: OpenAI Responses image input uses `input_image.image_url` as a string.

## Constraints

- Linear moved out of Todo before production implementation.
- Keep change narrow to payload normalization and regression coverage.
- Preserve OpenAI subscription credential resolution/failover behavior.
- Preserve existing endpoint proxy behavior except for the proven `/v1/images/edits` Codex Responses rebuild shape.

## Current Repo Findings

- `internal/proxy/handler/responses.go` reads Responses request JSON into `map[string]any`; raw OpenAI route can proxy unchanged, while ChatGPT Codex backend route rebuilds a backend request via `BuildResponsesRequest`.
- `internal/provider/chatgptcodex/payload.go` currently models outbound image content as `ImageURL *model.ImageURL json:"image_url"`, which serializes as an object.
- `mapAnyContentPart` currently accepts only object-shaped `image_url`, not official string-shaped `image_url`.
- `internal/provider/chatgptcodex/transport.go` currently rebuilds multipart `/v1/images/edits` into object-shaped `image_url`.
- OpenClaw's Codex OAuth image generator sends `input_image` with string `image_url` and sibling `detail`.

## Proposed Design

1. Introduce a Responses-compatible image content representation for outbound upstream payloads where:
   - `type` remains `input_image`.
   - `image_url` serializes as a string.
   - `detail` serializes beside `image_url` when present.
2. Normalize accepted client inputs:
   - Official `{"type":"input_image","image_url":"https://..."}`.
   - Legacy `{"type":"image_url","image_url":{"url":"https://...","detail":"high"}}`.
   - Legacy `{"type":"input_image","image_url":{"url":"https://...","detail":"high"}}`.
3. Reject malformed image content locally when rebuilding payloads:
   - missing `image_url`,
   - non-string URL,
   - object without string `url`,
   - unsupported content part type.
4. Add tests that inspect the actual outbound request body sent by Tianji after subscription routing/failover, not just unit-level structs.
5. Update docs to clarify the OpenClaw/Tianji reference-image compatibility boundary.
6. Keep audit payload redaction safe for string-shaped data URLs.

## Verification Plan

- `go test ./internal/provider/chatgptcodex -run 'Payload|Responses|Image' -count=1`
- `go test ./internal/proxy/handler -run 'OpenAISubscriptionRouting.*Image|OpenAISubscriptionRouting.*Responses|CreateResponse|ChatGPTCodexBackendImage' -count=1`
- `go test ./internal/proxy/handler -run 'OpenAISubscription' -count=1`
- `go test ./internal/provider/chatgptcodex ./internal/proxy/handler -count=1`
- `git diff --check origin/main...HEAD`

## Rollout Notes

- This is behavior-preserving for callers that already send official string-shaped image references.
- Legacy object-shaped callers should continue to work at Tianji's boundary but receive official upstream shape.
- The implementation should avoid logging image URL/data URL contents beyond existing request logging behavior.

## Out of Scope

- OpenClaw client changes.
- New image upload/file hosting feature.
- Transparent background fallback changes.
- Pricing config changes.
- OAuth/credential lifecycle changes.
