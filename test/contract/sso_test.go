package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/auth"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/handler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSO_Login_NotConfigured(t *testing.T) {
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	req := httptest.NewRequest(http.MethodGet, "/sso/login", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	errObj, _ := resp["error"].(map[string]any)
	assert.Equal(t, "SSO not configured", errObj["message"])
}

func TestSSO_Callback_NotConfigured(t *testing.T) {
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}
	handlers := &handler.Handlers{Config: cfg}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	req := httptest.NewRequest(http.MethodGet, "/sso/callback?code=abc&state=xyz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestSSO_Login_Configured_Redirects(t *testing.T) {
	cfg := &config.ProxyConfig{
		GeneralSettings: config.GeneralSettings{
			MasterKey: "sk-master",
		},
	}
	ssoHandler := &handler.SSOHandler{
		SSO: auth.NewSSOHandler(auth.SSOConfig{
			ClientID:    "test-client",
			AuthURL:     "https://idp.example.com/authorize",
			TokenURL:    "https://idp.example.com/token",
			UserInfoURL: "https://idp.example.com/userinfo",
			RedirectURI: "https://proxy.example.com/sso/callback",
			Scopes:      []string{"openid", "profile", "email"},
		}),
	}
	handlers := &handler.Handlers{
		Config:     cfg,
		SSOHandler: ssoHandler,
	}
	srv := proxy.NewServer(proxy.ServerConfig{
		Handlers:  handlers,
		MasterKey: cfg.GeneralSettings.MasterKey,
	})

	req := httptest.NewRequest(http.MethodGet, "/sso/login", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	// Should redirect to IDP
	assert.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	assert.Contains(t, location, "https://idp.example.com/authorize")
	assert.Contains(t, location, "client_id=test-client")
}
