package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSocialAuthConfig() *ProxyConfig {
	return &ProxyConfig{GeneralSettings: GeneralSettings{
		MasterKey:   "test-master-key",
		DatabaseURL: "postgres://user:pass@localhost:5432/tianji",
		SocialAuth: SocialAuthConfig{
			Enabled:       true,
			PublicBaseURL: "https://tianji.example",
			GitHub: SocialProviderConfig{
				ClientID:     "github-client-id",
				ClientSecret: "github-client-secret",
			},
		},
	}}
}

func TestValidateSocialAuthAcceptsCompleteConfiguration(t *testing.T) {
	require.NoError(t, Validate(validSocialAuthConfig()))
}

func TestValidateSocialAuthRejectsUnsafeOrIncompleteConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ProxyConfig)
		want   string
	}{
		{
			name: "database required",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.DatabaseURL = ""
			},
			want: "requires database_url",
		},
		{
			name: "absolute http URL required",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.SocialAuth.PublicBaseURL = "javascript:alert(1)"
			},
			want: "absolute http or https URL",
		},
		{
			name: "origin URL cannot contain path",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.SocialAuth.PublicBaseURL = "https://tianji.example/subpath"
			},
			want: "only scheme and host",
		},
		{
			name: "remote HTTP origin rejected",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.SocialAuth.PublicBaseURL = "http://tianji.example"
			},
			want: "must use https unless the host is localhost",
		},
		{
			name: "provider credentials must be paired",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.SocialAuth.GitHub.ClientSecret = ""
			},
			want: "requires both client_id and client_secret",
		},
		{
			name: "at least one provider required",
			mutate: func(cfg *ProxyConfig) {
				cfg.GeneralSettings.SocialAuth.GitHub = SocialProviderConfig{}
			},
			want: "requires at least one configured provider",
		},
		{
			name: "break glass requires master key",
			mutate: func(cfg *ProxyConfig) {
				enabled := true
				cfg.GeneralSettings.MasterKey = ""
				cfg.GeneralSettings.SocialAuth.MasterKeyLoginEnabled = &enabled
			},
			want: "requires master_key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validSocialAuthConfig()
			tc.mutate(cfg)
			err := Validate(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestResolveEnvVarsIncludesSocialProviderConfiguration(t *testing.T) {
	env := map[string]string{
		"SOCIAL_PUBLIC_URL":     "https://tianji.example",
		"GITHUB_CLIENT_ID":      "github-id",
		"GITHUB_CLIENT_SECRET":  "github-secret",
		"DISCORD_CLIENT_ID":     "discord-id",
		"DISCORD_CLIENT_SECRET": "discord-secret",
	}
	for key, value := range env {
		t.Setenv(key, value)
	}

	cfg := &ProxyConfig{GeneralSettings: GeneralSettings{SocialAuth: SocialAuthConfig{
		PublicBaseURL: "os.environ/SOCIAL_PUBLIC_URL",
		GitHub: SocialProviderConfig{
			ClientID:     "os.environ/GITHUB_CLIENT_ID",
			ClientSecret: "os.environ/GITHUB_CLIENT_SECRET",
		},
		Discord: SocialProviderConfig{
			ClientID:     "os.environ/DISCORD_CLIENT_ID",
			ClientSecret: "os.environ/DISCORD_CLIENT_SECRET",
		},
	}}}
	resolveEnvVars(cfg)

	assert.Equal(t, env["SOCIAL_PUBLIC_URL"], cfg.GeneralSettings.SocialAuth.PublicBaseURL)
	assert.Equal(t, env["GITHUB_CLIENT_ID"], cfg.GeneralSettings.SocialAuth.GitHub.ClientID)
	assert.Equal(t, env["GITHUB_CLIENT_SECRET"], cfg.GeneralSettings.SocialAuth.GitHub.ClientSecret)
	assert.Equal(t, env["DISCORD_CLIENT_ID"], cfg.GeneralSettings.SocialAuth.Discord.ClientID)
	assert.Equal(t, env["DISCORD_CLIENT_SECRET"], cfg.GeneralSettings.SocialAuth.Discord.ClientSecret)
}

func TestSocialAuthDisabledPreservesBackwardCompatibleConfig(t *testing.T) {
	cfg := &ProxyConfig{GeneralSettings: GeneralSettings{
		MasterKey: "legacy-master-key",
		SocialAuth: SocialAuthConfig{
			Enabled:       false,
			PublicBaseURL: "not-a-url",
		},
	}}
	require.NoError(t, Validate(cfg))
}
