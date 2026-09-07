-- 000001_init_schema.up.sql
-- Migración inicial para el Módulo 1: Autenticación, Registro y Verificación KYC (Thumbi Carpooling)

-- Extensiones necesarias
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Enumeraciones
CREATE TYPE auth_provider_enum AS ENUM ('GOOGLE', 'APPLE');
CREATE TYPE user_role_enum AS ENUM ('PASSENGER', 'DRIVER');
CREATE TYPE kyc_status_enum AS ENUM (
    'NOT_STARTED',
    'PENDING_VERIFICATION',
    'APPROVED',
    'REJECTED',
    'PENDING_MANUAL_REVIEW'
);
CREATE TYPE vehicle_status_enum AS ENUM (
    'PENDING_VERIFICATION',
    'APPROVED',
    'REJECTED'
);

-- 1. Tabla de Usuarios (RF-01, RF-02, RF-03)
-- Perfil unificado base de Pasajero por defecto
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(64) PRIMARY KEY,
    email VARCHAR(255) NOT NULL UNIQUE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    avatar_url TEXT NOT NULL DEFAULT '',
    provider auth_provider_enum NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    role user_role_enum NOT NULL DEFAULT 'PASSENGER',
    kyc_status kyc_status_enum NOT NULL DEFAULT 'NOT_STARTED',
    is_driver_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE UNIQUE INDEX idx_users_provider_id ON users(provider, provider_id);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_kyc_status ON users(kyc_status);

-- 2. Tabla de Sesiones y Tokens de Acceso (RF-04, RF-05, RF-06)
-- Control estricto de TTL de 20 minutos
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    user_agent TEXT NOT NULL DEFAULT '',
    client_ip VARCHAR(45) NOT NULL DEFAULT '',
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- 3. Tabla de Verificaciones KYC y Biometría (RF-07, RF-08, RF-09)
-- Registra DNI, prueba de vida biométrica y conteo de reintentos
CREATE TABLE IF NOT EXISTS kyc_verifications (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type VARCHAR(30) NOT NULL DEFAULT 'DNI',
    document_number VARCHAR(50) NOT NULL,
    front_image_url TEXT NOT NULL,
    back_image_url TEXT NOT NULL,
    selfie_3d_url TEXT NOT NULL,
    liveness_score NUMERIC(5, 4) NOT NULL DEFAULT 0.0000,
    retry_count INT NOT NULL DEFAULT 0,
    status kyc_status_enum NOT NULL DEFAULT 'PENDING_VERIFICATION',
    rejection_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kyc_user_id ON kyc_verifications(user_id);
CREATE INDEX idx_kyc_status ON kyc_verifications(status);
CREATE INDEX idx_kyc_created_at ON kyc_verifications(created_at DESC);

-- 4. Tabla de Vehículos y Documentación Automotor (RF-10, RF-11, RF-12, RF-13)
-- Registra vehículo en estado PENDING_VERIFICATION para proceso asíncrono
CREATE TABLE IF NOT EXISTS vehicles (
    id VARCHAR(64) PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    brand VARCHAR(60) NOT NULL,
    model VARCHAR(60) NOT NULL,
    year INT NOT NULL,
    plate_number VARCHAR(20) NOT NULL UNIQUE,
    color VARCHAR(40) NOT NULL,
    seat_capacity INT NOT NULL CHECK (seat_capacity BETWEEN 1 AND 8),
    driver_license_url TEXT NOT NULL,
    vehicle_cedula_url TEXT NOT NULL,
    insurance_policy_url TEXT,
    status vehicle_status_enum NOT NULL DEFAULT 'PENDING_VERIFICATION',
    rejection_reason TEXT,
    verified_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX idx_vehicles_plate_number ON vehicles(plate_number);
CREATE INDEX idx_vehicles_status ON vehicles(status);
