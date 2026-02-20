package ui

import (
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

func (h *UIHandler) handleModels(w http.ResponseWriter, r *http.Request) {
	var rows []pages.ModelRow
	for _, m := range h.Config.ModelList {
		provider := ""
		modelID := ""
		if parts := strings.SplitN(m.TianjiParams.Model, "/", 2); len(parts) == 2 {
			provider = parts[0]
			modelID = parts[1]
		} else {
			provider = "openai"
			modelID = m.TianjiParams.Model
		}
		apiBase := ""
		if m.TianjiParams.APIBase != nil {
			apiBase = *m.TianjiParams.APIBase
		}
		rows = append(rows, pages.ModelRow{
			ModelName: m.ModelName,
			Provider:  provider,
			ModelID:   modelID,
			APIBase:   apiBase,
		})
	}
	render(r.Context(), w, pages.ModelsPage(rows))
}
