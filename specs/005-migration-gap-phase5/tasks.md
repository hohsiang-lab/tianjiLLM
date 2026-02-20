# Tasks: Phase 5 Migration Gap Closure

**Input**: Design documents from `/specs/005-migration-gap-phase5/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add MCP SDK dependency, extend config types, add new fields to request model

- [x] T001 Run `go get github.com/modelcontextprotocol/go-sdk@v1.3.0` to add MCP SDK dependency to `go.mod`
- [x] T002 [P] Add `MCPServerConfig` struct to `internal/config/config.go` — fields: ServerID, Alias, Transport (stdio|sse|http), URL, Command, Args, AuthType, AuthToken, StaticHeaders, AllowedTools, DisallowedTools
- [x] T003 [P] Add `SearchToolConfig` struct to `internal/config/config.go` — fields: SearchToolName, SearchProvider (enum), APIKey, APIBase, Description
- [x] T004 [P] Add `AutoRouterConfig` fields to `internal/config/config.go` — AutoRouterConfigPath, AutoRouterConfig (JSON string), AutoRouterDefaultModel, AutoRouterEmbeddingModel on TianjiLLMParams
- [x] T005 [P] Add `PromptName`, `PromptVariables`, `PromptVersion` fields to `internal/model/request.go` on `ChatCompletionRequest`
- [x] T006 [P] Add top-level `MCPServers map[string]MCPServerConfig` and `SearchTools []SearchToolConfig` fields to `internal/config/config.go` root config struct

**Checkpoint**: Config types ready. Run `make build` to verify compilation.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared interfaces and registries that multiple user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [x] T007 Create `internal/search/provider.go` — define `SearchProvider` interface (Name, HTTPMethod, ValidateEnvironment, GetCompleteURL, TransformRequest, TransformResponse), `SearchResult` struct (Title, URL, Snippet, Date, LastUpdated), `SearchResponse` struct (Results, Object)
- [x] T008 Create `internal/search/registry.go` — global registry `map[string]SearchProvider` with `Register()`, `Get()`, `List()` functions; protected by `sync.RWMutex`

**Checkpoint**: Foundation ready. Run `make build`.

---

## Phase 3: User Story 1 — MCP Server (Priority: P0)

**Goal**: MCP clients can discover and invoke tools through tianjiLLM proxy

**Independent Test**: Start tianjiLLM with MCP config, connect `mcp-inspector`, verify `tools/list` and `tools/call` round-trip

### MCP Core

- [x] T009 [US1] Create `internal/mcp/config.go` — define `MCPServerEntry` (parsed from config), `MCPTool` (Name, PrefixedName, Description, InputSchema, ServerID), `ToolSeparator` constant (default "-")
- [x] T010 [US1] Create `internal/mcp/manager.go` — `MCPServerManager` struct with `configServers map[string]MCPServerEntry`, `toolToServer map[string]string`; methods: `LoadFromConfig()`, `ListTools()`, `CallTool()`, `GetServer()`; tool name prefixing via `{alias}{sep}{tool_name}`
- [x] T011 [US1] Create `internal/mcp/server.go` — initialize `mcp.NewServer()` from SDK; register `tools/list` handler that delegates to `MCPServerManager.ListTools()`; register `tools/call` handler that delegates to `MCPServerManager.CallTool()`; error wrapping to `CallToolResult{isError: true}`
- [x] T012 [US1] Create `internal/mcp/transport.go` — `NewSSEHandler()` wrapping `mcp.NewSSEHandler()` and `NewStreamableHTTPHandler()` wrapping `mcp.NewStreamableHTTPHandler()`; both return `http.Handler` for chi mounting
- [x] T013 [US1] Implement upstream MCP client connections in `internal/mcp/manager.go` — for each configured server, create `mcp.NewClient()` with appropriate transport (stdio/sse/http); handle auth token injection; store active clients in manager

### MCP REST + Management Endpoints

- [x] T014 [P] [US1] Create `internal/mcp/rest.go` — `RESTListTools(w, r)` handler wrapping `MCPServerManager.ListTools()` as JSON; `RESTCallTool(w, r)` handler wrapping `MCPServerManager.CallTool()` as JSON
- [x] T015 [P] [US1] Create `internal/proxy/handler/mcp_mgmt.go` — CRUD handlers: `MCPServerList` (GET), `MCPServerCreate` (POST), `MCPServerUpdate` (PUT), `MCPServerDelete` (DELETE), `MCPToolsList` (GET with server_id filter); use DB queries via sqlc
- [x] T016 [US1] Add sqlc queries for MCP server CRUD in `internal/db/queries/mcp.sql` — `CreateMCPServer`, `GetMCPServer`, `ListMCPServers`, `UpdateMCPServer`, `DeleteMCPServer`; run `make generate`
- [x] T017 [US1] Add DB migration for MCP servers table in `internal/db/schema/` — table `mcp_servers` with columns: server_id (PK), alias, transport, url, command, args (jsonb), auth_type, auth_token_encrypted, static_headers (jsonb), allowed_tools (jsonb), disallowed_tools (jsonb), created_at, updated_at

### MCP Route Wiring

- [x] T018 [US1] Wire MCP routes in `internal/proxy/server.go` — mount SSE handler at `/mcp/sse`, Streamable HTTP at `/mcp`, REST at `/mcp-rest/tools/list` and `/mcp-rest/tools/call`, management at `/v1/mcp/server` and `/v1/mcp/tools`
- [x] T019 [US1] Initialize `MCPServerManager` in `cmd/tianji/main.go` — load MCP server configs, start upstream connections, pass manager to handlers

### MCP Tests

- [x] T020 [US1] Write contract test `test/contract/mcp_test.go` — test `tools/list` returns tools from mock upstream; test `tools/call` dispatches to mock upstream and returns result; test error propagation (tool not found, server unavailable, permission denied); test tool name prefixing
- [x] T021 [US1] Add MCP request/response fixtures in `test/fixtures/mcp/` — tools_list_response.json, tools_call_request.json, tools_call_response.json, error_response.json

**Checkpoint**: MCP functional. Run `go test ./internal/mcp/... -v` + `go test ./test/contract/... -run TestMCP -v`.

---

## Phase 4: User Story 2 — Search Providers (Priority: P0)

**Goal**: Tool-calling agents can execute web searches through the proxy

**Independent Test**: Configure Brave search, send `POST /v1/search/brave`, verify results

### Search Provider Implementations

- [x] T022 [P] [US2] Implement Brave search in `internal/search/brave.go` — GET to `api.search.brave.com/res/v1/web/search`; auth via `X-Subscription-Token` header; map `query`→`q`, `max_results`→`count` (cap 20), domain filter appended as `site:` to query; parse `web.results[]` + `news.results[]`; register in `init()`
- [x] T023 [P] [US2] Implement Tavily search in `internal/search/tavily.go` — POST to `api.tavily.com/search`; API key in request body `api_key` field; map `query`→`query`, `max_results`→`max_results`, `search_domain_filter`→`include_domains`; parse `results[].content` as snippet; register in `init()`
- [x] T024 [P] [US2] Implement SearXNG search in `internal/search/searxng.go` — GET to `{user_base}/search?format=json`; optional Bearer auth; map `query`→`q`, `country`→`language` (manual mapping: us→en, de→de, fr→fr, es→es, jp→ja); parse `results[].content` as snippet; register in `init()`
- [x] T025 [P] [US2] Implement Exa AI search in `internal/search/exa.go` — POST to `api.exa.ai/search`; auth via `x-api-key` header; map `query`→`query`, `max_results`→`numResults` (cap 100), `search_domain_filter`→`includeDomains`; auto-add `contents.text: true`; parse `results[].text` as snippet; register in `init()`
- [x] T026 [P] [US2] Implement Google PSE search in `internal/search/google_pse.go` — GET to `googleapis.com/customsearch/v1`; auth via query params `key=` + `cx=` (from `GOOGLE_PSE_API_KEY` + `GOOGLE_PSE_ENGINE_ID`); map `max_results`→`num` (cap 10), domain filter→`siteSearch` (first domain only); parse `items[].snippet`; register in `init()`
- [x] T027 [P] [US2] Implement DataForSEO search in `internal/search/dataforseo.go` — POST to `api.dataforseo.com/v3/serp/google/organic/live/advanced`; HTTP Basic Auth from `DATAFORSEO_LOGIN`+`DATAFORSEO_PASSWORD`; request body as JSON array `[{keyword, depth, language_code}]`; parse `tasks[0].result[0].items[]` where `type=="organic"`; register in `init()`

### Search Handler + Routing

- [x] T028 [US2] Create `internal/proxy/handler/search.go` — `SearchHandler(w, r)` extracts `{search_tool_name}` from URL, looks up `SearchToolConfig` from config, gets `SearchProvider` from registry, executes search, writes `SearchResponse` JSON
- [x] T029 [US2] Wire search route in `internal/proxy/server.go` — add `r.Post("/v1/search/{search_tool_name}", s.Handlers.SearchHandler)` inside the `/v1` auth-protected group
- [x] T030 [US2] Load search tools from config in `cmd/tianji/main.go` — parse `search_tools` from YAML, validate provider names against registry, pass to handlers

### Search Tests

- [x] T031 [P] [US2] Write unit tests `internal/search/brave_test.go` — test request URL construction, header setup, response parsing from fixture JSON
- [x] T032 [P] [US2] Write unit tests `internal/search/tavily_test.go` — test request body construction, response parsing
- [x] T033 [US2] Write contract test `test/contract/search_test.go` — mock upstream per provider, test full round-trip through handler; test error cases (provider not found, upstream error, missing query)
- [x] T034 [P] [US2] Add search fixtures in `test/fixtures/search/` — brave_response.json, tavily_response.json, searxng_response.json, exa_response.json, google_pse_response.json, dataforseo_response.json

**Checkpoint**: Search providers functional. Run `go test ./internal/search/... -v` + `go test ./test/contract/... -run TestSearch -v`.

---

## Phase 5: User Story 3 — Image Variations (Priority: P0)

**Goal**: `POST /v1/images/variations` proxied to upstream providers

**Independent Test**: Send multipart form request with image to `/v1/images/variations`, verify response

- [x] T035 [US3] Add `ImageVariation` handler method in `internal/proxy/handler/native_format.go` — identical to `ImagesEdit`: `func (h *Handlers) ImageVariation(w http.ResponseWriter, r *http.Request) { h.assistantsProxy(w, r) }`
- [x] T036 [US3] Wire route in `internal/proxy/server.go` — add `r.Post("/images/variations", s.Handlers.ImageVariation)` after the existing `/images/edits` line
- [x] T037 [US3] Write contract test in `test/contract/images_test.go` — mock upstream OpenAI, send multipart form to `/v1/images/variations`, verify request is forwarded and response returned

**Checkpoint**: Image variations functional. Run `go test ./test/contract/... -run TestImageVariation -v`.

---

## Phase 6: User Story 4 — Prompt Template Resolution (Priority: P1)

**Goal**: Chat completions can reference stored prompt templates for automatic expansion

**Independent Test**: Store a prompt via `/prompts`, send chat completion with `prompt_name`, verify expanded template reaches mock provider

- [x] T038 [US4] Implement prompt resolution function in `internal/proxy/handler/prompt_resolve.go` — `resolvePromptTemplate(ctx, db, req)` checks `req.PromptName`; if set, fetches template from DB (by name + optional version); substitutes `{{var}}` placeholders using `req.PromptVariables`; returns modified `ChatCompletionRequest` with resolved messages
- [x] T039 [US4] Integrate prompt resolution into chat handler `internal/proxy/handler/chat.go` — call `resolvePromptTemplate()` early in `ChatCompletion()` before provider dispatch; if `PromptName` is empty, skip resolution (zero overhead path)
- [x] T040 [US4] Write contract test `test/contract/prompt_resolve_test.go` — create prompt via handler, send chat completion with `prompt_name` + `prompt_variables`, verify mock provider receives expanded messages; test version pinning; test non-existent prompt error

**Checkpoint**: Prompt resolution functional. Run `go test ./test/contract/... -run TestPrompt -v`.

---

## Phase 7: User Story 5 — High-Value Providers (Priority: P1)

**Goal**: ~20 new providers registered and functional

**Independent Test**: For each provider, configure in YAML, send request, verify upstream receives correct format

### OpenAI-Compatible Providers (embed `*openai.Provider`, ~15 lines each)

- [x] T041 [P] [US5] Create `internal/provider/baseten/baseten.go` — embed openai.Provider, baseURL `https://model-{model_id}.api.baseten.co/production/predict`, register "baseten"
- [x] T042 [P] [US5] Create `internal/provider/hostedvllm/hostedvllm.go` — embed openai.Provider, register "hosted_vllm" with configurable api_base
- [x] T043 [P] [US5] Create `internal/provider/codestral/codestral.go` — embed openai.Provider, baseURL `https://codestral.mistral.ai/v1`, register "codestral"
- [x] T044 [P] [US5] Create `internal/provider/friendliai/friendliai.go` — embed openai.Provider, baseURL `https://inference.friendli.ai/v1`, register "friendliai"
- [x] T045 [P] [US5] Create `internal/provider/jina/jina.go` — embed openai.Provider, baseURL `https://api.jina.ai/v1`, register "jina_ai"
- [x] T046 [P] [US5] Create `internal/provider/voyage/voyage.go` — embed openai.Provider, baseURL `https://api.voyageai.com/v1`, register "voyage"
- [x] T047 [P] [US5] Create `internal/provider/infinity/infinity.go` — embed openai.Provider, register "infinity" with configurable api_base (self-hosted)
- [x] T048 [P] [US5] Create `internal/provider/nebius/nebius.go` — embed openai.Provider, baseURL `https://api.studio.nebius.ai/v1`, register "nebius"
- [x] T049 [P] [US5] Create `internal/provider/ovhcloud/ovhcloud.go` — embed openai.Provider, baseURL `https://llm.api.cloud.ovh.net/v1`, register "ovhcloud"
- [x] T050 [P] [US5] Create `internal/provider/lambdaai/lambdaai.go` — embed openai.Provider, baseURL `https://api.lambdalabs.com/v1`, register "lambda_ai"
- [x] T051 [P] [US5] Create `internal/provider/nscale/nscale.go` — embed openai.Provider, baseURL `https://inference.api.nscale.com/v1`, register "nscale"
- [x] T052 [P] [US5] Create `internal/provider/gigachat/gigachat.go` — embed openai.Provider, baseURL `https://gigachat.devices.sberbank.ru/api/v1`, register "gigachat"; override SetupHeaders for OAuth bearer token

