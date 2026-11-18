-- 000012_add_rate_limits_table.down.sql
DROP INDEX IF EXISTS idx_rate_limits_expires_at;
DROP TABLE IF EXISTS rate_limits;
