-- 000012_add_rate_limits_table.up.sql
-- Tabla UNLOGGED en PostgreSQL para rate limiting distribuido sin Redis.
-- UNLOGGED evita escritura en WAL para contadores efímeros de ventana corta,
-- manteniendo atomicidad por fila y compatibilidad con FOR UPDATE SKIP LOCKED.

CREATE UNLOGGED TABLE IF NOT EXISTS rate_limits (
    key TEXT PRIMARY KEY,
    count INT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_rate_limits_expires_at ON rate_limits(expires_at);
