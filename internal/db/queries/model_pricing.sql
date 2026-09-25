-- name: UpsertModelPricing :exec
INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url,
    cache_read_input_token_cost, cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k, cache_creation_input_token_cost_above_200k,
    long_context_input_threshold_tokens,
    input_cost_per_token_above_threshold, output_cost_per_token_above_threshold,
    cache_read_input_token_cost_above_threshold, cache_creation_input_token_cost_above_threshold,
    synced_at
) VALUES (
    @model_name, @input_cost_per_token, @output_cost_per_token,
    @max_input_tokens, @max_output_tokens, @max_tokens,
    @mode, @provider, @source_url,
    @cache_read_input_token_cost, @cache_creation_input_token_cost,
    @cache_read_input_token_cost_above_200k, @cache_creation_input_token_cost_above_200k,
    @long_context_input_threshold_tokens,
    @input_cost_per_token_above_threshold, @output_cost_per_token_above_threshold,
    @cache_read_input_token_cost_above_threshold, @cache_creation_input_token_cost_above_threshold,
    NOW()
)
ON CONFLICT (model_name) DO UPDATE SET
    input_cost_per_token  = EXCLUDED.input_cost_per_token,
    output_cost_per_token = EXCLUDED.output_cost_per_token,
    max_input_tokens      = EXCLUDED.max_input_tokens,
    max_output_tokens     = EXCLUDED.max_output_tokens,
    max_tokens            = EXCLUDED.max_tokens,
    mode                  = EXCLUDED.mode,
    provider              = EXCLUDED.provider,
    source_url            = EXCLUDED.source_url,
    cache_read_input_token_cost             = EXCLUDED.cache_read_input_token_cost,
    cache_creation_input_token_cost         = EXCLUDED.cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k  = EXCLUDED.cache_read_input_token_cost_above_200k,
    cache_creation_input_token_cost_above_200k = EXCLUDED.cache_creation_input_token_cost_above_200k,
    long_context_input_threshold_tokens     = EXCLUDED.long_context_input_threshold_tokens,
    input_cost_per_token_above_threshold    = EXCLUDED.input_cost_per_token_above_threshold,
    output_cost_per_token_above_threshold   = EXCLUDED.output_cost_per_token_above_threshold,
    cache_read_input_token_cost_above_threshold = EXCLUDED.cache_read_input_token_cost_above_threshold,
    cache_creation_input_token_cost_above_threshold = EXCLUDED.cache_creation_input_token_cost_above_threshold,
    synced_at             = NOW(),
    updated_at            = NOW();

-- name: ListModelPricing :many
SELECT * FROM "ModelPricing"
ORDER BY model_name;

-- name: DeleteAllModelPricing :exec
DELETE FROM "ModelPricing";
