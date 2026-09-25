-- 011_model_pricing.sql
-- ModelPricing table — stores synced model cost data from upstream (LiteLLM).

CREATE TABLE IF NOT EXISTS "ModelPricing" (
    model_name            TEXT PRIMARY KEY,
    input_cost_per_token  DOUBLE PRECISION NOT NULL DEFAULT 0,
    output_cost_per_token DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_input_tokens      INTEGER NOT NULL DEFAULT 0,
    max_output_tokens     INTEGER NOT NULL DEFAULT 0,
    max_tokens            INTEGER NOT NULL DEFAULT 0,
    mode                  TEXT NOT NULL DEFAULT '',
    provider              TEXT NOT NULL DEFAULT '',
    source_url            TEXT NOT NULL DEFAULT '',
    synced_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_model_pricing_provider  ON "ModelPricing" (provider);
CREATE INDEX IF NOT EXISTS idx_model_pricing_synced_at ON "ModelPricing" (synced_at);
