package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_AllNewSections(t *testing.T) {
	yaml := `
model_list:
  - model_name: gpt-4o
    tianji_params:
      model: openai/gpt-4o
      api_key: sk-test

general_settings:
  master_key: sk-master
  port: 8080
  secret_manager:
    type: aws_secrets_manager
    region: us-east-1
    cache_ttl: 3600
  prompt_management:
    type: langfuse
    public_key: pk-test
    secret_key: sk-test
    base_url: https://langfuse.example.com

tianji_settings:
  cache: true
  cache_params:
    type: redis_cluster
    addrs:
      - "redis-1:6379"
      - "redis-2:6379"
    password: secret
    embedding_model: text-embedding-3-small
  callbacks:
    - prometheus
    - langfuse
  callback_configs:
    - type: s3
      bucket: my-logs
      prefix: tianji/
      region: us-west-2
    - type: langsmith
      api_key: ls-key
      project: my-project

guardrails:
  - guardrail_name: content-safety
    tianji_params:
      mode: openai_moderation
      api_key: sk-mod
    default_on: true
    failure_policy: fail_closed
  - guardrail_name: pii-filter
    tianji_params:
      mode: presidio
      analyzer_url: http://presidio:5002
    failure_policy: fail_open

router_settings:
  routing_strategy: lowest-latency
  num_retries: 3
  allowed_fails: 5
  cooldown_time: 30
`

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0644))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	// Models
	assert.Len(t, cfg.ModelList, 1)
	assert.Equal(t, "gpt-4o", cfg.ModelList[0].ModelName)

	// General settings
	assert.Equal(t, 8080, cfg.GeneralSettings.Port)

	// Secret manager
	require.NotNil(t, cfg.GeneralSettings.SecretManager)
	assert.Equal(t, "aws_secrets_manager", cfg.GeneralSettings.SecretManager.Type)
	assert.Equal(t, "us-east-1", cfg.GeneralSettings.SecretManager.Region)
	require.NotNil(t, cfg.GeneralSettings.SecretManager.CacheTTL)
	assert.Equal(t, 3600, *cfg.GeneralSettings.SecretManager.CacheTTL)

	// Prompt management
	require.NotNil(t, cfg.GeneralSettings.PromptManagement)
	assert.Equal(t, "langfuse", cfg.GeneralSettings.PromptManagement.Type)
	assert.Equal(t, "pk-test", cfg.GeneralSettings.PromptManagement.PublicKey)

	// Cache params
	require.NotNil(t, cfg.TianjiSettings.CacheParams)
	assert.Equal(t, "redis_cluster", cfg.TianjiSettings.CacheParams.Type)
	assert.Equal(t, []string{"redis-1:6379", "redis-2:6379"}, cfg.TianjiSettings.CacheParams.Addrs)
	assert.Equal(t, "text-embedding-3-small", cfg.TianjiSettings.CacheParams.EmbeddingModel)

	// String callbacks
	assert.Equal(t, []string{"prometheus", "langfuse"}, cfg.TianjiSettings.Callbacks)

	// Structured callback configs
	require.Len(t, cfg.TianjiSettings.CallbackConfigs, 2)
	assert.Equal(t, "s3", cfg.TianjiSettings.CallbackConfigs[0].Type)
	assert.Equal(t, "my-logs", cfg.TianjiSettings.CallbackConfigs[0].Bucket)
	assert.Equal(t, "langsmith", cfg.TianjiSettings.CallbackConfigs[1].Type)
	assert.Equal(t, "my-project", cfg.TianjiSettings.CallbackConfigs[1].Project)

	// Guardrails
	require.Len(t, cfg.Guardrails, 2)
	assert.Equal(t, "content-safety", cfg.Guardrails[0].GuardrailName)
	assert.Equal(t, "fail_closed", cfg.Guardrails[0].FailurePolicy)
	assert.True(t, cfg.Guardrails[0].DefaultOn)
	assert.Equal(t, "pii-filter", cfg.Guardrails[1].GuardrailName)
	assert.Equal(t, "fail_open", cfg.Guardrails[1].FailurePolicy)

	// Router settings
	require.NotNil(t, cfg.RouterSettings)
	assert.Equal(t, "lowest-latency", cfg.RouterSettings.RoutingStrategy)
	require.NotNil(t, cfg.RouterSettings.NumRetries)
	assert.Equal(t, 3, *cfg.RouterSettings.NumRetries)
	require.NotNil(t, cfg.RouterSettings.AllowedFails)
	assert.Equal(t, 5, *cfg.RouterSettings.AllowedFails)
	require.NotNil(t, cfg.RouterSettings.CooldownTime)
	assert.Equal(t, 30, *cfg.RouterSettings.CooldownTime)
}

func TestLoad_MinimalConfig(t *testing.T) {
	yaml := `
model_list:
  - model_name: test
    tianji_params:
      model: openai/test
general_settings:
  master_key: sk-test
`

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(yaml), 0644))

	cfg, err := Load(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, 4000, cfg.GeneralSettings.Port) // default
	assert.Nil(t, cfg.GeneralSettings.SecretManager)
	assert.Nil(t, cfg.GeneralSettings.PromptManagement)
	assert.Nil(t, cfg.RouterSettings)
	assert.Empty(t, cfg.Guardrails)
}
