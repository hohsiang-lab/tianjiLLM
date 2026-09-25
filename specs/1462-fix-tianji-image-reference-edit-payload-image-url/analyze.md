# Analysis: HO-1462 Fix Tianji image reference edit payload image_url type

## Gate Result

- Fatal: 0
- Critical: 0
- Owner input needed: 0

## Evidence Checked

- Linear HO-1462 moved out of Todo before production implementation.
- Linear project `TianjiLLM` maps to `hohsiang-lab/tianjiLLM`.
- Repo baseline: `origin/main` at `dc3166a` when this docs-only worktree was created.
- `docs/openclaw-tianjillm-gateway.md` documents `gpt-image-2` image generation/edit routing through Tianji.
- `internal/proxy/handler/responses.go` owns `/v1/responses` routing and delegates ChatGPT Codex backend requests to `BuildResponsesRequest`.
- `internal/provider/chatgptcodex/payload.go` maps image content to `ImageURL *model.ImageURL`, which serializes `image_url` as an object.
- `internal/provider/chatgptcodex/transport.go` maps multipart `/v1/images/edits` images into object-shaped `image_url`, so that route is in scope.
- OpenClaw's Codex OAuth image generator sends `input_image` with string `image_url` and sibling `detail`.
- OpenAI Images/Vision docs show Responses `input_image` content with string `image_url`.

## Root Cause Chain

OpenClaw reference image edit enters Tianji as a Responses/image-generation request; Tianji's subscription/Codex payload rebuild path represents image references with Chat Completions-style `image_url.url` object shape; OpenAI Responses validates `input[].content[].image_url` as a string for `input_image`; upstream rejects the request with HTTP 400 before image edit execution.

## Scope Decision

The implementation scope should be a narrow payload compatibility fix:

- Normalize official and legacy image content inputs at Tianji's payload boundary.
- Ensure outbound OpenAI Responses `input_image.image_url` is string-shaped.
- Preserve `detail` in the supported upstream location.
- Add outbound body-shape regression tests.
- Leave model defaults, credentials, pricing, endpoint routing, and OpenClaw config unchanged.
- Preserve existing audit redaction for data URLs after changing `image_url` from object to string.

## Risks

- Data URLs may be large; tests and logs must avoid leaking full real user image data.
- Legacy callers may rely on object-shaped `image_url`; accepting and normalizing that shape is safer than hard rejection.
- Raw proxy paths may forward malformed client input unchanged; implementation should distinguish pass-through behavior from payload-rebuild behavior and only normalize where Tianji owns the outbound shape.

## Scope Confirmation

Implementation is cleared after Linear moved out of Todo. No OpenClaw client changes are needed.
