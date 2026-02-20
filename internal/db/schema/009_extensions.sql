-- 009_extensions.sql
-- Operational extension tables: health checks, error logs, org/end-user spend

CREATE TABLE IF NOT EXISTS "HealthCheckTable" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    model_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT '',
    response_time_ms DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    error_message TEXT NOT NULL DEFAULT '',
    checked_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_health_check_model ON "HealthCheckTable" (model_name);
CREATE INDEX IF NOT EXISTS idx_health_check_time ON "HealthCheckTable" (checked_at);

CREATE TABLE IF NOT EXISTS "ErrorLogs" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    request_id TEXT NOT NULL DEFAULT '',
    api_key_hash TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    provider TEXT NOT NULL DEFAULT '',
    status_code INTEGER NOT NULL DEFAULT 0,
    error_type TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    traceback TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_error_logs_request ON "ErrorLogs" (request_id);
CREATE INDEX IF NOT EXISTS idx_error_logs_created ON "ErrorLogs" (created_at);
CREATE INDEX IF NOT EXISTS idx_error_logs_model ON "ErrorLogs" (model);

CREATE TABLE IF NOT EXISTS "DailyOrganizationSpend" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    organization_id TEXT NOT NULL DEFAULT '',
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    model TEXT NOT NULL DEFAULT '',
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    api_requests BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (organization_id, date, model)
);

CREATE INDEX IF NOT EXISTS idx_daily_org_spend_org ON "DailyOrganizationSpend" (organization_id);
CREATE INDEX IF NOT EXISTS idx_daily_org_spend_date ON "DailyOrganizationSpend" (date);

CREATE TABLE IF NOT EXISTS "DailyEndUserSpend" (
    id TEXT PRIMARY KEY DEFAULT gen_random_uuid()::TEXT,
    end_user_id TEXT NOT NULL DEFAULT '',
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    model TEXT NOT NULL DEFAULT '',
    api_key TEXT NOT NULL DEFAULT '',
    spend DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    api_requests BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (end_user_id, date, model, api_key)
);

CREATE INDEX IF NOT EXISTS idx_daily_eu_spend_user ON "DailyEndUserSpend" (end_user_id);
CREATE INDEX IF NOT EXISTS idx_daily_eu_spend_date ON "DailyEndUserSpend" (date);
