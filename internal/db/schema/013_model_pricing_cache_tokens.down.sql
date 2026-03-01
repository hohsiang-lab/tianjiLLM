ALTER TABLE "ModelPricing"
    DROP COLUMN IF EXISTS cache_read_input_token_cost,
    DROP COLUMN IF EXISTS cache_creation_input_token_cost,
    DROP COLUMN IF EXISTS cache_read_input_token_cost_above_200k,
    DROP COLUMN IF EXISTS cache_creation_input_token_cost_above_200k;
