package config

import (
	"fmt"
	"log"
	"sort"
)

// Validate checks the config for unrecognized fields and logs warnings.
// Enables loading any Python proxy_config.yaml without errors (FR-027, FR-029).
func Validate(cfg *ProxyConfig) {
	warnOverflow("config", cfg.Overflow)
	warnOverflow("tianji_settings", cfg.TianjiSettings.Overflow)
	warnOverflow("general_settings", cfg.GeneralSettings.Overflow)
	if cfg.RouterSettings != nil {
		warnOverflow("router_settings", cfg.RouterSettings.Overflow)
	}
	if cfg.TianjiSettings.CacheParams != nil {
		warnOverflow("cache_params", cfg.TianjiSettings.CacheParams.Overflow)
	}
	for i, m := range cfg.ModelList {
		section := fmt.Sprintf("model_list[%d].tianji_params(%s)", i, m.ModelName)
		warnOverflow(section, m.TianjiParams.Overflow)
	}
	for i, g := range cfg.Guardrails {
		section := fmt.Sprintf("guardrails[%d](%s)", i, g.GuardrailName)
		warnOverflow(section, g.Overflow)
	}
	for i, p := range cfg.PassThroughEndpoints {
		section := fmt.Sprintf("pass_through_endpoints[%d](%s)", i, p.Path)
		warnOverflow(section, p.Overflow)
	}
}

func warnOverflow(section string, overflow map[string]any) {
	if len(overflow) == 0 {
		return
	}
	keys := make([]string, 0, len(overflow))
	for k := range overflow {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		log.Printf("[WARNING] Unrecognized config field %s.%s — field will be ignored", section, k)
	}
}
