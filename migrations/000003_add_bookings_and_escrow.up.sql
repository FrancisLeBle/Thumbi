-- 000003_add_bookings_and_escrow.up.sql
-- Módulo 3: Reservas, Gestión Atómica de Asientos y Pagos en Custodia (Escrow)

-- 1. Enumeración para el ciclo de vida de la reserva
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'booking_status_enum') THEN
        CREATE TYPE booking_status_enum AS ENUM (
            'PENDING_PAYMENT',          -- Reserva creada, asientos en retención temporal (TTL 15m)
            'CONFIRMED',                -- Pago exitoso capturado en Escrow
            'REJECTED',                 -- Rechazada por falta de cupo u otra validación
            'CANCELLED_BY_PASSENGER',   -- Cancelada por el pasajero
            'CANCELLED_BY_DRIVER',      -- Cancelada por el conductor (viaje cancelado)
            'EXPIRED',                  -- Expiró el TTL de 15m sin confirmación de pago
            'COMPLETED'                 -- Viaje completado satisfactoriamente
        );
    END IF;
END$$;

-- 2. Enumeración para el estado de los fondos en custodia (Escrow)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'escrow_status_enum') THEN
        CREATE TYPE escrow_status_enum AS ENUM (
            'HELD',             -- Fondos retenidos en custodia hasta la finalización del viaje
            'RELEASED',         -- Fondos liquidados al conductor tras completar el viaje
            'REFUNDED_FULL',    -- Reembolso del 100% al pasajero
            'REFUNDED_PARTIAL', -- Reembolso parcial (ej. 50% pasajero / 50% compensación conductor)
            'DISPUTED'          -- Fondos congelados por reporte de incidencia o reclamo
        );
    END IF;
END$$;

-- 3. Tabla de Reservas (bookings)
CREATE TABLE IF NOT EXISTS bookings (
    id VARCHAR(64) PRIMARY KEY,
    trip_id VARCHAR(64) NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    passenger_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    seats_booked INT NOT NULL CHECK (seats_booked > 0),
    unit_price NUMERIC(10, 2) NOT NULL CHECK (unit_price >= 0),
    total_price NUMERIC(10, 2) NOT NULL CHECK (total_price >= 0),
    status booking_status_enum NOT NULL DEFAULT 'PENDING_PAYMENT',
    
    -- Paradas de ascenso/descenso opcionales para viajes con paradas intermedias
    pickup_stop_id VARCHAR(64) REFERENCES trip_stops(id) ON DELETE SET NULL,
    dropoff_stop_id VARCHAR(64) REFERENCES trip_stops(id) ON DELETE SET NULL,
    
    -- Tiempos de ciclo de vida y expiración de retención
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    confirmed_at TIMESTAMP WITH TIME ZONE,
    cancelled_at TIMESTAMP WITH TIME ZONE,
    cancellation_reason TEXT,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Invariante de coherencia de precio total
    CONSTRAINT chk_booking_total_price CHECK (total_price = ROUND(unit_price * seats_booked, 2))
);

-- Índices B-Tree para optimización de consultas de reservas
CREATE INDEX IF NOT EXISTS idx_bookings_trip_status ON bookings (trip_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_passenger_status ON bookings (passenger_id, status);
CREATE INDEX IF NOT EXISTS idx_bookings_pending_expires ON bookings (status, expires_at) WHERE status = 'PENDING_PAYMENT';
CREATE INDEX IF NOT EXISTS idx_bookings_created_at ON bookings (created_at DESC);

-- 4. Tabla de Transacciones en Custodia (escrow_transactions)
CREATE TABLE IF NOT EXISTS escrow_transactions (
    id VARCHAR(64) PRIMARY KEY,
    booking_id VARCHAR(64) NOT NULL UNIQUE REFERENCES bookings(id) ON DELETE CASCADE,
    trip_id VARCHAR(64) NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    payer_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Pasajero que paga
    payee_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- Conductor que percibirá los fondos
    amount NUMERIC(10, 2) NOT NULL CHECK (amount > 0),
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    payment_gateway_ref VARCHAR(255) NOT NULL,
    status escrow_status_enum NOT NULL DEFAULT 'HELD',
    
    held_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    released_at TIMESTAMP WITH TIME ZONE,
    refunded_at TIMESTAMP WITH TIME ZONE,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- Índices para búsqueda de transacciones en custodia
CREATE INDEX IF NOT EXISTS idx_escrow_trip_status ON escrow_transactions (trip_id, status);
CREATE INDEX IF NOT EXISTS idx_escrow_payer_payee ON escrow_transactions (payer_id, payee_id);
CREATE INDEX IF NOT EXISTS idx_escrow_gateway_ref ON escrow_transactions (payment_gateway_ref);

-- 5. Tabla de Registro y Auditoría de Reembolsos (refund_transactions)
CREATE TABLE IF NOT EXISTS refund_transactions (
    id VARCHAR(64) PRIMARY KEY,
    escrow_transaction_id VARCHAR(64) NOT NULL REFERENCES escrow_transactions(id) ON DELETE CASCADE,
    booking_id VARCHAR(64) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    passenger_refund_amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (passenger_refund_amount >= 0),
    driver_compensation_amount NUMERIC(10, 2) NOT NULL DEFAULT 0.00 CHECK (driver_compensation_amount >= 0),
    refund_type VARCHAR(32) NOT NULL CHECK (
        refund_type IN ('DRIVER_CANCELLATION', 'PASSENGER_EARLY', 'PASSENGER_LATE', 'ADMIN_DISPUTE')
    ),
    gateway_refund_ref VARCHAR(255),
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Invariante de coherencia de montos en reembolsos
    CONSTRAINT chk_refund_amounts_positive CHECK (passenger_refund_amount + driver_compensation_amount > 0)
);

-- Índices para trazabilidad de reembolsos
CREATE INDEX IF NOT EXISTS idx_refund_escrow_id ON refund_transactions (escrow_transaction_id);
CREATE INDEX IF NOT EXISTS idx_refund_booking_id ON refund_transactions (booking_id);
CREATE INDEX IF NOT EXISTS idx_refund_processed_at ON refund_transactions (processed_at DESC);
