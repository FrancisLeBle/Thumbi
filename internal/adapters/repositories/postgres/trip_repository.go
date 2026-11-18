package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

type tripRepository struct {
	db  DBExecutor
	uow TxUnitOfWork
}

// NewTripRepository crea una nueva instancia del repositorio PostgreSQL + PostGIS para viajes
func NewTripRepository(db DBExecutor, uow ...TxUnitOfWork) ports.TripRepository {
	return &tripRepository{
		db:  db,
		uow: resolveTxUnitOfWork(db, uow...),
	}
}

// Save persiste un viaje y sus paradas intermedias de forma atómica usando constructores nativos de PostGIS
func (r *tripRepository) Save(ctx context.Context, trip *domain.Trip) error {
	if !trip.Status.IsValid() {
		return fmt.Errorf("%w: estado de viaje %s", domain.ErrInvalidStatus, trip.Status)
	}
	if !trip.OriginCoords.IsValid() || !trip.DestinationCoords.IsValid() {
		return domain.ErrInvalidRouteCoordinates
	}
	routeLons, routeLats, err := lineStringToLonLatSlices(trip.RoutePath)
	if err != nil {
		return err
	}
	for _, stop := range trip.Stops {
		if !stop.Coords.IsValid() {
			return domain.ErrInvalidRouteCoordinates
		}
	}

	tripCur := trip.Currency
	if strings.TrimSpace(string(tripCur)) == "" {
		tripCur = trip.PricePerSeat.EffectiveCurrency()
	}
	parsedCur, err := domain.ParseCurrency(string(tripCur))
	if err != nil {
		return err
	}

	return runInRepoTx(ctx, r.uow, func(tx pgx.Tx) error {
		insertTripQuery := `
			INSERT INTO trips (
				id, driver_id, vehicle_id,
				origin_title, origin_geom,
				destination_title, destination_geom,
				route_path,
				departure_time, estimated_arrival_time,
				total_distance_km, total_duration_minutes,
				total_seats_offered, available_seats,
				currency,
				price_per_seat, cap_price_per_seat,
				estimated_fuel_cost, estimated_toll_cost,
				status, created_at, updated_at
			) VALUES (
				$1, $2, $3,
				$4, ST_SetSRID(ST_MakePoint($5, $6), 4326),
				$7, ST_SetSRID(ST_MakePoint($8, $9), 4326),
				ST_SetSRID(
					ST_MakeLine(
						ARRAY(
							SELECT ST_MakePoint(lon, lat)
							FROM UNNEST($10::float8[], $11::float8[]) WITH ORDINALITY AS pt(lon, lat, ord)
							ORDER BY ord
						)
					),
					4326
				),
				$12, $13,
				$14, $15,
				$16, $17,
				$18,
				$19, $20,
				$21, $22,
				$23, $24, $25
			)
		`

		if _, err := tx.Exec(ctx, insertTripQuery,
			trip.ID,
			trip.DriverID,
			trip.VehicleID,
			trip.OriginTitle,
			trip.OriginCoords.Longitude,
			trip.OriginCoords.Latitude,
			trip.DestinationTitle,
			trip.DestinationCoords.Longitude,
			trip.DestinationCoords.Latitude,
			routeLons,
			routeLats,
			trip.DepartureTime,
			trip.EstimatedArrivalTime,
			trip.TotalDistanceKm,
			trip.TotalDurationMinutes,
			trip.TotalSeatsOffered,
			trip.AvailableSeats,
			parsedCur.String(),
			moneyToPGNumeric(trip.PricePerSeat),
			moneyToPGNumeric(trip.CapPricePerSeat),
			moneyToPGNumeric(trip.EstimatedFuelCost),
			moneyToPGNumeric(trip.EstimatedTollCost),
			trip.Status.String(),
			trip.CreatedAt,
			trip.UpdatedAt,
		); err != nil {
			return fmt.Errorf("error al insertar viaje en trips: %w", err)
		}

		// Inserción de paradas intermedias
		if len(trip.Stops) > 0 {
			insertStopQuery := `
				INSERT INTO trip_stops (
					id, trip_id, stop_order, location_title, location_geom, estimated_arrival_time, created_at
				) VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint($5, $6), 4326), $7, $8)
			`
			for _, stop := range trip.Stops {
				if _, err := tx.Exec(ctx, insertStopQuery,
					stop.ID,
					trip.ID,
					stop.StopOrder,
					stop.LocationTitle,
					stop.Coords.Longitude,
					stop.Coords.Latitude,
					stop.EstimatedArrivalTime,
					stop.CreatedAt,
				); err != nil {
					return fmt.Errorf("error al insertar parada intermedia %d: %w", stop.StopOrder, err)
				}
			}
		}

		return nil
	})
}

