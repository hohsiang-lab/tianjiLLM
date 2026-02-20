package integration

import (
	"testing"

	"github.com/praxisllmlab/tianjiLLM/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_FullPythonCompatibility(t *testing.T) {
	cfg, err := config.Load("../../test/fixtures/proxy_config_full.yaml")
	require.NoError(t, err, "loading full Python-compatible config should not error")

	// Model list
	assert.Len(t, cfg.ModelList, 5)
	assert.Equal(t, "gpt-4o", cfg.ModelList[0].ModelName)
	assert.Equal(t, "openai/gpt-4o", cfg.ModelList[0].TianjiParams.Model)
	assert.NotNil(t, cfg.ModelList[0].TianjiParams.RPM)
	assert.Equal(t, int64(100), *cfg.ModelList[0].TianjiParams.RPM)
	assert.NotNil(t, cfg.ModelList[0].TianjiParams.TPM)
	assert.Equal(t, int64(100000), *cfg.ModelList[0].TianjiParams.TPM)

	// Model info
	assert.NotNil(t, cfg.ModelList[0].ModelInfo)
	assert.Equal(t, "gpt-4o-deployment-1", cfg.ModelList[0].ModelInfo.ID)
	assert.NotNil(t, cfg.ModelList[0].ModelInfo.InputCost)
	assert.Equal(t, 0.000005, *cfg.ModelList[0].ModelInfo.InputCost)

	// Tags
	assert.Equal(t, []string{"region:us-east", "tier:premium"}, cfg.ModelList[0].Tags)

	// Azure deployment
	assert.NotNil(t, cfg.ModelList[1].TianjiParams.APIBase)
	assert.Equal(t, "https://my-deployment.openai.azure.com", *cfg.ModelList[1].TianjiParams.APIBase)
	assert.NotNil(t, cfg.ModelList[1].TianjiParams.APIVersion)

	// TianjiLLM settings
	assert.True(t, cfg.TianjiSettings.Cache)
	assert.Contains(t, cfg.TianjiSettings.Callbacks, "webhook")
	assert.Contains(t, cfg.TianjiSettings.SuccessCallback, "langfuse")
	assert.Contains(t, cfg.TianjiSettings.FailureCallback, "slack")

	// General settings
	assert.Equal(t, "sk-master-test", cfg.GeneralSettings.MasterKey)
	assert.Equal(t, 4000, cfg.GeneralSettings.Port)

	// Router settings
	assert.NotNil(t, cfg.RouterSettings)
	assert.Equal(t, "simple-shuffle", cfg.RouterSettings.RoutingStrategy)
	assert.NotNil(t, cfg.RouterSettings.NumRetries)
	assert.Equal(t, 3, *cfg.RouterSettings.NumRetries)
	assert.True(t, cfg.RouterSettings.EnableTagFiltering)

	// Guardrails
	assert.Len(t, cfg.Guardrails, 1)
	assert.Equal(t, "prompt_injection", cfg.Guardrails[0].GuardrailName)
	assert.True(t, cfg.Guardrails[0].DefaultOn)
}
