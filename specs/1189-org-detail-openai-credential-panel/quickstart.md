# Quickstart: Org detail OpenAI credential panel

## State Gate

Implementation is blocked until Linear HO-1189 moves to `In Progress`.

## Local Verification

```bash
go test ./test/e2e -tags e2e -run 'TestOrgDetail_OpenAICredential|TestCredentials' -count=1
go test ./internal/ui/... -count=1
go test ./internal/proxy/handler/... ./internal/callback/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

## Manual UI Smoke

1. Start the Tianji UI using the repo's existing local dev/test server flow。
2. Login as proxy admin。
3. Open `/ui/orgs` and click an organization。
4. Confirm `OpenAI credentials` panel appears after overview cards。
5. Compare desktop and mobile result screens against the owner-approved mockscreen from the issue thread；placement, hierarchy, copy tone, populated table, empty state, detail link, status/quota badges, and mobile responsiveness must remain visually equivalent。
6. Confirm org with credentials shows table rows and org without credentials shows empty state。
7. Click a credential name/detail link and confirm navigation to `/ui/credentials/{credential_id}`。
8. Inspect page text/HTML and confirm no token material appears。

## Expected Panel Contract

- Shows only current org `openai_subscription` credentials。
- Shows safe metadata/status/quota summary only。
- Links to existing credential detail/actions page。
- Does not duplicate lifecycle action buttons。
- Does not call OpenAI。
- Does not display `credential_value` or raw token metadata。
- Matches the approved desktop/mobile mockscreen before `Waiting Merge`。
