package handler

import (
	"encoding/json"
	"net/http"
	"sort"

	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// modelGroupInfoResponse describes aggregated info for a model group.
type modelGroupInfoResponse struct {
	ModelGroup      string   `json:"model_group"`
	Providers       []string `json:"providers"`
	MaxInputTokens  int      `json:"max_input_tokens"`
	MaxOutputTokens int      `json:"max_output_tokens"`
	Mode            string   `json:"mode,omitempty"`
	InputCost       float64  `json:"input_cost_per_token,omitempty"`
	OutputCost      float64  `json:"output_cost_per_token,omitempty"`
	NumDeployments  int      `json:"num_deployments"`
}

// ModelGroupInfo returns aggregated info for model groups from the router.
func (h *Handlers) ModelGroupInfo(w http.ResponseWriter, r *http.Request) {
	if h.Router == nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}

	filter := r.URL.Query().Get("model_group")
	// NOTE: DB-managed models (proxy_model_table) are not part of the router;
	// they are only served via the UI. Access control filtering here applies
	// only to router-based (config) deployments.
	groups := h.Router.ListModelGroups(r.Context())
	calc := pricing.Default()

	var results []modelGroupInfoResponse

	for groupName, deployments := range groups {
		if filter != "" && groupName != filter {
			continue
		}

		info := modelGroupInfoResponse{
			ModelGroup:     groupName,
			NumDeployments: len(deployments),
		}

		providerSet := make(map[string]bool)
		for _, d := range deployments {
			providerSet[d.ProviderName] = true

			// Enrich from pricing data
			modelKey := d.ProviderName + "/" + d.ModelName
			if mi := calc.GetModelInfo(modelKey); mi != nil {
				if mi.MaxInputTokens > info.MaxInputTokens {
					info.MaxInputTokens = mi.MaxInputTokens
				}
				if mi.MaxOutputTokens > info.MaxOutputTokens {
					info.MaxOutputTokens = mi.MaxOutputTokens
				}
				if info.Mode == "" {
					info.Mode = mi.Mode
				}
				if mi.InputCostPerToken > 0 && (info.InputCost == 0 || mi.InputCostPerToken < info.InputCost) {
					info.InputCost = mi.InputCostPerToken
				}
				if mi.OutputCostPerToken > 0 && (info.OutputCost == 0 || mi.OutputCostPerToken < info.OutputCost) {
					info.OutputCost = mi.OutputCostPerToken
				}
			}
		}

		providers := make([]string, 0, len(providerSet))
		for p := range providerSet {
			providers = append(providers, p)
		}
		sort.Strings(providers)
		info.Providers = providers

		results = append(results, info)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ModelGroup < results[j].ModelGroup
	})

	writeJSON(w, http.StatusOK, map[string]any{"data": results})
}

// PublicProviders returns sorted list of all registered provider names.
func (h *Handlers) PublicProviders(w http.ResponseWriter, _ *http.Request) {
	names := provider.List()
	sort.Strings(names)
	writeJSON(w, http.StatusOK, map[string]any{"data": names})
}

// PublicModelCostMap returns the full model pricing data.
func (h *Handlers) PublicModelCostMap(w http.ResponseWriter, _ *http.Request) {
	// Return the raw embedded JSON from the pricing package
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(pricing.ModelCostMap())
}
