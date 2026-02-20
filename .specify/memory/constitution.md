<!--
Sync Impact Report:
- Version: 1.1.0 → 1.2.0 (MINOR — new principle added)
- Modified principles: none
- Added principles:
  - VII. sqlc-First Database Access (NON-NEGOTIABLE) — all DB queries
    MUST be defined as .sql files and code-generated via sqlc; hand-written
    query methods in Go are prohibited
- Added sections: none
- Removed sections: none
- Templates requiring updates:
  - .specify/templates/plan-template.md — ✅ no changes needed
  - .specify/templates/spec-template.md — ✅ no changes needed
  - .specify/templates/tasks-template.md — ✅ no changes needed
- Follow-up TODOs: none
-->

# TianjiLLM-Go Constitution

## Core Principles

### I. Python-First Reference (NON-NEGOTIABLE)

The Python TianjiLLM codebase (`tianji/`) is the single source of truth for
all feature behavior, API contracts, and edge case handling.

- When implementing any feature, MUST first read the corresponding Python
  source code to understand exact behavior, data structures, and edge cases.
- When a design question arises, MUST check how Python TianjiLLM handles it
  before proposing a solution.
- Use Claude Context (`mcp__claude-context__search_code`) to search the
  indexed Python codebase for specific implementations.
- Use `mcp__claude-context__index_codebase` to index the tianji codebase
  if not already indexed.
- Deviations from Python behavior MUST be explicitly documented with
  rationale and user approval.

### II. Feature Parity

Every feature in tianjiLLM MUST replicate the corresponding Python TianjiLLM
feature's external behavior.

- API contracts (request/response formats) MUST match Python version.
- YAML configuration format (`proxy_config.yaml`) MUST be compatible.
- JSON configuration files (`providers.json`,
  `model_prices_and_context_window.json`) MUST be directly reusable.
- Model name format (`provider/model`) MUST be identical.
- Error codes and error response structure MUST match.
- Environment variable syntax (`os.environ/VAR_NAME`) MUST be supported.
- When in doubt about behavior, the Python version is correct.

### III. Research Before Build (NON-NEGOTIABLE)

Before implementing any component, MUST research available Go libraries,
architecture patterns, and established conventions using external sources.

- **Step 1 — Context7**: Use `mcp__context7__resolve-library-id` +
  `mcp__context7__query-docs` to find up-to-date official documentation
  for every Go library being considered. This MUST be the first step
  for any library evaluation.
- **Step 2 — GitHub**: Use `mcp__grep__searchGitHub` to find real-world
  usage patterns, production examples, and integration patterns of
  candidate libraries.
- **Step 3 — Web Search**: Use web search to evaluate library maturity,
  latest release dates, maintenance activity, breaking changes, and
  community adoption.
- **Step 4 — Claude Context**: Use `mcp__claude-context__search_code`
  to search the Python codebase for the corresponding implementation
  details that the Go code must replicate.
- Document all technology decisions in `research.md` with:
  decision, rationale, alternatives considered, and source links.
- Prefer well-maintained, widely-adopted Go libraries over custom
  implementations.
- MUST NOT proceed to implementation until research is documented.

### IV. Test-Driven Migration

Each migrated feature MUST have tests that verify behavior parity with
the Python version.

- Unit tests MUST cover translation layer format conversions using
  real request/response fixtures extracted from Python test data.
- Contract tests MUST verify API endpoint compatibility with
  OpenAI SDK expectations.
- Integration tests MUST verify end-to-end provider communication.
- Use Python TianjiLLM's test cases as reference for test scenarios
  and edge cases.
- Test coverage for translation layers MUST be >= 90%.

### V. Go Best Practices & Idioms

Go code MUST follow Go conventions, best practices, and idiomatic
patterns. Architecture MUST follow established Go project standards.

**Architecture & Project Layout:**

- Follow the Go community standard project layout conventions.
- Use `cmd/` for application entry points, `internal/` for private
  packages, `pkg/` only for genuinely reusable public libraries.
- Use `context.Context` as first parameter for all functions that
  perform I/O, cancellation, or timeout-sensitive operations.
- Use dependency injection via interfaces — pass dependencies as
  constructor parameters, not global variables.

**Code Style:**

- Use Go interfaces for polymorphism (not class hierarchies).
- Use Go channels for streaming (not Python async generators).
- Use Go's standard `error` return pattern (not exceptions).
- Prefer composition over inheritance.
- Eliminate the Python version's 74-elif provider routing with
  Go's interface dispatch + registry pattern.
- No unnecessary abstractions — if 3 lines of code are clearer
  than a helper function, use the 3 lines.
- Functions MUST be short and do one thing.
- Maximum 3 levels of nesting; refactor if deeper.

**Error Handling:**

- Use `fmt.Errorf` with `%w` verb for error wrapping.
- Define sentinel errors and custom error types at package level.
- MUST NOT silently swallow errors — every error MUST be handled
  or explicitly propagated.

**Concurrency:**

