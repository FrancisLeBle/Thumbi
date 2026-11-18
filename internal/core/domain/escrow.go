package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowStatus define los estados de custodia y liquidación de fondos
type EscrowStatus string

const (
	EscrowStatusHeld            EscrowStatus = "HELD"             // Fondos retenidos en garantía
	EscrowStatusReleased        EscrowStatus = "RELEASED"         // Fondos liquidados al conductor
	EscrowStatusRefundedFull    EscrowStatus = "REFUNDED_FULL"    // 100% reembolsado al pasajero
	EscrowStatusRefundedPartial EscrowStatus = "REFUNDED_PARTIAL" // Reembolso dividido (penalización tardía)
	EscrowStatusDisputed        EscrowStatus = "DISPUTED"         // Fondos congelados por reclamo abierto
)

// IsValid verifica que el estado del escrow pertenezca a los valores válidos del dominio.
func (s EscrowStatus) IsValid() bool {
	switch s {
	case EscrowStatusHeld,
		EscrowStatusReleased,
		EscrowStatusRefundedFull,
		EscrowStatusRefundedPartial,
		EscrowStatusDisputed:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del estado del escrow.
func (s EscrowStatus) String() string {
	return string(s)
}

// RefundType clasifica la razón de reembolso según las políticas de la plataforma
type RefundType string

const (
	RefundTypeDriverCancellation RefundType = "DRIVER_CANCELLATION"
	RefundTypePassengerEarly     RefundType = "PASSENGER_EARLY"
	RefundTypePassengerLate      RefundType = "PASSENGER_LATE"
	RefundTypeAdminDispute       RefundType = "ADMIN_DISPUTE"
)

// IsValid verifica que el tipo de reembolso pertenezca al conjunto permitido.
func (r RefundType) IsValid() bool {
	switch r {
	case RefundTypeDriverCancellation,
		RefundTypePassengerEarly,
		RefundTypePassengerLate,
		RefundTypeAdminDispute:
		return true
	default:
		return false
	}
}

// String retorna la representación textual del tipo de reembolso.
func (r RefundType) String() string {
	return string(r)
}

// EscrowTransaction representa la transacción de fondos retenidos en depósito en garantía
type EscrowTransaction struct {
	ID                uuid.UUID    `json:"id"`
	BookingID         uuid.UUID    `json:"booking_id"`
	TripID            uuid.UUID    `json:"trip_id"`
	PayerID           uuid.UUID    `json:"payer_id"` // Pasajero
	PayeeID           uuid.UUID    `json:"payee_id"` // Conductor
	Amount            Money        `json:"amount"`
	Currency          Currency     `json:"currency"`
	PaymentGatewayRef string       `json:"payment_gateway_ref"`
	Status            EscrowStatus `json:"status"`
	HeldAt            time.Time    `json:"held_at"`
	ReleasedAt        *time.Time   `json:"released_at,omitempty"`
	RefundedAt        *time.Time   `json:"refunded_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// NewEscrowTransaction crea un registro de retención de fondos en custodia exigiendo moneda explícita válida dentro de Money.
func NewEscrowTransaction(id, bookingID, tripID, payerID, payeeID uuid.UUID, amount Money, gatewayRef string) (*EscrowTransaction, error) {
	if id == uuid.Nil || bookingID == uuid.Nil || tripID == uuid.Nil {
		return nil, ErrEscrowNotFound
	}
	if payerID == uuid.Nil || payeeID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if amount.Amount <= 0 {
		return nil, ErrPriceExceedsCapPrice
	}
	cur, err := ParseCurrency(string(amount.Currency))
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(gatewayRef) == "" {
		return nil, ErrPaymentFailed
	}

	normalizedAmount := MoneyFromCents(amount.Amount, cur)
	nowUTC := time.Now().UTC()
	return &EscrowTransaction{
		ID:                id,
		BookingID:         bookingID,
		TripID:            tripID,
		PayerID:           payerID,
		PayeeID:           payeeID,
		Amount:            normalizedAmount,
		Currency:          cur,
		PaymentGatewayRef: gatewayRef,
		Status:            EscrowStatusHeld,
		HeldAt:            nowUTC,
		CreatedAt:         nowUTC,
		UpdatedAt:         nowUTC,
	}, nil
}

// Release liquida los fondos en custodia hacia el saldo del conductor al completar el viaje
func (e *EscrowTransaction) Release(now time.Time) error {
	if e.Status != EscrowStatusHeld && e.Status != EscrowStatusDisputed {
		return ErrEscrowAlreadySettled
	}

	nowUTC := now.UTC()
	e.Status = EscrowStatusReleased
	e.ReleasedAt = &nowUTC
	e.UpdatedAt = nowUTC
	return nil
}

// RefundFull transiciona la custodia a reembolso total a favor del pasajero
func (e *EscrowTransaction) RefundFull(now time.Time) error {
	if e.Status != EscrowStatusHeld && e.Status != EscrowStatusDisputed {
		return ErrEscrowAlreadySettled
	}

	nowUTC := now.UTC()
	e.Status = EscrowStatusRefundedFull
	e.RefundedAt = &nowUTC
	e.UpdatedAt = nowUTC
	return nil
}

// RefundPartial transiciona la custodia a reembolso parcial según política de cancelación
func (e *EscrowTransaction) RefundPartial(now time.Time) error {
	if e.Status != EscrowStatusHeld && e.Status != EscrowStatusDisputed {
		return ErrEscrowAlreadySettled
	}

	nowUTC := now.UTC()
	e.Status = EscrowStatusRefundedPartial
	e.RefundedAt = &nowUTC
	e.UpdatedAt = nowUTC
	return nil
}

// OpenDispute congela la liquidación ante reclamo fundamentado del pasajero
func (e *EscrowTransaction) OpenDispute(now time.Time) error {
	if e.Status != EscrowStatusHeld {
		return ErrEscrowAlreadySettled
	}

	nowUTC := now.UTC()
	e.Status = EscrowStatusDisputed
	e.UpdatedAt = nowUTC
	return nil
}

// RestoreHeld restaura el estado HELD cuando una disputa es desestimada sin mérito
func (e *EscrowTransaction) RestoreHeld(now time.Time) error {
	if e.Status != EscrowStatusDisputed && e.Status != EscrowStatusHeld {
		return ErrEscrowAlreadySettled
	}

	nowUTC := now.UTC()
	e.Status = EscrowStatusHeld
	e.UpdatedAt = nowUTC
	return nil
}

// RefundTransaction documenta la trazabilidad y distribución contable de los reembolsos
type RefundTransaction struct {
	ID                       uuid.UUID  `json:"id"`
	EscrowTransactionID      uuid.UUID  `json:"escrow_transaction_id"`
	BookingID                uuid.UUID  `json:"booking_id"`
	PassengerRefundAmount    Money      `json:"passenger_refund_amount"`
	DriverCompensationAmount Money      `json:"driver_compensation_amount"`
	RefundType               RefundType `json:"refund_type"`
	GatewayRefundRef         string     `json:"gateway_refund_ref,omitempty"`
	ProcessedAt              time.Time  `json:"processed_at"`
	CreatedAt                time.Time  `json:"created_at"`
}

// CalculateCancellationRefund computa la política de reembolsos de gastos compartidos en centavos exactos:
// - Si cancela el conductor: 100% pasajero, 0% conductor (DRIVER_CANCELLATION)
// - Si cancela el pasajero con >= 24hs de antelación: 100% pasajero, 0% conductor (PASSENGER_EARLY)
// - Si cancela el pasajero con < 24hs de antelación: 50% pasajero, 50% compensación conductor (PASSENGER_LATE)
func CalculateCancellationRefund(escrowAmount Money, departureTime, cancellationTime time.Time, isDriverCancellation bool) (passengerRefund Money, driverCompensation Money, refundType RefundType) {
	cur := escrowAmount.EffectiveCurrency()
	normalizedAmount := MoneyFromCents(escrowAmount.Amount, cur)
	if isDriverCancellation {
		return normalizedAmount, ZeroMoney(cur), RefundTypeDriverCancellation
	}

	// Tiempo restante hasta la partida
	timeUntilDeparture := departureTime.Sub(cancellationTime)

	if timeUntilDeparture >= 24*time.Hour {
		return normalizedAmount, ZeroMoney(cur), RefundTypePassengerEarly
	}

	// Cancelación tardía con menos de 24 horas: división entera conservando exactamente el total y la moneda
	passengerHalf, driverHalf := normalizedAmount.SplitHalf()
	return passengerHalf, driverHalf, RefundTypePassengerLate
}

// NewRefundTransaction genera el registro inmutable del reembolso procesado
func NewRefundTransaction(id, escrowID, bookingID uuid.UUID, passengerRefund, driverCompensation Money, refundType RefundType, gatewayRefundRef string) (*RefundTransaction, error) {
	if id == uuid.Nil || escrowID == uuid.Nil || bookingID == uuid.Nil {
		return nil, ErrEscrowNotFound
	}
	if !refundType.IsValid() {
		return nil, ErrInvalidStatus
	}
	pCur := passengerRefund.EffectiveCurrency()
	dCur := driverCompensation.EffectiveCurrency()
	if pCur != dCur {
		return nil, fmt.Errorf("%w: reembolso en %s vs compensación en %s", ErrCurrencyMismatch, pCur, dCur)
	}
	if passengerRefund.Amount < 0 || driverCompensation.Amount < 0 || (passengerRefund.Amount+driverCompensation.Amount <= 0) {
		return nil, ErrInvalidRefundAmount
	}

	nowUTC := time.Now().UTC()
	return &RefundTransaction{
		ID:                       id,
		EscrowTransactionID:      escrowID,
		BookingID:                bookingID,
		PassengerRefundAmount:    MoneyFromCents(passengerRefund.Amount, pCur),
		DriverCompensationAmount: MoneyFromCents(driverCompensation.Amount, dCur),
		RefundType:               refundType,
		GatewayRefundRef:         gatewayRefundRef,
		ProcessedAt:              nowUTC,
		CreatedAt:                nowUTC,
	}, nil
}
