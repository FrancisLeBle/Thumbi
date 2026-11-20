-- 000010_convert_ids_to_uuid.up.sql
-- Convierte todas las claves primarias y foráneas de VARCHAR(64) a tipo nativo UUID (16 bytes).
-- Compatible con UUIDv7 generados en capa de aplicación para preservar localidad secuencial en índices B-Tree.

-- 1. Eliminar dinámicamente las restricciones de clave foránea existentes antes de cambiar los tipos de columna
DO $$
DECLARE
    fk RECORD;
BEGIN
    FOR fk IN
        SELECT conrelid::regclass AS table_name, conname AS constraint_name
        FROM pg_constraint
        WHERE contype = 'f'
          AND connamespace = 'public'::regnamespace
          AND conrelid::regclass::text IN (
              'sessions',
              'kyc_verifications',
              'vehicles',
              'user_identities',
              'trips',
              'trip_stops',
              'bookings',
              'escrow_transactions',
              'refund_transactions',
              'reviews',
              'disputes'
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.table_name, fk.constraint_name);
    END LOOP;
END$$;

-- 1b. Eliminar temporalmente el trigger dependiente de reviews.reviewee_id antes de cambiar su tipo
DROP TRIGGER IF EXISTS trg_reviews_update_reputation ON reviews;

-- 2. Convertir columnas PK y FK a UUID nativo en las 12 tablas
ALTER TABLE users
    ALTER COLUMN id TYPE UUID USING id::uuid;

ALTER TABLE sessions
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN user_id TYPE UUID USING user_id::uuid;

ALTER TABLE kyc_verifications
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN user_id TYPE UUID USING user_id::uuid;

ALTER TABLE vehicles
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN user_id TYPE UUID USING user_id::uuid;

ALTER TABLE user_identities
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN user_id TYPE UUID USING user_id::uuid;

ALTER TABLE trips
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN driver_id TYPE UUID USING driver_id::uuid,
    ALTER COLUMN vehicle_id TYPE UUID USING vehicle_id::uuid;

ALTER TABLE trip_stops
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN trip_id TYPE UUID USING trip_id::uuid;

ALTER TABLE bookings
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN trip_id TYPE UUID USING trip_id::uuid,
    ALTER COLUMN passenger_id TYPE UUID USING passenger_id::uuid,
    ALTER COLUMN pickup_stop_id TYPE UUID USING pickup_stop_id::uuid,
    ALTER COLUMN dropoff_stop_id TYPE UUID USING dropoff_stop_id::uuid;

ALTER TABLE escrow_transactions
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN booking_id TYPE UUID USING booking_id::uuid,
    ALTER COLUMN trip_id TYPE UUID USING trip_id::uuid,
    ALTER COLUMN payer_id TYPE UUID USING payer_id::uuid,
    ALTER COLUMN payee_id TYPE UUID USING payee_id::uuid;

ALTER TABLE refund_transactions
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN escrow_transaction_id TYPE UUID USING escrow_transaction_id::uuid,
    ALTER COLUMN booking_id TYPE UUID USING booking_id::uuid;

ALTER TABLE reviews
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN trip_id TYPE UUID USING trip_id::uuid,
    ALTER COLUMN booking_id TYPE UUID USING booking_id::uuid,
    ALTER COLUMN reviewer_id TYPE UUID USING reviewer_id::uuid,
    ALTER COLUMN reviewee_id TYPE UUID USING reviewee_id::uuid;

ALTER TABLE disputes
    ALTER COLUMN id TYPE UUID USING id::uuid,
    ALTER COLUMN escrow_transaction_id TYPE UUID USING escrow_transaction_id::uuid,
    ALTER COLUMN booking_id TYPE UUID USING booking_id::uuid,
    ALTER COLUMN trip_id TYPE UUID USING trip_id::uuid,
    ALTER COLUMN reporter_id TYPE UUID USING reporter_id::uuid,
    ALTER COLUMN defendant_id TYPE UUID USING defendant_id::uuid,
    ALTER COLUMN resolved_by TYPE UUID USING resolved_by::uuid;

-- 3. Reconstruir restricciones de clave foránea con integridad referencial exacta
ALTER TABLE sessions
    ADD CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE kyc_verifications
    ADD CONSTRAINT fk_kyc_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE vehicles
    ADD CONSTRAINT fk_vehicles_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE user_identities
    ADD CONSTRAINT fk_user_identities_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE trips
    ADD CONSTRAINT fk_trips_driver FOREIGN KEY (driver_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_trips_vehicle FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE RESTRICT;

ALTER TABLE trip_stops
    ADD CONSTRAINT fk_trip_stops_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE;

ALTER TABLE bookings
    ADD CONSTRAINT fk_bookings_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_bookings_passenger FOREIGN KEY (passenger_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_bookings_pickup_stop FOREIGN KEY (pickup_stop_id) REFERENCES trip_stops(id) ON DELETE SET NULL,
    ADD CONSTRAINT fk_bookings_dropoff_stop FOREIGN KEY (dropoff_stop_id) REFERENCES trip_stops(id) ON DELETE SET NULL;

ALTER TABLE escrow_transactions
    ADD CONSTRAINT fk_escrow_booking FOREIGN KEY (booking_id) REFERENCES bookings(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_escrow_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_escrow_payer FOREIGN KEY (payer_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_escrow_payee FOREIGN KEY (payee_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE refund_transactions
    ADD CONSTRAINT fk_refund_escrow FOREIGN KEY (escrow_transaction_id) REFERENCES escrow_transactions(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_refund_booking FOREIGN KEY (booking_id) REFERENCES bookings(id) ON DELETE CASCADE;

ALTER TABLE reviews
    ADD CONSTRAINT fk_reviews_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_reviews_booking FOREIGN KEY (booking_id) REFERENCES bookings(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_reviews_reviewer FOREIGN KEY (reviewer_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_reviews_reviewee FOREIGN KEY (reviewee_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE disputes
    ADD CONSTRAINT fk_disputes_escrow FOREIGN KEY (escrow_transaction_id) REFERENCES escrow_transactions(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_disputes_booking FOREIGN KEY (booking_id) REFERENCES bookings(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_disputes_trip FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_disputes_reporter FOREIGN KEY (reporter_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_disputes_defendant FOREIGN KEY (defendant_id) REFERENCES users(id) ON DELETE CASCADE,
    ADD CONSTRAINT fk_disputes_resolved_by FOREIGN KEY (resolved_by) REFERENCES users(id) ON DELETE SET NULL;

-- 4. Actualizar variables locales de la función trigger de reputación a UUID nativo
CREATE OR REPLACE FUNCTION fn_update_user_reputation()
RETURNS TRIGGER AS $$
DECLARE
    v_reviewee_id UUID;
    v_old_reviewee_id UUID;
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

CREATE TRIGGER trg_reviews_update_reputation
AFTER INSERT OR UPDATE OF rating, reviewee_id OR DELETE ON reviews
FOR EACH ROW
EXECUTE FUNCTION fn_update_user_reputation();
