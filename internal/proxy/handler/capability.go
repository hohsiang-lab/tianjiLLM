package handler

import (
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	azureProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/azure"
	openaiProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
	openaicompatProvider "github.com/praxisllmlab/tianjiLLM/internal/provider/openaicompat"
)

func supportsStandardOpenAIChatCapabilities(p provider.Provider, apiVersion *string) bool {
	switch p := p.(type) {
	case *azureProvider.Provider:
		return p.SupportsStandardCapabilities(apiVersion)
	case interface{ SupportsStandardOpenAIChatCapabilities() bool }:
		return p.SupportsStandardOpenAIChatCapabilities()
	case interface{ UsesDefaultBaseURL() bool }:
		return p.UsesDefaultBaseURL()
	default:
		return false
	}
}

func supportsOpenAICompatibleProvider(providerName string, params config.TianjiParams) bool {
	if providerName == "openaicompat" {
		return true
	}

	p, err := provider.Get(providerName)
	if err != nil {
		return false
	}
	for {
		wrapped, ok := p.(interface{ UnderlyingProvider() provider.Provider })
		if !ok {
			break
		}
		underlying := wrapped.UnderlyingProvider()
		if underlying == nil {
			return false
		}
		p = underlying
	}

	if _, ok := p.(*openaicompatProvider.Provider); ok {
		return true
	}
	return supportsStandardOpenAIChatCapabilities(p, params.APIVersion)
}

func (h *Handlers) capabilityForRoute(route resolvedProviderRoute) model.CapabilityRecord {
	if h.Capabilities != nil {
		record, _ := h.Capabilities.Lookup(route.Backend, route.PublicModel)
		return withNonStreamOverride(route, record)
	}
	if route.Backend == model.BackendChatGPTCodex {
		record := model.CapabilityRecord{SupportsStream: true}
		modelName := strings.TrimPrefix(strings.TrimPrefix(route.ModelName, "openai/"), "chatgpt/")
		switch modelName {
		case "gpt-5.6-terra":
			record.SupportsResponseFormat = true
			record.SupportsJSONObject = true
			record.SupportsJSONSchema = true
			record.SupportsTools = true
			record.SupportsToolChoice = true
			record.SupportsStreamOptionsIncludeUsage = true
		case "gpt-5.4-mini", "gpt-5.6-luna":
			record.SupportsResponseFormat = true
			record.SupportsJSONObject = true
			record.SupportsJSONSchema = true
			// The provider adapter applies an exact-model capability-scoped
			// drop for upstream-rejected temperature/token fields.
			record.SupportsTemperature = true
			record.SupportsMaxTokens = true
		}
		return withNonStreamOverride(route, record)
	}

	supported := make(map[string]bool)
	if route.Provider != nil {
		for _, param := range route.Provider.GetSupportedParams() {
			supported[param] = true
		}
	}
	supportsExact := func(param string) bool {
		return supported[param]
	}
	preservesStandardParameter := func(param string) bool {
		if configured, ok := route.Provider.(interface {
			PreservesStandardParameter(string) bool
		}); ok {
			return configured.PreservesStandardParameter(param)
		}
		return true
	}
	supports := func(param string) bool {
		if supported[param] {
			return true
		}
		if route.Provider != nil {
			for mapped := range route.Provider.MapParams(map[string]any{param: true}) {
				if supported[mapped] {
					return true
				}
			}
		}
		return false
	}
	standardProvider := route.Provider
	if wrapped, ok := standardProvider.(interface{ UnderlyingProvider() provider.Provider }); ok {
		standardProvider = wrapped.UnderlyingProvider()
	}
	standardOpenAI := false
	supportsDimensions := false
	supportsResponseFormat := supports("response_format")
	supportsMaxCompletionTokens := supports("max_completion_tokens")
	switch p := standardProvider.(type) {
	case *openaiProvider.Provider:
		standardOpenAI = p.UsesDefaultBaseURL()
		supportsDimensions = standardOpenAI && supportsOpenAIEmbeddingDimensions(route.ModelName)
		if !standardOpenAI {
			supportsResponseFormat = false
		}
	case *azureProvider.Provider:
		standardOpenAI = p.SupportsStandardCapabilities(route.TianjiParams.APIVersion)
		if !standardOpenAI {
			supportsResponseFormat = false
			supportsMaxCompletionTokens = false
		}
	case interface{ SupportsStandardOpenAIChatCapabilities() bool }:
		standardOpenAI = p.SupportsStandardOpenAIChatCapabilities()
	case interface{ UsesDefaultBaseURL() bool }:
		standardOpenAI = p.UsesDefaultBaseURL()
	}
	_, supportsEmbeddings := route.Provider.(provider.EmbeddingProvider)
	return withNonStreamOverride(route, model.CapabilityRecord{
		SupportsStream:                    supports("stream"),
		SupportsNonStream:                 standardOpenAI,
		SupportsResponseFormat:            supportsResponseFormat,
		SupportsJSONObject:                standardOpenAI && supportsExact("response_format") && preservesStandardParameter("response_format"),
		SupportsJSONSchema:                standardOpenAI && supportsExact("response_format") && preservesStandardParameter("response_format"),
		SupportsTools:                     supportsExact("tools") && preservesStandardParameter("tools"),
		SupportsToolChoice:                supportsExact("tool_choice") && preservesStandardParameter("tool_choice"),
		SupportsTemperature:               supports("temperature"),
		SupportsTopP:                      supports("top_p"),
		SupportsMaxTokens:                 supports("max_tokens"),
		SupportsMaxCompletionTokens:       supportsMaxCompletionTokens,
		SupportsStreamOptionsIncludeUsage: standardOpenAI && supportsExact("stream_options") && preservesStandardParameter("stream_options"),
		SupportsEmbeddings:                supportsEmbeddings,
		SupportsDimensions:                supportsDimensions,
	})
}