// FindByID recupera un viaje por su identificador primario leyendo coordenadas y vértices directamente desde PostGIS
func (r *tripRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error) {
	if id == uuid.Nil {
		return nil, domain.ErrTripNotFound
	}
	query := `
		SELECT 
			id, driver_id, vehicle_id,
			origin_title, ST_X(origin_geom), ST_Y(origin_geom),
			destination_title, ST_X(destination_geom), ST_Y(destination_geom),
			ARRAY(SELECT ST_X((dp).geom) FROM ST_DumpPoints(route_path) AS dp ORDER BY (dp).path[1]),
			ARRAY(SELECT ST_Y((dp).geom) FROM ST_DumpPoints(route_path) AS dp ORDER BY (dp).path[1]),
			departure_time, estimated_arrival_time,
			total_distance_km, total_duration_minutes,
			total_seats_offered, available_seats,
			currency,
			price_per_seat, cap_price_per_seat,
			estimated_fuel_cost, estimated_toll_cost,
			status, created_at, updated_at
		FROM trips
		WHERE id = $1
	`

	var (
		trip               domain.Trip
		originLon          float64
		originLat          float64
		destLon            float64
		destLat            float64
		routeLons          []float64
		routeLats          []float64
		currencyStr        string
		pricePerSeatNum    pgtype.Numeric
		capPricePerSeatNum pgtype.Numeric
		fuelCostNum        pgtype.Numeric
		tollCostNum        pgtype.Numeric
		statusStr          string
	)

	exec := executor(ctx, r.db)
	err := exec.QueryRow(ctx, query, id).Scan(
		&trip.ID,
		&trip.DriverID,
		&trip.VehicleID,
		&trip.OriginTitle,
		&originLon,
		&originLat,
		&trip.DestinationTitle,
		&destLon,
		&destLat,
		&routeLons,
		&routeLats,
		&trip.DepartureTime,
		&trip.EstimatedArrivalTime,
		&trip.TotalDistanceKm,
		&trip.TotalDurationMinutes,
		&trip.TotalSeatsOffered,
		&trip.AvailableSeats,
		&currencyStr,
		&pricePerSeatNum,
		&capPricePerSeatNum,
		&fuelCostNum,
		&tollCostNum,
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

	cur, err := domain.ParseCurrency(currencyStr)
	if err != nil {
		return nil, fmt.Errorf("error parseando currency de viaje: %w", err)
	}
	trip.Currency = cur

	if trip.PricePerSeat, err = pgNumericToMoney(pricePerSeatNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo price_per_seat: %w", err)
	}
	if trip.CapPricePerSeat, err = pgNumericToMoney(capPricePerSeatNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo cap_price_per_seat: %w", err)
	}
	if trip.EstimatedFuelCost, err = pgNumericToMoney(fuelCostNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo estimated_fuel_cost: %w", err)
	}
	if trip.EstimatedTollCost, err = pgNumericToMoney(tollCostNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo estimated_toll_cost: %w", err)
	}

	trip.Status = domain.TripStatus(statusStr)
	if !trip.Status.IsValid() {
		return nil, fmt.Errorf("%w: estado de viaje inválido %s", domain.ErrInvalidStatus, statusStr)
	}

	origCoords, err := domain.NewCoordinates(originLat, originLon)
	if err != nil {
		return nil, fmt.Errorf("error construyendo origin_geom: %w", err)
	}
	trip.OriginCoords = origCoords

	destCoords, err := domain.NewCoordinates(destLat, destLon)
	if err != nil {
		return nil, fmt.Errorf("error construyendo destination_geom: %w", err)
	}
	trip.DestinationCoords = destCoords

	routeLine, err := lineStringFromLonLatSlices(routeLons, routeLats)
	if err != nil {
		return nil, fmt.Errorf("error construyendo route_path: %w", err)
	}
	trip.RoutePath = routeLine

	// Cargar paradas intermedias
	stopsQuery := `
		SELECT id, trip_id, stop_order, location_title, ST_X(location_geom), ST_Y(location_geom), estimated_arrival_time, created_at
		FROM trip_stops
		WHERE trip_id = $1
		ORDER BY stop_order ASC
	`
	rows, err := exec.Query(ctx, stopsQuery, id)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stop domain.TripStop
			var stopLon, stopLat float64
			if err := rows.Scan(&stop.ID, &stop.TripID, &stop.StopOrder, &stop.LocationTitle, &stopLon, &stopLat, &stop.EstimatedArrivalTime, &stop.CreatedAt); err == nil {
				if stopCoords, errCoords := domain.NewCoordinates(stopLat, stopLon); errCoords == nil {
					stop.Coords = stopCoords
					trip.Stops = append(trip.Stops, stop)
				}
			}
		}
	}

	return &trip, nil
}

