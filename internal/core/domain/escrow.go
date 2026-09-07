package domain

import (
	"math"
	"strings"
	"time"
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

// RefundType clasifica la razón de reembolso según las políticas de la plataforma
type RefundType string

const (
	RefundTypeDriverCancellation RefundType = "DRIVER_CANCELLATION"
	RefundTypePassengerEarly     RefundType = "PASSENGER_EARLY"
	RefundTypePassengerLate      RefundType = "PASSENGER_LATE"
	RefundTypeAdminDispute       RefundType = "ADMIN_DISPUTE"
)

// EscrowTransaction representa la transacción de fondos retenidos en depósito en garantía
type EscrowTransaction struct {
	ID                string       `json:"id"`
	BookingID         string       `json:"booking_id"`
	TripID            string       `json:"trip_id"`
	PayerID           string       `json:"payer_id"` // Pasajero
	PayeeID           string       `json:"payee_id"` // Conductor
	Amount            float64      `json:"amount"`
	Currency          string       `json:"currency"`
	PaymentGatewayRef string       `json:"payment_gateway_ref"`
	Status            EscrowStatus `json:"status"`
	HeldAt            time.Time    `json:"held_at"`
	ReleasedAt        *time.Time   `json:"released_at,omitempty"`
	RefundedAt        *time.Time   `json:"refunded_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// NewEscrowTransaction crea un registro de retención de fondos en custodia
func NewEscrowTransaction(id, bookingID, tripID, payerID, payeeID string, amount float64, currency, gatewayRef string) (*EscrowTransaction, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(bookingID) == "" || strings.TrimSpace(tripID) == "" {
		return nil, ErrEscrowNotFound
	}
	if strings.TrimSpace(payerID) == "" || strings.TrimSpace(payeeID) == "" {
		return nil, ErrUnauthorized
	}
	if amount <= 0 {
		return nil, ErrPriceExceedsCapPrice
	}
	if strings.TrimSpace(currency) == "" {
		currency = "USD"
	}
	if strings.TrimSpace(gatewayRef) == "" {
		return nil, ErrPaymentFailed
	}

	nowUTC := time.Now().UTC()
	return &EscrowTransaction{
		ID:                id,
		BookingID:         bookingID,
		TripID:            tripID,
		PayerID:           payerID,
		PayeeID:           payeeID,
		Amount:            math.Round(amount*100) / 100,
		Currency:          strings.ToUpper(currency),
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

// RefundTransaction documenta la trazabilidad y distribución contable de los reembolsos
type RefundTransaction struct {
	ID                       string     `json:"id"`
	EscrowTransactionID      string     `json:"escrow_transaction_id"`
	BookingID                string     `json:"booking_id"`
	PassengerRefundAmount    float64    `json:"passenger_refund_amount"`
	DriverCompensationAmount float64    `json:"driver_compensation_amount"`
	RefundType               RefundType `json:"refund_type"`
	GatewayRefundRef         string     `json:"gateway_refund_ref,omitempty"`
	ProcessedAt              time.Time  `json:"processed_at"`
	CreatedAt                time.Time  `json:"created_at"`
}

// CalculateCancellationRefund computa la política de reembolsos de gastos compartidos
// - Si cancela el conductor: 100% pasajero, 0% conductor (DRIVER_CANCELLATION)
// - Si cancela el pasajero con > 24hs de antelación: 100% pasajero, 0% conductor (PASSENGER_EARLY)
// - Si cancela el pasajero con < 24hs de antelación: 50% pasajero, 50% compensación conductor (PASSENGER_LATE)
func CalculateCancellationRefund(escrowAmount float64, departureTime, cancellationTime time.Time, isDriverCancellation bool) (passengerRefund float64, driverCompensation float64, refundType RefundType) {
	roundedTotal := math.Round(escrowAmount*100) / 100

	if isDriverCancellation {
		return roundedTotal, 0.0, RefundTypeDriverCancellation
	}

	// Tiempo restante hasta la partida
	timeUntilDeparture := departureTime.Sub(cancellationTime)

	if timeUntilDeparture >= 24*time.Hour {
		return roundedTotal, 0.0, RefundTypePassengerEarly
	}

	// Cancelación tardía con menos de 24 horas: 50% pasajero / 50% conductor
	passengerHalf := math.Round(roundedTotal*0.5*100) / 100
	driverHalf := math.Round((roundedTotal-passengerHalf)*100) / 100

	return passengerHalf, driverHalf, RefundTypePassengerLate
}

// NewRefundTransaction genera el registro inmutable del reembolso procesado
func NewRefundTransaction(id, escrowID, bookingID string, passengerRefund, driverCompensation float64, refundType RefundType, gatewayRefundRef string) (*RefundTransaction, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(escrowID) == "" || strings.TrimSpace(bookingID) == "" {
		return nil, ErrEscrowNotFound
	}
	if passengerRefund < 0 || driverCompensation < 0 || (passengerRefund+driverCompensation <= 0) {
		return nil, ErrInvalidRefundAmount
	}

	nowUTC := time.Now().UTC()
	return &RefundTransaction{
		ID:                       id,
		EscrowTransactionID:      escrowID,
		BookingID:                bookingID,
		PassengerRefundAmount:    math.Round(passengerRefund*100) / 100,
		DriverCompensationAmount: math.Round(driverCompensation*100) / 100,
		RefundType:               refundType,
		GatewayRefundRef:         gatewayRefundRef,
		ProcessedAt:              nowUTC,
		CreatedAt:                nowUTC,
	}, nil
}
