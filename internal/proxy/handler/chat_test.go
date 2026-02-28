package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
	"github.com/praxisllmlab/tianjiLLM/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/praxisllmlab/tianjiLLM/internal/provider/openai"
)

func TestChatCompletion_AccessDenied_Returns403(t *testing.T) {
	apiKey := "sk-test"
	// Create a router with a single deployment restricted to org_acme
	models := []config.ModelConfig{
		{
			ModelName:    "gpt-4o",
			TianjiParams: config.TianjiParams{Model: "openai/gpt-4o", APIKey: &apiKey},
			AccessControl: &config.AccessControl{
				AllowedOrgs: []string{"org_acme"},
			},
		},
	}
	r := router.New(models, nil, router.RouterSettings{})

	h := &Handlers{
		Config: &config.ProxyConfig{},
		Router: r,
	}

	body, _ := json.Marshal(map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "hi"}},
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	// Set caller as org_other → access denied
	ctx := context.WithValue(req.Context(), middleware.ContextKeyOrgID, "org_other")
	ctx = context.WithValue(ctx, middleware.ContextKeyIsMasterKey, false)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.ChatCompletion(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var errResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &errResp))
	errObj, ok := errResp["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "access_denied", errObj["code"])
}
