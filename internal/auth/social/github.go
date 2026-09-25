package social

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/markbates/goth/providers/github"
	"golang.org/x/oauth2"
)

type GitHubProvider struct {
	oauth        oauth2.Config
	clientID     string
	clientSecret string
	callbackURL  string
	authURL      string
	tokenURL     string
	profileURL   string
	emailURL     string
}

func NewGitHubProvider(clientID, clientSecret, publicBaseURL string) (*GitHubProvider, error) {
	callback, err := callbackURL(publicBaseURL, "github")
	if err != nil {
		return nil, err
	}
	scopes := []string{"read:user", "user:email"}
	return &GitHubProvider{
		oauth:        oauthConfig(clientID, clientSecret, callback, github.AuthURL, github.TokenURL, scopes),
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  callback,
		authURL:      github.AuthURL,
		tokenURL:     github.TokenURL,
		profileURL:   github.ProfileURL,
		emailURL:     github.EmailURL,
	}, nil
}

func (p *GitHubProvider) Name() string { return "github" }

func (p *GitHubProvider) AuthorizationURL(state, verifier string) string {
	return authorizationURL(p.oauth, state, verifier)
}

func (p *GitHubProvider) Complete(ctx context.Context, code, verifier string) (Identity, error) {
	ctx = withOAuthHTTPClient(ctx)
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("github token exchange: %w", err)
	}

	profile := github.NewCustomisedURL(
		p.clientID,
		p.clientSecret,
		p.callbackURL,
		p.authURL,
		p.tokenURL,
		p.profileURL,
		p.emailURL,
		"read:user",
	)
	profile.HTTPClient = p.oauth.Client(ctx, token)
	user, err := profile.FetchUser(&github.Session{AccessToken: token.AccessToken})
	if err != nil {
		return Identity{}, fmt.Errorf("github fetch user: %w", err)
	}
	email, err := p.fetchVerifiedPrimaryEmail(ctx, token)
	if err != nil {
		return Identity{}, err
	}

	return Identity{
		Provider:       p.Name(),
		ProviderUserID: user.UserID,
		Email:          email,
		EmailVerified:  true,
		DisplayName:    user.Name,
		AvatarURL:      user.AvatarURL,
	}, nil
}

func (p *GitHubProvider) fetchVerifiedPrimaryEmail(ctx context.Context, token *oauth2.Token) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.emailURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := p.oauth.Client(ctx, token).Do(req)
	if err != nil {
		return "", fmt.Errorf("github fetch emails: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github email endpoint returned %d", resp.StatusCode)
	}
	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("github decode emails: %w", err)
	}
	for _, email := range emails {
		if email.Primary && email.Verified {
			return email.Email, nil
		}
	}
	return "", ErrUnverifiedEmail
}