### Custom-Format Providers

- [x] T053 [P] [US5] Create `internal/provider/elevenlabs/elevenlabs.go` — implement full Provider interface for TTS; TransformRequest maps OpenAI audio/speech format to ElevenLabs `POST /v1/text-to-speech/{voice_id}`; custom headers `xi-api-key`; register "elevenlabs"
- [x] T054 [P] [US5] Create `internal/provider/deepgram/deepgram.go` — implement full Provider interface for STT; TransformRequest maps OpenAI audio/transcriptions format to Deepgram `POST /v1/listen`; auth via `Authorization: Token {key}`; register "deepgram"
- [x] T055 [P] [US5] Create `internal/provider/awspolly/awspolly.go` — implement full Provider interface for TTS; TransformRequest maps to AWS Polly `SynthesizeSpeech`; AWS Signature V4 auth; register "aws_polly"
- [x] T056 [P] [US5] Create `internal/provider/stability/stability.go` — implement full Provider interface for image generation; TransformRequest maps OpenAI images/generations to Stability `POST /v1/generation/{engine}/text-to-image`; auth via `Authorization: Bearer`; register "stability"
- [x] T057 [P] [US5] Create `internal/provider/falai/falai.go` — implement full Provider interface; async job pattern: POST to submit, poll for result; auth via `Authorization: Key {key}`; register "fal_ai"
- [x] T058 [P] [US5] Create `internal/provider/recraft/recraft.go` — implement full Provider interface for image generation; custom request format; auth via `Authorization: Bearer`; register "recraft"

