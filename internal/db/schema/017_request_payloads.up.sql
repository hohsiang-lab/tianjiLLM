CREATE TABLE IF NOT EXISTS "RequestPayloads" (
    request_id TEXT PRIMARY KEY,
    messages TEXT NOT NULL DEFAULT '{}',
    response TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_request_payloads_created ON "RequestPayloads" (created_at);
