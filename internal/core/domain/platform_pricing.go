package domain

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// BpsDenominator representa el 100.00% en puntos básicos enteros (100 bps = 1.00%).
	BpsDenominator int64 = 10000

	// DefaultPassengerServiceFeeBps tarifa de servicio por defecto al pasajero (15.00% = 1500 bps).
	DefaultPassengerServiceFeeBps int64 = 1500
	// MaxPassengerServiceFeeBps tope máximo de seguridad permitido para el service fee al pasajero (50.00% = 5000 bps).
	MaxPassengerServiceFeeBps int64 = 5000

	// DefaultEarlyCancelFeeRetentionBps retención por defecto del service fee ante cancelación temprana del pasajero (100%).
	DefaultEarlyCancelFeeRetentionBps int64 = 10000
	// DefaultLateCancelWindowHours ventana por defecto previa a la salida considerada cancelación tardía (24 horas).
	DefaultLateCancelWindowHours = 24
	// MinLateCancelWindowHours ventana mínima configurable (1 hora).
	MinLateCancelWindowHours = 1
	// MaxLateCancelWindowHours ventana máxima configurable (7 días = 168 horas).
	MaxLateCancelWindowHours = 168

	// DefaultLateCancelFeeRetentionBps retención por defecto del service fee ante cancelación tardía del pasajero (100%).
	DefaultLateCancelFeeRetentionBps int64 = 10000
	// DefaultLateCancelPassengerRefundBps porcentaje del monto base reembolsado al pasajero en cancelación tardía (50.00%).
	DefaultLateCancelPassengerRefundBps int64 = 5000
	// DefaultLateCancelDriverCompBps porcentaje del monto base compensado al conductor en cancelación tardía (50.00%).
	DefaultLateCancelDriverCompBps int64 = 5000
	// DefaultLateCancelPlatformRetentionBps porcentaje adicional del monto base retenido por la plataforma (0.00%).
	DefaultLateCancelPlatformRetentionBps int64 = 0
)

// DefaultPlatformPricingConfigID identificador determinista de la configuración singleton sembrada en BD.
var DefaultPlatformPricingConfigID = uuid.MustParse("01960000-0000-7000-8000-000000000001")

// PlatformPricingConfig define las reglas económicas parametrizables por el Administrador para monetización
// (Passenger Service Fee) y retención operativa en cancelaciones y reembolsos.
type PlatformPricingConfig struct {
	ID                             uuid.UUID  `json:"id"`
	Currency                       Currency   `json:"currency"`
	PassengerServiceFeeBps         int64      `json:"passenger_service_fee_bps"`
	PassengerFixedFee              Money      `json:"passenger_fixed_fee"`
	EarlyCancelFeeRetentionBps     int64      `json:"early_cancel_fee_retention_bps"`
	LateCancelWindowHours          int        `json:"late_cancel_window_hours"`
	LateCancelFeeRetentionBps      int64      `json:"late_cancel_fee_retention_bps"`
	LateCancelPassengerRefundBps   int64      `json:"late_cancel_passenger_refund_bps"`
	LateCancelDriverCompBps        int64      `json:"late_cancel_driver_comp_bps"`
	LateCancelPlatformRetentionBps int64      `json:"late_cancel_platform_retention_bps"`
	IsActive                       bool       `json:"is_active"`
	UpdatedBy                      *uuid.UUID `json:"updated_by,omitempty"`
	CreatedAt                      time.Time  `json:"created_at"`
	UpdatedAt                      time.Time  `json:"updated_at"`
}

