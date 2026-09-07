package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type tripRepository struct {
	db DBExecutor
}

// NewTripRepository crea una nueva instancia del repositorio PostgreSQL + PostGIS para viajes
func NewTripRepository(db DBExecutor) ports.TripRepository {
	return &tripRepository{db: db}
}

// Save persiste un viaje y sus paradas intermedias de forma atómica convirtiendo geometrías a PostGIS
func (r *tripRepository) Save(ctx context.Context, trip *domain.Trip) error {
	// Manejo de transacción si el executor subyacente la soporta
	type transactor interface {
		Begin(context.Context) (pgx.Tx, error)
	}

	exec := r.db
	var tx pgx.Tx
	var err error

	if t, ok := r.db.(transactor); ok {
		tx, err = t.Begin(ctx)
		if err != nil {
			return fmt.Errorf("error iniciando transacción para guardar viaje: %w", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		exec = tx
	}

	insertTripQuery := `
		INSERT INTO trips (
			id, driver_id, vehicle_id,
			origin_title, origin_geom,
			destination_title, destination_geom,
			route_path,
			departure_time, estimated_arrival_time,
			total_distance_km, total_duration_minutes,
			total_seats_offered, available_seats,
			price_per_seat, cap_price_per_seat,
			estimated_fuel_cost, estimated_toll_cost,
			status, created_at, updated_at
		) VALUES (
			$1, $2, $3,
			$4, ST_GeomFromText($5, 4326),
			$6, ST_GeomFromText($7, 4326),
			ST_GeomFromText($8, 4326),
			$9, $10,
			$11, $12,
			$13, $14,
			$15, $16,
			$17, $18,
			$19, $20, $21
		)
	`

	_, err = exec.Exec(ctx, insertTripQuery,
		trip.ID,
		trip.DriverID,
		trip.VehicleID,
		trip.OriginTitle,
		trip.OriginCoords.ToWKTPoint(),
		trip.DestinationTitle,
		trip.DestinationCoords.ToWKTPoint(),
		trip.RoutePath.ToWKTLineString(),
		trip.DepartureTime,
		trip.EstimatedArrivalTime,
		trip.TotalDistanceKm,
		trip.TotalDurationMinutes,
		trip.TotalSeatsOffered,
		trip.AvailableSeats,
		trip.PricePerSeat,
		trip.CapPricePerSeat,
		trip.EstimatedFuelCost,
		trip.EstimatedTollCost,
		string(trip.Status),
		trip.CreatedAt,
		trip.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error al insertar viaje en trips: %w", err)
	}

	// Inserción de paradas intermedias
	if len(trip.Stops) > 0 {
		insertStopQuery := `
			INSERT INTO trip_stops (
				id, trip_id, stop_order, location_title, location_geom, estimated_arrival_time, created_at
			) VALUES ($1, $2, $3, $4, ST_GeomFromText($5, 4326), $6, $7)
		`
		for _, stop := range trip.Stops {
			_, err = exec.Exec(ctx, insertStopQuery,
				stop.ID,
				trip.ID,
				stop.StopOrder,
				stop.LocationTitle,
				stop.Coords.ToWKTPoint(),
				stop.EstimatedArrivalTime,
				stop.CreatedAt,
			)
			if err != nil {
				return fmt.Errorf("error al insertar parada intermedia %d: %w", stop.StopOrder, err)
			}
		}
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("error al confirmar transacción de viaje: %w", err)
		}
		tx = nil
	}

	return nil
}

// FindByID recupera un viaje por su identificador primario parseando geometrías PostGIS
func (r *tripRepository) FindByID(ctx context.Context, id string) (*domain.Trip, error) {
	query := `
		SELECT 
			id, driver_id, vehicle_id,
			origin_title, ST_AsText(origin_geom),
			destination_title, ST_AsText(destination_geom),
			ST_AsText(route_path),
			departure_time, estimated_arrival_time,
			total_distance_km, total_duration_minutes,
			total_seats_offered, available_seats,
			price_per_seat, cap_price_per_seat,
			estimated_fuel_cost, estimated_toll_cost,
			status, created_at, updated_at
		FROM trips
		WHERE id = $1
	`

	var (
		trip           domain.Trip
		originWKT      string
		destWKT        string
		routeWKT       string
		statusStr      string
	)

	err := r.db.QueryRow(ctx, query, id).Scan(
		&trip.ID,
		&trip.DriverID,
		&trip.VehicleID,
		&trip.OriginTitle,
		&originWKT,
		&trip.DestinationTitle,
		&destWKT,
		&routeWKT,
		&trip.DepartureTime,
		&trip.EstimatedArrivalTime,
		&trip.TotalDistanceKm,
		&trip.TotalDurationMinutes,
		&trip.TotalSeatsOffered,
		&trip.AvailableSeats,
		&trip.PricePerSeat,
		&trip.CapPricePerSeat,
		&trip.EstimatedFuelCost,
		&trip.EstimatedTollCost,
		&statusStr,
		&trip.CreatedAt,
		&trip.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("error al consultar viaje por ID: %w", err)
	}

	trip.Status = domain.TripStatus(statusStr)

	// Reconstruir coordenadas y trazado desde WKT
	origCoords, err := domain.ParseWKTPoint(originWKT)
	if err != nil {
		return nil, fmt.Errorf("error parseando origin_geom: %w", err)
	}
	trip.OriginCoords = origCoords

	destCoords, err := domain.ParseWKTPoint(destWKT)
	if err != nil {
		return nil, fmt.Errorf("error parseando destination_geom: %w", err)
	}
	trip.DestinationCoords = destCoords

	routeLine, err := domain.ParseWKTLineString(routeWKT)
	if err != nil {
		return nil, fmt.Errorf("error parseando route_path: %w", err)
	}
	trip.RoutePath = routeLine

	// Cargar paradas intermedias
	stopsQuery := `
		SELECT id, trip_id, stop_order, location_title, ST_AsText(location_geom), estimated_arrival_time, created_at
		FROM trip_stops
		WHERE trip_id = $1
		ORDER BY stop_order ASC
	`
	rows, err := r.db.Query(ctx, stopsQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stop domain.TripStop
			var stopWKT string
			if err := rows.Scan(&stop.ID, &stop.TripID, &stop.StopOrder, &stop.LocationTitle, &stopWKT, &stop.EstimatedArrivalTime, &stop.CreatedAt); err == nil {
				if stopCoords, errCoords := domain.ParseWKTPoint(stopWKT); errCoords == nil {
					stop.Coords = stopCoords
					trip.Stops = append(trip.Stops, stop)
				}
			}
		}
	}

	return &trip, nil
}

