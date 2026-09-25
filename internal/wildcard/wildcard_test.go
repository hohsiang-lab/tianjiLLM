package wildcard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected []string
	}{
		{
			name:     "basic suffix wildcard",
			pattern:  "claude-*",
			input:    "claude-sonnet-4-5",
			expected: []string{"sonnet-4-5"},
		},
		{
			name:     "provider prefix wildcard",
			pattern:  "openai/*",
			input:    "openai/gpt-4o",
			expected: []string{"gpt-4o"},
		},
		{
			name:     "catch-all wildcard",
			pattern:  "*",
			input:    "anything-goes",
			expected: []string{"anything-goes"},
		},
		{
			name:     "no match",
			pattern:  "claude-*",
			input:    "gpt-4o",
			expected: nil,
		},
		{
			name:     "multi wildcard",
			pattern:  "*::static::*",
			input:    "a::static::b",
			expected: []string{"a", "b"},
		},
		{
			name:     "no wildcard in pattern",
			pattern:  "exact-model",
			input:    "exact-model",
			expected: nil, // Match requires "*" in pattern
		},
		{
			name:     "wildcard with dots",
			pattern:  "gpt-4o.*",
			input:    "gpt-4o.2024-11-20",
			expected: []string{"2024-11-20"},
		},
		{
			name:     "partial no match",
			pattern:  "claude-*-opus",
			input:    "claude-sonnet-4-5",
			expected: nil,
		},
		{
			name:     "middle wildcard match",
			pattern:  "claude-*-latest",
			input:    "claude-sonnet-latest",
			expected: []string{"sonnet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Match(tt.pattern, tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name     string
		template string
		captured []string
		expected string
	}{
		{
			name:     "single wildcard replacement",
			template: "anthropic/claude-*",
			captured: []string{"sonnet-4-5"},
			expected: "anthropic/claude-sonnet-4-5",
		},
		{
			name:     "multi wildcard replacement",
			template: "*::static::*",
			captured: []string{"foo", "bar"},
			expected: "foo::static::bar",
		},
		{
			name:     "no wildcards in template",
			template: "anthropic/claude-fixed",
			captured: []string{"ignored"},
			expected: "anthropic/claude-fixed",
		},
		{
			name:     "empty captured",
			template: "anthropic/claude-*",
			captured: []string{""},
			expected: "anthropic/claude-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveModel(tt.template, tt.captured)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestSpecificity(t *testing.T) {
	// "claude-sonnet-*" is more specific than "claude-*"
	l1, wc1 := Specificity("claude-sonnet-*")
	l2, wc2 := Specificity("claude-*")

	assert.Greater(t, l1, l2, "longer pattern should have greater length")
	assert.Equal(t, wc1, wc2, "same wildcard count")

	// Same length, fewer wildcards is more specific
	l3, wc3 := Specificity("ab*cd")
	l4, wc4 := Specificity("a**cd")
	assert.Equal(t, l3, l4)
	assert.Less(t, wc3, wc4)
}

func TestPatternToRegex(t *testing.T) {
	// Verify regex special chars are properly escaped
	re := PatternToRegex("model.v2.*")
	assert.True(t, re.MatchString("model.v2.latest"))
	assert.False(t, re.MatchString("modelXv2Xlatest"))
}
