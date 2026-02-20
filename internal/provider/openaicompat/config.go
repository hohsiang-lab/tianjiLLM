package openaicompat

// SimpleProviderConfig defines a JSON-config-driven OpenAI-compatible provider.
// Loaded from providers.json at startup.
type SimpleProviderConfig struct {
	Name            string            `json:"name"`
	BaseURL         string            `json:"base_url"`
	AuthHeader      string            `json:"auth_header,omitempty"` // default: "Authorization"
	AuthPrefix      string            `json:"auth_prefix,omitempty"` // default: "Bearer "
	Headers         map[string]string `json:"headers,omitempty"`
	SupportedParams []string          `json:"supported_params,omitempty"`
	ParamMappings   map[string]string `json:"param_mappings,omitempty"`
	Constraints     []ParamConstraint `json:"constraints,omitempty"`
}

// ParamConstraint defines min/max value enforcement for a parameter.
type ParamConstraint struct {
	Param string   `json:"param"`
	Min   *float64 `json:"min,omitempty"`
	Max   *float64 `json:"max,omitempty"`
}

// ProvidersFile represents the top-level providers.json structure.
type ProvidersFile struct {
	Providers map[string]SimpleProviderConfig `json:"providers"`
}

// ApplyConstraints clamps parameter values to configured min/max ranges.
func ApplyConstraints(params map[string]any, constraints []ParamConstraint) map[string]any {
	for _, c := range constraints {
		val, ok := params[c.Param]
		if !ok {
			continue
		}

		fval, ok := toFloat64(val)
		if !ok {
			continue
		}

		if c.Min != nil && fval < *c.Min {
			fval = *c.Min
		}
		if c.Max != nil && fval > *c.Max {
			fval = *c.Max
		}
		params[c.Param] = fval
	}
	return params
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}
