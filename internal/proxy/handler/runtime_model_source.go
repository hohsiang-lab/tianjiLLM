package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/praxisllmlab/tianjiLLM/internal/router/strategy"
)

type RuntimeModelSource struct {
	config *config.ProxyConfig
	db     db.Store
	router *router.Router
	value  atomic.Value
}

type runtimeModelSnapshot struct {
	models []config.ModelConfig
	router *router.Router
}

func NewRuntimeModelSource(ctx context.Context, cfg *config.ProxyConfig, store db.Store, baseRouter *router.Router) *RuntimeModelSource {
	source := &RuntimeModelSource{
		config: cfg,
		db:     store,
		router: baseRouter,
	}
	source.store(runtimeModelSnapshot{
		models: configModelList(cfg),
		router: baseRouter,
	})
	_ = source.Refresh(ctx)
	return source
}

func (s *RuntimeModelSource) Models() []config.ModelConfig {
	if s == nil {
		return nil
	}
	snapshot, ok := s.value.Load().(runtimeModelSnapshot)
	if !ok {
		return nil
	}
	models := make([]config.ModelConfig, len(snapshot.models))
	copy(models, snapshot.models)
	return models
}

func (s *RuntimeModelSource) Router() *router.Router {
	if s == nil {
		return nil
	}
	snapshot, ok := s.value.Load().(runtimeModelSnapshot)
	if !ok {
		return nil
	}
	return snapshot.router
}

func (s *RuntimeModelSource) Refresh(ctx context.Context) error {
	if s == nil {
		return nil
	}

	models, hasDBModels, err := s.loadMergedModels(ctx)
	if err != nil {
		return err
	}

	var nextRouter *router.Router
	if s.router != nil && hasDBModels {
		nextRouter = s.router.WithModels(models)
	} else if s.router != nil {
		nextRouter = s.router
	} else if len(models) > 0 {
		nextRouter = router.New(models, strategy.NewShuffle(), router.RouterSettings{})
	}
	s.store(runtimeModelSnapshot{models: models, router: nextRouter})
	return nil
}

func (s *RuntimeModelSource) store(snapshot runtimeModelSnapshot) {
	models := make([]config.ModelConfig, len(snapshot.models))
	copy(models, snapshot.models)
	snapshot.models = models
	s.value.Store(snapshot)
}

func (s *RuntimeModelSource) loadMergedModels(ctx context.Context) ([]config.ModelConfig, bool, error) {
	merged := configModelList(s.config)
	indexByName := make(map[string]int, len(merged))
	for i, m := range merged {
		indexByName[m.ModelName] = i
	}

	if s.db == nil {
		return merged, false, nil
	}

	rows, err := listRuntimeProxyModels(ctx, s.db)
	if err != nil {
		return nil, false, err
	}

	dbModels := make([]config.ModelConfig, 0, len(rows))
	for _, row := range rows {
		modelCfg, err := proxyModelToConfig(row)
		if err != nil {
			continue
		}
		dbModels = append(dbModels, modelCfg)
	}
	sort.SliceStable(dbModels, func(i, j int) bool {
		return dbModels[i].ModelName < dbModels[j].ModelName
	})

	for _, modelCfg := range dbModels {
		if idx, ok := indexByName[modelCfg.ModelName]; ok {
			merged[idx] = modelCfg
			continue
		}
		indexByName[modelCfg.ModelName] = len(merged)
		merged = append(merged, modelCfg)
	}

	return merged, len(dbModels) > 0, nil
}

func configModelList(cfg *config.ProxyConfig) []config.ModelConfig {
	if cfg == nil {
		return nil
	}
	models := make([]config.ModelConfig, 0, len(cfg.ModelList))
	for _, m := range cfg.ModelList {
		if strings.TrimSpace(m.ModelName) == "" {
			continue
		}
		models = append(models, m)
	}
	return models
}

func (h *Handlers) runtimeModelList(ctx context.Context) []config.ModelConfig {
	source := h.ensureRuntimeModelSource(ctx)
	if source == nil {
		return nil
	}
	return source.Models()
}

func listRuntimeProxyModels(ctx context.Context, store db.Store) (rows []db.ProxyModelTable, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			rows = nil
			err = fmt.Errorf("list proxy models panic: %v", recovered)
		}
	}()
	return store.ListProxyModels(ctx)
}

func (h *Handlers) runtimeRouter(ctx context.Context) *router.Router {
	source := h.ensureRuntimeModelSource(ctx)
	if source == nil {
		return nil
	}
	return source.Router()
}

func (h *Handlers) ensureRuntimeModelSource(ctx context.Context) *RuntimeModelSource {
	if h == nil {
		return nil
	}
	h.runtimeModelsMu.Lock()
	defer h.runtimeModelsMu.Unlock()
	if h.RuntimeModels == nil {
		h.RuntimeModels = NewRuntimeModelSource(ctx, h.Config, h.DB, h.Router)
	}
	return h.RuntimeModels
}

