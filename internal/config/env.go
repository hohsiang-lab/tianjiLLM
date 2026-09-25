package config

import (
	"os"
	"strings"
)

// ResolveEnvVar resolves a value that may reference an environment variable.
// Supports the "os.environ/VAR_NAME" syntax used in Python LiteLLM configs.
// Returns the resolved value or the original string if no env var pattern found.
func ResolveEnvVar(value string) string {
	if envKey, ok := strings.CutPrefix(value, "os.environ/"); ok {
		if v, found := os.LookupEnv(envKey); found {
			return v
		}
		return ""
	}
	return value
}

// ResolveEnvVarPtr resolves a pointer string value.
func ResolveEnvVarPtr(value *string) *string {
	if value == nil {
		return nil
	}
	resolved := ResolveEnvVar(*value)
	return &resolved
}
