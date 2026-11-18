-- 000009_add_currency_to_trips_and_bookings.up.sql
-- Agrega la columna currency CHAR(3) ISO 4217 con restricción de formato a trips y bookings,
-- y alinea escrow_transactions.currency con el valor por defecto ARS (pesos argentinos) consistente con el Cap Pricing.

ALTER TABLE trips
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'ARS';

ALTER TABLE trips
    DROP CONSTRAINT IF EXISTS chk_trips_currency_iso4217;

ALTER TABLE trips
    ADD CONSTRAINT chk_trips_currency_iso4217 CHECK (currency ~ '^[A-Z]{3}$');

ALTER TABLE bookings
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'ARS';

ALTER TABLE bookings
    DROP CONSTRAINT IF EXISTS chk_bookings_currency_iso4217;

ALTER TABLE bookings
    ADD CONSTRAINT chk_bookings_currency_iso4217 CHECK (currency ~ '^[A-Z]{3}$');

ALTER TABLE escrow_transactions
    ALTER COLUMN currency TYPE CHAR(3),
    ALTER COLUMN currency SET DEFAULT 'ARS',
    ALTER COLUMN currency SET NOT NULL;

ALTER TABLE escrow_transactions
    DROP CONSTRAINT IF EXISTS chk_escrow_currency_iso4217;

ALTER TABLE escrow_transactions
    ADD CONSTRAINT chk_escrow_currency_iso4217 CHECK (currency ~ '^[A-Z]{3}$');