func (h *Handlers) RefreshRuntimeModels(ctx context.Context) error {
	source := h.ensureRuntimeModelSource(ctx)
	if source == nil {
		return nil
	}
	return source.Refresh(ctx)
}

func proxyModelToConfig(row db.ProxyModelTable) (config.ModelConfig, error) {
	modelName := strings.TrimSpace(row.ModelName)
	if modelName == "" {
		return config.ModelConfig{}, fmt.Errorf("model_name required")
	}

	params, err := decodeDBTianjiParams(row.TianjiParams)
	if err != nil {
		return config.ModelConfig{}, err
	}
	if strings.TrimSpace(params.Model) == "" {
		return config.ModelConfig{}, fmt.Errorf("tianji_params.model required")
	}
	config.NormalizeOfficialOpenAIParams(&params)

	modelCfg := config.ModelConfig{
		ModelName:    modelName,
		TianjiParams: params,
	}
	applyDBModelInfo(row.ModelInfo, &modelCfg)
	return modelCfg, nil
}

func decodeDBTianjiParams(raw []byte) (config.TianjiParams, error) {
	var values map[string]json.RawMessage
	if len(raw) == 0 {
		return config.TianjiParams{}, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return config.TianjiParams{}, fmt.Errorf("decode tianji_params: %w", err)
	}

	params := config.TianjiParams{Overflow: map[string]any{}}
	for key, rawValue := range values {
		switch key {
		case "model", "Model":
			_ = json.Unmarshal(rawValue, &params.Model)
		case "api_key", "APIKey":
			params.APIKey = stringPtrFromRaw(rawValue)
		case "api_base", "APIBase":
			params.APIBase = stringPtrFromRaw(rawValue)
		case "api_version", "APIVersion":
			params.APIVersion = stringPtrFromRaw(rawValue)
		case "openai_subscription_credential_ids", "OpenAISubscriptionCredentialIDs":
			_ = json.Unmarshal(rawValue, &params.OpenAISubscriptionCredentialIDs)
		case "openai_subscription_transport", "OpenAISubscriptionTransport":
			_ = json.Unmarshal(rawValue, &params.OpenAISubscriptionTransport)
		case "tpm", "TPM":
			params.TPM = int64PtrFromRaw(rawValue)
		case "rpm", "RPM":
			params.RPM = int64PtrFromRaw(rawValue)
		case "timeout", "Timeout":
			params.Timeout = intPtrFromRaw(rawValue)
		case "region", "Region":
			_ = json.Unmarshal(rawValue, &params.Region)
		case "auto_router_config", "AutoRouterConfig":
			_ = json.Unmarshal(rawValue, &params.AutoRouterConfig)
		case "auto_router_config_path", "AutoRouterConfigPath":
			_ = json.Unmarshal(rawValue, &params.AutoRouterConfigPath)
		case "auto_router_default_model", "AutoRouterDefaultModel":
			_ = json.Unmarshal(rawValue, &params.AutoRouterDefaultModel)
		case "auto_router_embedding_model", "AutoRouterEmbeddingModel":
			_ = json.Unmarshal(rawValue, &params.AutoRouterEmbeddingModel)
		default:
			var value any
			if json.Unmarshal(rawValue, &value) == nil {
				params.Overflow[key] = value
			}
		}
	}
	if len(params.Overflow) == 0 {
		params.Overflow = nil
	}
	return params, nil
}

func applyDBModelInfo(raw []byte, modelCfg *config.ModelConfig) {
	if len(raw) == 0 || modelCfg == nil {
		return
	}

	var values map[string]json.RawMessage
	if json.Unmarshal(raw, &values) != nil {
		return
	}

	info := &config.ModelInfo{}
	hasInfo := false
	for key, rawValue := range values {
		switch key {
		case "id":
			hasInfo = true
			_ = json.Unmarshal(rawValue, &info.ID)
		case "mode":
			hasInfo = true
			_ = json.Unmarshal(rawValue, &info.Mode)
		case "input_cost_per_token":
			hasInfo = true
			info.InputCost = float64PtrFromRaw(rawValue)
		case "output_cost_per_token":
			hasInfo = true
			info.OutputCost = float64PtrFromRaw(rawValue)
		case "max_tokens":
			hasInfo = true
			info.MaxTokens = intPtrFromRaw(rawValue)
		case "max_input_tokens":
			hasInfo = true
			info.MaxInputTokens = intPtrFromRaw(rawValue)
		case "max_output_tokens":
			hasInfo = true
			info.MaxOutputTokens = intPtrFromRaw(rawValue)
		}
	}
	if hasInfo {
		modelCfg.ModelInfo = info
	}
}

func stringPtrFromRaw(raw json.RawMessage) *string {
	var value string
	if json.Unmarshal(raw, &value) != nil || value == "" {
		return nil
	}
	return &value
}

func intPtrFromRaw(raw json.RawMessage) *int {
	var value int
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}

func int64PtrFromRaw(raw json.RawMessage) *int64 {
	var value int64
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}

func float64PtrFromRaw(raw json.RawMessage) *float64 {
	var value float64
	if json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return &value
}
