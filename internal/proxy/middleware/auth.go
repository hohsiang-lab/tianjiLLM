package middleware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
)

type contextKey string

const (
	ContextKeyUserID        contextKey = "user_id"
	ContextKeyTeamID        contextKey = "team_id"
	ContextKeyOrgID         contextKey = "org_id"
	ContextKeyTokenHash     contextKey = "token_hash"
	ContextKeyIsMasterKey   contextKey = "is_master_key"
	ContextKeyRole          contextKey = "role"
	ContextKeyAllowedModels contextKey = "allowed_models"
	ContextKeyGuardrails    contextKey = "guardrails"
)

// TokenValidator looks up a virtual key by its hash.
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenHash string) (userID, teamID *string, blocked bool, err error)
}

// GuardrailProvider optionally returns guardrail names for a token.
// If TokenValidator also implements this, guardrail names are added to context.
type GuardrailProvider interface {
	GetGuardrails(ctx context.Context, tokenHash string) ([]string, error)
}

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	MasterKey     string
	Validator     TokenValidator
	JWTValidator  *auth.JWTValidator
	RBACEngine    *auth.RBACEngine
	EnableJWTAuth bool
}

// NewAuthMiddleware creates an auth middleware that validates
// the master key, JWT tokens, or virtual keys from the database.
// Follows Python LiteLLM's decision tree:
//  1. Master key check
//  2. JWT auth (if enabled and token looks like JWT)
//  3. Virtual key from DB
func NewAuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	masterKeyHash := hashToken(cfg.MasterKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				authError(w, "missing API key", http.StatusUnauthorized)
				return
			}

			// 1. Check master key (constant-time via hash comparison)
			tokenHash := hashToken(token)
			if cfg.MasterKey != "" && tokenHash == masterKeyHash {
				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, true)
				ctx = context.WithValue(ctx, ContextKeyTokenHash, tokenHash)
				ctx = context.WithValue(ctx, ContextKeyRole, auth.RoleProxyAdmin)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. JWT auth (if enabled and token looks like a JWT — 3 dot-separated segments)
			if cfg.EnableJWTAuth && cfg.JWTValidator != nil && isJWT(token) {
				claims, err := cfg.JWTValidator.ValidateToken(r.Context(), token)
				if err != nil {
					log.Printf("JWT validation failed: %v", err)
					authError(w, "invalid JWT token", http.StatusUnauthorized)
					return
				}

				role := resolveRole(claims)

				// RBAC route check
				if cfg.RBACEngine != nil {
					if err := cfg.RBACEngine.CheckRouteAccess(role, r.URL.Path); err != nil {
						authError(w, fmt.Sprintf("access denied: %s", err), http.StatusForbidden)
						return
					}
				}

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, false)
				ctx = context.WithValue(ctx, ContextKeyRole, role)
				if claims.UserID != "" {
					ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
				}
				if claims.TeamID != "" {
					ctx = context.WithValue(ctx, ContextKeyTeamID, claims.TeamID)
				}
				if claims.OrgID != "" {
					ctx = context.WithValue(ctx, ContextKeyOrgID, claims.OrgID)
				}
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 3. Virtual key from DB
			if cfg.Validator != nil {
				userID, teamID, blocked, err := cfg.Validator.ValidateToken(r.Context(), tokenHash)
				if err != nil {
					if errors.Is(err, ErrDBUnavailable) {
						log.Printf("auth error: database unavailable: %v", err)
						authError(w, "service temporarily unavailable", http.StatusServiceUnavailable)
					} else {
						log.Printf("auth failed: key not found (hash=%s...)", tokenHash[:8])
						authError(w, "invalid API key", http.StatusUnauthorized)
					}
					return
				}
				if blocked {
					log.Printf("auth failed: key is blocked (hash=%s...)", tokenHash[:8])
					authError(w, "API key is blocked", http.StatusForbidden)
					return
				}
				log.Printf("virtual key auth: user=%v team=%v", userID, teamID)

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, false)
				ctx = context.WithValue(ctx, ContextKeyTokenHash, tokenHash)
				if userID != nil {
					ctx = context.WithValue(ctx, ContextKeyUserID, *userID)
				}
				if teamID != nil {
					ctx = context.WithValue(ctx, ContextKeyTeamID, *teamID)
				}

				// Load guardrail names if validator supports it
				if gp, ok := cfg.Validator.(GuardrailProvider); ok {
					if names, err := gp.GetGuardrails(r.Context(), tokenHash); err == nil && len(names) > 0 {
						ctx = context.WithValue(ctx, ContextKeyGuardrails, names)
					}
				}

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authError(w, "invalid API key", http.StatusUnauthorized)
		})
	}
}

// isJWT checks if a token looks like a JWT (3 dot-separated segments).
// Matches Python LiteLLM's JWTHandler.is_jwt().
func isJWT(token string) bool {
	return strings.Count(token, ".") == 2
}

// resolveRole determines the RBAC role from JWT claims.
// Follows Python's JWTHandler.get_rbac_role() priority:
//  1. Admin scope → PROXY_ADMIN
//  2. Explicit role claim
//  3. TeamID present → TEAM
//  4. UserID present → INTERNAL_USER
//  5. Default → INTERNAL_USER
func resolveRole(claims *auth.JWTClaims) auth.Role {
	// Check scopes for admin
	for _, scope := range claims.Scopes {
		if scope == "tianji_proxy_admin" {
			return auth.RoleProxyAdmin
		}
	}

	// Check explicit role claim
	if claims.Role != "" {
		if role, err := auth.ParseRole(claims.Role); err == nil {
			return role
		}
	}

	// Infer from presence of team/user IDs
	if claims.TeamID != "" {
		return auth.RoleTeam
	}

	return auth.RoleInternalUser
}

// extractToken extracts the API token from the request.
// Supports multiple header formats matching Python LiteLLM:
//   - Authorization: Bearer <token>
//   - api-key: <token> (Azure)
//   - x-api-key: <token> (Anthropic)
func extractToken(r *http.Request) string {
	// Standard Bearer token (highest priority after custom headers)
	if auth := r.Header.Get("Authorization"); auth != "" {
		if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
			return strings.TrimSpace(token)
		}
		// Also accept lowercase "bearer"
		if token, ok := strings.CutPrefix(auth, "bearer "); ok {
			return strings.TrimSpace(token)
		}
	}

	// Azure format
	if key := r.Header.Get("api-key"); key != "" {
		return key
	}

	// Anthropic format
	if key := r.Header.Get("x-api-key"); key != "" {
		return key
	}

	return ""
}

func authError(w http.ResponseWriter, msg string, status int) {
	http.Error(w, fmt.Sprintf(`{"error":{"message":"%s","type":"authentication_error"}}`, msg), status)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