// SearchMatchingTrips ejecuta la búsqueda geoespacial optimizada con ST_DWithin y ST_LineLocatePoint
func (r *tripRepository) SearchMatchingTrips(ctx context.Context, params ports.SearchTripsParams) ([]ports.MatchedTripRecord, error) {
	query := `
		WITH passenger_input AS (
			SELECT 
				ST_SetSRID(ST_MakePoint($1, $2), 4326) AS orig_pt,
				ST_SetSRID(ST_MakePoint($3, $4), 4326) AS dest_pt
		),
		matched_trips AS (
			SELECT 
				t.id,
				t.driver_id,
				t.vehicle_id,
				t.origin_title,
				t.destination_title,
				ST_AsText(t.origin_geom) AS origin_wkt,
				ST_AsText(t.destination_geom) AS dest_wkt,
				ST_AsText(t.route_path) AS route_wkt,
				t.departure_time,
				t.estimated_arrival_time,
				t.total_distance_km,
				t.total_duration_minutes,
				t.total_seats_offered,
				t.available_seats,
				t.price_per_seat,
				t.cap_price_per_seat,
				t.estimated_fuel_cost,
				t.estimated_toll_cost,
				t.status,
				t.created_at,
				t.updated_at,
				ST_Distance(t.route_path::geography, p.orig_pt::geography) AS pickup_walk_distance_m,
				ST_Distance(t.route_path::geography, p.dest_pt::geography) AS dropoff_walk_distance_m,
				ST_LineLocatePoint(t.route_path, p.orig_pt) AS pickup_fraction,
				ST_LineLocatePoint(t.route_path, p.dest_pt) AS dropoff_fraction
			FROM trips t
			CROSS JOIN passenger_input p
			WHERE t.status = 'PUBLISHED'
			  AND t.available_seats >= $8
			  AND t.departure_time BETWEEN $6 AND $7
			  AND ST_DWithin(t.route_path::geography, p.orig_pt::geography, $5)
			  AND ST_DWithin(t.route_path::geography, p.dest_pt::geography, $5)
		)
		SELECT 
			m.id,
			m.driver_id,
			u.first_name,
			u.last_name,
			COALESCE(u.avatar_url, ''),
			v.brand,
			v.model,
			v.plate_number,
			m.origin_title,
			m.destination_title,
			m.origin_wkt,
			m.dest_wkt,
			m.route_wkt,
			m.departure_time,
			m.estimated_arrival_time,
			m.total_distance_km,
			m.total_duration_minutes,
			m.total_seats_offered,
			m.available_seats,
			m.price_per_seat,
			m.cap_price_per_seat,
			m.estimated_fuel_cost,
			m.estimated_toll_cost,
			ROUND(m.pickup_walk_distance_m::numeric, 1) AS pickup_walk_distance_m,
			ROUND(m.dropoff_walk_distance_m::numeric, 1) AS dropoff_walk_distance_m,
			m.status,
			m.created_at,
			m.updated_at
		FROM matched_trips m
		INNER JOIN users u ON u.id = m.driver_id
		INNER JOIN vehicles v ON v.id = m.vehicle_id
		WHERE m.pickup_fraction < m.dropoff_fraction
		ORDER BY 
			(m.pickup_walk_distance_m + m.dropoff_walk_distance_m) ASC,
			m.departure_time ASC,
			m.price_per_seat ASC
		LIMIT $9 OFFSET $10;
	`

	rows, err := r.db.Query(ctx, query,
		params.OriginCoords.Longitude,
		params.OriginCoords.Latitude,
		params.DestCoords.Longitude,
		params.DestCoords.Latitude,
		params.ToleranceMeters,
		params.DepartureFrom,
		params.DepartureTo,
		params.RequiredSeats,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, fmt.Errorf("error ejecutando consulta PostGIS de matching: %w", err)
	}
	defer rows.Close()

	var records []ports.MatchedTripRecord
	for rows.Next() {
		var (
			rec        ports.MatchedTripRecord
			originWKT  string
			destWKT    string
			routeWKT   string
			statusStr  string
		)

		err := rows.Scan(
			&rec.Trip.ID,
			&rec.Trip.DriverID,
			&rec.DriverFirstName,
			&rec.DriverLastName,
			&rec.DriverAvatarURL,
			&rec.VehicleBrand,
			&rec.VehicleModel,
			&rec.VehiclePlate,
			&rec.Trip.OriginTitle,
			&rec.Trip.DestinationTitle,
			&originWKT,
			&destWKT,
			&routeWKT,
			&rec.Trip.DepartureTime,
			&rec.Trip.EstimatedArrivalTime,
			&rec.Trip.TotalDistanceKm,
			&rec.Trip.TotalDurationMinutes,
			&rec.Trip.TotalSeatsOffered,
			&rec.Trip.AvailableSeats,
			&rec.Trip.PricePerSeat,
			&rec.Trip.CapPricePerSeat,
			&rec.Trip.EstimatedFuelCost,
			&rec.Trip.EstimatedTollCost,
			&rec.PickupWalkDistanceM,
			&rec.DropoffWalkDistanceM,
			&statusStr,
			&rec.Trip.CreatedAt,
			&rec.Trip.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila de matched trip: %w", err)
		}

		rec.Trip.Status = domain.TripStatus(statusStr)

		if origCoords, errCoords := domain.ParseWKTPoint(originWKT); errCoords == nil {
			rec.Trip.OriginCoords = origCoords
		}
		if destCoords, errCoords := domain.ParseWKTPoint(destWKT); errCoords == nil {
			rec.Trip.DestinationCoords = destCoords
		}
		if routeLine, errRoute := domain.ParseWKTLineString(routeWKT); errRoute == nil {
			rec.Trip.RoutePath = routeLine
		}

		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando resultados de matching: %w", err)
	}

	return records, nil
}

