package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/praxisllmlab/tianjiLLM/internal/a2a"
	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	dbmigrate "github.com/praxisllmlab/tianjiLLM/internal/db/migrate"
	"github.com/praxisllmlab/tianjiLLM/internal/guardrail"
	"github.com/praxisllmlab/tianjiLLM/internal/mcp"
	"github.com/praxisllmlab/tianjiLLM/internal/policy"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/openaicompat"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/hook"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/passthrough"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy/auto"
	"github.com/praxisllmlab/tianjiLLM/internal/scheduler"
	"github.com/praxisllmlab/tianjiLLM/internal/spend"
	"github.com/praxisllmlab/tianjiLLM/internal/ui"

	// Register all providers via init()
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/ai21"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/anthropic"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/azure"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/bedrock"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/cerebras"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/cloudflare"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/cohere"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/databricks"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/deepseek"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/fireworks"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/gemini"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/github"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/groq"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/huggingface"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/mistral"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/perplexity"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/replicate"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/sagemaker"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/sambanova"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/together"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/vertexai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/watsonx"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/xai"

	// Search providers (self-register via init())
	_ "github.com/praxisllmlab/tianjiLLM/internal/search"

	// Phase 3 providers
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/azureai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/dashscope"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/deepinfra"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/githubcopilot"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/minimax"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/moonshot"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/nvidia"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/oci"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openrouter"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/sap"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/snowflake"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/volcengine"

	// Phase 5 providers
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/awspolly"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/baseten"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/codestral"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/deepgram"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/elevenlabs"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/falai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/friendliai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/gigachat"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/hostedvllm"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/infinity"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/jina"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/lambdaai"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/nebius"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/nscale"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/ovhcloud"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/recraft"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/stability"
	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/voyage"
)

