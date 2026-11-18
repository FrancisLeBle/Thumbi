-- 000013_add_platform_monetization_and_pricing_config.down.sql

ALTER TABLE refund_transactions
    DROP CONSTRAINT IF EXISTS chk_refund_amounts_positive,
    DROP CONSTRAINT IF EXISTS chk_refund_platform_retained_non_negative;

ALTER TABLE refund_transactions
    DROP COLUMN IF EXISTS platform_retained_amount;

ALTER TABLE refund_transactions
    ADD CONSTRAINT chk_refund_amounts_positive CHECK (passenger_refund_amount + driver_compensation_amount > 0);

ALTER TABLE escrow_transactions
    DROP CONSTRAINT IF EXISTS chk_escrow_amount_breakdown,
    DROP CONSTRAINT IF EXISTS chk_escrow_platform_fee_non_negative,
    DROP CONSTRAINT IF EXISTS chk_escrow_base_amount_non_negative;

ALTER TABLE escrow_transactions
    DROP COLUMN IF EXISTS platform_fee,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS chk_booking_total_price,
    DROP CONSTRAINT IF EXISTS chk_booking_base_amount,
    DROP CONSTRAINT IF EXISTS chk_booking_platform_fee_non_negative,
    DROP CONSTRAINT IF EXISTS chk_booking_base_amount_non_negative;

ALTER TABLE bookings
    DROP COLUMN IF EXISTS platform_fee,
    DROP COLUMN IF EXISTS base_amount;

ALTER TABLE bookings
    ADD CONSTRAINT chk_booking_total_price CHECK (total_price = ROUND(unit_price * seats_booked, 2));

DROP INDEX IF EXISTS uq_platform_pricing_config_active;
DROP TABLE IF EXISTS platform_pricing_config;
