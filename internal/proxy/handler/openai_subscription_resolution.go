package handler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/praxisllmlab/tianjiLLM/internal/codexapp"
	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/praxisllmlab/tianjiLLM/internal/db"
	"github.com/praxisllmlab/tianjiLLM/internal/provider"
	"github.com/praxisllmlab/tianjiLLM/internal/proxy/middleware"
)

type openAISubscriptionTransport string

const (
	openAISubscriptionTransportCodexAppServer      openAISubscriptionTransport = "codex_app_server"
	openAISubscriptionTransportChatGPTCodexBackend openAISubscriptionTransport = "chatgpt_codex_backend"
	openAISubscriptionTransportGenericHTTP         openAISubscriptionTransport = "generic_http"
)

type resolvedOpenAISubscriptionCredential struct {
	CredentialID string
	BearerToken  string
	AccountID    string
	CodexLogin   *codexapp.LoginStartParams
}

func (h *Handlers) resolveOpenAIAPIKeyForParams(ctx context.Context, params config.TianjiParams) (string, error) {
	route, err := h.resolveOpenAISubscriptionRouteForParams(ctx, params)
	if err != nil {
		return "", err
	}
	return route.APIKey, nil
}

type openAISubscriptionHTTPRoute struct {
	APIKey     string
	Candidates []resolvedOpenAISubscriptionCredential
}

func (h *Handlers) resolveOpenAISubscriptionRouteForParams(ctx context.Context, params config.TianjiParams) (openAISubscriptionHTTPRoute, error) {
	providerName := ""
	if strings.TrimSpace(params.Model) != "" {
		providerName, _ = provider.ParseModelName(params.Model)
		if err := validateOpenAISubscriptionProviderParams(providerName, params); err != nil {
			return openAISubscriptionHTTPRoute{}, err
		}
		if providerName == "openai" {
			return h.resolveOpenAISubscriptionGlobalRoute(ctx, normalizeOfficialOpenAIParams(params))
		}
	}
	if len(params.OpenAISubscriptionCredentialIDs) > 0 || strings.TrimSpace(params.OpenAISubscriptionTransport) != "" {
		return openAISubscriptionHTTPRoute{}, errors.New("OpenAI subscription credential resolution failed: canonical model provider is required")
	}
	if params.APIKey == nil {
		return openAISubscriptionHTTPRoute{}, nil
	}
	return openAISubscriptionHTTPRoute{APIKey: *params.APIKey}, nil
}

func (h *Handlers) openAISubscriptionCredentialIDsInRequestScope(ctx context.Context) ([]string, error) {
	if h == nil || h.DB == nil {
		return nil, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}
	credentials, err := h.DB.ListCredentials(ctx)
	if err != nil {
		return nil, errors.New("OpenAI subscription credential resolution failed: credential list failed")
	}

	organizationID, hasOrganization := "", false
	if ctx != nil {
		organizationID, hasOrganization = ctx.Value(middleware.ContextKeyOrgID).(string)
	}
	ids := make([]string, 0, len(credentials))
	for _, credential := range credentials {
		if credential.CredentialType != CredentialTypeOpenAISubscription || !openAISubscriptionCredentialInRequestScope(credential, organizationID, hasOrganization) {
			continue
		}
		ids = append(ids, credential.CredentialID)
	}
	sort.Strings(ids)
	return ids, nil
}

func (h *Handlers) resolveOpenAISubscriptionGlobalRoute(ctx context.Context, params config.TianjiParams) (openAISubscriptionHTTPRoute, error) {
	ids, err := h.openAISubscriptionCredentialIDsInRequestScope(ctx)
	if err != nil {
		return openAISubscriptionHTTPRoute{}, err
	}
	if len(ids) == 0 {
		return openAISubscriptionHTTPRoute{}, errors.New("OpenAI subscription credential resolution failed: no usable global openai_subscription credentials configured")
	}

	params.OpenAISubscriptionCredentialIDs = ids
	params.OpenAISubscriptionTransport = config.OpenAISubscriptionTransportChatGPTCodexBackend
	candidates, err := h.resolveOpenAISubscriptionAvailableCandidates(ctx, params, openAISubscriptionTransportChatGPTCodexBackend)
	if err != nil {
		return openAISubscriptionHTTPRoute{}, err
	}
	return openAISubscriptionHTTPRoute{Candidates: resolvedOpenAISubscriptionCredentials(candidates)}, nil
}