func main() {
	configPath := flag.String("config", "proxy_config.yaml", "path to proxy config YAML")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load config
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	log.Printf("loaded %d models from config", len(cfg.ModelList))

	// Load JSON-configured providers (optional)
	providersPath := filepath.Join(filepath.Dir(*configPath), "providers.json")
	if _, err := os.Stat(providersPath); err == nil {
		if err := openaicompat.LoadProviders(providersPath); err != nil {
			log.Printf("warn: failed to load providers.json: %v", err)
		} else {
			log.Printf("loaded providers from %s", providersPath)
		}
	}

	// Init DB (optional — skip if no database_url)
	var queries *db.Queries
	var dbPool *pgxpool.Pool
	if cfg.GeneralSettings.DatabaseURL != "" {
		pool, err := pgxpool.New(ctx, cfg.GeneralSettings.DatabaseURL)
		if err != nil {
			log.Fatalf("connect database: %v", err)
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			log.Fatalf("ping database: %v", err)
		}
		log.Println("database connected")

		log.Println("running database migrations...")
		if err := dbmigrate.RunMigrations(ctx, pool); err != nil {
			log.Fatalf("migration failed: %v", err)
		}
		log.Println("migrations complete")

		queries = db.New(pool)
		dbPool = pool
	}

	// Init cache (config-driven)
	var cacheBackend cache.Cache
	var redisClient redis.UniversalClient
	memCache := cache.NewMemoryCache()

	if cfg.TianjiSettings.Cache {
		if cfg.TianjiSettings.CacheParams != nil && cfg.TianjiSettings.CacheParams.Type == "redis_cluster" {
			cluster := cache.NewRedisCluster(cfg.TianjiSettings.CacheParams.Addrs, cfg.TianjiSettings.CacheParams.Password)
			cacheBackend = cluster
			log.Println("redis cluster cache configured")
		} else {
			rc, err := cache.NewRedisClient(ctx)
			if err != nil {
				log.Printf("warn: redis not available, using memory-only cache: %v", err)
				cacheBackend = memCache
			} else {
				log.Println("redis connected")
				redisClient = rc
				redisCache := cache.NewRedisCache(redisClient)
				cacheBackend = cache.NewDualCache(memCache, redisCache)
			}
		}
	} else {
		cacheBackend = memCache
	}

	// Init callbacks (config-driven)
	callbackRegistry := callback.NewRegistry()
	for _, name := range cfg.TianjiSettings.Callbacks {
		cb, err := callback.NewFromConfig(name, "", "", "", "", "", "", "", "", "")
		if err != nil {
			log.Printf("warn: callback %q: %v", name, err)
			continue
		}
		callbackRegistry.Register(cb)
		log.Printf("callback registered: %s", name)
	}
	for _, cc := range cfg.TianjiSettings.CallbackConfigs {
		cb, err := callback.NewFromConfig(cc.Type, cc.APIKey, cc.BaseURL, cc.Project, cc.Entity, cc.Bucket, cc.Prefix, cc.Region, cc.QueueURL, cc.TableName)
		if err != nil {
			log.Printf("warn: callback config %q: %v", cc.Type, err)
			continue
		}
		callbackRegistry.Register(cb)
		log.Printf("callback registered: %s", cc.Type)
	}

	// Register spend tracker as callback (writes SpendLogs to DB)
	if queries != nil {
		spendTracker := spend.NewTracker(queries, nil)
		callbackRegistry.Register(spendTracker)
		log.Println("spend tracker registered")
	}

	// Init guardrails (config-driven)
	guardrailRegistry := guardrail.NewRegistry()
	for _, gc := range cfg.Guardrails {
		g, err := guardrail.NewFromConfig(gc)
		if err != nil {
			log.Printf("warn: guardrail %q: %v", gc.GuardrailName, err)
			continue
		}
		failOpen := gc.FailurePolicy == "fail_open"
		guardrailRegistry.RegisterWithPolicy(g, failOpen)
		log.Printf("guardrail registered: %s (fail_open=%v)", gc.GuardrailName, failOpen)
	}

	// Init router (config-driven)
	var rtr *router.Router
	if cfg.RouterSettings != nil {
		strategyName := cfg.RouterSettings.RoutingStrategy
		routeStrategy, err := strategy.NewFromConfig(strategyName)
		if err != nil {
			log.Printf("warn: routing strategy %q: %v, using shuffle", strategyName, err)
			routeStrategy = strategy.NewShuffle()
		}

		settings := router.RouterSettings{
			NumRetries: 2,
		}
		if cfg.RouterSettings.NumRetries != nil {
			settings.NumRetries = *cfg.RouterSettings.NumRetries
		}
		if cfg.RouterSettings.AllowedFails != nil {
			settings.AllowedFails = *cfg.RouterSettings.AllowedFails
		}
		if cfg.RouterSettings.CooldownTime != nil {
			settings.CooldownTime = time.Duration(*cfg.RouterSettings.CooldownTime) * time.Second
		}

		// Wire model group alias
		settings.ModelGroupAlias = parseModelGroupAlias(cfg.RouterSettings.ModelGroupAlias)

		// Wire fallbacks (from both router_settings and tianji_settings)
		settings.Fallbacks = parseFallbackMaps(cfg.RouterSettings.Fallbacks)
		if len(settings.Fallbacks) == 0 {
			settings.Fallbacks = parseFallbackStringMaps(cfg.TianjiSettings.Fallbacks)
		}
		settings.DefaultFallbacks = cfg.RouterSettings.DefaultFallbacks
		if len(settings.DefaultFallbacks) == 0 {
			settings.DefaultFallbacks = cfg.TianjiSettings.DefaultFallbacks
		}
		settings.ContentPolicyFallbacks = parseFallbackMaps(cfg.RouterSettings.ContentPolicyFallbacks)

		// Wire context window fallbacks
		settings.ContextWindowFallbacks = parseFallbackStringMaps(cfg.TianjiSettings.ContextWindowFallbacks)

		// Wire per-group retry policy
		settings.ModelGroupRetryPolicy = parseRetryPolicies(cfg.RouterSettings.ModelGroupRetryPolicy)

		// Wire tag filtering
		settings.EnableTagFiltering = cfg.RouterSettings.EnableTagFiltering
		settings.TagFilteringMatchAny = cfg.RouterSettings.TagFilteringMatchAny

		rtr = router.New(cfg.ModelList, routeStrategy, settings)
		log.Printf("router configured: strategy=%s", strategyName)

		// Init auto-routers for model entries with "auto_router/" prefix
		initAutoRouters(cfg, rtr)
	}

	// Init policy engine (requires DB)
	var policyEng *policy.Engine
	if queries != nil {
		policyEng = policy.NewEngine(queries)
		if err := policyEng.Load(ctx); err != nil {
			log.Printf("warn: failed to load policies: %v", err)
		} else {
			log.Println("policy engine loaded")
		}
	}

	// Init SSO (optional — skip if not configured)
	var ssoHandler *handler.SSOHandler
	if cfg.GeneralSettings.SSOClientID != "" && cfg.GeneralSettings.SSOIssuerURL != "" {
		roleMapping := make(map[string]auth.Role, len(cfg.GeneralSettings.SSORoleMapping))
		for group, role := range cfg.GeneralSettings.SSORoleMapping {
			roleMapping[group] = auth.Role(role)
		}
		ssoHandler = &handler.SSOHandler{
			SSO: auth.NewSSOHandler(auth.SSOConfig{
				ClientID:     cfg.GeneralSettings.SSOClientID,
				ClientSecret: cfg.GeneralSettings.SSOClientSecret,
				AuthURL:      cfg.GeneralSettings.SSOIssuerURL + "/authorize",
				TokenURL:     cfg.GeneralSettings.SSOIssuerURL + "/oauth/token",
				UserInfoURL:  cfg.GeneralSettings.SSOIssuerURL + "/userinfo",
				RedirectURI:  cfg.GeneralSettings.SSORedirectURI,
				Scopes:       cfg.GeneralSettings.SSOScopes,
				RoleMapping:  roleMapping,
			}),
		}
		log.Println("SSO configured")
	}

	// Init pass-through proxy (built-in providers + config-driven endpoints)
	var passthroughHandler http.Handler
	{
		var endpoints []passthrough.Endpoint
		// Built-in provider pass-through routes
		builtins := map[string]string{
			"/v1/anthropic": "https://api.anthropic.com",
			"/v1/vertex-ai": "https://us-central1-aiplatform.googleapis.com",
			"/v1/bedrock":   "https://bedrock-runtime.us-east-1.amazonaws.com",
			"/v1/azure":     "", // requires api_base from config
			"/v1/gemini":    "https://generativelanguage.googleapis.com",
		}
		for path, target := range builtins {
			if target != "" {
				providerName := strings.TrimPrefix(path, "/v1/")
				endpoints = append(endpoints, passthrough.Endpoint{
					Path:     path,
					Target:   target,
					Provider: providerName,
				})
			}
		}
		// Config-driven pass-through endpoints
		for _, pt := range cfg.PassThroughEndpoints {
			endpoints = append(endpoints, passthrough.Endpoint{
				Path:   pt.Path,
				Target: pt.Target,
			})
		}
		if len(endpoints) > 0 {
			ptRouter := passthrough.NewRouter(endpoints, nil)
			ptRouter.RegisterLogger("anthropic", &passthrough.AnthropicLoggingHandler{})
			ptRouter.RegisterLogger("vertex-ai", &passthrough.VertexAILoggingHandler{})
			ptRouter.RegisterLogger("gemini", &passthrough.GeminiLoggingHandler{})
			passthroughHandler = ptRouter.Handler()
			log.Printf("pass-through proxy configured with %d endpoints", len(endpoints))
		}
	}

	// Init MCP (config-driven)
	var mcpSSEHandler, mcpStreamHandler, mcpRESTHandler http.Handler
	if len(cfg.MCPServers) > 0 {
		mcpManager := mcp.NewManager()
		entries := make(map[string]mcp.MCPServerEntry, len(cfg.MCPServers))
		for id, sc := range cfg.MCPServers {
			entries[id] = mcp.MCPServerEntry{
				Transport:       sc.Transport,
				URL:             sc.URL,
				Command:         sc.Command,
				Args:            sc.Args,
				AuthType:        sc.AuthType,
				AuthToken:       sc.AuthToken,
				StaticHeaders:   sc.StaticHeaders,
				AllowedTools:    sc.AllowedTools,
				DisallowedTools: sc.DisallowedTools,
			}
		}
		if err := mcpManager.LoadFromConfig(ctx, entries); err != nil {
			log.Printf("warn: MCP server load: %v", err)
		}

		mcpServer := mcp.NewMCPServer(mcpManager)
		mcp.SyncTools(mcpServer, mcpManager)

		mcpSSEHandler = mcp.NewSSEHandler(mcpServer)
		mcpStreamHandler = mcp.NewStreamableHTTPHandler(mcpServer)
		mcpRESTHandler = (&mcp.RESTHandler{Manager: mcpManager}).Handler()
		log.Printf("MCP configured with %d upstream servers", len(cfg.MCPServers))
	}

	// Create handlers
	// Init A2A agent registry
	agentRegistry := a2a.NewAgentRegistry()
	if queries != nil {
		if err := agentRegistry.LoadFromDB(ctx, queries); err != nil {
			log.Printf("warn: failed to load agents from DB: %v", err)
		}
	}

	eventDispatcher := hook.NewManagementEventDispatcher(cfg.GeneralSettings.ManagementWebhookURL)

	discordAlerter := callback.NewDiscordRateLimitAlerter(cfg.DiscordWebhookURL, cfg.RatelimitAlertThreshold)
	rateLimitStore := callback.NewInMemoryRateLimitStore()
	// FR-015: prune stale entries every minute.
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimitStore.Prune(5 * time.Minute)
		}
	}()

	handlers := &handler.Handlers{
		Config:          cfg,
		DB:              queries,
		Cache:           cacheBackend,
		Router:          rtr,
		Callbacks:       callbackRegistry,
		Guardrails:      guardrailRegistry,
		PolicyEng:       policyEng,
		SSOHandler:      ssoHandler,
		AgentRegistry:   agentRegistry,
		EventDispatcher: eventDispatcher,
		DiscordAlerter:  discordAlerter,
		RateLimitStore:  rateLimitStore,
	}

	// Init scheduler
	sched := scheduler.New()
	if queries != nil {
		sched.AddWithStartupRun(&scheduler.BudgetResetJob{DB: queries}, 1*time.Minute)
		sched.Add(&scheduler.SpendLogCleanupJob{DB: queries, Retention: 90 * 24 * time.Hour}, 24*time.Hour)
		if policyEng != nil {
			sched.Add(&scheduler.PolicyHotReloadJob{Engine: policyEng}, 30*time.Second)
		}
	}
	sched.Start()

	// Init pricing calculator — always non-nil, regardless of DB availability.
	pricingCalc := pricing.Default()
	if queries != nil {
		entries, err := queries.ListModelPricing(ctx)
		if err != nil {
			log.Printf("warn: failed to load DB pricing on startup: %v", err)
		} else if len(entries) > 0 {
			pricingCalc.ReloadFromDB(entries)
			log.Printf("loaded %d model prices from database", len(entries))
		}
	}

	// Init admin dashboard UI
	uiHandler := &ui.UIHandler{
		DB:             queries,
		Pool:           dbPool,
		Config:         cfg,
		Cache:          cacheBackend,
		MasterKey:      cfg.GeneralSettings.MasterKey,
		Pricing:        pricingCalc,
		RateLimitStore: rateLimitStore,
	}

	// Create server
	var dbValidator middleware.TokenValidator
	if queries != nil {
		dbValidator = &middleware.DBValidator{DB: queries}
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:           handlers,
		MasterKey:          cfg.GeneralSettings.MasterKey,
		DBQueries:          dbValidator,
		RedisClient:        redisClient,
		PassthroughHandler: passthroughHandler,
		MCPSSEHandler:      mcpSSEHandler,
		MCPStreamHandler:   mcpStreamHandler,
		MCPRESTHandler:     mcpRESTHandler,
		UIHandler:          uiHandler,
	})

	// Start HTTP server
	addr := fmt.Sprintf(":%d", cfg.GeneralSettings.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("shutting down...")
		sched.Stop()

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
		cancel()
	}()

	log.Printf("tianjiLLM listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// parseModelGroupAlias converts config map[string]any to typed alias map.
// Supports both string shorthand ("alias": "model") and object form ("alias": {"model": "x", "hidden": true}).
func parseModelGroupAlias(raw map[string]any) map[string]router.ModelGroupAliasItem {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]router.ModelGroupAliasItem, len(raw))
	for alias, v := range raw {
		switch val := v.(type) {
		case string:
			result[alias] = router.ModelGroupAliasItem{Model: val}
		case map[string]any:
			item := router.ModelGroupAliasItem{}
			if m, ok := val["model"].(string); ok {
				item.Model = m
			}
			if h, ok := val["hidden"].(bool); ok {
				item.Hidden = h
			}
			result[alias] = item
		}
	}
	return result
}