### Provider Wiring

- [x] T059 [US5] Add `_ import` lines for all new providers in `cmd/tianji/main.go` — one `_ "github.com/praxisllmlab/tianjiLLM/internal/provider/{pkg}"` per new provider package
- [x] T060 [US5] Write unit tests for custom-format providers — `internal/provider/elevenlabs/elevenlabs_test.go`, `internal/provider/deepgram/deepgram_test.go`; test TransformRequest and TransformResponse with fixture data

**Checkpoint**: All new providers registered. Run `go test ./internal/provider/... -v`.

---

## Phase 8: User Story 6 — Discovery Endpoints (Priority: P1)

**Goal**: API consumers can query available models, providers, and capabilities

**Independent Test**: Configure multiple providers, query `/model_group/info`, verify response lists all models with capabilities

- [x] T061 [US6] Create `internal/proxy/handler/discovery.go` — implement `ModelGroupInfo(w, r)` handler: iterate router's model groups, aggregate capabilities per group (OR for booleans, MAX for numerics, SUM for TPM/RPM, UNION for string arrays), enrich with `model_prices_and_context_window.json` data; support `?model_group=` filter
- [x] T062 [P] [US6] Implement `PublicProviders(w, r)` handler in `internal/proxy/handler/discovery.go` — return sorted list of all registered provider names from `provider.List()`
- [x] T063 [P] [US6] Implement `PublicModelCostMap(w, r)` handler in `internal/proxy/handler/discovery.go` — return the full `model_prices_and_context_window.json` content as JSON
- [x] T064 [US6] Verify `provider.List()` in `internal/provider/registry.go` returns sorted slice — function already exists; add `sort.Strings()` if not already sorted, ensure output matches `/public/providers` contract
- [x] T065 [US6] Wire discovery routes in `internal/proxy/server.go` — add `r.Get("/model_group/info", ...)` (auth required), `r.Get("/public/providers", ...)` (no auth), `r.Get("/public/tianji_model_cost_map", ...)` (no auth)
- [x] T066 [US6] Write contract test `test/contract/discovery_test.go` — test `ModelGroupInfo` returns aggregated capabilities; test `PublicProviders` returns provider list; test `?model_group=` filter

