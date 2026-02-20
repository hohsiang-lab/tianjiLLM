package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTValidator validates JWT tokens using a JWKS endpoint.
type JWTValidator struct {
	jwksURL    string
	keyFunc    jwt.Keyfunc
	issuer     string
	audiences  []string
	mu         sync.RWMutex
	cachedKeys map[string]any
	cacheTime  time.Time
	cacheTTL   time.Duration
}

// JWTConfig holds JWT validation configuration.
type JWTConfig struct {
	JWKSURL   string
	Issuer    string
	Audiences []string
	CacheTTL  time.Duration
}

// NewJWTValidator creates a new JWT validator.
func NewJWTValidator(cfg JWTConfig) *JWTValidator {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 5 * time.Minute
	}

	v := &JWTValidator{
		jwksURL:    cfg.JWKSURL,
		issuer:     cfg.Issuer,
		audiences:  cfg.Audiences,
		cachedKeys: make(map[string]any),
		cacheTTL:   cfg.CacheTTL,
	}

	// Use static key func that delegates to cached JWKS
	v.keyFunc = func(token *jwt.Token) (any, error) {
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("missing kid in token header")
		}

		v.mu.RLock()
		key, exists := v.cachedKeys[kid]
		v.mu.RUnlock()

		if exists {
			return key, nil
		}

		return nil, fmt.Errorf("unknown key ID: %s", kid)
	}

	return v
}

// JWTClaims holds the parsed JWT claims relevant to TianjiLLM.
type JWTClaims struct {
	UserID string   `json:"sub"`
	Email  string   `json:"email"`
	Role   string   `json:"role"`
	TeamID string   `json:"team_id"`
	OrgID  string   `json:"org_id"`
	Scopes []string `json:"scopes"`
	jwt.RegisteredClaims
}

// ValidateToken parses and validates a JWT token string.
func (v *JWTValidator) ValidateToken(_ context.Context, tokenStr string) (*JWTClaims, error) {
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{"RS256", "ES256", "HS256"}),
	}

	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}

	token, err := jwt.ParseWithClaims(tokenStr, &JWTClaims{}, v.keyFunc, opts...)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid JWT claims")
	}

	// Validate audience if configured
	if len(v.audiences) > 0 {
		found := false
		for _, aud := range v.audiences {
			for _, claimAud := range claims.Audience {
				if aud == claimAud {
					found = true
					break
				}
			}
		}
		if !found {
			return nil, errors.New("invalid audience")
		}
	}

	return claims, nil
}

// SetKeys manually sets the JWKS keys (for testing or static config).
func (v *JWTValidator) SetKeys(keys map[string]any) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.cachedKeys = keys
	v.cacheTime = time.Now()
}
