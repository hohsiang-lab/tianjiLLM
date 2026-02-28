package token

import (
	"testing"
)

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New returned nil")
	}
}

func TestCountTextGPT4o(t *testing.T) {
	c := New()
	n := c.CountText("gpt-4o", "hello world")
	if n <= 0 {
		t.Fatalf("CountText = %d, want > 0", n)
	}
}

func TestCountTextGPT35(t *testing.T) {
	c := New()
	n := c.CountText("gpt-3.5-turbo", "hello world")
	if n <= 0 {
		t.Fatalf("CountText = %d, want > 0", n)
	}
}

func TestCountTextWithProvider(t *testing.T) {
	c := New()
	n := c.CountText("openai/gpt-4o", "test")
	if n <= 0 {
		t.Fatalf("CountText with provider prefix = %d, want > 0", n)
	}
}

func TestCountTextUnsupportedModel(t *testing.T) {
	c := New()
	n := c.CountText("claude-3-opus", "hello")
	if n != -1 {
		t.Fatalf("CountText unsupported = %d, want -1", n)
	}
}

func TestCountTextEmpty(t *testing.T) {
	c := New()
	n := c.CountText("gpt-4o", "")
	if n != 0 {
		t.Fatalf("CountText empty = %d, want 0", n)
	}
}

func TestCountMessages(t *testing.T) {
	c := New()
	msgs := []Message{
		{Role: "system", Content: "You are a helper."},
		{Role: "user", Content: "Hello!"},
	}
	n := c.CountMessages("gpt-4o", msgs)
	if n <= 0 {
		t.Fatalf("CountMessages = %d, want > 0", n)
	}
}

func TestCountMessagesWithName(t *testing.T) {
	c := New()
	msgs := []Message{
		{Role: "user", Content: "Hi", Name: "alice"},
	}
	n := c.CountMessages("gpt-4o", msgs)
	nNoName := c.CountMessages("gpt-4o", []Message{{Role: "user", Content: "Hi"}})
	if n <= nNoName {
		t.Fatalf("message with name (%d) should have more tokens than without (%d)", n, nNoName)
	}
}

func TestCountMessagesUnsupported(t *testing.T) {
	c := New()
	n := c.CountMessages("claude-3", []Message{{Role: "user", Content: "hi"}})
	if n != -1 {
		t.Fatalf("CountMessages unsupported = %d, want -1", n)
	}
}

func TestModelToEncodingO1(t *testing.T) {
	enc := modelToEncoding("o1-mini")
	if enc != "o200k_base" {
		t.Fatalf("o1-mini encoding = %q, want o200k_base", enc)
	}
}

func TestModelToEncodingO3(t *testing.T) {
	enc := modelToEncoding("o3-mini")
	if enc != "o200k_base" {
		t.Fatalf("o3-mini encoding = %q, want o200k_base", enc)
	}
}

func TestModelToEncodingChatGPT4o(t *testing.T) {
	enc := modelToEncoding("chatgpt-4o-latest")
	if enc != "o200k_base" {
		t.Fatalf("chatgpt-4o encoding = %q, want o200k_base", enc)
	}
}

func TestCounterCachesEncoders(t *testing.T) {
	c := New()
	c.CountText("gpt-4o", "a")
	c.CountText("gpt-4o", "b")
	// Should use cached encoder — just verify no panic and correct result
	n := c.CountText("gpt-4o", "hello")
	if n <= 0 {
		t.Fatalf("cached encoder CountText = %d", n)
	}
}