**Checkpoint**: Discovery endpoints functional. Run `go test ./test/contract/... -run TestDiscovery -v`.

---

## Phase 9: User Story 7 — On-demand Plugins (Priority: P2)

**Goal**: Complete guardrail and callback plugin parity (34 plugins)

**Independent Test**: `go test ./internal/guardrail/... -v` + `go test ./internal/callback/... -v`

### Guardrails (each is independent — implement as needed)

- [x] T067 [P] [US7] Implement AIM guardrail in `internal/guardrail/aim.go` + register `case "aim"` in `internal/guardrail/factory.go`
- [x] T068 [P] [US7] Implement Aporia guardrail in `internal/guardrail/aporia.go` + register `case "aporia"` in factory.go
- [x] T069 [P] [US7] Implement Custom Code guardrail in `internal/guardrail/custom_code.go` + register `case "custom_code"` in factory.go
- [x] T070 [P] [US7] Implement DynamoAI guardrail in `internal/guardrail/dynamoai.go` + register `case "dynamoai"` in factory.go
- [x] T071 [P] [US7] Implement EnkryptAI guardrail in `internal/guardrail/enkryptai.go` + register `case "enkryptai"` in factory.go
- [x] T072 [P] [US7] Implement GraySwan guardrail in `internal/guardrail/grayswan.go` + register `case "grayswan"` in factory.go
- [x] T073 [P] [US7] Implement Guardrails AI guardrail in `internal/guardrail/guardrails_ai.go` + register `case "guardrails_ai"` in factory.go
- [x] T074 [P] [US7] Implement HiddenLayer guardrail in `internal/guardrail/hiddenlayer.go` + register `case "hiddenlayer"` in factory.go
- [x] T075 [P] [US7] Implement IBM Guardrails in `internal/guardrail/ibm.go` + register `case "ibm_guardrails"` in factory.go
- [x] T076 [P] [US7] Implement Javelin guardrail in `internal/guardrail/javelin.go` + register `case "javelin"` in factory.go
- [x] T077 [P] [US7] Implement Lakera AI v2 guardrail in `internal/guardrail/lakera_v2.go` + register `case "lakera_v2"` in factory.go
- [x] T078 [P] [US7] Implement Lasso guardrail in `internal/guardrail/lasso.go` + register `case "lasso"` in factory.go
- [x] T079 [P] [US7] Implement Model Armor guardrail in `internal/guardrail/model_armor.go` + register `case "model_armor"` in factory.go
- [x] T080 [P] [US7] Implement Noma guardrail in `internal/guardrail/noma.go` + register `case "noma"` in factory.go
- [x] T081 [P] [US7] Implement Onyx guardrail in `internal/guardrail/onyx.go` + register `case "onyx"` in factory.go
- [x] T082 [P] [US7] Implement Pangea guardrail in `internal/guardrail/pangea.go` + register `case "pangea"` in factory.go
- [x] T083 [P] [US7] Implement PANW Prisma AIRS guardrail in `internal/guardrail/panw_prisma.go` + register `case "panw_prisma_airs"` in factory.go
- [x] T084 [P] [US7] Implement Pillar guardrail in `internal/guardrail/pillar.go` + register `case "pillar"` in factory.go
- [x] T085 [P] [US7] Implement Prompt Security guardrail in `internal/guardrail/prompt_security.go` + register `case "prompt_security"` in factory.go
- [x] T086 [P] [US7] Implement Qualifire guardrail in `internal/guardrail/qualifire.go` + register `case "qualifire"` in factory.go
- [x] T087 [P] [US7] Implement Unified Guardrail in `internal/guardrail/unified.go` + register `case "unified_guardrail"` in factory.go
- [x] T088 [P] [US7] Implement Zscaler AI Guard in `internal/guardrail/zscaler.go` + register `case "zscaler_ai_guard"` in factory.go

