-- 000008_drop_duplicate_unique_indexes.up.sql
-- Elimina índices secundarios redundantes sobre columnas que ya poseen restricciones UNIQUE
-- (PostgreSQL crea automáticamente índices B-Tree únicos implícitos: users_email_key y sessions_token_key).
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_sessions_token;
