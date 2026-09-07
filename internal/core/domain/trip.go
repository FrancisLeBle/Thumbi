package domain

import (
	"strings"
	"time"
)

// TripStatus define los estados del ciclo de vida de un viaje
type TripStatus string

const (
	TripStatusPublished  TripStatus = "PUBLISHED"
	TripStatusFull       TripStatus = "FULL"
	TripStatusInProgress TripStatus = "IN_PROGRESS"
	TripStatusCompleted  TripStatus = "COMPLETED"
	TripStatusCancelled  TripStatus = "CANCELLED"
	TripStatusExpired    TripStatus = "EXPIRED"
)

// TripStop representa una parada intermedia planificada a lo largo del itinerario
type TripStop struct {
	ID                   string      `json:"id"`
	TripID               string      `json:"trip_id"`
	StopOrder            int         `json:"stop_order"`
	LocationTitle        string      `json:"location_title"`
	Coords               Coordinates `json:"coords"`
	EstimatedArrivalTime time.Time   `json:"estimated_arrival_time"`
	CreatedAt            time.Time   `json:"created_at"`
}

// NewTripStop construye y valida una parada intermedia
func NewTripStop(id, tripID string, stopOrder int, title string, coords Coordinates, eta time.Time) (*TripStop, error) {
	if strings.TrimSpace(title) == "" || !coords.IsValid() {
		return nil, ErrInvalidRouteCoordinates
	}
	if stopOrder < 1 {
		return nil, ErrInvalidRouteCoordinates
	}
	return &TripStop{
		ID:                   id,
		TripID:               tripID,
		StopOrder:            stopOrder,
		LocationTitle:        strings.TrimSpace(title),
		Coords:               coords,
		EstimatedArrivalTime: eta,
		CreatedAt:            time.Now().UTC(),
	}, nil
}

// Trip representa la entidad principal de un viaje compartido en Thumbi
type Trip struct {
	ID                   string       `json:"id"`
	DriverID             string       `json:"driver_id"`
	VehicleID            string       `json:"vehicle_id"`
	OriginTitle          string       `json:"origin_title"`
	OriginCoords         Coordinates  `json:"origin_coords"`
	DestinationTitle     string       `json:"destination_title"`
	DestinationCoords    Coordinates  `json:"destination_coords"`
	RoutePath            LineString   `json:"route_path"`
	DepartureTime        time.Time    `json:"departure_time"`
	EstimatedArrivalTime time.Time    `json:"estimated_arrival_time"`
	TotalDistanceKm      float64      `json:"total_distance_km"`
	TotalDurationMinutes int          `json:"total_duration_minutes"`
	TotalSeatsOffered    int          `json:"total_seats_offered"`
	AvailableSeats       int          `json:"available_seats"`
	PricePerSeat         float64      `json:"price_per_seat"`
	CapPricePerSeat      float64      `json:"cap_price_per_seat"`
	EstimatedFuelCost    float64      `json:"estimated_fuel_cost"`
	EstimatedTollCost    float64      `json:"estimated_toll_cost"`
	Status               TripStatus   `json:"status"`
	Stops                []TripStop   `json:"stops,omitempty"`
	CreatedAt            time.Time    `json:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at"`
}

// TripCreationParams agrupa los datos necesarios para instanciar un Trip
type TripCreationParams struct {
	ID                   string
	DriverID             string
	VehicleID            string
	OriginTitle          string
	OriginCoords         Coordinates
	DestinationTitle     string
	DestinationCoords    Coordinates
	RoutePath            LineString
	DepartureTime        time.Time
	EstimatedArrivalTime time.Time
	TotalDistanceKm      float64
	TotalDurationMinutes int
	TotalSeatsOffered    int
	MaxVehicleSeats      int
	PricePerSeat         float64
	EstimatedFuelCost    float64
	EstimatedTollCost    float64
	Stops                []TripStop
}

