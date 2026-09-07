-- 000002_add_trips_and_postgis.up.sql
-- Módulo 2: Core Match, Publicación y Búsqueda de Viajes (PostGIS & Cap Pricing)

-- 1. Habilitar extensión PostGIS para cálculos espaciales
CREATE EXTENSION IF NOT EXISTS postgis;

-- 2. Enumeración para el ciclo de vida del viaje
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'trip_status_enum') THEN
        CREATE TYPE trip_status_enum AS ENUM (
            'PUBLISHED',     -- Publicado y disponible para reservas
            'FULL',          -- Asientos agotados
            'IN_PROGRESS',   -- En tránsito
            'COMPLETED',     -- Finalizado exitosamente
            'CANCELLED',     -- Cancelado por el conductor
            'EXPIRED'        -- Fecha superada sin haber iniciado
        );
    END IF;
END$$;

-- 3. Tabla principal de Viajes (trips)
CREATE TABLE IF NOT EXISTS trips (
    id VARCHAR(64) PRIMARY KEY,
    driver_id VARCHAR(64) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vehicle_id VARCHAR(64) NOT NULL REFERENCES vehicles(id) ON DELETE RESTRICT,
    
    -- Información descriptiva de Origen y Destino
    origin_title VARCHAR(255) NOT NULL,
    origin_geom GEOMETRY(Point, 4326) NOT NULL,
    destination_title VARCHAR(255) NOT NULL,
    destination_geom GEOMETRY(Point, 4326) NOT NULL,
    
    -- Trazado vectorial de la ruta completa (SRID 4326 WGS 84)
    route_path GEOMETRY(LineString, 4326) NOT NULL,
    
    -- Tiempos y distancias de ruteo
    departure_time TIMESTAMP WITH TIME ZONE NOT NULL,
    estimated_arrival_time TIMESTAMP WITH TIME ZONE NOT NULL,
    total_distance_km NUMERIC(8, 2) NOT NULL CHECK (total_distance_km > 0),
    total_duration_minutes INT NOT NULL CHECK (total_duration_minutes > 0),
    
    -- Capacidad y disponibilidad de asientos
    total_seats_offered INT NOT NULL CHECK (total_seats_offered BETWEEN 1 AND 8),
    available_seats INT NOT NULL CHECK (available_seats >= 0),
    
    -- Precios y Cap Pricing (Economía no lucrativa)
    price_per_seat NUMERIC(10, 2) NOT NULL CHECK (price_per_seat >= 0),
    cap_price_per_seat NUMERIC(10, 2) NOT NULL CHECK (cap_price_per_seat >= 0),
    estimated_fuel_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    estimated_toll_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.00,
    
    -- Estado y auditoría
    status trip_status_enum NOT NULL DEFAULT 'PUBLISHED',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Invariante de negocio: El precio fijado nunca puede superar el Cap Price
    CONSTRAINT chk_price_cannot_exceed_cap CHECK (price_per_seat <= cap_price_per_seat),
    -- Invariante de asientos: Asientos disponibles no pueden superar los ofertados
    CONSTRAINT chk_available_seats_valid CHECK (available_seats <= total_seats_offered)
);

-- Índices espaciales GIST (Críticos para rendimiento de ST_DWithin y ST_Distance)
CREATE INDEX IF NOT EXISTS idx_trips_route_path_gist ON trips USING GIST (route_path);
CREATE INDEX IF NOT EXISTS idx_trips_origin_geom_gist ON trips USING GIST (origin_geom);
CREATE INDEX IF NOT EXISTS idx_trips_destination_geom_gist ON trips USING GIST (destination_geom);

-- Índices B-Tree para filtros combinados de búsqueda
CREATE INDEX IF NOT EXISTS idx_trips_departure_status ON trips (departure_time, status, available_seats);
CREATE INDEX IF NOT EXISTS idx_trips_driver_id ON trips (driver_id);
CREATE INDEX IF NOT EXISTS idx_trips_vehicle_id ON trips (vehicle_id);

-- 4. Tabla de Paradas Intermedias (trip_stops)
CREATE TABLE IF NOT EXISTS trip_stops (
    id VARCHAR(64) PRIMARY KEY,
    trip_id VARCHAR(64) NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    stop_order INT NOT NULL CHECK (stop_order >= 1),
    location_title VARCHAR(255) NOT NULL,
    location_geom GEOMETRY(Point, 4326) NOT NULL,
    estimated_arrival_time TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    
    CONSTRAINT uq_trip_stop_order UNIQUE (trip_id, stop_order)
);

-- Índice espacial GIST para paradas intermedias
CREATE INDEX IF NOT EXISTS idx_trip_stops_location_gist ON trip_stops USING GIST (location_geom);
CREATE INDEX IF NOT EXISTS idx_trip_stops_trip_id ON trip_stops (trip_id);
