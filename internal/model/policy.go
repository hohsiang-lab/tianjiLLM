package model

import (
	"encoding/json"
	"time"
)

// Policy represents a guardrail assignment policy with optional inheritance.
type Policy struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	ParentID         *string         `json:"parent_id,omitempty"`
	Conditions       json.RawMessage `json:"conditions,omitempty"`
	GuardrailsAdd    []string        `json:"guardrails_add"`
	GuardrailsRemove []string        `json:"guardrails_remove"`
	Pipeline         json.RawMessage `json:"pipeline,omitempty"`
	Description      *string         `json:"description,omitempty"`
	CreatedBy        *string         `json:"created_by,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// PolicyConditions defines match conditions for a policy.
type PolicyConditions struct {
	Model string `json:"model,omitempty"` // regexp pattern for model matching
}

// PolicyAttachment binds a policy to a multi-dimensional scope.
type PolicyAttachment struct {
	ID         string    `json:"id"`
	PolicyName string    `json:"policy_name"`
	Scope      *string   `json:"scope,omitempty"` // "*" for global
	Teams      []string  `json:"teams"`
	Keys       []string  `json:"keys"`
	Models     []string  `json:"models"`
	Tags       []string  `json:"tags"`
	CreatedBy  *string   `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PipelineConfig holds the pipeline mode and steps, stored as JSONB on Policy.
type PipelineConfig struct {
	Mode  string         `json:"mode"` // "pre_call" or "post_call"
	Steps []PipelineStep `json:"steps"`
}

// PipelineStep represents a single guardrail execution step in a pipeline.
type PipelineStep struct {
	Guardrail             string `json:"guardrail"`
	OnPass                string `json:"on_pass"` // next, allow, modify_response
	OnFail                string `json:"on_fail"` // next, block, modify_response
	PassData              bool   `json:"pass_data"`
	ModifyResponseMessage string `json:"modify_response_message,omitempty"`
}

// CreatePolicyRequest is the request body for creating a policy.
type CreatePolicyRequest struct {
	Name             string          `json:"name"`
	ParentID         *string         `json:"parent_id,omitempty"`
	Conditions       json.RawMessage `json:"conditions,omitempty"`
	GuardrailsAdd    []string        `json:"guardrails_add,omitempty"`
	GuardrailsRemove []string        `json:"guardrails_remove,omitempty"`
	Pipeline         json.RawMessage `json:"pipeline,omitempty"`
	Description      *string         `json:"description,omitempty"`
}

// UpdatePolicyRequest is the request body for updating a policy.
type UpdatePolicyRequest struct {
	Name             *string         `json:"name,omitempty"`
	ParentID         *string         `json:"parent_id,omitempty"`
	Conditions       json.RawMessage `json:"conditions,omitempty"`
	GuardrailsAdd    []string        `json:"guardrails_add,omitempty"`
	GuardrailsRemove []string        `json:"guardrails_remove,omitempty"`
	Pipeline         json.RawMessage `json:"pipeline,omitempty"`
	Description      *string         `json:"description,omitempty"`
}

// CreatePolicyAttachmentRequest is the request body for creating a policy attachment.
type CreatePolicyAttachmentRequest struct {
	PolicyName string   `json:"policy_name"`
	Scope      *string  `json:"scope,omitempty"`
	Teams      []string `json:"teams,omitempty"`
	Keys       []string `json:"keys,omitempty"`
	Models     []string `json:"models,omitempty"`
	Tags       []string `json:"tags,omitempty"`
}

// PolicyResolutionRequest is the request to resolve guardrails for a context.
type PolicyResolutionRequest struct {
	Model  string `json:"model"`
	TeamID string `json:"team_id,omitempty"`
	KeyID  string `json:"key_id,omitempty"`
}

// PolicyResolutionResponse returns the resolved guardrail list.
type PolicyResolutionResponse struct {
	Guardrails []string `json:"guardrails"`
	Policies   []string `json:"policies"` // names of matched policies
}

// TestPipelineRequest is the request body for testing a pipeline.
type TestPipelineRequest struct {
	PolicyName string         `json:"policy_name"`
	Input      map[string]any `json:"input"`
}

// TestPipelineResponse returns the pipeline test result.
type TestPipelineResponse struct {
	Result  string               `json:"result"` // allow, block, modify_response
	Message string               `json:"message,omitempty"`
	Steps   []PipelineStepResult `json:"steps"`
}

// PipelineStepResult records the outcome of a single pipeline step.
type PipelineStepResult struct {
	Guardrail string `json:"guardrail"`
	Passed    bool   `json:"passed"`
	Action    string `json:"action"` // what action was taken
}
