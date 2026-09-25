-- 019_upstream_token.down.sql
DROP INDEX IF EXISTS idx_error_logs_upstream_token;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS organization_id;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS end_user;
ALTER TABLE "ErrorLogs" DROP COLUMN IF EXISTS upstream_token_key;
DROP INDEX IF EXISTS idx_spend_logs_upstream_token;
ALTER TABLE "SpendLogs" DROP COLUMN IF EXISTS upstream_token_key;
