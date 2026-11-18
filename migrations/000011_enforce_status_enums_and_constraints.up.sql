-- 000011_enforce_status_enums_and_constraints.up.sql
-- Asegura que todas las columnas de estado, rol y motivo posean tipos ENUM o restricciones CHECK estrictas en PostgreSQL.

-- 1. Incorporar el rol ADMIN a user_role_enum (faltante en la migración inicial)
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'ADMIN';

-- 2. Restringir disputes.reason al conjunto cerrado de motivos válidos del dominio (DisputeReason)
ALTER TABLE disputes
    DROP CONSTRAINT IF EXISTS chk_disputes_reason;

ALTER TABLE disputes
    ADD CONSTRAINT chk_disputes_reason CHECK (
        reason IN (
            'NO_SHOW',
            'ROUTE_DEVIATION',
            'RECKLESS_DRIVING',
            'VEHICLE_MISMATCH',
            'PASSENGER_MISCONDUCT',
            'EXTRA_CHARGE_REQUESTED',
            'OTHER'
        )
    );
