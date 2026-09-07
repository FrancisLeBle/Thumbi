-- 000001_init_schema.down.sql
-- Reversión de la migración inicial del Módulo 1 (Auth & KYC)

DROP TABLE IF EXISTS vehicles CASCADE;
DROP TABLE IF EXISTS kyc_verifications CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS users CASCADE;

DROP TYPE IF EXISTS vehicle_status_enum;
DROP TYPE IF EXISTS kyc_status_enum;
DROP TYPE IF EXISTS user_role_enum;
DROP TYPE IF EXISTS auth_provider_enum;
