package proxy

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/redis/go-redis/v9"

	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

// Server holds dependencies for the HTTP proxy server.
type Server struct {
	Router             chi.Router
	Handlers           *handler.Handlers
	AuthMiddleware     func(http.Handler) http.Handler
	SCIMHandler        http.Handler // optional SCIM 2.0 server
	PassthroughHandler http.Handler // optional pass-through proxy
	MCPSSEHandler      http.Handler // optional MCP SSE transport
	MCPStreamHandler   http.Handler // optional MCP Streamable HTTP transport
	MCPRESTHandler     http.Handler // optional MCP REST API handler

	// Rate limiting middleware (nil-safe: act as pass-through when nil)
	parallelMW     func(http.Handler) http.Handler
	dynamicRateMW  func(http.Handler) http.Handler
	cacheControlMW func(http.Handler) http.Handler
}

// ServerConfig holds configuration for creating a new Server.
type ServerConfig struct {
	Handlers           *handler.Handlers
	MasterKey          string
	DBQueries          middleware.TokenValidator
	RedisClient        redis.UniversalClient // optional, enables rate limiting middleware
	PassthroughHandler http.Handler
	MCPSSEHandler      http.Handler
	MCPStreamHandler   http.Handler
	MCPRESTHandler     http.Handler
}

// NewServer creates a chi router with all routes configured.
func NewServer(cfg ServerConfig) *Server {
	return NewServerWithAuth(cfg, middleware.AuthConfig{
		MasterKey: cfg.MasterKey,
		Validator: cfg.DBQueries,
	})
}