// FindByIDsForUpdate bloquea y recupera viajes en orden determinista de ID para coordinación de cambios en lote.
func (r *tripRepository) FindByIDsForUpdate(ctx context.Context, ids []uuid.UUID) ([]*domain.Trip, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	query := `
		SELECT
			id, driver_id, vehicle_id,
			origin_title, ST_X(origin_geom), ST_Y(origin_geom),
			destination_title, ST_X(destination_geom), ST_Y(destination_geom),
			departure_time, estimated_arrival_time,
			total_distance_km, total_duration_minutes,
			total_seats_offered, available_seats,
			currency,
			price_per_seat, cap_price_per_seat,
			estimated_fuel_cost, estimated_toll_cost,
			status, created_at, updated_at
		FROM trips
		WHERE id = ANY($1)
		ORDER BY id ASC
		FOR UPDATE
	`

	rows, err := executor(ctx, r.db).Query(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("error bloqueando viajes para actualización en lote: %w", err)
	}
	defer rows.Close()

	var trips []*domain.Trip
	for rows.Next() {
		var (
			trip               domain.Trip
			originLon          float64
			originLat          float64
			destLon            float64
			destLat            float64
			currencyStr        string
			pricePerSeatNum    pgtype.Numeric
			capPricePerSeatNum pgtype.Numeric
			fuelCostNum        pgtype.Numeric
			tollCostNum        pgtype.Numeric
			statusStr          string
		)

		if err := rows.Scan(
			&trip.ID,
			&trip.DriverID,
			&trip.VehicleID,
			&trip.OriginTitle,
			&originLon,
			&originLat,
			&trip.DestinationTitle,
			&destLon,
			&destLat,
			&trip.DepartureTime,
			&trip.EstimatedArrivalTime,
			&trip.TotalDistanceKm,
			&trip.TotalDurationMinutes,
			&trip.TotalSeatsOffered,
			&trip.AvailableSeats,
			&currencyStr,
			&pricePerSeatNum,
			&capPricePerSeatNum,
			&fuelCostNum,
			&tollCostNum,
			&statusStr,
			&trip.CreatedAt,
			&trip.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando viaje bloqueado: %w", err)
		}

		cur, err := domain.ParseCurrency(currencyStr)
		if err != nil {
			return nil, fmt.Errorf("error parseando currency de viaje bloqueado: %w", err)
		}
		trip.Currency = cur

		if trip.PricePerSeat, err = pgNumericToMoney(pricePerSeatNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo price_per_seat: %w", err)
		}
		if trip.CapPricePerSeat, err = pgNumericToMoney(capPricePerSeatNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo cap_price_per_seat: %w", err)
		}
		if trip.EstimatedFuelCost, err = pgNumericToMoney(fuelCostNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo estimated_fuel_cost: %w", err)
		}
		if trip.EstimatedTollCost, err = pgNumericToMoney(tollCostNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo estimated_toll_cost: %w", err)
		}

		trip.Status = domain.TripStatus(statusStr)
		if !trip.Status.IsValid() {
			return nil, fmt.Errorf("%w: estado de viaje inválido %s", domain.ErrInvalidStatus, statusStr)
		}
		if origCoords, errCoords := domain.NewCoordinates(originLat, originLon); errCoords == nil {
			trip.OriginCoords = origCoords
		}
		if destCoords, errCoords := domain.NewCoordinates(destLat, destLon); errCoords == nil {
			trip.DestinationCoords = destCoords
		}

		trips = append(trips, &trip)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando viajes bloqueados: %w", err)
	}

	return trips, nil
}