// NewTrip crea una nueva entidad Trip aplicando rigurosamente todas las invariantes de dominio
func NewTrip(params TripCreationParams, now time.Time) (*Trip, error) {
	// Invariante 1: Fechas de salida estrictamente futuras
	if !params.DepartureTime.After(now) {
		return nil, ErrDepartureTimeMustBeFuture
	}
	if !params.EstimatedArrivalTime.After(params.DepartureTime) {
		return nil, ErrDepartureTimeMustBeFuture
	}

	// Invariante 2: Coordenadas válidas
	if !params.OriginCoords.IsValid() || !params.DestinationCoords.IsValid() {
		return nil, ErrInvalidRouteCoordinates
	}
	if len(params.RoutePath.Coordinates) < 2 {
		return nil, ErrInsufficientRoutePoints
	}

	// Invariante 3: Asientos ofertados entre 1 y la capacidad del vehículo
	if params.TotalSeatsOffered <= 0 {
		return nil, ErrInvalidSeatCount
	}
	if params.MaxVehicleSeats > 0 && params.TotalSeatsOffered > params.MaxVehicleSeats {
		return nil, ErrInvalidSeatCount
	}

	// Invariante 4: Cap Pricing transparente y no lucrativo
	costMatrix := CalculateCapPrice(params.EstimatedFuelCost, params.EstimatedTollCost, params.TotalSeatsOffered)
	if !costMatrix.IsPricePermitted(params.PricePerSeat) {
		return nil, ErrPriceExceedsCapPrice
	}

	nowUTC := now.UTC()
	return &Trip{
		ID:                   params.ID,
		DriverID:             params.DriverID,
		VehicleID:            params.VehicleID,
		OriginTitle:          strings.TrimSpace(params.OriginTitle),
		OriginCoords:         params.OriginCoords,
		DestinationTitle:     strings.TrimSpace(params.DestinationTitle),
		DestinationCoords:    params.DestinationCoords,
		RoutePath:            params.RoutePath,
		DepartureTime:        params.DepartureTime.UTC(),
		EstimatedArrivalTime: params.EstimatedArrivalTime.UTC(),
		TotalDistanceKm:      params.TotalDistanceKm,
		TotalDurationMinutes: params.TotalDurationMinutes,
		TotalSeatsOffered:    params.TotalSeatsOffered,
		AvailableSeats:       params.TotalSeatsOffered,
		PricePerSeat:         params.PricePerSeat,
		CapPricePerSeat:      costMatrix.CapPricePerSeat,
		EstimatedFuelCost:    params.EstimatedFuelCost,
		EstimatedTollCost:    params.EstimatedTollCost,
		Status:               TripStatusPublished,
		Stops:                params.Stops,
		CreatedAt:            nowUTC,
		UpdatedAt:            nowUTC,
	}, nil
}

// CanBeCancelled valida si el viaje puede cancelarse
func (t *Trip) CanBeCancelled() bool {
	return t.Status == TripStatusPublished || t.Status == TripStatusFull
}

// Cancel cancela un viaje si se encuentra en un estado cancelable
func (t *Trip) Cancel() error {
	if !t.CanBeCancelled() {
		return ErrTripCannotBeCancelled
	}
	t.Status = TripStatusCancelled
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateAvailableSeats actualiza los asientos disponibles y transiciona a FULL si se agotan
func (t *Trip) UpdateAvailableSeats(seatsRemaining int) error {
	if seatsRemaining < 0 || seatsRemaining > t.TotalSeatsOffered {
		return ErrInvalidSeatCount
	}

	t.AvailableSeats = seatsRemaining
	if t.AvailableSeats == 0 {
		t.Status = TripStatusFull
	} else if t.Status == TripStatusFull && t.AvailableSeats > 0 {
		t.Status = TripStatusPublished
	}
	t.UpdatedAt = time.Now().UTC()
	return nil
}

// TransitionStatus gestiona las transiciones válidas del ciclo de vida del viaje
func (t *Trip) TransitionStatus(newStatus TripStatus) error {
	switch newStatus {
	case TripStatusInProgress:
		if t.Status != TripStatusPublished && t.Status != TripStatusFull {
			return ErrInvalidTripStatusChange
		}
	case TripStatusCompleted:
		if t.Status != TripStatusInProgress && t.Status != TripStatusPublished && t.Status != TripStatusFull {
			return ErrInvalidTripStatusChange
		}
	case TripStatusCancelled:
		return t.Cancel()
	case TripStatusExpired:
		if t.Status != TripStatusPublished && t.Status != TripStatusFull {
			return ErrInvalidTripStatusChange
		}
	default:
		return ErrInvalidTripStatusChange
	}

	t.Status = newStatus
	t.UpdatedAt = time.Now().UTC()
	return nil
}
