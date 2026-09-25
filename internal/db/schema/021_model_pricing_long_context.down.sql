ALTER TABLE "ModelPricing"
    DROP COLUMN IF EXISTS cache_creation_input_token_cost_above_threshold,
    DROP COLUMN IF EXISTS cache_read_input_token_cost_above_threshold,
    DROP COLUMN IF EXISTS output_cost_per_token_above_threshold,
    DROP COLUMN IF EXISTS input_cost_per_token_above_threshold,
    DROP COLUMN IF EXISTS long_context_input_threshold_tokens;