// SearchMatchingTrips ejecuta la búsqueda geoespacial optimizada usando el índice GIST de expresión (route_path::geography),
// filtrando dirección con ST_LineLocatePoint antes de calcular distancias geodésicas exactas (ST_Distance), proyectando
// origen/destino vía ST_X/ST_Y y retornando una polilínea simplificada y comprimida (ST_Simplify + ST_AsEncodedPolyline).
func (r *tripRepository) SearchMatchingTrips(ctx context.Context, params ports.SearchTripsParams) ([]ports.MatchedTripRecord, error) {
	query := `
		WITH passenger_input AS NOT MATERIALIZED (
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
				t.origin_geom,
				t.destination_geom,
				t.route_path,
				t.departure_time,
				t.estimated_arrival_time,
				t.total_distance_km,
				t.total_duration_minutes,
				t.total_seats_offered,
				t.available_seats,
				t.currency,
				t.price_per_seat,
				t.cap_price_per_seat,
				t.estimated_fuel_cost,
				t.estimated_toll_cost,
				t.status,
				t.created_at,
				t.updated_at,
				p.orig_pt,
				p.dest_pt,
				ST_LineLocatePoint(t.route_path, p.orig_pt) AS pickup_fraction,
				ST_LineLocatePoint(t.route_path, p.dest_pt) AS dropoff_fraction
			FROM trips t
			CROSS JOIN passenger_input p
			WHERE t.status = 'PUBLISHED'
			  AND t.available_seats >= $8
			  AND t.departure_time BETWEEN $6 AND $7
			  AND ST_DWithin(t.route_path::geography, p.orig_pt::geography, $5)
			  AND ST_DWithin(t.route_path::geography, p.dest_pt::geography, $5)
		),
		ranked_trips AS (
			SELECT
				m.*,
				ST_Distance(m.route_path::geography, m.orig_pt::geography) AS pickup_walk_distance_m,
				ST_Distance(m.route_path::geography, m.dest_pt::geography) AS dropoff_walk_distance_m
			FROM matched_trips m
			WHERE m.pickup_fraction < m.dropoff_fraction
		)
		SELECT
			r.id,
			r.driver_id,
			u.first_name,
			u.last_name,
			COALESCE(u.avatar_url, ''),
			v.brand,
			v.model,
			v.plate_number,
			r.origin_title,
			r.destination_title,
			ST_X(r.origin_geom) AS origin_lon,
			ST_Y(r.origin_geom) AS origin_lat,
			ST_X(r.destination_geom) AS dest_lon,
			ST_Y(r.destination_geom) AS dest_lat,
			COALESCE(ST_AsEncodedPolyline(ST_Simplify(r.route_path, 0.001), 5), '') AS route_preview_polyline,
			r.departure_time,
			r.estimated_arrival_time,
			r.total_distance_km,
			r.total_duration_minutes,
			r.total_seats_offered,
			r.available_seats,
			r.currency,
			r.price_per_seat,
			r.cap_price_per_seat,
			r.estimated_fuel_cost,
			r.estimated_toll_cost,
			ROUND(r.pickup_walk_distance_m::numeric, 1) AS pickup_walk_distance_m,
			ROUND(r.dropoff_walk_distance_m::numeric, 1) AS dropoff_walk_distance_m,
			r.status,
			r.created_at,
			r.updated_at
		FROM ranked_trips r
		INNER JOIN users u ON u.id = r.driver_id
		INNER JOIN vehicles v ON v.id = r.vehicle_id
		ORDER BY
			(r.pickup_walk_distance_m + r.dropoff_walk_distance_m) ASC,
			r.departure_time ASC,
			r.price_per_seat ASC
		LIMIT $9 OFFSET $10;
	`

	rows, err := executor(ctx, r.db).Query(ctx, query,
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
			rec                ports.MatchedTripRecord
			originLon          float64
			originLat          float64
			destLon            float64
			destLat            float64
			currencyStr        string
			pricePerSeatNum    pgtype.Numeric
			capPricePerSeatNum pgtype.Numeric
			fuelCostNum        pgtype.Numeric
			tollCostNum        pgtype.Numeric
			statusStr          string
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
			&originLon,
			&originLat,
			&destLon,
			&destLat,
			&rec.RoutePreviewPolyline,
			&rec.Trip.DepartureTime,
			&rec.Trip.EstimatedArrivalTime,
			&rec.Trip.TotalDistanceKm,
			&rec.Trip.TotalDurationMinutes,
			&rec.Trip.TotalSeatsOffered,
			&rec.Trip.AvailableSeats,
			&currencyStr,
			&pricePerSeatNum,
			&capPricePerSeatNum,
			&fuelCostNum,
			&tollCostNum,
			&rec.PickupWalkDistanceM,
			&rec.DropoffWalkDistanceM,
			&statusStr,
			&rec.Trip.CreatedAt,
			&rec.Trip.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error al escanear fila de matched trip: %w", err)
		}

		cur, err := domain.ParseCurrency(currencyStr)
		if err != nil {
			return nil, fmt.Errorf("error parseando currency de matched trip: %w", err)
		}
		rec.Trip.Currency = cur

		if rec.Trip.PricePerSeat, err = pgNumericToMoney(pricePerSeatNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo price_per_seat: %w", err)
		}
		if rec.Trip.CapPricePerSeat, err = pgNumericToMoney(capPricePerSeatNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo cap_price_per_seat: %w", err)
		}
		if rec.Trip.EstimatedFuelCost, err = pgNumericToMoney(fuelCostNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo estimated_fuel_cost: %w", err)
		}
		if rec.Trip.EstimatedTollCost, err = pgNumericToMoney(tollCostNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo estimated_toll_cost: %w", err)
		}

		rec.Trip.Status = domain.TripStatus(statusStr)
		if !rec.Trip.Status.IsValid() {
			return nil, fmt.Errorf("%w: estado de viaje inválido %s", domain.ErrInvalidStatus, statusStr)
		}

		if origCoords, errCoords := domain.NewCoordinates(originLat, originLon); errCoords == nil {
			rec.Trip.OriginCoords = origCoords
		}
		if destCoords, errCoords := domain.NewCoordinates(destLat, destLon); errCoords == nil {
			rec.Trip.DestinationCoords = destCoords
		}

		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando resultados de matching: %w", err)
	}

	return records, nil
}

