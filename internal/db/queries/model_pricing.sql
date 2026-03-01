-- name: UpsertModelPricing :exec
INSERT INTO "ModelPricing" (
    model_name, input_cost_per_token, output_cost_per_token,
    max_input_tokens, max_output_tokens, max_tokens,
    mode, provider, source_url,
    cache_read_input_token_cost, cache_creation_input_token_cost,
    cache_read_input_token_cost_above_200k, cache_creation_input_token_cost_above_200k,
    synced_at
) VALUES (
    @model_name, @input_cost_per_token, @output_cost_per_token,
    @max_input_tokens, @max_output_tokens, @max_tokens,
    @mode, @provider, @source_url,
    @cache_read_input_token_cost, @cache_creation_input_token_cost,
    @cache_read_input_token_cost_above_200k, @cache_creation_input_token_cost_above_200k,
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
    synced_at             = NOW(),
    updated_at            = NOW();

-- name: ListModelPricing :many
SELECT * FROM "ModelPricing"
ORDER BY model_name;

-- name: DeleteAllModelPricing :exec
DELETE FROM "ModelPricing";