- Use `errgroup` for managing concurrent goroutines with errors.
- MUST pass `context.Context` for cancellation propagation.
- Use `sync.Once` for one-time initialization (e.g., registries).
- Streaming MUST use channels with proper cleanup on context
  cancellation.

### VI. No Stale Knowledge (NON-NEGOTIABLE)

MUST NOT rely on the AI agent's pre-trained knowledge for technology
decisions, library APIs, or best practices. All technical information
MUST be verified from external sources before use.

- Before choosing any Go library, MUST query Context7 for its
  latest documentation and API surface.
- Before using any library API, MUST verify the function signatures
  and usage patterns via Context7 docs or GitHub code search.
- Before recommending any architecture pattern, MUST search GitHub
  for real-world Go projects using that pattern at scale.
- Before claiming any Go version feature or stdlib behavior, MUST
  verify via Context7 or web search.
- If external verification is unavailable for a specific claim,
  MUST explicitly flag it as "unverified — needs manual check"
  rather than presenting it as fact.
- This principle applies to: package names, function signatures,
  configuration syntax, version compatibility, deprecation status,
  and performance characteristics.

### VII. sqlc-First Database Access (NON-NEGOTIABLE)

All database queries MUST be defined as SQL in `.sql` files under
`internal/db/queries/` and code-generated via `sqlc generate`.
Hand-written query methods in Go are prohibited.

- Every new database query MUST be written as a named sqlc query
  in the appropriate `.sql` file (e.g., `team.sql`, `user.sql`).
- MUST run `make generate` (which executes `sqlc generate`) after
  adding or modifying any `.sql` query file.
- MUST NOT write Go methods that directly construct SQL strings,
  use `pgx.Query`/`pgx.Exec` inline, or define custom Params
  structs that duplicate sqlc-generated types.
- Schema changes MUST be defined in `internal/db/schema/` as SQL
  migration files that sqlc uses to validate queries.
- The only hand-written Go code allowed in `internal/db/` is
  `extensions.go` for methods that require runtime type assertions
  (e.g., `Pool()`, `Ping()`) which sqlc cannot generate.
- Handler and service code MUST use the sqlc-generated `*db.Queries`
  methods and `db.*Params` structs — never raw SQL.
- sqlc configuration lives in `sqlc.yaml` at the repository root;
  changes to sqlc config MUST be reviewed for impact on all
  generated code.

## Technology Stack & Tooling

- **Language**: Go (latest stable version — verify via web search)
- **HTTP Framework**: To be determined via Research Phase
  (evaluate net/http, chi, gin, echo — prefer stdlib-aligned;
  MUST verify latest versions and breaking changes via Context7)
- **Database**: PostgreSQL (primary), Redis (cache/rate-limit)
- **DB Query Layer**: sqlc (code generation from SQL — NON-NEGOTIABLE;
  see Principle VII)
- **DB Driver**: pgx/v5 (PostgreSQL driver, used by sqlc)
- **YAML Parsing**: `gopkg.in/yaml.v3` (verify latest via Context7)
- **JSON Parsing**: `encoding/json` (stdlib)
- **Testing**: `go test` + `testify` for assertions
  (verify testify latest via Context7)
- **Linting**: `golangci-lint` (verify latest config format)
- **Build**: Standard `go build` / `go install`
- **Configuration files**: Reuse Python TianjiLLM's JSON/YAML files
  directly

## Development Workflow

1. **Check Python first**: Before writing any Go code for a feature,
   read and understand the Python implementation using Claude Context
   code search.
2. **Research externally**: Use Context7 for library docs, GitHub
   search for real-world patterns, and web search for latest info.
   MUST NOT skip this step or rely on cached agent knowledge.
3. **Document decisions**: Record all technology choices in
   `research.md` with sources and rationale.
4. **Define SQL queries**: Write new database queries as named sqlc
   queries in `internal/db/queries/*.sql`, then run `make generate`
   to produce Go code. MUST NOT hand-write query methods.
5. **Write tests**: Create test cases based on Python TianjiLLM's test
   data and acceptance scenarios.
6. **Implement**: Write Go code following Go best practices and
   idioms, not Python transliteration.
7. **Verify parity**: Confirm behavior matches Python version using
   the same inputs and expected outputs.
8. **Document deviations**: If Go implementation intentionally differs
   from Python (e.g., better error handling, Go-idiomatic streaming),
   document the deviation and rationale.

## Governance

This constitution supersedes all other development practices for the
tianjiLLM project. All code contributions MUST comply with these
principles.

- Amendments require explicit documentation of what changed and why.
- Complexity beyond these principles MUST be justified in the
  Complexity Tracking section of the implementation plan.
- The Python TianjiLLM codebase remains the authoritative reference
  for feature behavior throughout the migration.
- All technology decisions MUST be backed by external source
  verification (Context7, GitHub, web search) — not agent memory.
- All database queries MUST go through the sqlc pipeline — any
  hand-written SQL in Go code is a governance violation.

**Version**: 1.2.0 | **Ratified**: 2026-02-15 | **Last Amended**: 2026-02-17
