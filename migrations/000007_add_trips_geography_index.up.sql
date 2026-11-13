-- 000007_add_trips_geography_index.up.sql
-- Crea el índice espacial GIST sobre la expresión (route_path::geography) para habilitar
-- Index/Bitmap Scan en consultas PostGIS ST_DWithin(t.route_path::geography, ...) sin cambiar el tipo GEOMETRY de la columna.
CREATE INDEX IF NOT EXISTS idx_trips_route_path_geography ON trips USING GIST ((route_path::geography));
