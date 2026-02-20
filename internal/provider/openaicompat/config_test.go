package openaicompat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyConstraints_ClampMax(t *testing.T) {
	max := 2.0
	constraints := []ParamConstraint{
		{Param: "temperature", Max: &max},
	}

	params := map[string]any{
		"temperature": 3.0,
	}

	result := ApplyConstraints(params, constraints)
	assert.Equal(t, 2.0, result["temperature"])
}

func TestApplyConstraints_ClampMin(t *testing.T) {
	min := 0.0
	constraints := []ParamConstraint{
		{Param: "temperature", Min: &min},
	}

	params := map[string]any{
		"temperature": -1.0,
	}

	result := ApplyConstraints(params, constraints)
	assert.Equal(t, 0.0, result["temperature"])
}

func TestApplyConstraints_InRange(t *testing.T) {
	min := 0.0
	max := 2.0
	constraints := []ParamConstraint{
		{Param: "temperature", Min: &min, Max: &max},
	}

	params := map[string]any{
		"temperature": 1.0,
	}

	result := ApplyConstraints(params, constraints)
	assert.Equal(t, 1.0, result["temperature"])
}

func TestApplyConstraints_IntValue(t *testing.T) {
	max := 4096.0
	constraints := []ParamConstraint{
		{Param: "max_tokens", Max: &max},
	}

	params := map[string]any{
		"max_tokens": 8192,
	}

	result := ApplyConstraints(params, constraints)
	assert.Equal(t, 4096.0, result["max_tokens"])
}

func TestApplyConstraints_NoConstraints(t *testing.T) {
	params := map[string]any{
		"temperature": 1.5,
	}

	result := ApplyConstraints(params, nil)
	assert.Equal(t, 1.5, result["temperature"])
}