### Callbacks (each is independent — implement as needed)

- [x] T089 [P] [US7] Implement Arize full callback in `internal/callback/arize_full.go` + register `case "arize"` in `internal/callback/factory.go`
- [x] T090 [P] [US7] Implement AgentOps callback in `internal/callback/agentops.go` + register `case "agentops"` in factory.go
- [x] T091 [P] [US7] Implement Focus callback in `internal/callback/focus.go` + register `case "focus"` in factory.go
- [x] T092 [P] [US7] Implement HumanLoop callback in `internal/callback/humanloop.go` + register `case "humanloop"` in factory.go
- [x] T093 [P] [US7] Implement LangTrace callback in `internal/callback/langtrace.go` + register `case "langtrace"` in factory.go
- [x] T094 [P] [US7] Implement Levo callback in `internal/callback/levo.go` + register `case "levo"` in factory.go
- [x] T095 [P] [US7] Implement Weave callback in `internal/callback/weave.go` + register `case "weave"` in factory.go
- [x] T096 [P] [US7] Implement Bitbucket callback in `internal/callback/bitbucket.go` + register `case "bitbucket"` in factory.go
- [x] T097 [P] [US7] Implement GitLab callback in `internal/callback/gitlab.go` + register `case "gitlab"` in factory.go
- [x] T098 [P] [US7] Implement DotPrompt callback in `internal/callback/dotprompt.go` + register `case "dotprompt"` in factory.go
- [x] T099 [P] [US7] Implement WebSearch Interception callback in `internal/callback/websearch.go` + register `case "websearch_interception"` in factory.go
- [x] T100 [P] [US7] Implement Custom Batch Logger callback in `internal/callback/custom_batch.go` + register `case "custom_batch_logger"` in factory.go

