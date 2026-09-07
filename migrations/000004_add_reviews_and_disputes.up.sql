-- 000004_add_reviews_and_disputes.up.sql
-- Módulo 4: Finalización de Viajes, Liquidación de Escrow (Payouts), Sistema Reputacional (Reviews) y Disputas

-- 1. Enumeración para los estados del ciclo de vida de una disputa de Escrow
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'dispute_status_enum') THEN
        CREATE TYPE dispute_status_enum AS ENUM (
            'OPENED',                     -- Disputa iniciada por el pasajero o conductor
            'IN_REVIEW',                  -- Asignada y en análisis por el equipo de mediación/admin
            'RESOLVED_PASSENGER_REFUND',  -- Dictamen a favor del pasajero: reembolso de fondos retenidos
            'RESOLVED_DRIVER_PAYOUT',     -- Dictamen a favor del conductor: liberación de fondos retenidos
            'REJECTED'                    -- Disputa desestimada por falta de mérito o evidencia
        );
    END IF;
END$$;

-- 2. Modificación de la tabla users para métricas de reputación acumuladas
ALTER TABLE users 
    ADD COLUMN IF NOT EXISTS rating_avg NUMERIC(3, 2) NOT NULL DEFAULT 5.00 CHECK (rating_avg >= 1.00 AND rating_avg <= 5.00),
    ADD COLUMN IF NOT EXISTS rating_count INT NOT NULL DEFAULT 0 CHECK (rating_count >= 0);

CREATE INDEX IF NOT EXISTS idx_users_rating_avg ON users (rating_avg DESC);

-- 3. Tabla de Reseñas y Calificaciones Bidireccionales (reviews)
CREATE TABLE IF NOT EXISTS reviews (
    id VARCHAR(64) PRIMARY KEY,
    trip_id VARCHAR(64) NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    booking_id VARCHAR(64) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    reviewer_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reviewee_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating >= 1 AND rating <= 5),
    comment TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Invariante de evaluación única por reserva y evaluador
    CONSTRAINT uq_review_booking_reviewer UNIQUE (booking_id, reviewer_id),
    -- Invariante: Un usuario no puede calificarse a sí mismo
    CONSTRAINT chk_reviewer_not_reviewee CHECK (reviewer_id <> reviewee_id)
);

-- Índices para búsqueda de reseñas
CREATE INDEX IF NOT EXISTS idx_reviews_reviewee_id ON reviews (reviewee_id);
CREATE INDEX IF NOT EXISTS idx_reviews_reviewer_id ON reviews (reviewer_id);
CREATE INDEX IF NOT EXISTS idx_reviews_trip_id ON reviews (trip_id);
CREATE INDEX IF NOT EXISTS idx_reviews_booking_id ON reviews (booking_id);
CREATE INDEX IF NOT EXISTS idx_reviews_created_at ON reviews (created_at DESC);

-- 4. Tabla de Registro y Seguimiento de Disputas de Escrow (disputes)
CREATE TABLE IF NOT EXISTS disputes (
    id VARCHAR(64) PRIMARY KEY,
    escrow_transaction_id VARCHAR(64) NOT NULL UNIQUE REFERENCES escrow_transactions(id) ON DELETE CASCADE,
    booking_id VARCHAR(64) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    trip_id VARCHAR(64) NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    reporter_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    defendant_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason VARCHAR(64) NOT NULL,
    description TEXT NOT NULL,
    evidence_urls TEXT[] NOT NULL DEFAULT '{}',
    status dispute_status_enum NOT NULL DEFAULT 'OPENED',
    admin_notes TEXT,
    resolved_by VARCHAR(64) REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Invariante: Quien reporta no puede ser la contraparte denunciada
    CONSTRAINT chk_reporter_not_defendant CHECK (reporter_id <> defendant_id)
);

-- Índices para búsqueda y auditoría de disputas
CREATE INDEX IF NOT EXISTS idx_disputes_escrow_id ON disputes (escrow_transaction_id);
CREATE INDEX IF NOT EXISTS idx_disputes_booking_id ON disputes (booking_id);
CREATE INDEX IF NOT EXISTS idx_disputes_trip_id ON disputes (trip_id);
CREATE INDEX IF NOT EXISTS idx_disputes_reporter_id ON disputes (reporter_id);
CREATE INDEX IF NOT EXISTS idx_disputes_defendant_id ON disputes (defendant_id);
CREATE INDEX IF NOT EXISTS idx_disputes_status ON disputes (status);
CREATE INDEX IF NOT EXISTS idx_disputes_created_at ON disputes (created_at DESC);

-- 5. Función SQL y Disparador para Actualización Atómica de Reputación en users
CREATE OR REPLACE FUNCTION fn_update_user_reputation()
RETURNS TRIGGER AS $$
DECLARE
    v_reviewee_id VARCHAR(64);
    v_old_reviewee_id VARCHAR(64);
    v_new_avg NUMERIC(3, 2);
    v_new_count INT;
BEGIN
    IF (TG_OP = 'DELETE') THEN
        v_reviewee_id := OLD.reviewee_id;
    ELSIF (TG_OP = 'UPDATE') THEN
        v_reviewee_id := NEW.reviewee_id;
        v_old_reviewee_id := OLD.reviewee_id;
    ELSE
        v_reviewee_id := NEW.reviewee_id;
    END IF;

    -- Recalcular métricas para el usuario calificado actual
    SELECT 
        COALESCE(ROUND(AVG(rating), 2), 5.00),
        COUNT(*)
    INTO 
        v_new_avg,
        v_new_count
    FROM reviews
    WHERE reviewee_id = v_reviewee_id;

    IF v_new_count = 0 THEN
        v_new_avg := 5.00;
    END IF;

    UPDATE users
    SET 
        rating_avg = v_new_avg,
        rating_count = v_new_count,
        updated_at = NOW()
    WHERE id = v_reviewee_id;

    -- Si hubo un cambio de reviewee_id en un UPDATE, recalcular también para el anterior
    IF (TG_OP = 'UPDATE' AND v_old_reviewee_id IS NOT NULL AND v_old_reviewee_id <> v_reviewee_id) THEN
        SELECT 
            COALESCE(ROUND(AVG(rating), 2), 5.00),
            COUNT(*)
        INTO 
            v_new_avg,
            v_new_count
        FROM reviews
        WHERE reviewee_id = v_old_reviewee_id;

        IF v_new_count = 0 THEN
            v_new_avg := 5.00;
        END IF;

        UPDATE users
        SET 
            rating_avg = v_new_avg,
            rating_count = v_new_count,
            updated_at = NOW()
        WHERE id = v_old_reviewee_id;
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_reviews_update_reputation ON reviews;
CREATE TRIGGER trg_reviews_update_reputation
AFTER INSERT OR UPDATE OF rating, reviewee_id OR DELETE ON reviews
FOR EACH ROW
EXECUTE FUNCTION fn_update_user_reputation();
