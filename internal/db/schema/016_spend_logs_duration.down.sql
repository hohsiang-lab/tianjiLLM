DROP INDEX IF EXISTS idx_spend_logs_duration;
ALTER TABLE "SpendLogs" DROP COLUMN IF EXISTS request_duration_ms;