**Checkpoint**: Plugins added as needed. Each guardrail/callback is independent — implement based on demand.

---

## Phase 10: User Story 8 — AutoRouter (Priority: P2)

**Goal**: Embedding-based semantic routing selects optimal model group per prompt

**Independent Test**: Configure auto-router with two model groups, send simple vs complex prompts, verify routing decisions

- [x] T101 [US8] Create `internal/router/strategy/auto/cosine.go` — `CosineSimilarity(a, b []float32) float64` function; `BestMatch(query []float32, candidates [][]float32, threshold float64) (int, float64)` returning best index and score
- [x] T102 [US8] Create `internal/router/strategy/auto/encoder.go` — `Encoder` struct wrapping the proxy's own embedding endpoint; `Encode(ctx, texts []string) ([][]float32, error)` calls internal embedding function with configured `auto_router_embedding_model`
- [x] T103 [US8] Create `internal/router/strategy/auto/auto.go` — `AutoRouter` struct with `routes []Route`, `routeVectors [][][]float32` (lazy-init), `encoder *Encoder`, `defaultModel string`; `Route(ctx, messages) (string, error)` method: extract last user message, embed, find best matching route, return model group name or default
- [x] T104 [US8] Integrate AutoRouter into `internal/router/router.go` — in `Route()` method, detect `auto_router/` model prefix, lookup AutoRouter instance from `autoRouters map[string]*AutoRouter`, call `autoRouter.Route()` to get resolved model, continue with standard deployment selection
- [x] T105 [US8] Initialize AutoRouter deployments in `cmd/tianji/main.go` — detect `auto_router/` prefix in model_list, parse route config (from JSON string or file path), create AutoRouter instance, register in router's autoRouters map
- [x] T106 [US8] Write unit test `internal/router/strategy/auto/auto_test.go` — test cosine similarity computation; test route matching with mock encoder; test default model fallback; test empty messages handling; test embedding failure fallback