// parseFallbackMaps converts []map[string]any to map[string][]string.
// Input format: [{"gpt-4": ["claude-3", "gemini"]}]
func parseFallbackMaps(raw []map[string]any) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string][]string)
	for _, m := range raw {
		for k, v := range m {
			if arr, ok := v.([]any); ok {
				strs := make([]string, 0, len(arr))
				for _, item := range arr {
					if s, ok := item.(string); ok {
						strs = append(strs, s)
					}
				}
				result[k] = strs
			}
		}
	}
	return result
}

// parseFallbackStringMaps converts []map[string][]string to map[string][]string.
func parseFallbackStringMaps(raw []map[string][]string) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string][]string)
	for _, m := range raw {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// initAutoRouters scans model_list for entries with "auto_router/" prefix,
// parses their route config, creates AutoRouter instances, and registers them.
func initAutoRouters(cfg *config.ProxyConfig, rtr *router.Router) {
	for _, m := range cfg.ModelList {
		if !strings.HasPrefix(m.ModelName, "auto_router/") {
			continue
		}

		// Parse routes from inline JSON or file path
		var routes []auto.Route
		if m.TianjiParams.AutoRouterConfig != "" {
			if err := json.Unmarshal([]byte(m.TianjiParams.AutoRouterConfig), &routes); err != nil {
				log.Printf("warn: auto_router %q: parse config: %v", m.ModelName, err)
				continue
			}
		} else if m.TianjiParams.AutoRouterConfigPath != "" {
			data, err := os.ReadFile(m.TianjiParams.AutoRouterConfigPath)
			if err != nil {
				log.Printf("warn: auto_router %q: read config file: %v", m.ModelName, err)
				continue
			}
			if err := json.Unmarshal(data, &routes); err != nil {
				log.Printf("warn: auto_router %q: parse config file: %v", m.ModelName, err)
				continue
			}
		} else {
			log.Printf("warn: auto_router %q: no route config provided", m.ModelName)
			continue
		}

		if len(routes) == 0 {
			log.Printf("warn: auto_router %q: empty routes", m.ModelName)
			continue
		}

		embeddingModel := m.TianjiParams.AutoRouterEmbeddingModel
		if embeddingModel == "" {
			embeddingModel = "text-embedding-3-small"
		}
		defaultModel := m.TianjiParams.AutoRouterDefaultModel
		if defaultModel == "" && len(routes) > 0 {
			defaultModel = routes[0].Model
		}

		// Encoder calls our own proxy embedding endpoint (loopback)
		baseURL := fmt.Sprintf("http://localhost:%d", cfg.GeneralSettings.Port)
		encoder := auto.NewEncoder(embeddingModel, baseURL, cfg.GeneralSettings.MasterKey)

		ar := auto.New(routes, encoder, defaultModel, 0)
		rtr.RegisterAutoRouter(m.ModelName, ar.Route)
		log.Printf("auto_router registered: %s (%d routes, default=%s)", m.ModelName, len(routes), defaultModel)
	}
}

// parseRetryPolicies converts config map[string]any to typed retry policies.
func parseRetryPolicies(raw map[string]any) map[string]router.RetryPolicy {
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]router.RetryPolicy, len(raw))
	for model, v := range raw {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		p := router.RetryPolicy{}
		if n, ok := m["num_retries"].(int); ok {
			p.NumRetries = n
		}
		if t, ok := m["timeout"].(int); ok {
			p.TimeoutSeconds = t
		}
		if r, ok := m["retry_after"].(int); ok {
			p.RetryAfterSeconds = r
		}
		result[model] = p
	}
	return result
}
