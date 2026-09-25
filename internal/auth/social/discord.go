package social

import (
	"context"
	"fmt"

	"github.com/markbates/goth/providers/discord"
	"golang.org/x/oauth2"
)

const (
	discordAuthURL  = "https://discord.com/oauth2/authorize"
	discordTokenURL = "https://discord.com/api/oauth2/token"
)

type DiscordProvider struct {
	oauth        oauth2.Config
	clientID     string
	clientSecret string
	callbackURL  string
	scopes       []string
}

func NewDiscordProvider(clientID, clientSecret, publicBaseURL string) (*DiscordProvider, error) {
	callback, err := callbackURL(publicBaseURL, "discord")
	if err != nil {
		return nil, err
	}
	scopes := []string{discord.ScopeIdentify, discord.ScopeEmail}
	return &DiscordProvider{
		oauth:        oauthConfig(clientID, clientSecret, callback, discordAuthURL, discordTokenURL, scopes),
		clientID:     clientID,
		clientSecret: clientSecret,
		callbackURL:  callback,
		scopes:       scopes,
	}, nil
}

func (p *DiscordProvider) Name() string { return "discord" }

func (p *DiscordProvider) AuthorizationURL(state, verifier string) string {
	return authorizationURL(p.oauth, state, verifier)
}

func (p *DiscordProvider) Complete(ctx context.Context, code, verifier string) (Identity, error) {
	ctx = withOAuthHTTPClient(ctx)
	token, err := p.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("discord token exchange: %w", err)
	}

	profile := discord.New(p.clientID, p.clientSecret, p.callbackURL, p.scopes...)
	profile.HTTPClient = p.oauth.Client(ctx, token)
	user, err := profile.FetchUser(&discord.Session{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.Expiry,
	})
	if err != nil {
		return Identity{}, fmt.Errorf("discord fetch user: %w", err)
	}
	verified, _ := user.RawData["verified"].(bool)

	return Identity{
		Provider:       p.Name(),
		ProviderUserID: user.UserID,
		Email:          user.Email,
		EmailVerified:  verified,
		DisplayName:    user.Name,
		AvatarURL:      user.AvatarURL,
	}, nil
}
