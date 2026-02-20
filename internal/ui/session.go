package ui

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"
)

const (
	cookieName   = "tianji_session"
	cookieMaxAge = 24 * time.Hour
	cookiePath   = "/ui"
)

type sessionPayload struct {
	Role    string `json:"r"`
	UserID  string `json:"u,omitempty"`
	Expires int64  `json:"e"`
}

func signSession(key, role, userID string) string {
	p := sessionPayload{
		Role:    role,
		UserID:  userID,
		Expires: time.Now().Add(cookieMaxAge).Unix(),
	}
	data, _ := json.Marshal(p)
	encoded := base64.RawURLEncoding.EncodeToString(data)
	sig := computeHMAC(key, encoded)
	return encoded + "." + sig
}

func verifySession(key, value string) (*sessionPayload, bool) {
	dot := -1
	for i := len(value) - 1; i >= 0; i-- {
		if value[i] == '.' {
			dot = i
			break
		}
	}
	if dot < 0 {
		return nil, false
	}
	encoded := value[:dot]
	sig := value[dot+1:]

	if computeHMAC(key, encoded) != sig {
		return nil, false
	}

	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return nil, false
	}

	var p sessionPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, false
	}
	if time.Now().Unix() > p.Expires {
		return nil, false
	}
	return &p, true
}

func computeHMAC(key, message string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setSessionCookie(w http.ResponseWriter, key, role, userID string) {
	value := signSession(key, role, userID)
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     cookiePath,
		MaxAge:   int(cookieMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   cookieName,
		Value:  "",
		Path:   cookiePath,
		MaxAge: -1,
	})
}

func getSessionFromRequest(r *http.Request, key string) (*sessionPayload, bool) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, false
	}
	return verifySession(key, c.Value)
}
