-- 000006_add_booking_payment_intent.down.sql
DROP INDEX IF EXISTS idx_bookings_payment_intent_id;
ALTER TABLE bookings DROP COLUMN IF EXISTS payment_intent_id;
