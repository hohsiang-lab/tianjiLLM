package policy

import (
	"encoding/json"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// PipelineResult records the outcome of executing a pipeline.
type PipelineResult struct {
	Action  string // "allow", "block", "modify_response"
	Message string // message for modify_response
	Steps   []model.PipelineStepResult
}

// GuardrailChecker is the interface for checking whether a guardrail
// passes for given input. Implemented by the guardrail registry.
type GuardrailChecker interface {
	Check(guardrailName string, input map[string]any) (passed bool, err error)
}

// ExecutePipeline runs a pipeline's steps in order, following
// on_pass/on_fail actions. pass_data forwards input between steps.
func ExecutePipeline(pipeline []byte, checker GuardrailChecker, input map[string]any) PipelineResult {
	var cfg model.PipelineConfig
	if err := json.Unmarshal(pipeline, &cfg); err != nil {
		return PipelineResult{Action: "block", Message: "invalid pipeline configuration"}
	}

	if len(cfg.Steps) == 0 {
		return PipelineResult{Action: "allow"}
	}

	var stepResults []model.PipelineStepResult
	// Shallow-copy input so callers' map isn't mutated by pass_data.
	currentInput := make(map[string]any, len(input))
	for k, v := range input {
		currentInput[k] = v
	}

	for _, step := range cfg.Steps {
		passed, err := checker.Check(step.Guardrail, currentInput)
		if err != nil {
			passed = false
		}

		result := model.PipelineStepResult{
			Guardrail: step.Guardrail,
			Passed:    passed,
		}

		var action string
		if passed {
			action = step.OnPass
		} else {
			action = step.OnFail
		}

		result.Action = action
		stepResults = append(stepResults, result)

		switch action {
		case "allow":
			return PipelineResult{Action: "allow", Steps: stepResults}
		case "block":
			return PipelineResult{Action: "block", Steps: stepResults}
		case "modify_response":
			return PipelineResult{
				Action:  "modify_response",
				Message: step.ModifyResponseMessage,
				Steps:   stepResults,
			}
		case "next":
			if step.PassData {
				currentInput["_prev_result"] = passed
			}
			continue
		default:
			return PipelineResult{
				Action:  "block",
				Message: "unknown pipeline action: " + action,
				Steps:   stepResults,
			}
		}
	}

	// All steps completed with "next" — default allow
	return PipelineResult{Action: "allow", Steps: stepResults}
}