// UpdateStatus actualiza el estado de ciclo de vida del viaje
func (r *tripRepository) UpdateStatus(ctx context.Context, tripID uuid.UUID, newStatus domain.TripStatus) error {
	if tripID == uuid.Nil {
		return domain.ErrTripNotFound
	}
	if !newStatus.IsValid() {
		return fmt.Errorf("%w: estado de viaje %s", domain.ErrInvalidStatus, newStatus)
	}
	query := `
		UPDATE trips
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	cmdTag, err := executor(ctx, r.db).Exec(ctx, query, newStatus.String(), tripID)
	if err != nil {
		return fmt.Errorf("error actualizando estado de viaje: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTripNotFound
	}
	return nil
}

// ReleaseSeats aplica atómicamente la restitución de plazas y transición de estado decidida por el dominio,
// reportando si la actualización fue aplicada (RowsAffected() > 0).
func (r *tripRepository) ReleaseSeats(ctx context.Context, transition domain.SeatTransition) (bool, error) {
	if transition.TripID == uuid.Nil {
		return false, domain.ErrTripNotFound
	}
	if transition.SeatsDelta <= 0 {
		return false, domain.ErrInvalidSeatCount
	}
	if !transition.ExpectedStatus.IsValid() || !transition.NewStatus.IsValid() {
		return false, fmt.Errorf("%w: transición de estado de viaje %s -> %s", domain.ErrInvalidStatus, transition.ExpectedStatus, transition.NewStatus)
	}

	query := `
		UPDATE trips
		SET available_seats = available_seats + $1,
		    status = $2,
		    updated_at = NOW()
		WHERE id = $3
		  AND status = $4
		  AND available_seats + $1 <= total_seats_offered
	`

	cmdTag, err := executor(ctx, r.db).Exec(
		ctx,
		query,
		transition.SeatsDelta,
		transition.NewStatus.String(),
		transition.TripID,
		transition.ExpectedStatus.String(),
	)
	if err != nil {
		return false, fmt.Errorf("error restituyendo %d asientos al viaje %s: %w", transition.SeatsDelta, transition.TripID, err)
	}
	return cmdTag.RowsAffected() > 0, nil
}

// ApplySeatTransitionsBatch aplica en un único UPDATE ... FROM UNNEST las restituciones de asientos
// y transiciones de estado ya decididas por el dominio, validando las guardas de estado y capacidad.
func (r *tripRepository) ApplySeatTransitionsBatch(ctx context.Context, transitions []domain.SeatTransition, updatedAt time.Time) (int, error) {
	if len(transitions) == 0 {
		return 0, nil
	}

	tripIDs := make([]uuid.UUID, len(transitions))
	seatsDeltas := make([]int32, len(transitions))
	expectedStatuses := make([]string, len(transitions))
	newStatuses := make([]string, len(transitions))

	for i, tr := range transitions {
		if tr.TripID == uuid.Nil {
			return 0, domain.ErrTripNotFound
		}
		if tr.SeatsDelta <= 0 {
			return 0, domain.ErrInvalidSeatCount
		}
		if !tr.ExpectedStatus.IsValid() || !tr.NewStatus.IsValid() {
			return 0, fmt.Errorf("%w: transición de estado de viaje %s -> %s", domain.ErrInvalidStatus, tr.ExpectedStatus, tr.NewStatus)
		}
		tripIDs[i] = tr.TripID
		seatsDeltas[i] = int32(tr.SeatsDelta)
		expectedStatuses[i] = tr.ExpectedStatus.String()
		newStatuses[i] = tr.NewStatus.String()
	}

	query := `
		UPDATE trips t
		SET available_seats = t.available_seats + r.seats_to_restore,
		    status = r.new_status::trip_status_enum,
		    updated_at = $1
		FROM UNNEST($2::uuid[], $3::int[], $4::text[], $5::text[]) AS r(trip_id, seats_to_restore, expected_status, new_status)
		WHERE t.id = r.trip_id
		  AND t.status::text = r.expected_status
		  AND t.available_seats + r.seats_to_restore <= t.total_seats_offered
	`

	cmdTag, err := executor(ctx, r.db).Exec(ctx, query, updatedAt.UTC(), tripIDs, seatsDeltas, expectedStatuses, newStatuses)
	if err != nil {
		return 0, fmt.Errorf("error aplicando restitución en lote de asientos: %w", err)
	}
	return int(cmdTag.RowsAffected()), nil
}

// CalculateRouteDistanceKm delega a PostGIS (ST_Length sobre geography WGS 84) el cálculo geodésico de longitud de una polilínea en kilómetros.
func (r *tripRepository) CalculateRouteDistanceKm(ctx context.Context, routePath domain.LineString) (float64, error) {
	lons, lats, err := lineStringToLonLatSlices(routePath)
	if err != nil {
		return 0, err
	}

	query := `
		SELECT COALESCE(
			ST_Length(
				ST_SetSRID(
					ST_MakeLine(
						ARRAY(
							SELECT ST_MakePoint(lon, lat)
							FROM UNNEST($1::float8[], $2::float8[]) WITH ORDINALITY AS pt(lon, lat, ord)
							ORDER BY ord
						)
					),
					4326
				)::geography
			) / 1000.0,
			0
		)
	`

	var distanceKm float64
	if err := executor(ctx, r.db).QueryRow(ctx, query, lons, lats).Scan(&distanceKm); err != nil {
		return 0, fmt.Errorf("error calculando distancia geodésica de ruta en PostGIS: %w", err)
	}
	return distanceKm, nil
}

func lineStringToLonLatSlices(ls domain.LineString) ([]float64, []float64, error) {
	if len(ls.Coordinates) < 2 {
		return nil, nil, domain.ErrInsufficientRoutePoints
	}
	lons := make([]float64, len(ls.Coordinates))
	lats := make([]float64, len(ls.Coordinates))
	for i, c := range ls.Coordinates {
		if !c.IsValid() {
			return nil, nil, domain.ErrInvalidRouteCoordinates
		}
		lons[i] = c.Longitude
		lats[i] = c.Latitude
	}
	return lons, lats, nil
}

func lineStringFromLonLatSlices(lons, lats []float64) (domain.LineString, error) {
	if len(lons) != len(lats) {
		return domain.LineString{}, domain.ErrInvalidRouteCoordinates
	}
	if len(lons) < 2 {
		return domain.LineString{}, domain.ErrInsufficientRoutePoints
	}
	coords := make([]domain.Coordinates, len(lons))
	for i := range lons {
		c, err := domain.NewCoordinates(lats[i], lons[i])
		if err != nil {
			return domain.LineString{}, err
		}
		coords[i] = c
	}
	return domain.NewLineString(coords)
}
