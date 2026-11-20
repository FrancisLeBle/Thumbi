-- 000010_convert_ids_to_uuid.down.sql
-- Revierte las columnas PK y FK de UUID nativo a VARCHAR(64) y restaura fn_update_user_reputation().

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

-- Eliminar temporalmente el trigger dependiente de reviews.reviewee_id antes de revertir su tipo
DROP TRIGGER IF EXISTS trg_reviews_update_reputation ON reviews;

ALTER TABLE disputes
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN escrow_transaction_id TYPE VARCHAR(64) USING escrow_transaction_id::text,
    ALTER COLUMN booking_id TYPE VARCHAR(64) USING booking_id::text,
    ALTER COLUMN trip_id TYPE VARCHAR(64) USING trip_id::text,
    ALTER COLUMN reporter_id TYPE VARCHAR(64) USING reporter_id::text,
    ALTER COLUMN defendant_id TYPE VARCHAR(64) USING defendant_id::text,
    ALTER COLUMN resolved_by TYPE VARCHAR(64) USING resolved_by::text;

ALTER TABLE reviews
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN trip_id TYPE VARCHAR(64) USING trip_id::text,
    ALTER COLUMN booking_id TYPE VARCHAR(64) USING booking_id::text,
    ALTER COLUMN reviewer_id TYPE VARCHAR(64) USING reviewer_id::text,
    ALTER COLUMN reviewee_id TYPE VARCHAR(64) USING reviewee_id::text;

ALTER TABLE refund_transactions
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN escrow_transaction_id TYPE VARCHAR(64) USING escrow_transaction_id::text,
    ALTER COLUMN booking_id TYPE VARCHAR(64) USING booking_id::text;

ALTER TABLE escrow_transactions
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN booking_id TYPE VARCHAR(64) USING booking_id::text,
    ALTER COLUMN trip_id TYPE VARCHAR(64) USING trip_id::text,
    ALTER COLUMN payer_id TYPE VARCHAR(64) USING payer_id::text,
    ALTER COLUMN payee_id TYPE VARCHAR(64) USING payee_id::text;

ALTER TABLE bookings
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN trip_id TYPE VARCHAR(64) USING trip_id::text,
    ALTER COLUMN passenger_id TYPE VARCHAR(64) USING passenger_id::text,
    ALTER COLUMN pickup_stop_id TYPE VARCHAR(64) USING pickup_stop_id::text,
    ALTER COLUMN dropoff_stop_id TYPE VARCHAR(64) USING dropoff_stop_id::text;

ALTER TABLE trip_stops
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN trip_id TYPE VARCHAR(64) USING trip_id::text;

ALTER TABLE trips
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN driver_id TYPE VARCHAR(64) USING driver_id::text,
    ALTER COLUMN vehicle_id TYPE VARCHAR(64) USING vehicle_id::text;

ALTER TABLE user_identities
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN user_id TYPE VARCHAR(64) USING user_id::text;

ALTER TABLE vehicles
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN user_id TYPE VARCHAR(64) USING user_id::text;

ALTER TABLE kyc_verifications
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN user_id TYPE VARCHAR(64) USING user_id::text;

ALTER TABLE sessions
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text,
    ALTER COLUMN user_id TYPE VARCHAR(64) USING user_id::text;

ALTER TABLE users
    ALTER COLUMN id TYPE VARCHAR(64) USING id::text;

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
