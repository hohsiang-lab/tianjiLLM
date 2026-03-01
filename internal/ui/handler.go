package ui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/a-h/templ"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/ratelimitstate"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// UIHandler holds dependencies for the admin dashboard UI.
type UIHandler struct {
	DB            *db.Queries
	Pool          *pgxpool.Pool
	Config        *config.ProxyConfig
	Cache         cache.Cache
	MasterKey      string
	RateLimitStore *ratelimitstate.Store
	Pricing       *pricing.Calculator
	syncPricingMu sync.Mutex
}

func (h *UIHandler) masterKeyHash() string {
	sum := sha256.Sum256([]byte(h.MasterKey))
	return hex.EncodeToString(sum[:])
}

func (h *UIHandler) sessionKey() string {
	return h.masterKeyHash()[:32]
}

func (h *UIHandler) authenticateKey(apiKey string) (role string, ok bool) {
	keyHash := sha256.Sum256([]byte(apiKey))
	got := hex.EncodeToString(keyHash[:])
	if got == h.masterKeyHash() {
		return "admin", true
	}
	return "", false
}

// render writes a templ component to w, logging any rendering error.
func render(ctx context.Context, w io.Writer, c templ.Component) {
	if err := c.Render(ctx, w); err != nil {
		log.Printf("ui: render error: %v", err)
	}
}

func (h *UIHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		render(r.Context(), w, pages.LoginPage(""))
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	apiKey := r.FormValue("api_key")
	if apiKey == "" {
		render(r.Context(), w, pages.LoginPage("API key is required"))
		return
	}

	role, ok := h.authenticateKey(apiKey)
	if !ok {
		render(r.Context(), w, pages.LoginPage("Invalid API key"))
		return
	}

	setSessionCookie(w, h.sessionKey(), role, "")
	http.Redirect(w, r, "/ui/", http.StatusSeeOther)
}

func (h *UIHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
}

func (h *UIHandler) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := getSessionFromRequest(r, h.sessionKey())
		if !ok || session.Role != "admin" {
			if r.Header.Get("HX-Request") == "true" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *UIHandler) sessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := getSessionFromRequest(r, h.sessionKey())
		if !ok {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/ui/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
