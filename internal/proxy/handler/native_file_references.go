package handler

import (
	"context"
	"encoding/json"
)

// Validate every native file reference, including future nested fields, without rewriting bytes.
func (h *Handlers) nativeFileReferences(ctx context.Context, raw []byte, c resolvedOpenAISubscriptionCredential) bool {
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	var valid func(any) bool
	valid = func(v any) bool {
		switch v := v.(type) {
		case map[string]any:
			for k, item := range v {
				if k == "file_id" {
					id, ok := item.(string)
					if !ok {
						return false
					}
					b, e := h.loadNativeFileBinding(ctx, id)
					if e != nil || !b.Ready || b.CredentialID != c.CredentialID || b.AccountID != c.AccountID {
						return false
					}
				}
				if !valid(item) {
					return false
				}
			}
		case []any:
			for _, item := range v {
				if !valid(item) {
					return false
				}
			}
		}
		return true
	}
	return valid(value)
}