**Checkpoint**: AutoRouter functional. Run `go test ./internal/router/strategy/auto/... -v`.

---

## Phase 11: Polish & Cross-Cutting Concerns

**Purpose**: Validation, regression testing, final wiring

- [x] T107 Run full test suite `make check` — verify zero regressions across all existing tests
- [x] T108 Validate `quickstart.md` examples — e2e tests in `test/integration/phase5_quickstart_test.go` (12 tests: MCP REST, Search, Image Variations, Discovery x4, auth checks)
- [x] T109 [P] Verify all new provider packages have `_ import` in `cmd/tianji/main.go`
- [x] T110 [P] Verify all new guardrail cases in `internal/guardrail/factory.go` compile and match expected mode strings
- [x] T111 [P] Verify all new callback cases in `internal/callback/factory.go` compile and match expected type strings
- [x] T112 Run `make lint` — fix any linting issues in new code (go vet clean; golangci-lint skipped due to tool version mismatch)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS search providers
- **US1 MCP (Phase 3)**: Depends on Setup (T001 for SDK, T002 for config types)
- **US2 Search (Phase 4)**: Depends on Foundational (T007-T008 for interface/registry)
- **US3 Image Variations (Phase 5)**: Depends on Setup only — can start in parallel with anything
- **US4 Prompt Resolution (Phase 6)**: Depends on Setup (T005 for request fields)
- **US5 Providers (Phase 7)**: No dependencies — can start immediately (self-contained packages)
- **US6 Discovery (Phase 8)**: Depends on Setup — uses existing provider registry
- **US7 Plugins (Phase 9)**: No dependencies — uses existing factory pattern
- **US8 AutoRouter (Phase 10)**: Depends on Setup (T004 for config types)
- **Polish (Phase 11)**: Depends on all desired user stories being complete

