package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// BookingStatus enumera los estados posibles en el ciclo de vida de una reserva
type BookingStatus string

const (
	BookingStatusPendingPayment       BookingStatus = "PENDING_PAYMENT"
	BookingStatusConfirmed            BookingStatus = "CONFIRMED"
	BookingStatusRejected             BookingStatus = "REJECTED"
	BookingStatusCancelledByPassenger BookingStatus = "CANCELLED_BY_PASSENGER"
	BookingStatusCancelledByDriver    BookingStatus = "CANCELLED_BY_DRIVER"
	BookingStatusExpired              BookingStatus = "EXPIRED"
	BookingStatusCompleted            BookingStatus = "COMPLETED"
)

// IsValid verifica que el estado de la reserva pertenezca a los valores permitidos del dominio.
func (s BookingStatus) IsValid() bool {
	switch s {
	case BookingStatusPendingPayment,
		BookingStatusConfirmed,
		BookingStatusRejected,
		BookingStatusCancelledByPassenger,
		BookingStatusCancelledByDriver,
		BookingStatusExpired,
		BookingStatusCompleted:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del estado de la reserva.
func (s BookingStatus) String() string {
	return string(s)
}

// DefaultBookingTTL tiempo límite de retención de asientos sin pago confirmado (15 minutos)
const DefaultBookingTTL = 15 * time.Minute

// MaxPendingBookingsPerPassenger tope máximo de reservas en PENDING_PAYMENT activas por pasajero
const MaxPendingBookingsPerPassenger = 3

// BookingParams DTO para la instanciación de una nueva reserva en el dominio
type BookingParams struct {
	ID              uuid.UUID
	TripID          uuid.UUID
	PassengerID     uuid.UUID
	DriverID        uuid.UUID
	SeatsBooked     int
	Currency        Currency
	UnitPrice       Money
	PickupStopID    *uuid.UUID
	DropoffStopID   *uuid.UUID
	PaymentIntentID *string
	TTL             time.Duration
}

// Booking representa la reserva de asientos en un viaje publicado
type Booking struct {
	ID                 uuid.UUID     `json:"id"`
	TripID             uuid.UUID     `json:"trip_id"`
	PassengerID        uuid.UUID     `json:"passenger_id"`
	SeatsBooked        int           `json:"seats_booked"`
	Currency           Currency      `json:"currency"`
	UnitPrice          Money         `json:"unit_price"`
	TotalPrice         Money         `json:"total_price"`
	Status             BookingStatus `json:"status"`
	PickupStopID       *uuid.UUID    `json:"pickup_stop_id,omitempty"`
	DropoffStopID      *uuid.UUID    `json:"dropoff_stop_id,omitempty"`
	PaymentIntentID    *string       `json:"payment_intent_id,omitempty"`
	ExpiresAt          time.Time     `json:"expires_at"`
	ConfirmedAt        *time.Time    `json:"confirmed_at,omitempty"`
	CancelledAt        *time.Time    `json:"cancelled_at,omitempty"`
	CancellationReason *string       `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// NewBooking crea y valida una nueva reserva de asientos verificando las invariantes de negocio
func NewBooking(params BookingParams) (*Booking, error) {
	if params.ID == uuid.Nil || params.TripID == uuid.Nil || params.PassengerID == uuid.Nil {
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
	if params.UnitPrice.Amount < 0 {
		return nil, ErrPriceExceedsCapPrice
	}

	bookingCurrency := params.Currency
	if strings.TrimSpace(string(bookingCurrency)) == "" {
		bookingCurrency = params.UnitPrice.EffectiveCurrency()
	}
	parsedCurrency, err := ParseCurrency(string(bookingCurrency))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(params.UnitPrice.Currency)) != "" {
		unitCur, err := ParseCurrency(string(params.UnitPrice.Currency))
		if err != nil {
			return nil, err
		}
		if unitCur != parsedCurrency {
			return nil, fmt.Errorf("%w: precio unitario en %s incompatible con reserva en %s", ErrCurrencyMismatch, unitCur, parsedCurrency)
		}
	}

	unitPrice := MoneyFromCents(params.UnitPrice.Amount, parsedCurrency)
	totalPrice := unitPrice.Mul(int64(params.SeatsBooked))

	ttl := params.TTL
	if ttl <= 0 {
		ttl = DefaultBookingTTL
	}

	nowUTC := time.Now().UTC()

	return &Booking{
		ID:              params.ID,
		TripID:          params.TripID,
		PassengerID:     params.PassengerID,
		SeatsBooked:     params.SeatsBooked,
		Currency:        parsedCurrency,
		UnitPrice:       unitPrice,
		TotalPrice:      totalPrice,
		Status:          BookingStatusPendingPayment,
		PickupStopID:    params.PickupStopID,
		DropoffStopID:   params.DropoffStopID,
		PaymentIntentID: params.PaymentIntentID,
		ExpiresAt:       nowUTC.Add(ttl),
		CreatedAt:       nowUTC,
		UpdatedAt:       nowUTC,
	}, nil
}

// SetPaymentIntentID vincula el identificador de orden/intención de cobro de la pasarela
func (b *Booking) SetPaymentIntentID(intentID string) {
	trimmed := strings.TrimSpace(intentID)
	if trimmed != "" {
		b.PaymentIntentID = &trimmed
		b.UpdatedAt = time.Now().UTC()
	}
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

// CanExpire indica si una reserva pendiente venció su TTL en el instante now.
func (b *Booking) CanExpire(now time.Time) bool {
	return b.Status == BookingStatusPendingPayment && !now.UTC().Before(b.ExpiresAt)
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

// ExpireIfOverdue valida que la reserva esté pendiente y con TTL vencido antes de transicionarla a EXPIRED.
func (b *Booking) ExpireIfOverdue(now time.Time) error {
	if !b.CanExpire(now) {
		return ErrInvalidBookingStatus
	}
	return b.Expire(now)
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
