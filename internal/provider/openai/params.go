package openai

// SupportedParams lists all parameters the OpenAI API supports.
var SupportedParams = []string{
	"model",
	"messages",
	"temperature",
	"max_tokens",
	"max_completion_tokens",
	"top_p",
	"frequency_penalty",
	"presence_penalty",
	"n",
	"stop",
	"stream",
	"stream_options",
	"seed",
	"user",
	"tools",
	"tool_choice",
	"response_format",
	"logprobs",
	"top_logprobs",
}

// ParamMappings maps common parameter names to OpenAI-specific names.
var ParamMappings = map[string]string{
	"max_completion_tokens": "max_tokens",
}

// MapOpenAIParams applies parameter name mappings.
func MapOpenAIParams(params map[string]any) map[string]any {
	result := make(map[string]any, len(params))
	for k, v := range params {
		if mapped, ok := ParamMappings[k]; ok {
			result[mapped] = v
		} else {
			result[k] = v
		}
	}
	return result
}

// FilterUnsupportedParams removes parameters not in the supported list.
// Used when drop_params is enabled.
func FilterUnsupportedParams(params map[string]any, supported []string) map[string]any {
	supportedSet := make(map[string]struct{}, len(supported))
	for _, s := range supported {
		supportedSet[s] = struct{}{}
	}

	result := make(map[string]any, len(params))
	for k, v := range params {
		if _, ok := supportedSet[k]; ok {
			result[k] = v
		}
	}
	return result
}
