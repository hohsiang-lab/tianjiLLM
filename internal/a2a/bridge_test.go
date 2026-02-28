package a2a

import (
	"context"
	"testing"
)

func TestNewCompletionBridge(t *testing.T) {
	b := NewCompletionBridge(nil)
	if b == nil {
		t.Fatal("nil bridge")
	}
}

func TestSendMessageNilHandler(t *testing.T) {
	b := NewCompletionBridge(nil)
	_, err := b.SendMessage(context.Background(), &AgentConfig{}, "hello")
	if err == nil {
		t.Fatal("expected error with nil handler")
	}
}

func TestExtractModel(t *testing.T) {
	tests := []struct {
		params any
		want   string
	}{
		{map[string]any{"model": "gpt-4o"}, "gpt-4o"},
		{map[string]any{}, ""},
		{nil, ""},
		{"not a map", ""},
	}
	for _, tt := range tests {
		got := extractModel(tt.params)
		if got != tt.want {
			t.Errorf("extractModel(%v) = %q, want %q", tt.params, got, tt.want)
		}
	}
}

func TestExtractSystemMessage(t *testing.T) {
	tests := []struct {
		params any
		want   string
	}{
		{map[string]any{"system_message": "You are helpful"}, "You are helpful"},
		{map[string]any{}, ""},
		{nil, ""},
	}
	for _, tt := range tests {
		got := extractSystemMessage(tt.params)
		if got != tt.want {
			t.Errorf("extractSystemMessage(%v) = %q, want %q", tt.params, got, tt.want)
		}
	}
}
