package middleware

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// allowedCacheControlsKey is the context key for allowed cache controls.
var allowedCacheControlsKey contextKey = "allowed_cache_controls"

// NewCacheControlMiddleware creates middleware that validates cache control parameters.
// If a request includes a "cache" parameter, each cache control directive is checked
// against the allowed list from the VerificationToken.
func NewCacheControlMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			allowed, _ := r.Context().Value(allowedCacheControlsKey).([]string)

			// No allowed controls configured — skip check
			if len(allowed) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Only check POST requests with JSON bodies (chat/completion endpoints)
			if r.Method != http.MethodPost || r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Peek at body to check for cache field without consuming it
			// We use a lightweight approach: decode into a map, check, re-encode
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check if request has cache controls
			cacheVal, hasCacheField := body["cache"]
			if !hasCacheField || cacheVal == nil {
				// Re-encode body for downstream
				r.Body = reencodeBody(body)
				next.ServeHTTP(w, r)
				return
			}

			// Extract cache controls from request
			requestedControls := extractCacheControls(cacheVal)
			if len(requestedControls) == 0 {
				r.Body = reencodeBody(body)
				next.ServeHTTP(w, r)
				return
			}

			// Validate each requested control
			allowedSet := make(map[string]bool, len(allowed))
			for _, a := range allowed {
				allowedSet[a] = true
			}

			for _, ctrl := range requestedControls {
				if !allowedSet[ctrl] {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					writeJSONResponse(w, model.ErrorResponse{
						Error: model.ErrorDetail{
							Message: "cache control '" + ctrl + "' not allowed for this key",
							Type:    "permission_denied",
							Code:    "cache_control_not_allowed",
						},
					})
					return
				}
			}

			r.Body = reencodeBody(body)
			next.ServeHTTP(w, r)
		})
	}
}

// extractCacheControls pulls cache control directives from a request body value.
func extractCacheControls(v any) []string {
	switch val := v.(type) {
	case map[string]any:
		if t, ok := val["type"].(string); ok {
			return []string{t}
		}
		var result []string
		for k := range val {
			result = append(result, k)
		}
		return result
	case []any:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		return []string{val}
	default:
		return nil
	}
}

// reencodeBody re-encodes a decoded JSON body back into an io.ReadCloser.
func reencodeBody(body map[string]any) *readCloser {
	data, _ := json.Marshal(body)
	return &readCloser{data: data}
}

type readCloser struct {
	data []byte
	pos  int
}

func (rc *readCloser) Read(p []byte) (n int, err error) {
	if rc.pos >= len(rc.data) {
		return 0, io.EOF
	}
	n = copy(p, rc.data[rc.pos:])
	rc.pos += n
	return n, nil
}

func (rc *readCloser) Close() error { return nil }
