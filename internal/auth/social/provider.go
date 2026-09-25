package social

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

const oauthHTTPTimeout = 15 * time.Second

var (
	ErrUnknownProvider = errors.New("unknown social auth provider")
	ErrUnverifiedEmail = errors.New("provider email is not verified")
	ErrMissingUserID   = errors.New("provider returned no user ID")
	ErrMissingEmail    = errors.New("provider returned no email")
)

type Identity struct {
	Provider       string
	ProviderUserID string
	Email          string
	EmailVerified  bool
	DisplayName    string
	AvatarURL      string
}

type Provider interface {
	Name() string
	AuthorizationURL(state, verifier string) string
	Complete(ctx context.Context, code, verifier string) (Identity, error)
}

type Service struct {
	providers map[string]Provider
}

func NewService(providers ...Provider) *Service {
	service := &Service{providers: make(map[string]Provider, len(providers))}
	for _, provider := range providers {
		if provider != nil {
			service.providers[provider.Name()] = provider
		}
	}
	return service
}

func (s *Service) Providers() []string {
	result := make([]string, 0, len(s.providers))
	for _, name := range []string{"github", "discord"} {
		if _, ok := s.providers[name]; ok {
			result = append(result, name)
		}
	}
	return result
}

func (s *Service) Begin(provider, state, verifier string) (string, error) {
	p, ok := s.providers[provider]
	if !ok {
		return "", ErrUnknownProvider
	}
	return p.AuthorizationURL(state, verifier), nil
}

func (s *Service) Complete(ctx context.Context, provider, code, verifier string) (Identity, error) {
	p, ok := s.providers[provider]
	if !ok {
		return Identity{}, ErrUnknownProvider
	}
	identity, err := p.Complete(ctx, code, verifier)
	if err != nil {
		return Identity{}, err
	}
	identity.Provider = p.Name()
	identity.ProviderUserID = strings.TrimSpace(identity.ProviderUserID)
	identity.Email = strings.TrimSpace(identity.Email)
	if identity.ProviderUserID == "" {
		return Identity{}, ErrMissingUserID
	}
	if identity.Email == "" {
		return Identity{}, ErrMissingEmail
	}
	if !identity.EmailVerified {
		return Identity{}, ErrUnverifiedEmail
	}
	return identity, nil
}

func withOAuthHTTPClient(ctx context.Context) context.Context {
	if client, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok && client != nil {
		return ctx
	}
	return context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: oauthHTTPTimeout})
}

func oauthConfig(clientID, clientSecret, callbackURL, authURL, tokenURL string, scopes []string) oauth2.Config {
	return oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  callbackURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:   authURL,
			TokenURL:  tokenURL,
			AuthStyle: oauth2.AuthStyleInParams,
		},
		Scopes: scopes,
	}
}

func authorizationURL(config oauth2.Config, state, verifier string) string {
	return config.AuthCodeURL(
		state,
		oauth2.AccessTypeOnline,
		oauth2.S256ChallengeOption(verifier),
	)
}

func callbackURL(publicBaseURL, provider string) (string, error) {
	base, err := url.Parse(strings.TrimSpace(publicBaseURL))
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") {
		return "", fmt.Errorf("invalid social auth public base URL")
	}
	if base.User != nil || (base.Path != "" && base.Path != "/") || base.RawQuery != "" || base.Fragment != "" {
		return "", fmt.Errorf("social auth public base URL must contain only scheme and host")
	}
	if base.Scheme == "http" &&
		base.Hostname() != "localhost" &&
		base.Hostname() != "127.0.0.1" &&
		base.Hostname() != "::1" {
		return "", fmt.Errorf("social auth public base URL must use https unless the host is localhost")
	}
	return base.ResolveReference(&url.URL{Path: "/auth/" + provider + "/callback"}).String(), nil
}