func supportsOpenAIEmbeddingDimensions(modelName string) bool {
	modelName = strings.TrimPrefix(modelName, "openai/")
	return strings.HasPrefix(modelName, "text-embedding-3-")
}

func withNonStreamOverride(route resolvedProviderRoute, record model.CapabilityRecord) model.CapabilityRecord {
	if configured, ok := route.TianjiParams.Overflow["supports_non_stream"].(bool); ok {
		record.SupportsNonStream = configured
	}
	return record
}

func (h *Handlers) attachRouteCapabilities(route resolvedProviderRoute) resolvedProviderRoute {
	route.Capability = h.capabilityForRoute(route)
	return route
}

func validateChatCapabilities(req *model.ChatCompletionRequest, capability model.CapabilityRecord) *model.RequestError {
	if req.IsStreaming() {
		if !capability.SupportsStream {
			return model.UnsupportedParameter("stream", "This model does not support stream=true")
		}
	} else if !capability.SupportsNonStream && !capability.SupportsStream {
		return model.UnsupportedParameter("stream", "This model does not support non-streaming chat completions")
	}

	if req.ResponseFormat != nil {
		format, ok := req.ResponseFormat.(map[string]any)
		if !ok {
			return model.InvalidValue("response_format", "response_format must be an object")
		}
		formatType, ok := format["type"].(string)
		if !ok || strings.TrimSpace(formatType) == "" {
			return model.InvalidValue("response_format", "response_format.type is required")
		}
		switch formatType {
		case "text":
			req.ResponseFormat = nil
		case "json_object":
			if !capability.SupportsResponseFormat || !capability.SupportsJSONObject {
				return unsupportedResponseFormat(formatType)
			}
		case "json_schema":
			if err := validateJSONSchemaFormat(format); err != nil {
				return err
			}
			if !capability.SupportsResponseFormat || !capability.SupportsJSONSchema {
				return unsupportedResponseFormat(formatType)
			}
		default:
			return model.InvalidValue("response_format", fmt.Sprintf("unsupported response_format.type %q", formatType))
		}
	}

	if req.MaxTokens != nil && req.MaxCompletionTokens != nil {
		return model.InvalidRequest("max_tokens", "max_tokens and max_completion_tokens cannot be used together for this model")
	}
	if req.MaxTokens != nil {
		if *req.MaxTokens <= 0 {
			return model.InvalidValue("max_tokens", "max_tokens must be greater than 0")
		}
		if !capability.SupportsMaxTokens {
			return model.UnsupportedParameter("max_tokens", "This model does not support max_tokens")
		}
	}
	if req.MaxCompletionTokens != nil {
		if *req.MaxCompletionTokens <= 0 {
			return model.InvalidValue("max_completion_tokens", "max_completion_tokens must be greater than 0")
		}
		if !capability.SupportsMaxCompletionTokens {
			return model.UnsupportedParameter("max_completion_tokens", "This model does not support max_completion_tokens")
		}
	}
	if req.Temperature != nil {
		if *req.Temperature < 0 || *req.Temperature > 2 {
			return model.InvalidValue("temperature", "temperature must be between 0 and 2")
		}
		if !capability.SupportsTemperature {
			return model.UnsupportedParameter("temperature", "This model does not support temperature")
		}
	}
	if req.TopP != nil {
		if *req.TopP < 0 || *req.TopP > 1 {
			return model.InvalidValue("top_p", "top_p must be between 0 and 1")
		}
		if !capability.SupportsTopP {
			return model.UnsupportedParameter("top_p", "This model does not support top_p")
		}
	}
	if len(req.Tools) > 0 && !capability.SupportsTools {
		return model.UnsupportedParameter("tools", "This model does not support tools")
	}
	if req.ToolChoice != nil && !capability.SupportsToolChoice {
		return model.UnsupportedParameter("tool_choice", "This model does not support tool_choice")
	}
	if req.StreamOptions != nil && req.StreamOptions.IncludeUsage {
		if !req.IsStreaming() {
			return model.InvalidRequest("stream_options", "stream_options.include_usage requires stream=true")
		}
		if !capability.SupportsStreamOptionsIncludeUsage {
			return model.UnsupportedParameter("stream_options.include_usage", "This model does not support stream_options.include_usage")
		}
	}
	return nil
}

