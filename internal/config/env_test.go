package config

import (
	"os"
	"testing"
)

func TestResolveEnvVar(t *testing.T) {
	os.Setenv("TEST_TIANJI_KEY", "secret123")
	defer os.Unsetenv("TEST_TIANJI_KEY")

	got := ResolveEnvVar("os.environ/TEST_TIANJI_KEY")
	if got != "secret123" {
		t.Fatalf("got %q, want secret123", got)
	}

	got = ResolveEnvVar("plain_value")
	if got != "plain_value" {
		t.Fatalf("got %q, want plain_value", got)
	}

	got = ResolveEnvVar("os.environ/NONEXISTENT_VAR_XYZ")
	if got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestResolveEnvVarPtr(t *testing.T) {
	if ResolveEnvVarPtr(nil) != nil {
		t.Fatal("nil input should return nil")
	}

	os.Setenv("TEST_PTR_VAR", "val")
	defer os.Unsetenv("TEST_PTR_VAR")

	s := "os.environ/TEST_PTR_VAR"
	result := ResolveEnvVarPtr(&s)
	if result == nil || *result != "val" {
		t.Fatalf("got %v, want val", result)
	}
}
