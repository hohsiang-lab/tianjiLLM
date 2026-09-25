package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// VirtualKeyPrefix is the prefix for all TianjiLLM virtual keys.
// The sk-ant-oat01- prefix passes OpenClaw's OAuth token detection;
// tianji- distinguishes from real Anthropic OAuth tokens.
const VirtualKeyPrefix = "sk-ant-oat01-tianji-"

// GenerateVirtualKey produces a virtual key: sk-ant-oat01-tianji-<60 hex chars> (80 chars total).
func GenerateVirtualKey() string {
	b := make([]byte, 30)
	_, _ = rand.Read(b) // crypto/rand.Read never returns an error in Go 1.22+; it panics on failure.
	return VirtualKeyPrefix + hex.EncodeToString(b)
}

// HashKey returns the SHA256 hex digest of a key string.
func HashKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}
