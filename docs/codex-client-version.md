# Codex backend client version

Tianji's Codex catalog client uses its configured compatibility version in both
its `Version` request header and the catalog's `client_version` query. These are
separate from the proxy's per-request protocol headers. For Responses, compact,
image, and WebSocket requests, Tianji mechanically forwards supported client
headers when present; it does not synthesize `Accept`, `OpenAI-Beta`, `Version`,
turn-state, turn-metadata, or routing-hint values. A client-provided
`originator` is forwarded unchanged; if the client omits it, these proxy requests
omit it too. Tianji's separate catalog and usage clients continue to use their
configured originator. Authentication and account selection stay Tianji-controlled.

JSON Responses, compact, and image-generation requests require exactly one
`application/json` Content-Type (including media-type parameters); missing,
unsupported, or duplicate values are rejected. A compatible client Content-Type
is preserved. Image-edit requests require exactly one `multipart/form-data`
Content-Type with its boundary. The outbound Content-Type describes the
serialized body: JSON for Codex Responses and converted image requests. In
particular, an image-edit multipart Content-Type is used to parse the client body
and is not copied onto the re-encoded JSON request.

## Source of truth

- `.codex-client-version` is the candidate version compiled into Tianji.
- `.codex-client-version-reviewed` is the latest version whose upstream changes
  and Tianji compatibility have been reviewed.
- Both files must contain one stable `X.Y.Z` version.
- Production builds are blocked while the two files differ.

The Go fallback remains available for direct local builds, while Make, CI, and
Docker builds inject `.codex-client-version` with `-ldflags`. An explicit
`general_settings.openai_oauth.codex_backend_client_version` YAML value remains
an emergency runtime override.

## Automated update flow

`.github/workflows/codex-client-version.yml` runs daily and can also be
dispatched manually. It reads the latest non-draft, non-prerelease
`rust-vX.Y.Z` release from `openai/codex`.

When the upstream version is newer, the workflow:

1. Updates `.codex-client-version` on the bot-owned
   `automation/codex-client-version` branch.
2. Creates or updates a compatibility review PR.
3. Dispatches CI for the bot branch.
4. Leaves `.codex-client-version-reviewed` unchanged so the migration gate
   remains red until review is complete.

The workflow never downloads a Codex binary or npm package and refuses
automatic version downgrades.

The repository and its organization must allow GitHub Actions to create pull
requests. Keep the default `GITHUB_TOKEN` permission restricted to read, and
grant only the workflow-level `actions`, `contents`, and `pull-requests` write
permissions declared by this updater.

## Migration review

Review the upstream release comparison and check:

- model catalog and minimal client versions;
- Responses HTTP payload and response events;
- Responses WebSocket handshake and sequential turns;
- remote compact request and response behavior;
- image generation request behavior;
- required headers and originator behavior.

Implement any required migration in the version update PR. When the review is
complete, update `.codex-client-version-reviewed` to match
`.codex-client-version`. The `codex-compatibility` CI job runs focused Go tests
with the candidate version injected before enforcing the review marker.

## Local commands

```bash
scripts/codex-client-version.sh candidate
scripts/codex-client-version.sh reviewed
scripts/codex-client-version.sh latest
scripts/codex-client-version.sh review-status
scripts/codex-client-version_test.sh
```

The live `latest` lookup requires `curl` and `jq`. It uses `GITHUB_TOKEN` or
`GH_TOKEN` when available and otherwise performs an unauthenticated public
GitHub API request.
