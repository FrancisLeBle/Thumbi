-- 000011_enforce_status_enums_and_constraints.down.sql
-- Revierte las restricciones CHECK añadidas en 000011.
-- Nota: PostgreSQL no admite eliminar un valor individual de un tipo ENUM existente sin recrearlo.

ALTER TABLE disputes
    DROP CONSTRAINT IF EXISTS chk_disputes_reason;
