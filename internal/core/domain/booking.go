package domain

import (
	"math"
	"strings"
	"time"
)

// BookingStatus enumera los estados posibles en el ciclo de vida de una reserva
type BookingStatus string

const (
	BookingStatusPendingPayment        BookingStatus = "PENDING_PAYMENT"
	BookingStatusConfirmed             BookingStatus = "CONFIRMED"
	BookingStatusRejected              BookingStatus = "REJECTED"
	BookingStatusCancelledByPassenger  BookingStatus = "CANCELLED_BY_PASSENGER"
	BookingStatusCancelledByDriver     BookingStatus = "CANCELLED_BY_DRIVER"
	BookingStatusExpired               BookingStatus = "EXPIRED"
	BookingStatusCompleted             BookingStatus = "COMPLETED"
)

// DefaultBookingTTL tiempo límite de retención de asientos sin pago confirmado (15 minutos)
const DefaultBookingTTL = 15 * time.Minute

// BookingParams DTO para la instanciación de una nueva reserva en el dominio
type BookingParams struct {
	ID            string
	TripID        string
	PassengerID   string
	DriverID      string
	SeatsBooked   int
	UnitPrice     float64
	PickupStopID  *string
	DropoffStopID *string
	TTL           time.Duration
}

// Booking representa la reserva de asientos en un viaje publicado
type Booking struct {
	ID                 string        `json:"id"`
	TripID             string        `json:"trip_id"`
	PassengerID        string        `json:"passenger_id"`
	SeatsBooked        int           `json:"seats_booked"`
	UnitPrice          float64       `json:"unit_price"`
	TotalPrice         float64       `json:"total_price"`
	Status             BookingStatus `json:"status"`
	PickupStopID       *string       `json:"pickup_stop_id,omitempty"`
	DropoffStopID      *string       `json:"dropoff_stop_id,omitempty"`
	ExpiresAt          time.Time     `json:"expires_at"`
	ConfirmedAt        *time.Time    `json:"confirmed_at,omitempty"`
	CancelledAt        *time.Time    `json:"cancelled_at,omitempty"`
	CancellationReason *string       `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// NewBooking crea y valida una nueva reserva de asientos verificando las invariantes de negocio
func NewBooking(params BookingParams) (*Booking, error) {
	if strings.TrimSpace(params.ID) == "" || strings.TrimSpace(params.TripID) == "" || strings.TrimSpace(params.PassengerID) == "" {
		return nil, ErrBookingNotFound
	}

	// Invariante RF-02: El pasajero no puede ser el mismo conductor del viaje
	if params.PassengerID == params.DriverID {
		return nil, ErrCannotBookOwnTrip
	}

	// Invariante: Plazas solicitadas deben ser mayores a 0
	if params.SeatsBooked <= 0 {
		return nil, ErrInvalidSeatCount
	}

	// Invariante: Precio unitario no puede ser negativo
	if params.UnitPrice < 0 {
		return nil, ErrPriceExceedsCapPrice
	}

	ttl := params.TTL
	if ttl <= 0 {
		ttl = DefaultBookingTTL
	}

	nowUTC := time.Now().UTC()
	totalPrice := math.Round(params.UnitPrice*float64(params.SeatsBooked)*100) / 100

	return &Booking{
		ID:            params.ID,
		TripID:        params.TripID,
		PassengerID:   params.PassengerID,
		SeatsBooked:   params.SeatsBooked,
		UnitPrice:     params.UnitPrice,
		TotalPrice:    totalPrice,
		Status:        BookingStatusPendingPayment,
		PickupStopID:  params.PickupStopID,
		DropoffStopID: params.DropoffStopID,
		ExpiresAt:     nowUTC.Add(ttl),
		CreatedAt:     nowUTC,
		UpdatedAt:     nowUTC,
	}, nil
}

// IsExpired evalúa si la reserva ha superado el tiempo de retención de asientos sin pago
func (b *Booking) IsExpired(now time.Time) bool {
	return b.Status == BookingStatusPendingPayment && now.UTC().After(b.ExpiresAt)
}

// CanBeConfirmed determina si la reserva es elegible para ser confirmada con un pago
func (b *Booking) CanBeConfirmed(now time.Time) bool {
	return b.Status == BookingStatusPendingPayment && !b.IsExpired(now)
}

// Confirm transiciona la reserva a CONFIRMED tras la captura exitosa del pago en Escrow
func (b *Booking) Confirm(now time.Time) error {
	if b.Status != BookingStatusPendingPayment {
		return ErrInvalidBookingStatus
	}
	if b.IsExpired(now) {
		return ErrBookingExpired
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusConfirmed
	b.ConfirmedAt = &nowUTC
	b.UpdatedAt = nowUTC
	return nil
}

// Expire transiciona la reserva a EXPIRED para liberar los asientos retenidos
func (b *Booking) Expire(now time.Time) error {
	if b.Status != BookingStatusPendingPayment {
		return ErrInvalidBookingStatus
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusExpired
	b.UpdatedAt = nowUTC
	return nil
}

// CancelByPassenger procesa la cancelación solicitada por el pasajero
func (b *Booking) CancelByPassenger(reason string, now time.Time) error {
	if b.Status != BookingStatusPendingPayment && b.Status != BookingStatusConfirmed {
		return ErrBookingCannotBeCancelled
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusCancelledByPassenger
	b.CancelledAt = &nowUTC
	if strings.TrimSpace(reason) != "" {
		trimmed := strings.TrimSpace(reason)
		b.CancellationReason = &trimmed
	}
	b.UpdatedAt = nowUTC
	return nil
}

// CancelByDriver procesa la cancelación provocada por la baja del viaje por parte del conductor
func (b *Booking) CancelByDriver(reason string, now time.Time) error {
	if b.Status != BookingStatusPendingPayment && b.Status != BookingStatusConfirmed {
		return ErrBookingCannotBeCancelled
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusCancelledByDriver
	b.CancelledAt = &nowUTC
	if strings.TrimSpace(reason) != "" {
		trimmed := strings.TrimSpace(reason)
		b.CancellationReason = &trimmed
	}
	b.UpdatedAt = nowUTC
	return nil
}

// MarkCompleted finaliza la reserva una vez concluido el viaje exitosamente
func (b *Booking) MarkCompleted(now time.Time) error {
	if b.Status != BookingStatusConfirmed {
		return ErrInvalidBookingStatus
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusCompleted
	b.UpdatedAt = nowUTC
	return nil
}

// Reject marca la reserva como rechazada
func (b *Booking) Reject(reason string, now time.Time) error {
	if b.Status != BookingStatusPendingPayment {
		return ErrInvalidBookingStatus
	}

	nowUTC := now.UTC()
	b.Status = BookingStatusRejected
	if strings.TrimSpace(reason) != "" {
		trimmed := strings.TrimSpace(reason)
		b.CancellationReason = &trimmed
	}
	b.UpdatedAt = nowUTC
	return nil
}

// IsActive indica si la reserva ocupa plazas activas (en pago o confirmada)
func (b *Booking) IsActive() bool {
	return b.Status == BookingStatusPendingPayment || b.Status == BookingStatusConfirmed
}
