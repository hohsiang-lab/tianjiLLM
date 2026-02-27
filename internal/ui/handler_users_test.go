package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserStatusFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata []byte
		expected string
	}{
		{"nil metadata", nil, "active"},
		{"empty bytes", []byte{}, "active"},
		{"empty JSON object", []byte(`{}`), "active"},
		{"status active", []byte(`{"status":"active"}`), "active"},
		{"status disabled", []byte(`{"status":"disabled"}`), "disabled"},
		{"status deleted", []byte(`{"status":"deleted"}`), "deleted"},
		{"status empty string", []byte(`{"status":""}`), "active"},
		{"status non-string", []byte(`{"status":123}`), "active"},
		{"invalid JSON", []byte(`not json`), "active"},
		{"other keys no status", []byte(`{"foo":"bar"}`), "active"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, userStatusFromMetadata(tc.metadata))
		})
	}
}
