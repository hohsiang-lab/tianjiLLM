package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestNewJWTValidator(t *testing.T) {
	v := NewJWTValidator(JWTConfig{
		JWKSURL:   "https://example.com/.well-known/jwks.json",
		Issuer:    "test-issuer",
		Audiences: []string{"my-app"},
	})
	if v == nil {
		t.Fatal("NewJWTValidator returned nil")
	}
	if v.cacheTTL != 5*time.Minute {
		t.Fatalf("default cacheTTL = %v, want 5m", v.cacheTTL)
	}
}

func TestNewJWTValidatorCustomTTL(t *testing.T) {
	v := NewJWTValidator(JWTConfig{CacheTTL: 10 * time.Minute})
	if v.cacheTTL != 10*time.Minute {
		t.Fatalf("cacheTTL = %v, want 10m", v.cacheTTL)
	}
}

func TestSetKeys(t *testing.T) {
	v := NewJWTValidator(JWTConfig{})
	secret := []byte("test-secret")
	v.SetKeys(map[string]any{"kid1": secret})

	v.mu.RLock()
	defer v.mu.RUnlock()
	if _, ok := v.cachedKeys["kid1"]; !ok {
		t.Fatal("key not set")
	}
}

// makeHS256Token creates a minimal HS256 JWT for testing.
func makeHS256Token(t *testing.T, secret []byte, kid string, claims map[string]any) string {
	t.Helper()
	header := map[string]any{"alg": "HS256", "typ": "JWT", "kid": kid}
	hb, _ := json.Marshal(header)
	cb, _ := json.Marshal(claims)
	segments := base64.RawURLEncoding.EncodeToString(hb) + "." + base64.RawURLEncoding.EncodeToString(cb)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(segments))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return segments + "." + sig
}

func TestValidateTokenSuccess(t *testing.T) {
	secret := []byte("super-secret-key-at-least-32-bytes!")
	v := NewJWTValidator(JWTConfig{})
	v.SetKeys(map[string]any{"k1": secret})

	now := time.Now()
	token := makeHS256Token(t, secret, "k1", map[string]any{
		"sub":   "user-123",
		"email": "test@example.com",
		"role":  "proxy_admin",
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})

	claims, err := v.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %q, want user-123", claims.UserID)
	}
	if claims.Email != "test@example.com" {
		t.Errorf("Email = %q", claims.Email)
	}
}

func TestValidateTokenExpired(t *testing.T) {
	secret := []byte("super-secret-key-at-least-32-bytes!")
	v := NewJWTValidator(JWTConfig{})
	v.SetKeys(map[string]any{"k1": secret})

	past := time.Now().Add(-2 * time.Hour)
	token := makeHS256Token(t, secret, "k1", map[string]any{
		"sub": "user",
		"iat": past.Unix(),
		"exp": past.Add(time.Hour).Unix(),
	})

	_, err := v.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestValidateTokenUnknownKid(t *testing.T) {
	secret := []byte("super-secret-key-at-least-32-bytes!")
	v := NewJWTValidator(JWTConfig{})
	// No keys set

	token := makeHS256Token(t, secret, "unknown", map[string]any{
		"sub": "user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := v.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for unknown kid")
	}
}

func TestValidateTokenInvalidAudience(t *testing.T) {
	secret := []byte("super-secret-key-at-least-32-bytes!")
	v := NewJWTValidator(JWTConfig{Audiences: []string{"my-app"}})
	v.SetKeys(map[string]any{"k1": secret})

	token := makeHS256Token(t, secret, "k1", map[string]any{
		"sub": "user",
		"aud": "other-app",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := v.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for invalid audience")
	}
	if !strings.Contains(err.Error(), "audience") {
		t.Fatalf("error should mention audience: %v", err)
	}
}

func TestValidateTokenGarbageInput(t *testing.T) {
	v := NewJWTValidator(JWTConfig{})
	_, err := v.ValidateToken(context.Background(), "not.a.jwt")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateTokenWithIssuer(t *testing.T) {
	secret := []byte("super-secret-key-at-least-32-bytes!")
	v := NewJWTValidator(JWTConfig{Issuer: "correct-issuer"})
	v.SetKeys(map[string]any{"k1": secret})

	token := makeHS256Token(t, secret, "k1", map[string]any{
		"sub": "user",
		"iss": "wrong-issuer",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err := v.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}
