-- 013_model_pricing_cache_tokens.sql
-- Add cache token pricing fields to ModelPricing table.

ALTER TABLE "ModelPricing"
    ADD COLUMN IF NOT EXISTS cache_read_input_token_cost            DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_input_token_cost        DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_input_token_cost_above_200k  DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_input_token_cost_above_200k DOUBLE PRECISION NOT NULL DEFAULT 0;
