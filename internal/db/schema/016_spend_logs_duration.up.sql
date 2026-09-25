ALTER TABLE "SpendLogs" ADD COLUMN request_duration_ms INTEGER;
CREATE INDEX IF NOT EXISTS idx_spend_logs_duration ON "SpendLogs" (request_duration_ms);
