package redact

import (
	"encoding/json"
	"regexp"
	"strings"
)

const Marker = "[REDACTED]"

var (
	bearerTokenPattern = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]+`)
	jwtPattern         = regexp.MustCompile(`\b[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`)
	apiKeyPattern      = regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`)
	secretPairPattern  = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|id[_-]?token|credential[_-]?value|api[_-]?key|x-api-key|authorization[_-]?code|code[_-]?verifier|code[_-]?challenge|device[_-]?auth[_-]?id|user[_-]?code|authorization|jwt|token)"?\s*[:=]\s*"?[^"',\s}]+`)
)

var secretFieldNames = map[string]struct{}{
	"accesstoken":       {},
	"refreshtoken":      {},
	"idtoken":           {},
	"credentialvalue":   {},
	"authorization":     {},
	"apikey":            {},
	"xapikey":           {},
	"token":             {},
	"jwt":               {},
	"authorizationcode": {},
	"codeverifier":      {},
	"codechallenge":     {},
	"deviceauthid":      {},
	"usercode":          {},
}

// String redacts known token-bearing substrings from free text.
func String(input string) string {
	if input == "" {
		return input
	}
	output := bearerTokenPattern.ReplaceAllString(input, Marker)
	output = secretPairPattern.ReplaceAllStringFunc(output, func(match string) string {
		if idx := strings.IndexAny(match, ":="); idx >= 0 {
			return match[:idx+1] + Marker
		}
		return Marker
	})
	output = jwtPattern.ReplaceAllString(output, Marker)
	output = apiKeyPattern.ReplaceAllString(output, Marker)
	return output
}

// JSONValue returns a recursively redacted copy of JSON-like values.
func JSONValue(input any) any {
	switch typed := input.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, value := range typed {
			if IsSecretFieldName(key) {
				out[key] = Marker
				continue
			}
			out[key] = JSONValue(value)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, value := range typed {
			out[i] = JSONValue(value)
		}
		return out
	case string:
		return String(typed)
	default:
		return input
	}
}

// RawJSON redacts a JSON document, preserving valid JSON when possible.
func RawJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return []byte(String(string(raw)))
	}
	redacted, err := json.Marshal(JSONValue(payload))
	if err != nil {
		return []byte(String(string(raw)))
	}
	return redacted
}

func IsSecretFieldName(name string) bool {
	_, ok := secretFieldNames[normalizeFieldName(name)]
	return ok
}

func normalizeFieldName(name string) string {
	name = strings.ToLower(name)
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