// DefaultPlatformPricingConfig retorna la configuración estándar de monetización (15% service fee al pasajero,
// retención del 100% del service fee en cancelaciones del pasajero, reparto 50/50 del monto base en cancelación tardía).
func DefaultPlatformPricingConfig() PlatformPricingConfig {
	now := time.Now().UTC()
	return PlatformPricingConfig{
		ID:                             DefaultPlatformPricingConfigID,
		Currency:                       DefaultCurrency,
		PassengerServiceFeeBps:         DefaultPassengerServiceFeeBps,
		PassengerFixedFee:              ZeroMoney(DefaultCurrency),
		EarlyCancelFeeRetentionBps:     DefaultEarlyCancelFeeRetentionBps,
		LateCancelWindowHours:          DefaultLateCancelWindowHours,
		LateCancelFeeRetentionBps:      DefaultLateCancelFeeRetentionBps,
		LateCancelPassengerRefundBps:   DefaultLateCancelPassengerRefundBps,
		LateCancelDriverCompBps:        DefaultLateCancelDriverCompBps,
		LateCancelPlatformRetentionBps: DefaultLateCancelPlatformRetentionBps,
		IsActive:                       true,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	}
}

// Validate verifica todas las invariantes matemáticas y regulatorias de la configuración de precios.
func (c *PlatformPricingConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("%w: configuración nula", ErrInvalidPricingConfig)
	}

	curStr := strings.TrimSpace(string(c.Currency))
	if curStr == "" {
		c.Currency = DefaultCurrency
	} else {
		parsedCur, err := ParseCurrency(curStr)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidPricingConfig, err)
		}
		c.Currency = parsedCur
	}

	if c.PassengerServiceFeeBps < 0 || c.PassengerServiceFeeBps > MaxPassengerServiceFeeBps {
		return fmt.Errorf("%w: passenger_service_fee_bps debe estar entre 0 y %d (0%% a 50%%)", ErrInvalidPricingConfig, MaxPassengerServiceFeeBps)
	}

	if c.PassengerFixedFee.Amount < 0 {
		return fmt.Errorf("%w: passenger_fixed_fee no puede ser negativo", ErrInvalidPricingConfig)
	}
	if strings.TrimSpace(string(c.PassengerFixedFee.Currency)) == "" {
		c.PassengerFixedFee.Currency = c.Currency
	} else {
		fixedCur, err := ParseCurrency(string(c.PassengerFixedFee.Currency))
		if err != nil {
			return fmt.Errorf("%w: moneda de tarifa fija inválida: %v", ErrInvalidPricingConfig, err)
		}
		if fixedCur != c.Currency {
			return fmt.Errorf("%w: moneda de tarifa fija (%s) debe coincidir con la moneda de configuración (%s)", ErrInvalidPricingConfig, fixedCur, c.Currency)
		}
		c.PassengerFixedFee.Currency = fixedCur
	}

	if c.EarlyCancelFeeRetentionBps < 0 || c.EarlyCancelFeeRetentionBps > BpsDenominator {
		return fmt.Errorf("%w: early_cancel_fee_retention_bps debe estar entre 0 y %d", ErrInvalidPricingConfig, BpsDenominator)
	}

	if c.LateCancelWindowHours < MinLateCancelWindowHours || c.LateCancelWindowHours > MaxLateCancelWindowHours {
		return fmt.Errorf("%w: late_cancel_window_hours debe estar entre %d y %d horas", ErrInvalidPricingConfig, MinLateCancelWindowHours, MaxLateCancelWindowHours)
	}

	if c.LateCancelFeeRetentionBps < 0 || c.LateCancelFeeRetentionBps > BpsDenominator {
		return fmt.Errorf("%w: late_cancel_fee_retention_bps debe estar entre 0 y %d", ErrInvalidPricingConfig, BpsDenominator)
	}

	if c.LateCancelPassengerRefundBps < 0 || c.LateCancelPassengerRefundBps > BpsDenominator ||
		c.LateCancelDriverCompBps < 0 || c.LateCancelDriverCompBps > BpsDenominator ||
		c.LateCancelPlatformRetentionBps < 0 || c.LateCancelPlatformRetentionBps > BpsDenominator {
		return fmt.Errorf("%w: los porcentajes de distribución por cancelación tardía deben estar entre 0 y %d bps", ErrInvalidPricingConfig, BpsDenominator)
	}

	splitSum := c.LateCancelPassengerRefundBps + c.LateCancelDriverCompBps + c.LateCancelPlatformRetentionBps
	if splitSum != BpsDenominator {
		return fmt.Errorf("%w: la suma de reembolso al pasajero (%d), compensación al conductor (%d) y retención de plataforma (%d) debe ser exactamente %d bps (100%%)",
			ErrInvalidPricingConfig,
			c.LateCancelPassengerRefundBps,
			c.LateCancelDriverCompBps,
			c.LateCancelPlatformRetentionBps,
			BpsDenominator,
		)
	}

	return nil
}

