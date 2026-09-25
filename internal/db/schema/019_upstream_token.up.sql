-- 019_upstream_token.up.sql
-- Add upstream OAuth token tracking to SpendLogs and enrich ErrorLogs for observability parity.

-- SpendLogs: add upstream token tracking
ALTER TABLE "SpendLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_spend_logs_upstream_token ON "SpendLogs" (upstream_token_key);

-- ErrorLogs: add upstream token + end_user/organization_id for observability parity with SpendLogs
ALTER TABLE "ErrorLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN end_user TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN organization_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_error_logs_upstream_token ON "ErrorLogs" (upstream_token_key);
