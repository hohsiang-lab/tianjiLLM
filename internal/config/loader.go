package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Load reads a proxy_config.yaml file and returns a ProxyConfig
// with all environment variables resolved.
func Load(path string) (*ProxyConfig, error) {
	// Load .env from same directory as config file, if it exists.
	_ = godotenv.Load(filepath.Join(filepath.Dir(path), ".env"))

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}

	var cfg ProxyConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	applyEnvironmentVariables(&cfg)
	resolveEnvVars(&cfg)

	setDefaults(&cfg)
	if err := Validate(&cfg); err != nil {
		return nil, fmt.Errorf("validate config %s: %w", path, err)
	}

	return &cfg, nil
}

// applyEnvironmentVariables sets OS env vars from the config's
// environment_variables section, matching Python LiteLLM behavior.
func applyEnvironmentVariables(cfg *ProxyConfig) {
	for k, v := range cfg.EnvironmentVariables {
		resolved := ResolveEnvVar(v)
		os.Setenv(k, resolved)
	}
}

func resolveEnvVars(cfg *ProxyConfig) {
	cfg.GeneralSettings.MasterKey = ResolveEnvVar(cfg.GeneralSettings.MasterKey)
	cfg.GeneralSettings.DatabaseURL = ResolveEnvVar(cfg.GeneralSettings.DatabaseURL)
	cfg.GeneralSettings.SocialAuth.PublicBaseURL = ResolveEnvVar(cfg.GeneralSettings.SocialAuth.PublicBaseURL)
	cfg.GeneralSettings.SocialAuth.GitHub.ClientID = ResolveEnvVar(cfg.GeneralSettings.SocialAuth.GitHub.ClientID)
	cfg.GeneralSettings.SocialAuth.GitHub.ClientSecret = ResolveEnvVar(cfg.GeneralSettings.SocialAuth.GitHub.ClientSecret)
	cfg.GeneralSettings.SocialAuth.Discord.ClientID = ResolveEnvVar(cfg.GeneralSettings.SocialAuth.Discord.ClientID)
	cfg.GeneralSettings.SocialAuth.Discord.ClientSecret = ResolveEnvVar(cfg.GeneralSettings.SocialAuth.Discord.ClientSecret)

	for i := range cfg.ModelList {
		m := &cfg.ModelList[i]
		m.TianjiParams.APIKey = ResolveEnvVarPtr(m.TianjiParams.APIKey)
		m.TianjiParams.APIBase = ResolveEnvVarPtr(m.TianjiParams.APIBase)
		m.TianjiParams.APIVersion = ResolveEnvVarPtr(m.TianjiParams.APIVersion)
	}

	if cfg.TianjiSettings.CacheParams != nil {
		cfg.TianjiSettings.CacheParams.Password = ResolveEnvVar(cfg.TianjiSettings.CacheParams.Password)
	}
}

func setDefaults(cfg *ProxyConfig) {
	if cfg.GeneralSettings.Port == 0 {
		cfg.GeneralSettings.Port = 4000
	}
}
