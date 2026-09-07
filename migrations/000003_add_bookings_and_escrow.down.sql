-- 000003_add_bookings_and_escrow.down.sql

DROP TABLE IF EXISTS refund_transactions CASCADE;
DROP TABLE IF EXISTS escrow_transactions CASCADE;
DROP TABLE IF EXISTS bookings CASCADE;
DROP TYPE IF EXISTS escrow_status_enum CASCADE;
DROP TYPE IF EXISTS booking_status_enum CASCADE;
