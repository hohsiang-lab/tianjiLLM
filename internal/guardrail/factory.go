package guardrail

import (
	"fmt"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

// NewFromConfig creates a Guardrail from a config entry.
func NewFromConfig(gc config.GuardrailConfig) (Guardrail, error) {
	params := gc.TianjiParams
	mode := ""
	if m, ok := params["mode"].(string); ok {
		mode = m
	}
	apiKey, _ := params["api_key"].(string)
	apiBase, _ := params["api_base"].(string)

	switch mode {
	case "openai_moderation":
		return NewModerationGuardrail(apiKey, apiBase), nil
	case "presidio":
		analyzerURL, _ := params["analyzer_url"].(string)
		return NewPresidioGuardrail(analyzerURL, nil), nil
	case "prompt_injection":
		return NewPromptInjectionGuardrail(nil), nil
	case "lakera_guard":
		return &LakeraGuardrail{apiKey: apiKey, baseURL: apiBase}, nil
	case "bedrock_guardrail":
		guardrailID, _ := params["guardrail_id"].(string)
		version, _ := params["guardrail_version"].(string)
		region, _ := params["region"].(string)
		return NewBedrockGuardrail(guardrailID, version, region)
	case "azure_prompt_shield":
		endpoint, _ := params["endpoint"].(string)
		return &AzurePromptShield{apiKey: apiKey, endpoint: endpoint}, nil
	case "azure_text_moderation":
		endpoint, _ := params["endpoint"].(string)
		threshold := 4
		if t, ok := params["threshold"].(int); ok {
			threshold = t
		}
		return &AzureTextModeration{apiKey: apiKey, endpoint: endpoint, threshold: threshold}, nil
	case "content_filter":
		threshold := 2
		if t, ok := params["threshold"].(int); ok {
			threshold = t
		}
		return NewContentFilter(threshold), nil
	case "tool_permission":
		allowed := make(map[string][]string)
		if a, ok := params["allowed_tools"].(map[string]any); ok {
			for role, tools := range a {
				if ts, ok := tools.([]any); ok {
					for _, t := range ts {
						if s, ok := t.(string); ok {
							allowed[role] = append(allowed[role], s)
						}
					}
				}
			}
		}
		return NewToolPermission(allowed), nil
	case "generic":
		endpoint, _ := params["endpoint"].(string)
		return &GenericGuardrail{name: gc.GuardrailName, endpoint: endpoint}, nil
	case "aim":
		return NewAIMGuardrail(apiKey, apiBase), nil
	case "aporia":
		return NewAporiaGuardrail(apiKey, apiBase), nil
	case "custom_code":
		return NewCustomCodeGuardrail(apiKey, apiBase), nil
	case "dynamoai":
		return NewDynamoAIGuardrail(apiKey, apiBase), nil
	case "enkryptai":
		return NewEnkryptAIGuardrail(apiKey, apiBase), nil
	case "grayswan":
		return NewGraySwanGuardrail(apiKey, apiBase), nil
	case "guardrails_ai":
		return NewGuardrailsAIGuardrail(apiKey, apiBase), nil
	case "hiddenlayer":
		return NewHiddenLayerGuardrail(apiKey, apiBase), nil
	case "ibm_guardrails":
		return NewIBMGuardrail(apiKey, apiBase), nil
	case "javelin":
		return NewJavelinGuardrail(apiKey, apiBase), nil
	case "lakera_v2":
		return NewLakeraV2Guardrail(apiKey, apiBase), nil
	case "lasso":
		return NewLassoGuardrail(apiKey, apiBase), nil
	case "model_armor":
		return NewModelArmorGuardrail(apiKey, apiBase), nil
	case "noma":
		return NewNomaGuardrail(apiKey, apiBase), nil
	case "onyx":
		return NewOnyxGuardrail(apiKey, apiBase), nil
	case "pangea":
		return NewPangeaGuardrail(apiKey, apiBase), nil
	case "panw_prisma_airs":
		return NewPANWPrismaGuardrail(apiKey, apiBase), nil
	case "pillar":
		return NewPillarGuardrail(apiKey, apiBase), nil
	case "prompt_security":
		return NewPromptSecurityGuardrail(apiKey, apiBase), nil
	case "qualifire":
		return NewQualifireGuardrail(apiKey, apiBase), nil
	case "unified_guardrail":
		return NewUnifiedGuardrail(apiKey, apiBase), nil
	case "zscaler_ai_guard":
		return NewZscalerGuardrail(apiKey, apiBase), nil
	default:
		if mode == "" {
			return nil, fmt.Errorf("guardrail %q: missing tianji_params.mode", gc.GuardrailName)
		}
		return nil, fmt.Errorf("unknown guardrail mode: %s", mode)
	}
}