// UpdateStatus actualiza el estado de ciclo de vida del viaje
func (r *tripRepository) UpdateStatus(ctx context.Context, tripID string, newStatus domain.TripStatus) error {
	query := `
		UPDATE trips
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	cmdTag, err := r.db.Exec(ctx, query, string(newStatus), tripID)
	if err != nil {
		return fmt.Errorf("error actualizando estado de viaje: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTripNotFound
	}
	return nil
}

// ListByDriverID lista los viajes creados por un conductor ordenados por fecha de partida
func (r *tripRepository) ListByDriverID(ctx context.Context, driverID string, limit, offset int) ([]*domain.Trip, error) {
	query := `
		SELECT 
			id, driver_id, vehicle_id,
			origin_title, ST_AsText(origin_geom),
			destination_title, ST_AsText(destination_geom),
			ST_AsText(route_path),
			departure_time, estimated_arrival_time,
			total_distance_km, total_duration_minutes,
			total_seats_offered, available_seats,
			price_per_seat, cap_price_per_seat,
			estimated_fuel_cost, estimated_toll_cost,
			status, created_at, updated_at
		FROM trips
		WHERE driver_id = $1
		ORDER BY departure_time DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, driverID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error al listar viajes del conductor: %w", err)
	}
	defer rows.Close()

	var trips []*domain.Trip
	for rows.Next() {
		var (
			trip      domain.Trip
			originWKT string
			destWKT   string
			routeWKT  string
			statusStr string
		)

		err := rows.Scan(
			&trip.ID,
			&trip.DriverID,
			&trip.VehicleID,
			&trip.OriginTitle,
			&originWKT,
			&trip.DestinationTitle,
			&destWKT,
			&routeWKT,
			&trip.DepartureTime,
			&trip.EstimatedArrivalTime,
			&trip.TotalDistanceKm,
			&trip.TotalDurationMinutes,
			&trip.TotalSeatsOffered,
			&trip.AvailableSeats,
			&trip.PricePerSeat,
			&trip.CapPricePerSeat,
			&trip.EstimatedFuelCost,
			&trip.EstimatedTollCost,
			&statusStr,
			&trip.CreatedAt,
			&trip.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error escaneando viaje del conductor: %w", err)
		}

		trip.Status = domain.TripStatus(statusStr)
		if origCoords, errCoords := domain.ParseWKTPoint(originWKT); errCoords == nil {
			trip.OriginCoords = origCoords
		}
		if destCoords, errCoords := domain.ParseWKTPoint(destWKT); errCoords == nil {
			trip.DestinationCoords = destCoords
		}
		if routeLine, errRoute := domain.ParseWKTLineString(routeWKT); errRoute == nil {
			trip.RoutePath = routeLine
		}

		trips = append(trips, &trip)
	}

	return trips, nil
}