// BookingPriceBreakdown contiene el desglose financiero exacto en centavos para una reserva.
type BookingPriceBreakdown struct {
	Currency        Currency `json:"currency"`
	UnitPrice       Money    `json:"unit_price"`
	SeatsBooked     int      `json:"seats_booked"`
	BaseAmount      Money    `json:"base_amount"`
	PlatformFee     Money    `json:"platform_fee"`
	TotalPrice      Money    `json:"total_price"`
	DriverPayout    Money    `json:"driver_payout"`
	AppliedFeeBps   int64    `json:"applied_fee_bps"`
	AppliedFixedFee Money    `json:"applied_fixed_fee"`
}

// CalculateBookingBreakdown calcula el aporte al conductor (BaseAmount), el Service Fee al pasajero (PlatformFee)
// y el total a cobrar (TotalPrice = BaseAmount + PlatformFee) en aritmética entera exacta de centavos.
func (c PlatformPricingConfig) CalculateBookingBreakdown(unitPrice Money, seatsBooked int) (BookingPriceBreakdown, error) {
	if seatsBooked <= 0 {
		return BookingPriceBreakdown{}, ErrInvalidSeatCount
	}
	if unitPrice.Amount < 0 {
		return BookingPriceBreakdown{}, ErrPriceExceedsCapPrice
	}

	cur := unitPrice.EffectiveCurrency()
	normalizedUnit := MoneyFromCents(unitPrice.Amount, cur)
	baseAmount := normalizedUnit.Mul(int64(seatsBooked))

	if baseAmount.Amount == 0 {
		zero := ZeroMoney(cur)
		return BookingPriceBreakdown{
			Currency:        cur,
			UnitPrice:       normalizedUnit,
			SeatsBooked:     seatsBooked,
			BaseAmount:      zero,
			PlatformFee:     zero,
			TotalPrice:      zero,
			DriverPayout:    zero,
			AppliedFeeBps:   c.PassengerServiceFeeBps,
			AppliedFixedFee: zero,
		}, nil
	}

	feeBps := c.PassengerServiceFeeBps
	if feeBps < 0 {
		feeBps = 0
	}
	if feeBps > MaxPassengerServiceFeeBps {
		feeBps = MaxPassengerServiceFeeBps
	}

	pctFeeCents := RoundDivHalfUp(baseAmount.Amount*feeBps, BpsDenominator)
	var fixedFeeCents int64
	if c.PassengerFixedFee.Amount > 0 && c.PassengerFixedFee.EffectiveCurrency() == cur {
		fixedFeeCents = c.PassengerFixedFee.Amount
	}

	platformFee := MoneyFromCents(pctFeeCents+fixedFeeCents, cur)
	totalPrice := MoneyFromCents(baseAmount.Amount+platformFee.Amount, cur)

	return BookingPriceBreakdown{
		Currency:        cur,
		UnitPrice:       normalizedUnit,
		SeatsBooked:     seatsBooked,
		BaseAmount:      baseAmount,
		PlatformFee:     platformFee,
		TotalPrice:      totalPrice,
		DriverPayout:    baseAmount,
		AppliedFeeBps:   feeBps,
		AppliedFixedFee: MoneyFromCents(fixedFeeCents, cur),
	}, nil
}

// BpsToPercent convierte puntos básicos enteros (ej. 1500) a porcentaje legible (ej. 15.0).
func BpsToPercent(bps int64) float64 {
	return float64(bps) / 100.0
}