### User Story Independence

```
Setup ──┬── US1 (MCP)           ← P0, ~13 tasks
        ├── US2 (Search)         ← P0, ~13 tasks (needs Foundational)
        ├── US3 (Image)          ← P0, 3 tasks
        ├── US4 (Prompt)         ← P1, 3 tasks
        ├── US5 (Providers)      ← P1, ~20 tasks
        ├── US6 (Discovery)      ← P1, 6 tasks
        ├── US7 (Plugins)        ← P2, 34 tasks
        └── US8 (AutoRouter)     ← P2, 6 tasks
```

All user stories are independently testable after Setup + Foundational are complete.

### Parallel Opportunities

**Maximum parallelism** (all 8 user stories can run simultaneously after Setup):
- US3 (3 tasks) and US5 (20 tasks) have zero cross-dependencies
- All 34 plugin tasks (US7) are independently parallelizable
- All 18 provider tasks (US5) are independently parallelizable
- All 6 search provider tasks (US2 T022-T027) are independently parallelizable

---

## Parallel Example: User Story 2 (Search)

```bash
# Launch all 6 provider implementations in parallel (different files):
Task: "T022 Implement Brave search in internal/search/brave.go"
Task: "T023 Implement Tavily search in internal/search/tavily.go"
Task: "T024 Implement SearXNG search in internal/search/searxng.go"
Task: "T025 Implement Exa AI search in internal/search/exa.go"
Task: "T026 Implement Google PSE search in internal/search/google_pse.go"
Task: "T027 Implement DataForSEO search in internal/search/dataforseo.go"

# Then sequentially: handler → routing → tests
```

## Parallel Example: User Story 5 (Providers)

```bash
# Launch ALL provider implementations in parallel (each is a separate package):
Task: "T041-T058" — all 18 provider packages simultaneously
# Then: T059 (wiring) → T060 (tests)
```

---

## Implementation Strategy

### MVP First (US3 Image Variations — 3 tasks, instant value)

1. Complete Phase 1: Setup (T001-T006)
2. Complete Phase 5: US3 Image Variations (T035-T037)
3. **STOP and VALIDATE**: `POST /v1/images/variations` works
4. Deploy — immediate value for OpenAI SDK users

### P0 Sprint (MCP + Search + Image — core agent tooling)

1. Setup + Foundational (T001-T008)
2. US3 Image Variations (T035-T037) — quick win
3. US1 MCP Server (T009-T021) — highest complexity, do early
4. US2 Search Providers (T022-T034) — parallelizable
5. **VALIDATE**: Agent workflows fully functional

### Full Delivery

1. P0 Sprint above
2. US4 Prompt Resolution (T038-T040) — small, focused
3. US5 Providers (T041-T060) — max parallelism
4. US6 Discovery (T061-T066) — enables UI
5. US7 Plugins (T067-T100) — on-demand, parallelize freely
6. US8 AutoRouter (T101-T106) — last, highest complexity
7. Polish (T107-T112) — final validation

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- US7 plugins: implement based on customer demand — not required to do all 34 at once
