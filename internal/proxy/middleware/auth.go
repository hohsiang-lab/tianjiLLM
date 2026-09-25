package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/wildcard"
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
	ContextKeyRequesterIP   contextKey = "requester_ip"
	ContextKeyUpstreamToken contextKey = "upstream_token"
)

// TokenInfo holds the result of a virtual key lookup.
// A nil limit means "no limit set at any level".
type TokenInfo struct {
	UserID     *string
	TeamID     *string
	OrgID      *string
	Blocked    bool
	Guardrails []string

	// RPM/TPM/MaxBudget: resolved via inheritance chain (key → team → org).
	// MaxParallelRequests: from key's budget record (not inherited).
	// Models/Expires: key-level only (not inherited).
	RpmLimit            *int64
	TpmLimit            *int64
	MaxBudget           *float64
	Spend               float64
	MaxParallelRequests *int32
	Models              []string
	Expires             *time.Time
}

// TokenValidator looks up a virtual key by its hash.
// Returns all key metadata from a single DB call.
type TokenValidator interface {
	ValidateToken(ctx context.Context, tokenHash string) (*TokenInfo, error)
}

// AuthErrorLogger records authentication failures to persistent storage.
type AuthErrorLogger interface {
	LogAuthError(ctx context.Context, requestID string, apiKeyHash string, statusCode int, errorMsg string)
}

// AuthConfig holds configuration for the auth middleware.
type AuthConfig struct {
	MasterKey     string
	Validator     TokenValidator
	JWTValidator  *auth.JWTValidator
	RBACEngine    *auth.RBACEngine
	EnableJWTAuth bool
	ErrorLogger   AuthErrorLogger // optional: records auth failures to ErrorLogs
}

