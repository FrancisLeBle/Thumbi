-- 000002_add_trips_and_postgis.down.sql

DROP TABLE IF EXISTS trip_stops CASCADE;
DROP TABLE IF EXISTS trips CASCADE;
DROP TYPE IF EXISTS trip_status_enum CASCADE;
-- DROP EXTENSION IF EXISTS postgis; -- Opcional según si otros módulos lo requieren
