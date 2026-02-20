package handler

import (
	"context"
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// resolvePromptTemplate checks if the request has a PromptName and, if so,
// fetches the template from DB, substitutes variables, and sets the messages.
// Returns nil if no prompt resolution is needed.
func resolvePromptTemplate(ctx context.Context, queries *db.Queries, req *model.ChatCompletionRequest) error {
	if req.PromptName == "" {
		return nil
	}
	if queries == nil {
		return fmt.Errorf("prompt resolution requires database")
	}

	var template string

	if req.PromptVersion != nil {
		tmpl, err := queries.GetPromptTemplateByNameVersion(ctx, db.GetPromptTemplateByNameVersionParams{
			Name:    req.PromptName,
			Version: int32(*req.PromptVersion),
		})
		if err != nil {
			return fmt.Errorf("prompt %q version %d not found: %w", req.PromptName, *req.PromptVersion, err)
		}
		template = tmpl.Template
	} else {
		tmpl, err := queries.GetLatestPromptByName(ctx, req.PromptName)
		if err != nil {
			return fmt.Errorf("prompt %q not found: %w", req.PromptName, err)
		}
		template = tmpl.Template
	}

	resolved := template
	for k, v := range req.PromptVariables {
		resolved = strings.ReplaceAll(resolved, "{{"+k+"}}", v)
	}

	req.Messages = []model.Message{
		{Role: "user", Content: resolved},
	}

	// Clear prompt fields so they don't propagate to providers
	req.PromptName = ""
	req.PromptVariables = nil
	req.PromptVersion = nil

	return nil
}
