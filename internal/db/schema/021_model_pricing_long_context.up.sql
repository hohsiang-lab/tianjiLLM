-- 021_model_pricing_long_context.sql
-- Add generic long-context pricing fields for providers with thresholds other
-- than Anthropic's historical 200K tier (for example OpenAI GPT-5.5 at >272K).

ALTER TABLE "ModelPricing"
    ADD COLUMN IF NOT EXISTS long_context_input_threshold_tokens INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS input_cost_per_token_above_threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS output_cost_per_token_above_threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_read_input_token_cost_above_threshold DOUBLE PRECISION NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS cache_creation_input_token_cost_above_threshold DOUBLE PRECISION NOT NULL DEFAULT 0;

-- Existing live rows for OpenAI GPT-5.5 aliases are not guaranteed to be
-- touched by the next LiteLLM sync (for example openai/gpt-5.5 is a Tianji
-- route alias, while upstream publishes gpt-5.5). Backfill known aliases so
-- long-context requests are priced correctly immediately after migration.
UPDATE "ModelPricing"
SET
    long_context_input_threshold_tokens = 272000,
    input_cost_per_token_above_threshold = 0.00001,
    output_cost_per_token_above_threshold = 0.000045,
    cache_read_input_token_cost_above_threshold = 0.000001
WHERE model_name IN (
    'gpt-5.5',
    'gpt-5.5-2026-04-23',
    'openai/gpt-5.5',
    'openai/gpt-5.5-2026-04-23',
    'chatgpt/gpt-5.5',
    'chatgpt/gpt-5.5-2026-04-23'
)
AND long_context_input_threshold_tokens = 0;
