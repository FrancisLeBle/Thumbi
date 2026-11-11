-- 000005_add_user_identities.up.sql
-- Tabla separada de identidades externas federadas (Google, Apple, etc.)
-- Previene ataques de account takeover vinculando cuentas únicamente con email_verified=true
-- y manteniendo la auditoría desacoplada de la tabla users.

CREATE TABLE IF NOT EXISTS user_identities (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider auth_provider_enum NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_identities_provider_user UNIQUE (provider, provider_user_id)
);

CREATE INDEX idx_user_identities_user_id ON user_identities(user_id);
CREATE INDEX idx_user_identities_email ON user_identities(email);