func validateEmbeddingCapabilities(req *model.EmbeddingRequest, capability model.CapabilityRecord) *model.RequestError {
	if !capability.SupportsEmbeddings {
		return model.UnsupportedParameter("model", "This model does not support embeddings")
	}

	switch input := req.Input.(type) {
	case string:
		if strings.TrimSpace(input) == "" {
			return model.InvalidValue("input", "input must be a non-empty string or list of non-empty strings")
		}
	case []string:
		if len(input) == 0 {
			return model.InvalidValue("input", "input must be a non-empty string or list of non-empty strings")
		}
		for _, value := range input {
			if strings.TrimSpace(value) == "" {
				return model.InvalidValue("input", "input must contain only non-empty strings")
			}
		}
	case []any:
		// This feature intentionally supports only string and list-of-string
		// inputs. OpenAI token-array forms remain outside this public contract.
		if len(input) == 0 {
			return model.InvalidValue("input", "input must be a non-empty string or list of non-empty strings")
		}
		normalized := make([]string, len(input))
		for i, value := range input {
			text, ok := value.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return model.InvalidValue("input", "input must contain only non-empty strings")
			}
			normalized[i] = text
		}
		req.Input = normalized
	default:
		return model.InvalidValue("input", "input must be a non-empty string or list of non-empty strings")
	}

	if req.EncodingFormat != "" &&
		req.EncodingFormat != "float" &&
		req.EncodingFormat != "base64" {
		return model.InvalidValue("encoding_format", "encoding_format must be float or base64")
	}
	if req.Dimensions == nil {
		return nil
	}
	if *req.Dimensions <= 0 {
		return model.InvalidValue("dimensions", "dimensions must be greater than 0")
	}
	if !capability.SupportsDimensions {
		return model.UnsupportedParameter("dimensions", "This model does not support dimensions")
	}
	if len(capability.AllowedDimensions) == 0 {
		return nil
	}
	for _, allowed := range capability.AllowedDimensions {
		if *req.Dimensions == allowed {
			return nil
		}
	}
	return model.InvalidValue("dimensions", fmt.Sprintf(
		"dimensions=%d is not supported by this model",
		*req.Dimensions,
	))
}

func unsupportedResponseFormat(formatType string) *model.RequestError {
	return model.UnsupportedParameter(
		"response_format",
		fmt.Sprintf("This model does not support response_format=%s", formatType),
	)
}

func validateJSONSchemaFormat(format map[string]any) *model.RequestError {
	jsonSchema, ok := format["json_schema"].(map[string]any)
	if !ok {
		return model.InvalidValue("response_format", "response_format.json_schema must be an object")
	}
	name, ok := jsonSchema["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return model.InvalidValue("response_format", "response_format.json_schema.name is required")
	}
	if _, ok := jsonSchema["schema"].(map[string]any); !ok {
		return model.InvalidValue("response_format", "response_format.json_schema.schema must be an object")
	}
	if strict, present := jsonSchema["strict"]; present {
		if _, ok := strict.(bool); !ok {
			return model.InvalidValue("response_format", "response_format.json_schema.strict must be a boolean")
		}
	}
	return nil
}
