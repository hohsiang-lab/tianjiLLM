package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SecretResolver is the interface used by the config loader to resolve secrets.
// This avoids a circular dependency on the secretmanager package.
type SecretResolver interface {
	Get(ctx context.Context, path string) (string, error)
}

// Load reads a proxy_config.yaml file and returns a ProxyConfig
// with all environment variables resolved.
func Load(path string) (*ProxyConfig, error) {
	return LoadWithSecrets(path, nil)
}

// LoadWithSecrets reads a proxy_config.yaml and resolves secrets via the given resolver.
// If resolver is nil, os.environ/ references fall back to environment variables only.
func LoadWithSecrets(path string, resolver SecretResolver) (*ProxyConfig, error) {
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

	if resolver != nil {
		if err := resolveSecrets(context.Background(), &cfg, resolver); err != nil {
			return nil, fmt.Errorf("resolve secrets: %w", err)
		}
	}

	setDefaults(&cfg)
	Validate(&cfg)

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

// resolveSecrets resolves os.environ/ references via the secret manager.
// Called after resolveEnvVars — secrets override env var values.
func resolveSecrets(ctx context.Context, cfg *ProxyConfig, resolver SecretResolver) error {
	resolve := func(val string) (string, error) {
		key, ok := strings.CutPrefix(val, "os.environ/")
		if !ok {
			return val, nil
		}
		secret, err := resolver.Get(ctx, key)
		if err != nil {
			return "", fmt.Errorf("secret %q: %w", key, err)
		}
		return secret, nil
	}

	resolvePtr := func(val *string) (*string, error) {
		if val == nil {
			return nil, nil
		}
		s, err := resolve(*val)
		if err != nil {
			return nil, err
		}
		return &s, nil
	}

	var err error
	var unresolved []string

	if cfg.GeneralSettings.MasterKey, err = resolve(cfg.GeneralSettings.MasterKey); err != nil {
		unresolved = append(unresolved, err.Error())
	}
	if cfg.GeneralSettings.DatabaseURL, err = resolve(cfg.GeneralSettings.DatabaseURL); err != nil {
		unresolved = append(unresolved, err.Error())
	}

	for i := range cfg.ModelList {
		m := &cfg.ModelList[i]
		if m.TianjiParams.APIKey, err = resolvePtr(m.TianjiParams.APIKey); err != nil {
			unresolved = append(unresolved, err.Error())
		}
	}

	if cfg.TianjiSettings.CacheParams != nil {
		if cfg.TianjiSettings.CacheParams.Password, err = resolve(cfg.TianjiSettings.CacheParams.Password); err != nil {
			unresolved = append(unresolved, err.Error())
		}
	}

	if len(unresolved) > 0 {
		return fmt.Errorf("unresolved secrets: %s", strings.Join(unresolved, "; "))
	}
	return nil
}

func setDefaults(cfg *ProxyConfig) {
	if cfg.GeneralSettings.Port == 0 {
		cfg.GeneralSettings.Port = 4000
	}
}
