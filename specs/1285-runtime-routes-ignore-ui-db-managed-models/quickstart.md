# Quickstart: Runtime routes consume UI DB-managed models

## Todo Planning

Current phase:

```bash
git status -sb
git diff --name-only origin/main...HEAD
git diff --check origin/main...HEAD
```

Expected Todo diff:

```text
specs/1285-runtime-routes-ignore-ui-db-managed-models/**
```

No `cmd/`, `internal/`, `test/`, or production workflow changes are allowed in Todo.

## Implementation Start Gate

Implementation may start only after Linear HO-1285 moves to `In Progress`.

Before code changes:

```bash
git fetch origin
git status -sb
git log --oneline origin/main..HEAD
git diff --name-only origin/main...HEAD
```

## RED Test Commands

Write tests first, then run targeted commands and confirm failure is missing DB-runtime model source behavior:

```bash
go test ./test/e2e -tags e2e -run 'TestRuntimeModelSource|TestModelManagementRefreshesRuntimeSource' -count=1
go test ./test/integration -run 'TestRuntimeModelSource|TestModelGroupInfo' -count=1
go test ./internal/proxy/handler/... -run 'TestRuntimeModelSource|TestListModels|TestModelGroupInfo|TestResolveProviderBaseURL' -count=1
```

Expected RED failures:

- DB-managed model missing from `/v1/models`.
- DB-managed model returns model-not-found for chat completion.
- Non-chat route cannot resolve DB-managed model.
- `/model_group/info` omits DB model.
- create/update/delete does not refresh runtime source.

## GREEN Verification Commands

After implementation:

```bash
go test ./test/e2e -tags e2e -run 'TestRuntimeModelSource|TestModelManagementRefreshesRuntimeSource' -count=1
go test ./test/integration -run 'TestRuntimeModelSource|TestModelGroupInfo' -count=1
go test ./internal/proxy/handler/... -run 'TestRuntimeModelSource|TestListModels|TestModelGroupInfo|TestResolveProviderBaseURL' -count=1
go test ./internal/router/... ./internal/config/... ./internal/ui/... -count=1
go tool golangci-lint run
git diff --check origin/main...HEAD
```

## Manual Acceptance Shape

Use mocked upstreams only:

1. Seed `ProxyModelTable` with `model_name = db-chat-model`.
2. Set `tianji_params.model = openai/gpt-4o-mini`.
3. Set `tianji_params.api_base` to mock upstream URL.
4. Call `/v1/models` and confirm `db-chat-model` appears.
5. Call `/v1/chat/completions` with `model = db-chat-model`.
6. Confirm mocked upstream receives request.
7. Call one non-chat route with DB-managed model.
8. Confirm `/model_group/info` reports the DB group.
9. Update/delete the DB model through management handler.
10. Confirm refresh or explicit restart-required behavior.
