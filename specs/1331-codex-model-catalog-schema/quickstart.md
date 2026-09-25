# Quickstart: HO-1331 Codex model catalog schema

## Todo Phase Verification

```bash
git diff --check
git diff --name-only origin/main...HEAD
```

Expected Todo diff:

- only files under `specs/1331-codex-model-catalog-schema/`
- no production code

## Implementation RED Test

Add tests first, then run:

```bash
go test ./internal/proxy/handler -run 'ListModels|CodexModel' -count=1
```

Expected before fix:

- Codex decode test fails because top-level `models` is absent.

## Implementation Green Test

After implementation:

```bash
go test ./internal/proxy/handler -run 'ListModels|CodexModel' -count=1
go test ./internal/proxy/handler -run 'RuntimeModelSource|ListModels' -count=1
git diff --check
```

Expected after fix:

- `GET /models` includes `object`, `data`, and `models`.
- `GET /v1/models` includes the same dual shape.
- DB-managed model rows appear in both `data[]` and `models[]`.
- Hidden aliases are absent from both shapes.

## Manual Response Check

Run the Tianji server locally according to repo standard setup, then:

```bash
curl -s http://localhost:8000/models | jq '.object, (.data | length), (.models | length), .models[0].slug'
curl -s http://localhost:8000/v1/models | jq '.object, (.data | length), (.models | length), .models[0].slug'
```

## Live Verification

After the fix is deployed to Tianji live route:

```bash
codex exec --json 'say OK'
```

Configured with:

- `base_url = "https://tianji.hohsiang.com.tw/v1"`
- `wire_api = "responses"`
- `model = "openai/gpt-5.5"`

Acceptance:

- command succeeds;
- model refresh logs do not contain `failed to decode models response`;
- model refresh logs do not contain `missing field models`;
- temporary `model_catalog_json` workaround is not required.
