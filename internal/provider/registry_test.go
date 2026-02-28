package provider

import (
	"testing"
)

func TestParseModelName(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
	}{
		{"openai/gpt-4o", "openai", "gpt-4o"},
		{"anthropic/claude-3-opus", "anthropic", "claude-3-opus"},
		{"gpt-4o", "openai", "gpt-4o"},
		{"", "openai", ""},
		{"a/b/c", "a", "b/c"},
	}
	for _, tt := range tests {
		p, m := ParseModelName(tt.input)
		if p != tt.wantProvider || m != tt.wantModel {
			t.Errorf("ParseModelName(%q) = (%q,%q), want (%q,%q)", tt.input, p, m, tt.wantProvider, tt.wantModel)
		}
	}
}

func TestGetUnknown(t *testing.T) {
	_, err := Get("nonexistent-provider-xyz")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}
