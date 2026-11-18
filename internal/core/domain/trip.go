package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

// IsValid verifica que el estado del viaje pertenezca a los valores válidos del dominio.
func (s TripStatus) IsValid() bool {
	switch s {
	case TripStatusPublished,
		TripStatusFull,
		TripStatusInProgress,
		TripStatusCompleted,
		TripStatusCancelled,
		TripStatusExpired:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del estado del viaje.
func (s TripStatus) String() string {
	return string(s)
}

// TripStop representa una parada intermedia planificada a lo largo del itinerario
type TripStop struct {
	ID                   uuid.UUID   `json:"id"`
	TripID               uuid.UUID   `json:"trip_id"`
	StopOrder            int         `json:"stop_order"`
	LocationTitle        string      `json:"location_title"`
	Coords               Coordinates `json:"coords"`
	EstimatedArrivalTime time.Time   `json:"estimated_arrival_time"`
	CreatedAt            time.Time   `json:"created_at"`
}

// NewTripStop construye y valida una parada intermedia
func NewTripStop(id, tripID uuid.UUID, stopOrder int, title string, coords Coordinates, eta time.Time) (*TripStop, error) {
	if id == uuid.Nil || tripID == uuid.Nil {
		return nil, fmt.Errorf("%w: identificadores de parada inválidos", ErrInvalidInput)
	}
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
	ID                   uuid.UUID   `json:"id"`
	DriverID             uuid.UUID   `json:"driver_id"`
	VehicleID            uuid.UUID   `json:"vehicle_id"`
	OriginTitle          string      `json:"origin_title"`
	OriginCoords         Coordinates `json:"origin_coords"`
	DestinationTitle     string      `json:"destination_title"`
	DestinationCoords    Coordinates `json:"destination_coords"`
	RoutePath            LineString  `json:"route_path,omitempty"`
	RoutePreviewPolyline string      `json:"route_preview_polyline,omitempty"`
	DepartureTime        time.Time   `json:"departure_time"`
	EstimatedArrivalTime time.Time   `json:"estimated_arrival_time"`
	TotalDistanceKm      float64     `json:"total_distance_km"`
	TotalDurationMinutes int         `json:"total_duration_minutes"`
	TotalSeatsOffered    int         `json:"total_seats_offered"`
	AvailableSeats       int         `json:"available_seats"`
	Currency             Currency    `json:"currency"`
	PricePerSeat         Money       `json:"price_per_seat"`
	CapPricePerSeat      Money       `json:"cap_price_per_seat"`
	EstimatedFuelCost    Money       `json:"estimated_fuel_cost"`
	EstimatedTollCost    Money       `json:"estimated_toll_cost"`
	Status               TripStatus  `json:"status"`
	Stops                []TripStop  `json:"stops,omitempty"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

// TripCreationParams agrupa los datos necesarios para instanciar un Trip
type TripCreationParams struct {
	ID                   uuid.UUID
	DriverID             uuid.UUID
	VehicleID            uuid.UUID
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
	Currency             Currency
	PricePerSeat         Money
	EstimatedFuelCost    Money
	EstimatedTollCost    Money
	Stops                []TripStop
}

// NewTrip crea una nueva entidad Trip aplicando rigurosamente todas las invariantes de dominio
func NewTrip(params TripCreationParams, now time.Time) (*Trip, error) {
	if params.ID == uuid.Nil || params.DriverID == uuid.Nil || params.VehicleID == uuid.Nil {
		return nil, fmt.Errorf("%w: id, driver_id y vehicle_id son obligatorios", ErrInvalidInput)
	}

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

	// Invariante 4: Consistencia estricta de moneda ISO 4217 (por defecto ARS)
	tripCurrency := params.Currency
	if strings.TrimSpace(string(tripCurrency)) == "" {
		switch {
		case strings.TrimSpace(string(params.PricePerSeat.Currency)) != "":
			tripCurrency = params.PricePerSeat.Currency
		case strings.TrimSpace(string(params.EstimatedFuelCost.Currency)) != "":
			tripCurrency = params.EstimatedFuelCost.Currency
		case strings.TrimSpace(string(params.EstimatedTollCost.Currency)) != "":
			tripCurrency = params.EstimatedTollCost.Currency
		default:
			tripCurrency = DefaultCurrency
		}
	}
	parsedCurrency, err := ParseCurrency(string(tripCurrency))
	if err != nil {
		return nil, err
	}

	normalizeMoney := func(m Money) (Money, error) {
		if strings.TrimSpace(string(m.Currency)) == "" {
			return MoneyFromCents(m.Amount, parsedCurrency), nil
		}
		mCur, err := ParseCurrency(string(m.Currency))
		if err != nil {
			return Money{}, err
		}
		if mCur != parsedCurrency {
			return Money{}, fmt.Errorf("%w: se esperaba %s pero se recibió %s", ErrCurrencyMismatch, parsedCurrency, mCur)
		}
		return MoneyFromCents(m.Amount, parsedCurrency), nil
	}

	pricePerSeat, err := normalizeMoney(params.PricePerSeat)
	if err != nil {
		return nil, err
	}
	fuelCost, err := normalizeMoney(params.EstimatedFuelCost)
	if err != nil {
		return nil, err
	}
	tollCost, err := normalizeMoney(params.EstimatedTollCost)
	if err != nil {
		return nil, err
	}

	// Invariante 5: Cap Pricing transparente y no lucrativo
	costMatrix, err := CalculateCapPriceStrict(fuelCost, tollCost, params.TotalSeatsOffered)
	if err != nil {
		return nil, err
	}
	if !costMatrix.IsPricePermitted(pricePerSeat) {
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
		Currency:             parsedCurrency,
		PricePerSeat:         pricePerSeat,
		CapPricePerSeat:      costMatrix.CapPricePerSeat,
		EstimatedFuelCost:    fuelCost,
		EstimatedTollCost:    tollCost,
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

// SeatTransition encapsula la transición de asientos y estado validada por el dominio para persistencia atómica con guarda.
type SeatTransition struct {
	TripID                 uuid.UUID
	SeatsDelta             int
	ExpectedStatus         TripStatus
	ExpectedAvailableSeats int
	NewStatus              TripStatus
	NewAvailableSeats      int
}

// CanBook valida todas las invariantes de dominio para que un pasajero pueda reservar plazas en el viaje.
func (t *Trip) CanBook(passengerID uuid.UUID, seatsRequested int) error {
	if t.DriverID == passengerID {
		return ErrCannotBookOwnTrip
	}
	if seatsRequested <= 0 {
		return ErrInvalidSeatCount
	}
	if t.Status != TripStatusPublished {
		return ErrInvalidTripStatusChange
	}
	if t.AvailableSeats < seatsRequested {
		return ErrInsufficientSeats
	}
	return nil
}

// ReserveSeats valida invariantes de disponibilidad y estado, descuenta plazas y decide la transición a FULL si se agotan.
func (t *Trip) ReserveSeats(seats int, now time.Time) (SeatTransition, error) {
	if seats <= 0 {
		return SeatTransition{}, ErrInvalidSeatCount
	}
	if t.Status != TripStatusPublished {
		return SeatTransition{}, ErrInvalidTripStatusChange
	}
	if t.AvailableSeats < seats {
		return SeatTransition{}, ErrInsufficientSeats
	}

	expectedStatus := t.Status
	expectedAvailable := t.AvailableSeats

	t.AvailableSeats -= seats
	if t.AvailableSeats == 0 {
		t.Status = TripStatusFull
	}
	t.UpdatedAt = now.UTC()

	return SeatTransition{
		TripID:                 t.ID,
		SeatsDelta:             seats,
		ExpectedStatus:         expectedStatus,
		ExpectedAvailableSeats: expectedAvailable,
		NewStatus:              t.Status,
		NewAvailableSeats:      t.AvailableSeats,
	}, nil
}

// ReleaseSeats valida la restitución de plazas y decide si un viaje FULL vuelve a PUBLISHED.
func (t *Trip) ReleaseSeats(seats int, now time.Time) (SeatTransition, error) {
	if seats <= 0 {
		return SeatTransition{}, ErrInvalidSeatCount
	}
	if t.TotalSeatsOffered > 0 && t.AvailableSeats+seats > t.TotalSeatsOffered {
		return SeatTransition{}, ErrInvalidSeatCount
	}

	expectedStatus := t.Status
	expectedAvailable := t.AvailableSeats

	t.AvailableSeats += seats
	if t.Status == TripStatusFull && t.AvailableSeats > 0 {
		t.Status = TripStatusPublished
	}
	t.UpdatedAt = now.UTC()

	return SeatTransition{
		TripID:                 t.ID,
		SeatsDelta:             seats,
		ExpectedStatus:         expectedStatus,
		ExpectedAvailableSeats: expectedAvailable,
		NewStatus:              t.Status,
		NewAvailableSeats:      t.AvailableSeats,
	}, nil
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
	if !newStatus.IsValid() {
		return ErrInvalidTripStatusChange
	}
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
