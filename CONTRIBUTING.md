# Contributing to TianjiLLM

## Getting Started

### Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/)
- [templ](https://templ.guide/) and [templui](https://templui.io/) CLIs (`make tools`)
- PostgreSQL (for E2E tests)

### Build

```bash
make tools    # Install templ + templui CLIs
make build    # templ generate + tailwind build + go build → bin/tianji
```

### Test

```bash
make test     # go test -race -cover ./...
make lint     # golangci-lint run
make check    # lint + test + build (same as CI)

# Single package
go test ./internal/provider/anthropic/... -v

# E2E (requires PostgreSQL at localhost:5433)
make e2e
```

### Run Locally

```bash
make dev      # Hot-reload: watches .go/.templ/.css files
```

## Making Changes

### Adding a Provider

1. Create `internal/provider/<name>/` with a type implementing `provider.Provider`
2. Call `provider.Register("<name>", instance)` in `init()`
3. Add test fixtures in `test/fixtures/`
4. No changes to existing code required

### Database Changes

1. Add a new migration in `internal/db/schema/`
2. Update queries in `internal/db/queries/`
3. Run `make generate` to regenerate sqlc code

### UI Changes

```bash
make ui-dev   # Watch mode for templ + tailwind
```

Components live in `internal/ui/components/`, pages in `internal/ui/pages/`.

## Pull Request Process

1. Fork the repo and create a feature branch
2. Make your changes
3. Ensure `make check` passes (lint + test + build)
4. Submit a PR with a clear description of the change

Pre-commit hooks run `gofmt` and `golangci-lint` automatically via lefthook.

## Code Style

- Follow standard Go conventions (`gofmt`, `golangci-lint`)
- All request/response types live in `internal/model/` — providers import from there
- Test with `httptest.NewServer()` for mock upstreams, `testify` for assertions
- Keep functions short, avoid deep nesting
