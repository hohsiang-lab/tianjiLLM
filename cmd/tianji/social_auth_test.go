package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
)

func TestBuildSocialAuthServiceDisabled(t *testing.T) {
	service, err := buildSocialAuthService(config.SocialAuthConfig{})
	require.NoError(t, err)
	assert.Nil(t, service)
}

func TestBuildSocialAuthServiceConfiguresAvailableProviders(t *testing.T) {
	service, err := buildSocialAuthService(config.SocialAuthConfig{
		Enabled:       true,
		PublicBaseURL: "https://tianji.example",
		GitHub: config.SocialProviderConfig{
			ClientID:     "github-client-id",
			ClientSecret: "github-client-secret",
		},
		Discord: config.SocialProviderConfig{
			ClientID:     "discord-client-id",
			ClientSecret: "discord-client-secret",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, service)
	assert.Equal(t, []string{"github", "discord"}, service.Providers())
}

func TestBuildSocialAuthServiceRejectsNoProviders(t *testing.T) {
	service, err := buildSocialAuthService(config.SocialAuthConfig{
		Enabled:       true,
		PublicBaseURL: "https://tianji.example",
	})
	assert.Nil(t, service)
	assert.ErrorContains(t, err, "no providers")
}
