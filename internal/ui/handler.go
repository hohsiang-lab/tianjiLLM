package ui

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/praxisllmlab/tianjiLLM/internal/auth/social"
	"github.com/praxisllmlab/tianjiLLM/internal/cache"
	"github.com/praxisllmlab/tianjiLLM/internal/callback"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/pricing"
	"github.com/praxisllmlab/tianjiLLM/internal/provider/chatgptcodex"
	proxyhandler "github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/praxisllmlab/tianjiLLM/internal/ui/pages"
)

// UIHandler holds dependencies for the admin dashboard UI.
type UIHandler struct {
	DB                       *db.Queries
	Pool                     *pgxpool.Pool
	Config                   *config.ProxyConfig
	Cache                    cache.Cache
	MasterKey                string
	SessionManager           *scs.SessionManager
	SocialAuth               *social.Service
	Pricing                  *pricing.Calculator
	RateLimitStore           callback.RateLimitStore
	DisabledTokens           callback.DisabledTokenStore
	OpenAIOAuthHTTPClient    *http.Client
	OpenAIUpstreamHTTPClient *http.Client
	OpenAIUpstreamBaseURL    string
	CodexUsageFetcher        chatgptcodex.UsageFetcher
	CodexResetConsumer       chatgptcodex.ResetCreditConsumer
	CodexUsageCache          *proxyhandler.OpenAISubscriptionCodexUsageCache
	RefreshRuntimeModels     func(context.Context) error
	syncPricingMu            sync.Mutex
	sessionManagerOnce       sync.Once
	userMutationTxBeginner   transactionBeginner
}

func (h *UIHandler) authenticateKey(apiKey string) (role string, ok bool) {
	providedHash := sha256.Sum256([]byte(apiKey))
	expectedHash := sha256.Sum256([]byte(h.MasterKey))
	if subtle.ConstantTimeCompare(providedHash[:], expectedHash[:]) == 1 {
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
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodGet {
		render(r.Context(), w, pages.LoginPage(h.loginPageData(loginErrorMessage(r.URL.Query().Get("error")))))
		return
	}

	if !h.masterKeyLoginEnabled() {
		w.WriteHeader(http.StatusForbidden)
		render(r.Context(), w, pages.LoginPage(h.loginPageData("Master-key login is disabled")))
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	apiKey := r.FormValue("api_key")
	if apiKey == "" {
		render(r.Context(), w, pages.LoginPage(h.loginPageData("Master key is required")))
		return
	}

	role, ok := h.authenticateKey(apiKey)
	if !ok {
		render(r.Context(), w, pages.LoginPage(h.loginPageData("Invalid master key")))
		return
	}

	sessionManager := h.getSessionManager()
	if err := sessionManager.RenewToken(r.Context()); err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}
	sessionManager.Put(r.Context(), sessionAuthenticatedKey, true)
	sessionManager.Put(r.Context(), sessionRoleKey, normalizeUIRole(role))
	sessionManager.Put(r.Context(), sessionUserIDKey, "")
	sessionManager.Put(r.Context(), sessionAuthVersionKey, int64(0))
	http.Redirect(w, r, "/ui/", http.StatusSeeOther)
}

func (h *UIHandler) loginPageData(errMsg string) pages.LoginPageData {
	return pages.LoginPageData{
		Error:                 errMsg,
		Providers:             h.socialProviders(),
		MasterKeyLoginEnabled: h.masterKeyLoginEnabled(),
	}
}

func (h *UIHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.getSessionManager().Destroy(r.Context()); err != nil {
		http.Error(w, "failed to destroy session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
}

func (h *UIHandler) requireAdmin(next http.Handler) http.Handler {
	return h.requirePermission(PermissionUsersManage)(next)
}

func (h *UIHandler) sessionAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := h.resolveSessionPrincipal(r)
		if !ok {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/ui/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/ui/login", http.StatusSeeOther)
			return
		}
		ctx := context.WithValue(r.Context(), principalContextKey{}, principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
