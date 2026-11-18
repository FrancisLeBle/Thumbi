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
	BaseAmount        Money        `json:"base_amount"`
	PlatformFee       Money        `json:"platform_fee"`
	Currency          Currency     `json:"currency"`
	PaymentGatewayRef string       `json:"payment_gateway_ref"`
	Status            EscrowStatus `json:"status"`
	HeldAt            time.Time    `json:"held_at"`
	ReleasedAt        *time.Time   `json:"released_at,omitempty"`
	RefundedAt        *time.Time   `json:"refunded_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// DriverPayoutAmount retorna el monto neto correspondiente al conductor (BaseAmount si existe desglose, o Amount en registros legados).
func (e *EscrowTransaction) DriverPayoutAmount() Money {
	if e == nil {
		return ZeroMoney()
	}
	if e.BaseAmount.Amount > 0 || e.PlatformFee.Amount > 0 {
		return MoneyFromCents(e.BaseAmount.Amount, e.Currency)
	}
	return MoneyFromCents(e.Amount.Amount, e.Currency)
}

// NewEscrowTransaction crea un registro de retención de fondos en custodia exigiendo moneda explícita válida dentro de Money.
func NewEscrowTransaction(id, bookingID, tripID, payerID, payeeID uuid.UUID, amount Money, gatewayRef string) (*EscrowTransaction, error) {
	cur := amount.EffectiveCurrency()
	return NewEscrowTransactionWithBreakdown(id, bookingID, tripID, payerID, payeeID, amount, ZeroMoney(cur), gatewayRef)
}

// NewEscrowTransactionWithBreakdown crea un registro de custodia separando el monto base del conductor y la comisión de plataforma.
func NewEscrowTransactionWithBreakdown(id, bookingID, tripID, payerID, payeeID uuid.UUID, baseAmount, platformFee Money, gatewayRef string) (*EscrowTransaction, error) {
	if id == uuid.Nil || bookingID == uuid.Nil || tripID == uuid.Nil {
		return nil, ErrEscrowNotFound
	}
	if payerID == uuid.Nil || payeeID == uuid.Nil {
		return nil, ErrUnauthorized
	}
	if baseAmount.Amount < 0 || platformFee.Amount < 0 || (baseAmount.Amount+platformFee.Amount <= 0) {
		return nil, ErrPriceExceedsCapPrice
	}

	rawCur := string(baseAmount.Currency)
	if strings.TrimSpace(rawCur) == "" {
		rawCur = string(platformFee.Currency)
	}
	cur, err := ParseCurrency(rawCur)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(platformFee.Currency)) != "" {
		feeCur, err := ParseCurrency(string(platformFee.Currency))
		if err != nil {
			return nil, err
		}
		if feeCur != cur {
			return nil, fmt.Errorf("%w: base en %s vs fee en %s", ErrCurrencyMismatch, cur, feeCur)
		}
	}
	if strings.TrimSpace(gatewayRef) == "" {
		return nil, ErrPaymentFailed
	}

	normalizedBase := MoneyFromCents(baseAmount.Amount, cur)
	normalizedFee := MoneyFromCents(platformFee.Amount, cur)
	normalizedAmount := MoneyFromCents(normalizedBase.Amount+normalizedFee.Amount, cur)
	nowUTC := time.Now().UTC()
	return &EscrowTransaction{
		ID:                id,
		BookingID:         bookingID,
		TripID:            tripID,
		PayerID:           payerID,
		PayeeID:           payeeID,
		Amount:            normalizedAmount,
		BaseAmount:        normalizedBase,
		PlatformFee:       normalizedFee,
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

// RefundTransaction documenta la trazabilidad y distribución contable de los reembolsos en 3 vías
type RefundTransaction struct {
	ID                       uuid.UUID  `json:"id"`
	EscrowTransactionID      uuid.UUID  `json:"escrow_transaction_id"`
	BookingID                uuid.UUID  `json:"booking_id"`
	PassengerRefundAmount    Money      `json:"passenger_refund_amount"`
	DriverCompensationAmount Money      `json:"driver_compensation_amount"`
	PlatformRetainedAmount   Money      `json:"platform_retained_amount"`
	RefundType               RefundType `json:"refund_type"`
	GatewayRefundRef         string     `json:"gateway_refund_ref,omitempty"`
	ProcessedAt              time.Time  `json:"processed_at"`
	CreatedAt                time.Time  `json:"created_at"`
}

// CancellationRefundBreakdown detalla la distribución contable exacta de un reembolso por cancelación.
type CancellationRefundBreakdown struct {
	PassengerRefund    Money
	DriverCompensation Money
	PlatformRetained   Money
	RefundType         RefundType
}

// CalculateCancellationRefund computa la política estándar sobre un monto único (compatibilidad con llamadas sin desglose).
func CalculateCancellationRefund(escrowAmount Money, departureTime, cancellationTime time.Time, isDriverCancellation bool) (passengerRefund Money, driverCompensation Money, refundType RefundType) {
	cur := escrowAmount.EffectiveCurrency()
	breakdown := CalculateCancellationRefundWithPolicy(
		escrowAmount,
		ZeroMoney(cur),
		departureTime,
		cancellationTime,
		isDriverCancellation,
		DefaultPlatformPricingConfig(),
	)
	return breakdown.PassengerRefund, breakdown.DriverCompensation, breakdown.RefundType
}

// CalculateCancellationRefundWithPolicy computa en centavos exactos la política parametrizada de reembolso y retención operativa:
// - Si cancela el conductor: 100% pasajero (base + fee), 0% conductor, 0% plataforma.
// - Si cancela el pasajero tempranamente (>= LateCancelWindowHours): 100% del base al pasajero + retención parametrizada del PlatformFee.
// - Si cancela el pasajero tardíamente (< LateCancelWindowHours): reparto 3-vías del base (sumando 100%) + retención parametrizada del PlatformFee.
func CalculateCancellationRefundWithPolicy(
	baseAmount, platformFee Money,
	departureTime, cancellationTime time.Time,
	isDriverCancellation bool,
	policy PlatformPricingConfig,
) CancellationRefundBreakdown {
	cur := baseAmount.EffectiveCurrency()
	if strings.TrimSpace(string(baseAmount.Currency)) == "" && strings.TrimSpace(string(platformFee.Currency)) != "" {
		cur = platformFee.EffectiveCurrency()
	}

	baseCents := baseAmount.Amount
	if baseCents < 0 {
		baseCents = 0
	}
	feeCents := platformFee.Amount
	if feeCents < 0 {
		feeCents = 0
	}

	if err := policy.Validate(); err != nil {
		policy = DefaultPlatformPricingConfig()
	}

	if isDriverCancellation {
		return CancellationRefundBreakdown{
			PassengerRefund:    MoneyFromCents(baseCents+feeCents, cur),
			DriverCompensation: ZeroMoney(cur),
			PlatformRetained:   ZeroMoney(cur),
			RefundType:         RefundTypeDriverCancellation,
		}
	}

	lateWindow := time.Duration(policy.LateCancelWindowHours) * time.Hour
	timeUntilDeparture := departureTime.Sub(cancellationTime)

	if timeUntilDeparture >= lateWindow {
		retainedFee := clampInt64(RoundDivHalfUp(feeCents*policy.EarlyCancelFeeRetentionBps, BpsDenominator), 0, feeCents)
		refundedFee := feeCents - retainedFee
		return CancellationRefundBreakdown{
			PassengerRefund:    MoneyFromCents(baseCents+refundedFee, cur),
			DriverCompensation: ZeroMoney(cur),
			PlatformRetained:   MoneyFromCents(retainedFee, cur),
			RefundType:         RefundTypePassengerEarly,
		}
	}

	retainedFee := clampInt64(RoundDivHalfUp(feeCents*policy.LateCancelFeeRetentionBps, BpsDenominator), 0, feeCents)
	refundedFee := feeCents - retainedFee

	basePassenger, baseDriver, basePlatform := splitAmountByThreeBps(
		baseCents,
		policy.LateCancelPassengerRefundBps,
		policy.LateCancelDriverCompBps,
		policy.LateCancelPlatformRetentionBps,
	)

	return CancellationRefundBreakdown{
		PassengerRefund:    MoneyFromCents(basePassenger+refundedFee, cur),
		DriverCompensation: MoneyFromCents(baseDriver, cur),
		PlatformRetained:   MoneyFromCents(retainedFee+basePlatform, cur),
		RefundType:         RefundTypePassengerLate,
	}
}

func splitAmountByThreeBps(total, pBps, dBps, rBps int64) (p, d, r int64) {
	if total <= 0 {
		return 0, 0, 0
	}
	p = RoundDivHalfUp(total*pBps, BpsDenominator)
	d = RoundDivHalfUp(total*dBps, BpsDenominator)
	r = RoundDivHalfUp(total*rBps, BpsDenominator)

	diff := total - (p + d + r)
	for diff > 0 {
		switch {
		case pBps > 0 && pBps >= dBps && pBps >= rBps:
			p++
		case dBps > 0 && dBps >= rBps:
			d++
		case rBps > 0:
			r++
		default:
			p++
		}
		diff--
	}
	for diff < 0 {
		switch {
		case r > 0:
			r--
		case d > 0:
			d--
		case p > 0:
			p--
		}
		diff++
	}
	return p, d, r
}

func clampInt64(v, minVal, maxVal int64) int64 {
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// NewRefundTransaction genera el registro inmutable del reembolso procesado (sin retención adicional de plataforma).
func NewRefundTransaction(id, escrowID, bookingID uuid.UUID, passengerRefund, driverCompensation Money, refundType RefundType, gatewayRefundRef string) (*RefundTransaction, error) {
	cur := passengerRefund.EffectiveCurrency()
	return NewRefundTransactionWithRetention(id, escrowID, bookingID, passengerRefund, driverCompensation, ZeroMoney(cur), refundType, gatewayRefundRef)
}

// NewRefundTransactionWithRetention genera el registro inmutable del reembolso incluyendo el monto retenido por la plataforma.
func NewRefundTransactionWithRetention(id, escrowID, bookingID uuid.UUID, passengerRefund, driverCompensation, platformRetained Money, refundType RefundType, gatewayRefundRef string) (*RefundTransaction, error) {
	if id == uuid.Nil || escrowID == uuid.Nil || bookingID == uuid.Nil {
		return nil, ErrEscrowNotFound
	}
	if !refundType.IsValid() {
		return nil, ErrInvalidStatus
	}
	pCur := passengerRefund.EffectiveCurrency()
	dCur := driverCompensation.EffectiveCurrency()
	rCur := platformRetained.EffectiveCurrency()
	if pCur != dCur || pCur != rCur {
		return nil, fmt.Errorf("%w: monedas inconsistentes en reembolso (%s / %s / %s)", ErrCurrencyMismatch, pCur, dCur, rCur)
	}
	if passengerRefund.Amount < 0 || driverCompensation.Amount < 0 || platformRetained.Amount < 0 ||
		(passengerRefund.Amount+driverCompensation.Amount+platformRetained.Amount <= 0) {
		return nil, ErrInvalidRefundAmount
	}

	nowUTC := time.Now().UTC()
	return &RefundTransaction{
		ID:                       id,
		EscrowTransactionID:      escrowID,
		BookingID:                bookingID,
		PassengerRefundAmount:    MoneyFromCents(passengerRefund.Amount, pCur),
		DriverCompensationAmount: MoneyFromCents(driverCompensation.Amount, dCur),
		PlatformRetainedAmount:   MoneyFromCents(platformRetained.Amount, rCur),
		RefundType:               refundType,
		GatewayRefundRef:         gatewayRefundRef,
		ProcessedAt:              nowUTC,
		CreatedAt:                nowUTC,
	}, nil
}
