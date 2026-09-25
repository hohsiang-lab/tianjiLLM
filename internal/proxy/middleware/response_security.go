package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/model"
)

// ResponseIDSecurity encrypts/decrypts response IDs to prevent cross-user access.
// Format: base64(HMAC-SHA256("response_id;user_id;team_id"))
type ResponseIDSecurity struct {
	secretKey []byte
}

// NewResponseIDSecurity creates a response ID security handler.
func NewResponseIDSecurity(secretKey string) *ResponseIDSecurity {
	return &ResponseIDSecurity{
		secretKey: []byte(secretKey),
	}
}

// EncryptID creates a secure response ID encoding user/team ownership.
func (s *ResponseIDSecurity) EncryptID(responseID, userID, teamID string) string {
	payload := fmt.Sprintf("tianji_proxy:responses_api:response_id:%s;user_id:%s;team_id:%s", responseID, userID, teamID)
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write([]byte(payload))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", responseID, sig[:16])
}

// DecryptID validates and extracts the original response ID.
// Returns the original ID, user_id, team_id, and whether it's valid.
func (s *ResponseIDSecurity) DecryptID(encryptedID, requestUserID, requestTeamID string) (string, bool) {
	parts := strings.SplitN(encryptedID, ".", 2)
	if len(parts) != 2 {
		return encryptedID, true // not encrypted, pass through
	}

	responseID := parts[0]
	sig := parts[1]

	// Verify signature for the requesting user/team
	expected := s.EncryptID(responseID, requestUserID, requestTeamID)
	expectedParts := strings.SplitN(expected, ".", 2)
	if len(expectedParts) == 2 && sig == expectedParts[1] {
		return responseID, true
	}

	return "", false
}

// NewResponseSecurityMiddleware creates middleware for response ID security.
// Only applies to response-related endpoints (GET/POST /v1/responses/*).
func NewResponseSecurityMiddleware(security *ResponseIDSecurity) func(http.Handler) http.Handler {
	if security == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only apply to responses endpoints
			if !strings.Contains(r.URL.Path, "/responses/") {
				next.ServeHTTP(w, r)
				return
			}

			// Skip for admin users
			isMaster, _ := r.Context().Value(ContextKeyIsMasterKey).(bool)
			if isMaster {
				next.ServeHTTP(w, r)
				return
			}

			userID, _ := r.Context().Value(ContextKeyUserID).(string)
			teamID, _ := r.Context().Value(ContextKeyTeamID).(string)

			// Extract response ID from URL path
			pathParts := strings.Split(r.URL.Path, "/responses/")
			if len(pathParts) < 2 {
				next.ServeHTTP(w, r)
				return
			}

			responseID := strings.Split(pathParts[1], "/")[0]
			if responseID == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Validate the response ID
			_, valid := security.DecryptID(responseID, userID, teamID)
			if !valid {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				writeJSONResponse(w, model.ErrorResponse{
					Error: model.ErrorDetail{
						Message: "access denied: response does not belong to this user/team",
						Type:    "permission_denied",
						Code:    "response_access_denied",
					},
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
