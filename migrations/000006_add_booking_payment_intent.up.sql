-- 000006_add_booking_payment_intent.up.sql
-- Vinculación estricta de orden/intención de pago (Payment Intent) a la reserva
ALTER TABLE bookings ADD COLUMN IF NOT EXISTS payment_intent_id VARCHAR(255);
CREATE INDEX IF NOT EXISTS idx_bookings_payment_intent_id ON bookings(payment_intent_id);