// NewServerWithAuth creates a server with explicit auth configuration.
// Used for testing JWT/RBAC or custom auth setups.
func NewServerWithAuth(cfg ServerConfig, authCfg middleware.AuthConfig) *Server {
	r := chi.NewRouter()

	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)

	authMW := middleware.NewAuthMiddleware(authCfg)

	// Build rate limiting middleware (nil-safe: pass-through when no Redis)
	parallelLimiter := middleware.NewParallelRequestLimiter(cfg.RedisClient)
	dynamicLimiter := middleware.NewDynamicRateLimiter(cfg.RedisClient)

	s := &Server{
		Router:             r,
		Handlers:           cfg.Handlers,
		AuthMiddleware:     authMW,
		PassthroughHandler: cfg.PassthroughHandler,
		MCPSSEHandler:      cfg.MCPSSEHandler,
		MCPStreamHandler:   cfg.MCPStreamHandler,
		MCPRESTHandler:     cfg.MCPRESTHandler,
		parallelMW:         middleware.NewParallelRequestMiddleware(parallelLimiter),
		dynamicRateMW:      middleware.NewDynamicRateLimitMiddleware(dynamicLimiter),
		cacheControlMW:     middleware.NewCacheControlMiddleware(),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	r := s.Router

	// Health endpoints (no auth)
	r.Route("/health", func(r chi.Router) {
		r.Get("/", s.Handlers.HealthCheck)
		r.Get("/readiness", s.Handlers.HealthReadiness)
		r.Get("/liveness", s.Handlers.HealthLiveness)
		r.Get("/services", s.Handlers.HealthServices)
	})

	// OpenAI-compatible API — registered under /v1 and bare paths for client compat.
	// Python LiteLLM registers both; most SDKs use bare paths (no /v1 prefix).
	llmMiddleware := func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Use(s.parallelMW)
		r.Use(s.dynamicRateMW)
		r.Use(s.cacheControlMW)
	}
	registerLLMRoutes := func(r chi.Router) {
		r.Post("/chat/completions", s.Handlers.ChatCompletion)
		r.Post("/completions", s.Handlers.Completion)
		r.Post("/embeddings", s.Handlers.Embedding)
		r.Post("/images/generations", s.Handlers.ImageGeneration)
		r.Post("/audio/transcriptions", s.Handlers.AudioTranscription)
		r.Post("/audio/speech", s.Handlers.AudioSpeech)
		r.Post("/moderations", s.Handlers.Moderation)
		r.Post("/responses", s.Handlers.CreateResponse)
		r.Get("/models", s.Handlers.ListModels)

		// Files API
		r.Post("/files", s.Handlers.FilesUpload)
		r.Get("/files", s.Handlers.FilesList)
		r.Get("/files/{file_id}", s.Handlers.FilesGet)
		r.Get("/files/{file_id}/content", s.Handlers.FilesGetContent)
		r.Delete("/files/{file_id}", s.Handlers.FilesDelete)

		// Batches API
		r.Post("/batches", s.Handlers.BatchesCreate)
		r.Get("/batches/{batch_id}", s.Handlers.BatchesGet)
		r.Post("/batches/{batch_id}/cancel", s.Handlers.BatchesCancel)
		r.Get("/batches", s.Handlers.BatchesList)

		// Fine-tuning API
		r.Post("/fine_tuning/jobs", s.Handlers.FineTuningCreate)
		r.Get("/fine_tuning/jobs/{fine_tuning_job_id}", s.Handlers.FineTuningGet)
		r.Post("/fine_tuning/jobs/{fine_tuning_job_id}/cancel", s.Handlers.FineTuningCancel)
		r.Get("/fine_tuning/jobs/{fine_tuning_job_id}/events", s.Handlers.FineTuningListEvents)
		r.Get("/fine_tuning/jobs/{fine_tuning_job_id}/checkpoints", s.Handlers.FineTuningListCheckpoints)

		// Rerank API
		r.Post("/rerank", s.Handlers.Rerank)

		// Assistants API
		r.Post("/assistants", s.Handlers.AssistantCreate)
		r.Get("/assistants", s.Handlers.AssistantList)
		r.Get("/assistants/{assistant_id}", s.Handlers.AssistantGet)
		r.Post("/assistants/{assistant_id}", s.Handlers.AssistantModify)
		r.Delete("/assistants/{assistant_id}", s.Handlers.AssistantDelete)

		// Threads API
		r.Post("/threads", s.Handlers.ThreadCreate)
		r.Get("/threads/{thread_id}", s.Handlers.ThreadGet)
		r.Post("/threads/{thread_id}", s.Handlers.ThreadModify)
		r.Delete("/threads/{thread_id}", s.Handlers.ThreadDelete)

		// Messages API
		r.Post("/threads/{thread_id}/messages", s.Handlers.MessageCreate)
		r.Get("/threads/{thread_id}/messages", s.Handlers.MessageList)
		r.Get("/threads/{thread_id}/messages/{message_id}", s.Handlers.MessageGet)

		// Runs API
		r.Post("/threads/{thread_id}/runs", s.Handlers.RunCreate)
		r.Get("/threads/{thread_id}/runs", s.Handlers.RunList)
		r.Get("/threads/{thread_id}/runs/{run_id}", s.Handlers.RunGet)
		r.Post("/threads/{thread_id}/runs/{run_id}/cancel", s.Handlers.RunCancel)
		r.Get("/threads/{thread_id}/runs/{run_id}/steps", s.Handlers.RunStepsList)
		r.Get("/threads/{thread_id}/runs/{run_id}/steps/{step_id}", s.Handlers.RunStepGet)

		// Vector Stores API
		r.Post("/vector_stores", s.Handlers.VectorStoreCreate)
		r.Get("/vector_stores", s.Handlers.VectorStoreList)
		r.Get("/vector_stores/{vector_store_id}", s.Handlers.VectorStoreGet)
		r.Delete("/vector_stores/{vector_store_id}", s.Handlers.VectorStoreDelete)
		r.Post("/vector_stores/{vector_store_id}/files", s.Handlers.VectorStoreFilesCreate)
		r.Get("/vector_stores/{vector_store_id}/files", s.Handlers.VectorStoreFilesList)
		r.Get("/vector_stores/{vector_store_id}/files/{file_id}", s.Handlers.VectorStoreFilesGet)
		r.Delete("/vector_stores/{vector_store_id}/files/{file_id}", s.Handlers.VectorStoreFilesDelete)
		r.Post("/vector_stores/{vector_store_id}/search", s.Handlers.VectorStoreSearch)

		// Search API
		r.Post("/search/{search_tool_name}", s.Handlers.SearchHandler)

		// Responses API extensions
		r.Get("/responses/{response_id}", s.Handlers.GetResponse)
		r.Post("/responses/{response_id}/cancel", s.Handlers.CancelResponse)
		r.Get("/responses/{response_id}/input_items", s.Handlers.ListResponseInputItems)

		// Anthropic native format
		r.Post("/messages", s.Handlers.AnthropicMessages)
		r.Post("/messages/count_tokens", s.Handlers.AnthropicCountTokens)

		// Images edit + variations
		r.Post("/images/edits", s.Handlers.ImagesEdit)
		r.Post("/images/variations", s.Handlers.ImageVariation)

		// OCR API
		r.Post("/ocr", s.Handlers.OCRProcess)

		// Videos API
		r.Post("/videos", s.Handlers.VideoCreate)
		r.Get("/videos/{video_id}", s.Handlers.VideoGet)
		r.Get("/videos/{video_id}/content", s.Handlers.VideoContent)

		// Containers API
		r.Post("/containers", s.Handlers.ContainerCreate)
		r.Get("/containers", s.Handlers.ContainerList)
		r.Get("/containers/{container_id}", s.Handlers.ContainerGet)
		r.Delete("/containers/{container_id}", s.Handlers.ContainerDelete)
		r.HandleFunc("/containers/{container_id}/files/*", s.Handlers.ContainerFiles)

		// Agents API
		r.Post("/agents", s.Handlers.AgentCreate)
		r.Get("/agents", s.Handlers.AgentList)
		r.Get("/agents/{agent_id}", s.Handlers.AgentGet)
		r.Put("/agents/{agent_id}", s.Handlers.AgentUpdate)
		r.Patch("/agents/{agent_id}", s.Handlers.AgentPatch)
		r.Delete("/agents/{agent_id}", s.Handlers.AgentDelete)

		// Skills API
		r.Post("/skills", s.Handlers.SkillCreate)
		r.Get("/skills", s.Handlers.SkillList)
		r.Get("/skills/{skill_id}", s.Handlers.SkillGet)
		r.Delete("/skills/{skill_id}", s.Handlers.SkillDelete)

		// RAG API
		r.Post("/rag/ingest", s.Handlers.RAGIngest)
		r.Post("/rag/query", s.Handlers.RAGQuery)

		// Realtime API (WebSocket)
		if s.Handlers.RealtimeRelay != nil {
			r.Handle("/realtime", s.Handlers.RealtimeRelay)
		}
	}

	// /v1/* — canonical paths
	r.Route("/v1", func(r chi.Router) {
		llmMiddleware(r)
		registerLLMRoutes(r)

		// Pass-through proxy (catch-all for provider-specific routes)
		if s.PassthroughHandler != nil {
			r.Handle("/{provider}/*", s.PassthroughHandler)
		} else {
			r.HandleFunc("/{provider}/*", handler.NotImplemented("/v1/{provider}/*"))
		}
	})

	// Bare paths (no /v1 prefix) — most SDKs and clients use these
	r.Group(func(r chi.Router) {
		llmMiddleware(r)
		registerLLMRoutes(r)
	})

	// Azure-style engine/deployment paths
	r.Route("/engines/{model}", func(r chi.Router) {
		llmMiddleware(r)
		r.Post("/chat/completions", s.Handlers.ChatCompletion)
		r.Post("/completions", s.Handlers.Completion)
		r.Post("/embeddings", s.Handlers.Embedding)
	})
	r.Route("/openai/deployments/{model}", func(r chi.Router) {
		llmMiddleware(r)
		r.Post("/chat/completions", s.Handlers.ChatCompletion)
		r.Post("/completions", s.Handlers.Completion)
		r.Post("/embeddings", s.Handlers.Embedding)
	})

	// Key management (auth required, master key)
	r.Route("/key", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/generate", s.Handlers.KeyGenerateHandler)
		r.Get("/info", s.Handlers.KeyInfo)
		r.Get("/list", s.Handlers.KeyList)
		r.Post("/delete", s.Handlers.KeyDelete)
		r.Post("/block", s.Handlers.KeyBlock)
		r.Post("/unblock", s.Handlers.KeyUnblock)
		r.Post("/update", s.Handlers.KeyUpdate)
		r.Post("/regenerate", s.Handlers.KeyRegenerate)
		r.Post("/bulk_update", s.Handlers.KeyBulkUpdate)
		r.Get("/health", s.Handlers.KeyHealthCheck)
		r.Post("/service-account/generate", s.Handlers.ServiceAccountKeyGenerate)
		r.Post("/{key}/reset_spend", s.Handlers.ResetKeySpend)
		r.Get("/aliases", s.Handlers.KeyAliases)
	})

	// Key info v2 (batch lookup)
	r.Route("/v2/key", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/info", s.Handlers.KeyInfoV2)
	})

	// Team management
	r.Route("/team", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.TeamNew)
		r.Get("/info", s.Handlers.TeamInfo)
		r.Get("/info/{team_id}", s.Handlers.TeamInfo)
		r.Get("/list", s.Handlers.TeamList)
		r.Post("/delete", s.Handlers.TeamDelete)
		r.Post("/update", s.Handlers.TeamUpdate)
		r.Post("/block", s.Handlers.TeamBlock)
		r.Post("/unblock", s.Handlers.TeamUnblock)
		r.Get("/daily_activity", s.Handlers.TeamDailyActivity)
		r.Post("/member/add", s.Handlers.TeamMemberAdd)
		r.Post("/member_add", s.Handlers.TeamMemberAdd) // Python compat: underscore variant
		r.Post("/member/delete", s.Handlers.TeamMemberDelete)
		r.Post("/member_delete", s.Handlers.TeamMemberDelete) // Python compat: underscore variant
		r.Post("/member_update", s.Handlers.TeamMemberUpdate)
		r.Post("/model/add", s.Handlers.TeamModelAdd)
		r.Post("/model/remove", s.Handlers.TeamModelRemove)
		r.Get("/available", s.Handlers.TeamAvailable)
		r.Get("/permissions", s.Handlers.TeamPermissionsList)
		r.Post("/permissions", s.Handlers.TeamPermissionsUpdate)
		r.Post("/callback", s.Handlers.TeamCallbackSet)
		r.Get("/callback", s.Handlers.TeamCallbackGet)
		r.Get("/{team_id}/callback", s.Handlers.TeamCallbackGet) // Python compat: /team/{team_id}/callback
		r.Post("/{team_id}/reset_spend", s.Handlers.ResetTeamSpend)
	})

	// User management
	r.Route("/user", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.UserNew)
		r.Get("/info", s.Handlers.UserInfo)
		r.Get("/list", s.Handlers.UserList)
		r.Post("/update", s.Handlers.UserUpdate)
		r.Post("/delete", s.Handlers.UserDelete)
		r.Get("/daily_activity", s.Handlers.UserDailyActivity)
		r.Get("/daily/activity", s.Handlers.UserDailyActivity) // Python compat: /user/daily/activity
	})

	// Budget management
	r.Route("/budget", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.BudgetNew)
		r.Get("/info", s.Handlers.BudgetInfo)
		r.Post("/info", s.Handlers.BudgetInfo) // Python compat: POST /budget/info
		r.Post("/update", s.Handlers.BudgetUpdate)
		r.Get("/list", s.Handlers.BudgetList)
		r.Post("/delete", s.Handlers.BudgetDelete)
		r.Get("/settings", s.Handlers.BudgetSettings)
	})

	// Spend queries
	r.Route("/spend", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/keys", s.Handlers.SpendKeys)
		r.Get("/users", s.Handlers.SpendUsers)
		r.Get("/teams", s.Handlers.SpendByTeams)
		r.Get("/tags", s.Handlers.SpendByTags)
		r.Get("/models", s.Handlers.SpendByModels)
		r.Get("/end_users", s.Handlers.SpendByEndUsers)
		r.Get("/analytics", s.Handlers.SpendAnalytics)
		r.Get("/top", s.Handlers.SpendTopN)
		r.Get("/trend", s.Handlers.SpendTrend)
		r.Get("/logs", s.Handlers.SpendLogs)
	})

	// Global spend
	r.Route("/global", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/spend", s.Handlers.GlobalSpend)
		r.Get("/spend/keys", s.Handlers.GlobalSpendByKeys)
		r.Get("/spend/models", s.Handlers.GlobalSpendByModels)
		r.Get("/spend/teams", s.Handlers.GlobalSpendByTeams)
		r.Get("/spend/tags", s.Handlers.GlobalSpendByTags)
		r.Get("/spend/provider", s.Handlers.GlobalSpendByProvider)
		r.Get("/spend/report", s.Handlers.GlobalSpendReport)
		r.Post("/spend/reset", s.Handlers.GlobalSpendReset)
		r.Get("/activity", s.Handlers.GlobalActivity)
		r.Get("/activity/model", s.Handlers.GlobalActivityByModel)
		r.Get("/activity/cache_hits", s.Handlers.CacheHitStats)
	})

	// Audit logs
	r.Route("/audit", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/", s.Handlers.AuditLogList)
		r.Get("/{id}", s.Handlers.AuditLogGet)
	})

	// Organization management
	r.Route("/organization", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.OrgNew)
		r.Get("/info", s.Handlers.OrgInfo)
		r.Post("/update", s.Handlers.OrgUpdate)
		r.Delete("/delete/{organization_id}", s.Handlers.OrgDelete)
		r.Post("/member_add", s.Handlers.OrgMemberAdd)
		r.Patch("/member_update", s.Handlers.OrgMemberUpdate)
		r.Delete("/member_delete", s.Handlers.OrgMemberDelete)
	})

	// Credential management
	r.Route("/credentials", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.CredentialNew)
		r.Get("/list", s.Handlers.CredentialList)
		r.Get("/info/{credential_id}", s.Handlers.CredentialInfo)
		r.Post("/update", s.Handlers.CredentialUpdate)
		r.Delete("/delete/{credential_id}", s.Handlers.CredentialDelete)
	})

	// Access Group management
	r.Route("/model_access_group", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.AccessGroupNew)
		r.Get("/info/{group_id}", s.Handlers.AccessGroupInfo)
		r.Post("/update", s.Handlers.AccessGroupUpdate)
		r.Delete("/delete/{group_id}", s.Handlers.AccessGroupDelete)
	})

	// Policy management
	r.Route("/policy", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.PolicyCreate)
		r.Get("/list", s.Handlers.PolicyList)
		r.Get("/resolved-guardrails", s.Handlers.PolicyResolvedGuardrails)
		r.Post("/test-pipeline", s.Handlers.PolicyTestPipeline)
		r.Get("/{id}", s.Handlers.PolicyGet)
		r.Put("/{id}", s.Handlers.PolicyUpdate)
		r.Delete("/{id}", s.Handlers.PolicyDelete)

		// Policy attachments
		r.Post("/attachment", s.Handlers.PolicyAttachmentCreate)
		r.Get("/attachment/list", s.Handlers.PolicyAttachmentList)
		r.Get("/attachment/{id}", s.Handlers.PolicyAttachmentGet)
		r.Delete("/attachment/{id}", s.Handlers.PolicyAttachmentDelete)
	})

	// Model management
	r.Route("/model", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.ModelNew)
		r.Get("/info", s.Handlers.ModelInfo)
		r.Post("/update", s.Handlers.ModelUpdate)
		r.Post("/delete", s.Handlers.ModelDelete)
	})

	// Tag management
	r.Route("/tag", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.TagNew)
		r.Get("/info/{id}", s.Handlers.TagInfo)
		r.Get("/list", s.Handlers.TagList)
		r.Post("/update", s.Handlers.TagUpdate)
		r.Post("/delete", s.Handlers.TagDelete)
	})

	// End user (customer) management
	r.Route("/end_user", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/new", s.Handlers.EndUserNew)
		r.Get("/info/{id}", s.Handlers.EndUserInfo)
		r.Get("/list", s.Handlers.EndUserList)
		r.Post("/update", s.Handlers.EndUserUpdate)
		r.Post("/delete", s.Handlers.EndUserDelete)
		r.Post("/block", s.Handlers.EndUserBlock)
		r.Post("/unblock", s.Handlers.EndUserUnblock)
	})

	// Config management
	r.Route("/config", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/", s.Handlers.ConfigGet)
		r.Post("/update", s.Handlers.ConfigUpdate)
	})

	// Callback management
	r.Route("/callback", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/list", s.Handlers.CallbackList)
	})

	// Cache management
	r.Route("/cache", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/ping", s.Handlers.CachePing)
		r.Post("/delete", s.Handlers.CacheDelete)
		r.Post("/flushall", s.Handlers.CacheFlushAll)
	})

	// Router settings
	r.Route("/router", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/settings", s.Handlers.RouterSettingsGet)
		r.Patch("/settings", s.Handlers.RouterSettingsPatch)
	})

	// Guardrail management
	r.Route("/guardrails", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.GuardrailCreate)
		r.Get("/list", s.Handlers.GuardrailList)
		r.Get("/{id}", s.Handlers.GuardrailGet)
		r.Put("/{id}", s.Handlers.GuardrailUpdate)
		r.Delete("/{id}", s.Handlers.GuardrailDelete)
	})

	// Prompt management
	r.Route("/prompts", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.PromptCreate)
		r.Get("/", s.Handlers.PromptList)
		r.Post("/test", s.Handlers.PromptTest)
		r.Get("/{id}", s.Handlers.PromptGet)
		r.Put("/{id}", s.Handlers.PromptUpdate)
		r.Delete("/{id}", s.Handlers.PromptDelete)
		r.Get("/{id}/versions", s.Handlers.PromptVersions)
	})

	// IP whitelist management
	r.Route("/ip", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/add", s.Handlers.IPAdd)
		r.Delete("/delete", s.Handlers.IPDelete)
		r.Get("/list", s.Handlers.IPList)
	})

	// MCP server management (CRUD)
	r.Route("/mcp_server", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.MCPServerCreate)
		r.Get("/list", s.Handlers.MCPServerList)
		r.Get("/{id}", s.Handlers.MCPServerGet)
		r.Put("/{id}", s.Handlers.MCPServerUpdate)
		r.Delete("/{id}", s.Handlers.MCPServerDelete)
	})

	// MCP protocol transports (SSE, Streamable HTTP, REST)
	if s.MCPSSEHandler != nil {
		r.Route("/mcp/sse", func(r chi.Router) {
			r.Use(s.AuthMiddleware)
			r.Mount("/", s.MCPSSEHandler)
		})
	}
	if s.MCPStreamHandler != nil {
		r.Route("/mcp", func(r chi.Router) {
			r.Use(s.AuthMiddleware)
			r.Mount("/", s.MCPStreamHandler)
		})
	}
	if s.MCPRESTHandler != nil {
		r.Route("/mcp-rest", func(r chi.Router) {
			r.Use(s.AuthMiddleware)
			r.Mount("/", s.MCPRESTHandler)
		})
	}

	// Gemini native format (auth required)
	r.Route("/v1beta/models", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/{model}:generateContent", s.Handlers.GeminiGenerateContent)
		r.Post("/{model}:streamGenerateContent", s.Handlers.GeminiStreamGenerateContent)
		r.Post("/{model}:countTokens", s.Handlers.GeminiCountTokens)
	})

	// SCIM 2.0 (auth required — IDP provisioning)
	if s.SCIMHandler != nil {
		r.Route("/scim/v2", func(r chi.Router) {
			r.Use(s.AuthMiddleware)
			r.Mount("/", s.SCIMHandler)
		})
	}

	// A2A Protocol (agent-to-agent)
	r.Route("/a2a/{id}", func(r chi.Router) {
		r.Get("/.well-known/agent-card.json", s.Handlers.A2AAgentCard)
		r.Post("/", s.Handlers.A2AMessage)
	})

	// Discovery (auth required)
	r.Route("/model_group", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/info", s.Handlers.ModelGroupInfo)
	})

	// Claude Code Marketplace (public discovery)
	r.Get("/claude-code/marketplace.json", s.Handlers.MarketplaceJSON)

	// Claude Code Plugins (auth required)
	r.Route("/claude-code/plugins", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.PluginCreate)
		r.Get("/", s.Handlers.PluginList)
		r.Get("/{name}", s.Handlers.PluginGet)
		r.Post("/{name}/enable", s.Handlers.PluginEnable)
		r.Post("/{name}/disable", s.Handlers.PluginDisable)
		r.Delete("/{name}", s.Handlers.PluginDelete)
	})

	// Public endpoints (no auth)
	r.Get("/public/providers", s.Handlers.PublicProviders)
	r.Get("/public/tianji_model_cost_map", s.Handlers.PublicModelCostMap)

	// Config v2
	r.Route("/v2/config", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/", s.Handlers.ConfigV2Get)
	})

	// Fallback management
	r.Route("/fallback", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.FallbackCreate)
		r.Get("/{model}", s.Handlers.FallbackGet)
		r.Delete("/{model}", s.Handlers.FallbackDelete)
	})

	// Anthropic batches pass-through
	r.Route("/anthropic/v1/messages/batches", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Post("/", s.Handlers.AnthropicBatchesCreate)
		r.Get("/{id}", s.Handlers.AnthropicBatchesGet)
		r.Get("/{id}/results", s.Handlers.AnthropicBatchesResults)
	})

	// Misc endpoints (auth required)
	r.Route("/", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/routes", s.Handlers.RoutesList)
		r.Post("/transform_request", s.Handlers.TransformRequest)
		r.Get("/health/checks", s.Handlers.HealthCheckHistory)
		r.Get("/errors", s.Handlers.ErrorLogsList)
	})

	// Utility endpoints
	r.Route("/utils", func(r chi.Router) {
		r.Use(s.AuthMiddleware)
		r.Get("/supported_openai_params", s.Handlers.SupportedOpenAIParams)
		r.Post("/token_counter", s.Handlers.TokenCount)
	})

	// SSO (no auth — these are the login entry points)
	r.Route("/sso", func(r chi.Router) {
		r.Get("/login", s.Handlers.SSOLogin)
		r.Get("/callback", s.Handlers.SSOCallback)
	})
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Router.ServeHTTP(w, r)
}
