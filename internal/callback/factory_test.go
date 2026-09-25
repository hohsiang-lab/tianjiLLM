package callback

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFromConfig_AllTypes(t *testing.T) {
	types := []string{
		"langfuse", "langsmith", "helicone", "braintrust", "mlflow",
		"wandb", "arize", "phoenix", "prometheus", "otel",
		"datadog", "webhook", "lunary", "traceloop", "posthog",
		"opik", "datadog_llm", "gcs_pubsub", "openmeter", "greenscale",
		"promptlayer", "argilla", "lago", "azure_sentinel", "supabase",
		"cloudzero", "logfire", "athina", "deepeval", "galileo",
		"literal_ai", "arize_full", "agentops", "focus", "humanloop",
		"langtrace", "levo", "weave", "bitbucket", "gitlab",
		"dotprompt", "websearch_interception", "custom_batch_logger",
		"generic_api",
	}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			logger, err := NewFromConfig(typ, "test-key", "https://example.com", "proj", "entity", "bucket", "prefix", "us-east-1", "https://sqs.example.com", "table")
			require.NoError(t, err, "type: %s", typ)
			assert.NotNil(t, logger)

			// Test that LogSuccess and LogFailure don't panic
			logger.LogSuccess(LogData{Model: "test"})
			logger.LogFailure(LogData{Model: "test"})
		})
	}
}

func TestNewFromConfig_Unknown(t *testing.T) {
	_, err := NewFromConfig("nonexistent", "", "", "", "", "", "", "", "", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown callback type")
}

func TestNewFromConfig_AWSTypes(t *testing.T) {
	// These may fail due to missing AWS config but should not panic
	awsTypes := []string{"s3", "dynamodb", "sqs"}
	for _, typ := range awsTypes {
		t.Run(typ, func(t *testing.T) {
			_, _ = NewFromConfig(typ, "", "", "", "", "test-bucket", "prefix", "us-east-1", "https://sqs.example.com", "table")
			// Just verify no panic
		})
	}
}