func openAISubscriptionCredentialInRequestScope(credential db.CredentialTable, organizationID string, hasOrganization bool) bool {
	if !hasOrganization || organizationID == "" {
		return credential.OrganizationID == nil
	}
	if credential.OrganizationID == nil {
		return true
	}
	return *credential.OrganizationID == organizationID
}

func (h *Handlers) resolveOpenAISubscriptionRouteForResolvedModel(ctx context.Context, resolvedModel string, params config.TianjiParams, options ...providerRouteResolutionOptions) (openAISubscriptionHTTPRoute, error) {
	if strings.TrimSpace(resolvedModel) == "" || strings.TrimSpace(params.Model) == "" {
		return openAISubscriptionHTTPRoute{}, errors.New("OpenAI subscription credential resolution failed: canonical model provider is required")
	}
	providerName, _ := provider.ParseModelName(resolvedModel)
	if err := validateOpenAISubscriptionProviderParams(providerName, params); err != nil {
		return openAISubscriptionHTTPRoute{}, err
	}
	if len(options) > 0 && options[0].skipCredentialResolutionForUnsupportedCodexChat && isChatGPTCodexBackendTransport(params) {
		return openAISubscriptionHTTPRoute{}, nil
	}
	if providerName == "openai" {
		return h.resolveOpenAISubscriptionGlobalRoute(ctx, params)
	}
	return h.resolveOpenAISubscriptionRouteForParams(ctx, params)
}

func validateOpenAISubscriptionProviderParams(providerName string, params config.TianjiParams) error {
	if providerName == "openai" {
		return nil
	}
	if len(params.OpenAISubscriptionCredentialIDs) > 0 || strings.TrimSpace(params.OpenAISubscriptionTransport) != "" {
		return fmt.Errorf("OpenAI subscription credentials and transport only support the official OpenAI provider")
	}
	return nil
}

func isChatGPTCodexBackendTransport(params config.TianjiParams) bool {
	return params.OpenAISubscriptionTransport == config.OpenAISubscriptionTransportChatGPTCodexBackend
}

func normalizeOfficialOpenAIParams(params config.TianjiParams) config.TianjiParams {
	config.NormalizeOfficialOpenAIParams(&params)
	return params
}

func openAISubscriptionTransportForParams(params config.TianjiParams) openAISubscriptionTransport {
	switch params.OpenAISubscriptionTransport {
	case config.OpenAISubscriptionTransportChatGPTCodexBackend:
		return openAISubscriptionTransportChatGPTCodexBackend
	default:
		return openAISubscriptionTransportGenericHTTP
	}
}

func (h *Handlers) resolveOpenAISubscriptionCredential(ctx context.Context, params config.TianjiParams, transport openAISubscriptionTransport) (resolvedOpenAISubscriptionCredential, error) {
	ordered, err := h.resolveOpenAISubscriptionAttemptOrder(ctx, params, transport)
	if err != nil {
		return resolvedOpenAISubscriptionCredential{}, err
	}
	if len(ordered) == 0 {
		return resolvedOpenAISubscriptionCredential{}, nil
	}
	return ordered[0], nil
}

func (h *Handlers) resolveOpenAISubscriptionCredentialByID(ctx context.Context, credentialID string, transport openAISubscriptionTransport) (resolvedOpenAISubscriptionCredential, error) {
	if h.DB == nil {
		return resolvedOpenAISubscriptionCredential{}, errors.New("OpenAI subscription credential resolution failed: database not configured")
	}
	bundle, err := h.resolveUsableOpenAISubscriptionBundle(ctx, credentialID)
	if err != nil {
		return resolvedOpenAISubscriptionCredential{}, err
	}

	resolved := resolvedOpenAISubscriptionCredential{
		CredentialID: credentialID,
		AccountID:    bundle.AccountID,
	}
	switch transport {
	case openAISubscriptionTransportChatGPTCodexBackend:
		resolved.BearerToken = bundle.AccessToken
	case openAISubscriptionTransportCodexAppServer:
		resolved.CodexLogin = ptrCodexLogin(codexapp.NewChatGPTAuthTokensLogin(codexapp.ChatGPTAuthTokens{
			AccessToken:      bundle.AccessToken,
			ChatGPTAccountID: bundle.AccountID,
		}))
	default:
		return resolvedOpenAISubscriptionCredential{}, fmt.Errorf("OpenAI subscription credential resolution failed for %q: unsupported transport %q", credentialID, transport)
	}
	return resolved, nil
}

func ptrCodexLogin(value codexapp.LoginStartParams) *codexapp.LoginStartParams {
	return &value
}