// NewAuthMiddleware creates an auth middleware that validates
// the master key, JWT tokens, or virtual keys from the database.
// Follows Python LiteLLM's decision tree:
//  1. Master key check
//  2. JWT auth (if enabled and token looks like JWT)
//  3. Virtual key from DB
func NewAuthMiddleware(cfg AuthConfig) func(http.Handler) http.Handler {
	masterKeyHash := hashToken(cfg.MasterKey)

	logFailure := func(ctx context.Context, tokenHash string, status int, msg string) {
		if cfg.ErrorLogger != nil {
			reqID := chiMiddleware.GetReqID(ctx)
			// Use context.Background() to avoid silent DB write failures when the
			// request context is cancelled before the goroutine executes.
			go cfg.ErrorLogger.LogAuthError(context.Background(), reqID, tokenHash, status, msg)
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractToken(r)
			if token == "" {
				logFailure(r.Context(), "", http.StatusUnauthorized, "missing API key")
				authError(w, "missing API key", http.StatusUnauthorized)
				return
			}

			requesterIP := extractClientIP(r)

			// 1. Check master key (constant-time via hash comparison)
			tokenHash := hashToken(token)
			if cfg.MasterKey != "" && tokenHash == masterKeyHash {
				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, true)
				ctx = context.WithValue(ctx, ContextKeyTokenHash, tokenHash)
				ctx = context.WithValue(ctx, ContextKeyRole, auth.RoleProxyAdmin)
				ctx = context.WithValue(ctx, ContextKeyRequesterIP, requesterIP)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// 2. JWT auth (if enabled and token looks like a JWT — 3 dot-separated segments)
			if cfg.EnableJWTAuth && cfg.JWTValidator != nil && isJWT(token) {
				claims, err := cfg.JWTValidator.ValidateToken(r.Context(), token)
				if err != nil {
					zerolog.Ctx(r.Context()).Warn().Err(err).Msg("JWT validation failed")
					logFailure(r.Context(), tokenHash, http.StatusUnauthorized, "invalid JWT token")
					authError(w, "invalid JWT token", http.StatusUnauthorized)
					return
				}

				role := resolveRole(claims)

				// RBAC route check
				if cfg.RBACEngine != nil {
					if err := cfg.RBACEngine.CheckRouteAccess(role, r.URL.Path); err != nil {
						msg := fmt.Sprintf("access denied: %s", err)
						logFailure(r.Context(), tokenHash, http.StatusForbidden, msg)
						authError(w, msg, http.StatusForbidden)
						return
					}
				}

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, false)
				ctx = context.WithValue(ctx, ContextKeyRole, role)
				ctx = context.WithValue(ctx, ContextKeyRequesterIP, requesterIP)
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
				info, err := cfg.Validator.ValidateToken(r.Context(), tokenHash)
				if err != nil {
					if errors.Is(err, ErrDBUnavailable) {
						zerolog.Ctx(r.Context()).Error().Err(err).Msg("auth error: database unavailable")
						logFailure(r.Context(), tokenHash, http.StatusServiceUnavailable, "database unavailable")
						authError(w, "service temporarily unavailable", http.StatusServiceUnavailable)
					} else {
						zerolog.Ctx(r.Context()).Warn().Str("token_hash_prefix", tokenHash[:8]).Msg("auth failed: key not found")
						logFailure(r.Context(), tokenHash, http.StatusUnauthorized, "invalid API key")
						authError(w, "invalid API key", http.StatusUnauthorized)
					}
					return
				}
				if info.Blocked {
					zerolog.Ctx(r.Context()).Warn().Str("token_hash_prefix", tokenHash[:8]).Msg("auth failed: key is blocked")
					logFailure(r.Context(), tokenHash, http.StatusForbidden, "API key is blocked")
					authError(w, "API key is blocked", http.StatusForbidden)
					return
				}
				zerolog.Ctx(r.Context()).Info().Interface("user_id", info.UserID).Interface("team_id", info.TeamID).Msg("virtual key auth success")

				ctx := r.Context()
				ctx = context.WithValue(ctx, ContextKeyIsMasterKey, false)
				ctx = context.WithValue(ctx, ContextKeyTokenHash, tokenHash)
				ctx = context.WithValue(ctx, ContextKeyAllowedModels, info.Models)
				ctx = context.WithValue(ctx, ContextKeyRequesterIP, requesterIP)
				if info.UserID != nil {
					ctx = context.WithValue(ctx, ContextKeyUserID, *info.UserID)
				}
				if info.TeamID != nil {
					ctx = context.WithValue(ctx, ContextKeyTeamID, *info.TeamID)
				}
				if info.OrgID != nil {
					ctx = context.WithValue(ctx, ContextKeyOrgID, *info.OrgID)
				}
				if len(info.Guardrails) > 0 {
					ctx = context.WithValue(ctx, ContextKeyGuardrails, info.Guardrails)
				}

				// Inject per-key limits for downstream middleware.
				// Only inject positive values; zero is treated as "no limit" by DynamicRateLimiter.
				if info.RpmLimit != nil && *info.RpmLimit > 0 {
					ctx = context.WithValue(ctx, rpmLimitKey, *info.RpmLimit)
				}
				if info.TpmLimit != nil && *info.TpmLimit > 0 {
					ctx = context.WithValue(ctx, tpmLimitKey, *info.TpmLimit)
				}
				if info.MaxParallelRequests != nil && *info.MaxParallelRequests > 0 {
					ctx = context.WithValue(ctx, maxParallelLimitKey, int(*info.MaxParallelRequests))
				}
				if info.MaxBudget != nil && *info.MaxBudget > 0 {
					ctx = context.WithValue(ctx, maxBudgetKey, *info.MaxBudget)
				}
				ctx = context.WithValue(ctx, spendKey, info.Spend)

				// Check key expiry.
				if info.Expires != nil && time.Now().After(*info.Expires) {
					zerolog.Ctx(r.Context()).Warn().Str("token_hash_prefix", tokenHash[:8]).Msg("auth failed: key expired")
					logFailure(r.Context(), tokenHash, http.StatusForbidden, "API key has expired")
					authError(w, "API key has expired", http.StatusForbidden)
					return
				}

				// Models restriction check — fail-closed: if model cannot be
				// determined (body read error), reject rather than allow.
				if len(info.Models) > 0 {
					model := extractModelFromRequest(r)
					if model == "" {
						zerolog.Ctx(r.Context()).Warn().
							Str("token_hash_prefix", tokenHash[:8]).
							Strs("allowed_models", info.Models).
							Msg("auth failed: cannot determine model for restricted key")
						logFailure(r.Context(), tokenHash, http.StatusForbidden, "cannot determine model for restricted key")
						authError(w, "cannot determine model for restricted key", http.StatusForbidden)
						return
					}
					if !isModelAllowed(model, info.Models) {
						zerolog.Ctx(r.Context()).Warn().
							Str("token_hash_prefix", tokenHash[:8]).
							Str("model", model).
							Strs("allowed_models", info.Models).
							Msg("auth failed: model not allowed")
						logFailure(r.Context(), tokenHash, http.StatusForbidden, "model not allowed: "+model)
						authError(w, "model not allowed: "+model, http.StatusForbidden)
						return
					}
				}

				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			logFailure(r.Context(), tokenHash, http.StatusUnauthorized, "invalid API key")
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

// extractClientIP returns the real client IP from the request.
//
// Architecture: Client → Cloudflare Edge → cloudflared pod → tianji (direct, bypasses Traefik).
// cloudflared forwards CF-Connecting-IP set by Cloudflare — this header is not
// spoofable by clients because cloudflared initiates outbound connections to
// Cloudflare Edge; no external party can reach the pod directly.
//
// Falls back to RemoteAddr for local development where CF-Connecting-IP is absent.
// The returned value is validated with net.ParseIP; if invalid, RemoteAddr is used.
func extractClientIP(r *http.Request) string {
	if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
		if ip := net.ParseIP(cf); ip != nil {
			return ip.String()
		}
		zerolog.Ctx(r.Context()).Warn().
			Str("cf_connecting_ip_raw", cf).
			Msg("CF-Connecting-IP header present but failed net.ParseIP validation; falling back to RemoteAddr")
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// extractModelFromRequest reads the model name from the request body (JSON)
// or falls back to URL path patterns. The body is buffered and reset so
// downstream handlers can read it again.
func extractModelFromRequest(r *http.Request) string {
	if r.Body != nil {
		bodyBytes, err := io.ReadAll(r.Body)
		// Always restore body so downstream handlers can read it.
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		if err != nil {
			zerolog.Ctx(r.Context()).Warn().Err(err).Msg("failed to read request body for model extraction")
			return "" // caller treats empty string as "unknown model"
		}
		if len(bodyBytes) > 0 {
			var partial struct {
				Model string `json:"model"`
			}
			if json.Unmarshal(bodyBytes, &partial) == nil && partial.Model != "" {
				return partial.Model
			}
		}
	}
	return extractModelFromPath(r.URL.Path)
}

// extractModelFromPath extracts a model name from known URL patterns:
//   - /openai/deployments/{model}/...
//   - /v1beta/models/{model}:...
//   - /v1/models/{model}
//   - /models/{model}
func extractModelFromPath(path string) string {
	// /openai/deployments/{model}/...
	const deploymentsPrefix = "/openai/deployments/"
	if strings.HasPrefix(path, deploymentsPrefix) {
		rest := path[len(deploymentsPrefix):]
		if idx := strings.IndexByte(rest, '/'); idx > 0 {
			return rest[:idx]
		}
		if rest != "" {
			return rest
		}
	}

	// /v1beta/models/{model}:generateContent etc.
	const modelsPrefix = "/v1beta/models/"
	if strings.HasPrefix(path, modelsPrefix) {
		rest := path[len(modelsPrefix):]
		if idx := strings.IndexByte(rest, ':'); idx > 0 {
			return rest[:idx]
		}
		if rest != "" {
			return rest
		}
	}

	for _, prefix := range []string{"/v1/models/", "/models/"} {
		if strings.HasPrefix(path, prefix) {
			rest := path[len(prefix):]
			if idx := strings.IndexByte(rest, '/'); idx >= 0 {
				rest = rest[:idx]
			}
			return rest
		}
	}

	return ""
}

// isModelAllowed checks if the model matches any entry in the allowed list.
// Supports exact match and wildcard patterns via the wildcard package.
func isModelAllowed(model string, allowed []string) bool {
	for _, pattern := range allowed {
		if pattern == model {
			return true
		}
		if strings.Contains(pattern, "*") && wildcard.Match(pattern, model) != nil {
			return true
		}
	}
	return false
}
