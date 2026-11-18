-- 000009_add_currency_to_trips_and_bookings.down.sql
-- Revierte la adición de currency en trips y bookings y restaura el tipo previo en escrow_transactions.

ALTER TABLE escrow_transactions
    DROP CONSTRAINT IF EXISTS chk_escrow_currency_iso4217;

ALTER TABLE escrow_transactions
    ALTER COLUMN currency TYPE VARCHAR(3),
    ALTER COLUMN currency SET DEFAULT 'USD';

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS chk_bookings_currency_iso4217,
    DROP COLUMN IF EXISTS currency;

ALTER TABLE trips
    DROP CONSTRAINT IF EXISTS chk_trips_currency_iso4217,
    DROP COLUMN IF EXISTS currency;
