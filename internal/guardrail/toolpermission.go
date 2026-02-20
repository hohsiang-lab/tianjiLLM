package guardrail

import (
	"context"
	"fmt"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ToolPermission guardrail checks if requested tools are allowed.
type ToolPermission struct {
	allowedTools map[string][]string // key/team ID → allowed tool names
}

// NewToolPermission creates a tool permission guardrail.
func NewToolPermission(allowedTools map[string][]string) *ToolPermission {
	return &ToolPermission{allowedTools: allowedTools}
}

func (t *ToolPermission) Name() string { return "tool_permission" }

func (t *ToolPermission) SupportedHooks() []Hook { return []Hook{HookPreCall} }

func (t *ToolPermission) Run(ctx context.Context, hook Hook, req *model.ChatCompletionRequest, resp *model.ModelResponse) (Result, error) {
	if hook != HookPreCall || req == nil || len(req.Tools) == 0 {
		return Result{Passed: true}, nil
	}

	// Check global allowed list first, then specific key/team
	allowed := t.allowedTools["*"]

	var unauthorized []string
	for _, tool := range req.Tools {
		name := tool.Function.Name
		if !isToolAllowed(name, allowed) {
			unauthorized = append(unauthorized, name)
		}
	}

	if len(unauthorized) > 0 {
		return Result{
			Passed:  false,
			Message: fmt.Sprintf("Unauthorized tools: %s", strings.Join(unauthorized, ", ")),
		}, nil
	}

	return Result{Passed: true}, nil
}

func isToolAllowed(name string, allowed []string) bool {
	for _, a := range allowed {
		if a == "*" || a == name {
			return true
		}
	}
	return false
}
