-- 019_upstream_token.up.sql

-- SpendLogs: add upstream token tracking
ALTER TABLE "SpendLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_spend_logs_upstream_token ON "SpendLogs" (upstream_token_key);

-- ErrorLogs: add upstream token + missing fields for parity with SpendLogs
ALTER TABLE "ErrorLogs" ADD COLUMN upstream_token_key TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN end_user TEXT NOT NULL DEFAULT '';
ALTER TABLE "ErrorLogs" ADD COLUMN organization_id TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_error_logs_upstream_token ON "ErrorLogs" (upstream_token_key);

-- 019_upstream_token.down.sql
DROP INDEX IF EXISTS idx_error_logs_upstream_token;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS organization_id;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS end_user;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS upstream_token_key;
DROP INDEX IF EXISTS idx_spend_logs_upstream_token;
ALTER TABLE "SpendLogs" DROP COLUMN IF EXISTS upstream_token_key;
