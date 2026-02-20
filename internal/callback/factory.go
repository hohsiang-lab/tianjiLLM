package callback

import "fmt"

// NewFromConfig creates a CustomLogger from a CallbackConfig.
// Returns the logger and an optional Startable if the logger has a batch background loop.
func NewFromConfig(typ, apiKey, baseURL, project, entity, bucket, prefix, region, queueURL, tableName string) (CustomLogger, error) {
	switch typ {
	case "langfuse":
		return NewLangfuseCallback(baseURL, apiKey, ""), nil
	case "langsmith":
		return NewLangsmithLogger(apiKey, baseURL, project), nil
	case "helicone":
		return NewHeliconeLogger(apiKey, baseURL), nil
	case "braintrust":
		return NewBraintrustLogger(apiKey, baseURL, project), nil
	case "mlflow":
		return NewMLflowLogger(baseURL, project), nil
	case "wandb":
		return NewWandbLogger(apiKey, project, entity), nil
	case "arize":
		return NewArizeCallback(), nil
	case "phoenix":
		return NewPhoenixCallback(), nil
	case "prometheus":
		return NewPrometheusCallback(), nil
	case "otel":
		return NewOTelCallback(baseURL, nil), nil
	case "datadog":
		return NewDatadogCallback(apiKey, baseURL), nil
	case "webhook":
		return NewWebhookCallback(baseURL, nil), nil
	case "s3":
		return NewS3Logger(bucket, prefix, region)
	case "gcs":
		return NewGCSLogger(bucket, prefix)
	case "azure_blob":
		return NewAzureBlobLogger(baseURL, bucket, prefix)
	case "dynamodb":
		return NewDynamoDBLogger(tableName, region)
	case "sqs":
		return NewSQSLogger(queueURL, region)
	case "lunary":
		return NewLunaryCallback(apiKey, baseURL), nil
	case "traceloop":
		return NewTraceloopCallback(apiKey, baseURL), nil
	case "posthog":
		return NewPostHogCallback(apiKey, baseURL), nil
	case "opik":
		return NewOpikCallback(apiKey, baseURL), nil
	case "datadog_llm":
		return NewDatadogLLMCallback(apiKey, baseURL), nil
	case "gcs_pubsub":
		return NewGCSPubSubCallback(project, entity, baseURL), nil
	case "openmeter":
		return NewOpenMeterCallback(apiKey, baseURL), nil
	case "greenscale":
		return NewGreenscaleCallback(apiKey, baseURL), nil
	case "promptlayer":
		return NewPromptLayerCallback(apiKey, baseURL), nil
	case "argilla":
		return NewArgillaCallback(apiKey, baseURL), nil
	case "lago":
		return NewLagoCallback(apiKey, baseURL), nil
	case "azure_sentinel":
		return NewAzureSentinelCallback(apiKey, baseURL, ""), nil
	case "supabase":
		return NewSupabaseCallback(apiKey, baseURL), nil
	case "cloudzero":
		return NewCloudZeroCallback(apiKey, baseURL), nil
	case "logfire":
		return NewLogfireCallback(apiKey, baseURL), nil
	case "athina":
		return NewAthinaCallback(apiKey, baseURL), nil
	case "deepeval":
		return NewDeepEvalCallback(apiKey, baseURL), nil
	case "galileo":
		return NewGalileoCallback(apiKey, baseURL), nil
	case "literal_ai":
		return NewLiteralAICallback(apiKey, baseURL), nil
	case "arize_full":
		return NewArizeFullCallback(apiKey, baseURL), nil
	case "agentops":
		return NewAgentOpsCallback(apiKey, baseURL), nil
	case "focus":
		return NewFocusCallback(apiKey, baseURL), nil
	case "humanloop":
		return NewHumanLoopCallback(apiKey, baseURL), nil
	case "langtrace":
		return NewLangTraceCallback(apiKey, baseURL), nil
	case "levo":
		return NewLevoCallback(apiKey, baseURL), nil
	case "weave":
		return NewWeaveCallback(apiKey, baseURL), nil
	case "bitbucket":
		return NewBitbucketCallback(apiKey, baseURL), nil
	case "gitlab":
		return NewGitLabCallback(apiKey, baseURL), nil
	case "dotprompt":
		return NewDotPromptCallback(apiKey, baseURL), nil
	case "websearch_interception":
		return NewWebSearchCallback(apiKey, baseURL), nil
	case "custom_batch_logger":
		return NewCustomBatchCallback(apiKey, baseURL), nil
	case "generic_api":
		return NewGenericAPICallback(baseURL, nil), nil
	default:
		return nil, fmt.Errorf("unknown callback type: %s", typ)
	}
}
