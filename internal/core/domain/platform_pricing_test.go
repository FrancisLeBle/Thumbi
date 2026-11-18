package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestPlatformPricingConfig_Validate(t *testing.T) {
	t.Run("configuración por defecto es válida", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		require.NoError(t, cfg.Validate())
		assert.Equal(t, domain.CurrencyARS, cfg.Currency)
		assert.Equal(t, int64(1500), cfg.PassengerServiceFeeBps)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), cfg.PassengerFixedFee)
		assert.Equal(t, int64(10000), cfg.EarlyCancelFeeRetentionBps)
		assert.Equal(t, 24, cfg.LateCancelWindowHours)
		assert.Equal(t, int64(10000), cfg.LateCancelFeeRetentionBps)
		assert.Equal(t, int64(5000), cfg.LateCancelPassengerRefundBps)
		assert.Equal(t, int64(5000), cfg.LateCancelDriverCompBps)
		assert.Equal(t, int64(0), cfg.LateCancelPlatformRetentionBps)
	})

	t.Run("rechaza service fee negativo o mayor al tope de 50% (5000 bps)", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = -1
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)

		cfg = domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 5001
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)

		cfg = domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 5000
		require.NoError(t, cfg.Validate())
	})

	t.Run("rechaza tarifa fija negativa o con moneda incompatible", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerFixedFee = domain.MoneyFromCents(-100, domain.CurrencyARS)
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)

		cfg = domain.DefaultPlatformPricingConfig()
		cfg.PassengerFixedFee = domain.MoneyFromCents(5000, domain.CurrencyUSD)
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)
	})

	t.Run("rechaza ventana tardía fuera de rango 1..168 horas", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.LateCancelWindowHours = 0
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)

		cfg.LateCancelWindowHours = 169
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)
	})

	t.Run("rechaza distribución 3-vías de cancelación tardía que no sume exactamente 10000 bps", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.LateCancelPassengerRefundBps = 4000
		cfg.LateCancelDriverCompBps = 4000
		cfg.LateCancelPlatformRetentionBps = 1000 // suma 9000
		assert.ErrorIs(t, cfg.Validate(), domain.ErrInvalidPricingConfig)

		cfg.LateCancelPlatformRetentionBps = 2000 // suma 10000
		require.NoError(t, cfg.Validate())
	})
}

func TestPlatformPricingConfig_CalculateBookingBreakdown(t *testing.T) {
	t.Run("calcula 15% service fee sobre Cap Price manteniendo intacto el pago al conductor", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		unitPrice := domain.MoneyFromCents(150000, domain.CurrencyARS) // $1,500.00 ARS

		breakdown, err := cfg.CalculateBookingBreakdown(unitPrice, 2)
		require.NoError(t, err)

		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), breakdown.BaseAmount)
		assert.Equal(t, domain.MoneyFromCents(45000, domain.CurrencyARS), breakdown.PlatformFee)
		assert.Equal(t, domain.MoneyFromCents(345000, domain.CurrencyARS), breakdown.TotalPrice)
		assert.Equal(t, breakdown.BaseAmount, breakdown.DriverPayout)
		assert.Equal(t, int64(1500), breakdown.AppliedFeeBps)
	})

	t.Run("combina porcentaje parametrizado con tarifa fija por reserva", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 1000 // 10%
		cfg.PassengerFixedFee = domain.MoneyFromCents(10000, domain.CurrencyARS) // $100.00 fijo

		unitPrice := domain.MoneyFromCents(200000, domain.CurrencyARS)
		breakdown, err := cfg.CalculateBookingBreakdown(unitPrice, 1)
		require.NoError(t, err)

		assert.Equal(t, domain.MoneyFromCents(200000, domain.CurrencyARS), breakdown.BaseAmount)
		assert.Equal(t, domain.MoneyFromCents(30000, domain.CurrencyARS), breakdown.PlatformFee) // 200.00 + 100.00
		assert.Equal(t, domain.MoneyFromCents(230000, domain.CurrencyARS), breakdown.TotalPrice)
	})

	t.Run("no cobra comisión en viajes gratuitos ($0.00)", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerFixedFee = domain.MoneyFromCents(15000, domain.CurrencyARS)

		breakdown, err := cfg.CalculateBookingBreakdown(domain.ZeroMoney(domain.CurrencyARS), 2)
		require.NoError(t, err)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), breakdown.BaseAmount)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), breakdown.PlatformFee)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), breakdown.TotalPrice)
	})

	t.Run("aplica redondeo half-up exacto en centavos impares", func(t *testing.T) {
		cfg := domain.DefaultPlatformPricingConfig() // 15%
		// 1005 centavos * 1500 / 10000 = 150.75 -> 151 centavos
		breakdown, err := cfg.CalculateBookingBreakdown(domain.MoneyFromCents(1005, domain.CurrencyARS), 1)
		require.NoError(t, err)
		assert.Equal(t, int64(151), breakdown.PlatformFee.Amount)
		assert.Equal(t, int64(1156), breakdown.TotalPrice.Amount)
	})
}

func TestCalculateCancellationRefundWithPolicy(t *testing.T) {
	departure := time.Date(2026, 10, 15, 14, 0, 0, 0, time.UTC)
	baseAmount := domain.MoneyFromCents(1000000, domain.CurrencyARS) // $10,000.00
	platformFee := domain.MoneyFromCents(150000, domain.CurrencyARS) // $1,500.00 (15%)
	totalAmount := domain.MoneyFromCents(1150000, domain.CurrencyARS)

	t.Run("cancelación del conductor reembolsa 100% de base + platform fee sin retención", func(t *testing.T) {
		policy := domain.DefaultPlatformPricingConfig()
		res := domain.CalculateCancellationRefundWithPolicy(
			baseAmount,
			platformFee,
			departure,
			departure.Add(-2*time.Hour),
			true,
			policy,
		)

		assert.Equal(t, domain.RefundTypeDriverCancellation, res.RefundType)
		assert.Equal(t, totalAmount, res.PassengerRefund)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), res.DriverCompensation)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), res.PlatformRetained)
	})

	t.Run("cancelación temprana de pasajero retiene 100% del service fee por defecto y devuelve 100% del base", func(t *testing.T) {
		policy := domain.DefaultPlatformPricingConfig()
		res := domain.CalculateCancellationRefundWithPolicy(
			baseAmount,
			platformFee,
			departure,
			departure.Add(-36*time.Hour),
			false,
			policy,
		)

		assert.Equal(t, domain.RefundTypePassengerEarly, res.RefundType)
		assert.Equal(t, baseAmount, res.PassengerRefund)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), res.DriverCompensation)
		assert.Equal(t, platformFee, res.PlatformRetained)
	})

	t.Run("cancelación temprana con retención parcial (50% del fee) reembolsa base + 50% del fee", func(t *testing.T) {
		policy := domain.DefaultPlatformPricingConfig()
		policy.EarlyCancelFeeRetentionBps = 5000

		res := domain.CalculateCancellationRefundWithPolicy(
			baseAmount,
			platformFee,
			departure,
			departure.Add(-30*time.Hour),
			false,
			policy,
		)

		assert.Equal(t, domain.RefundTypePassengerEarly, res.RefundType)
		assert.Equal(t, domain.MoneyFromCents(1075000, domain.CurrencyARS), res.PassengerRefund)
		assert.Equal(t, domain.ZeroMoney(domain.CurrencyARS), res.DriverCompensation)
		assert.Equal(t, domain.MoneyFromCents(75000, domain.CurrencyARS), res.PlatformRetained)
	})

	t.Run("cancelación tardía con reparto 3-vías 40/40/20 + retención 100% del fee conserva la suma exacta", func(t *testing.T) {
		policy := domain.DefaultPlatformPricingConfig()
		policy.LateCancelWindowHours = 12
		policy.LateCancelFeeRetentionBps = 10000
		policy.LateCancelPassengerRefundBps = 4000
		policy.LateCancelDriverCompBps = 4000
		policy.LateCancelPlatformRetentionBps = 2000

		res := domain.CalculateCancellationRefundWithPolicy(
			baseAmount,
			platformFee,
			departure,
			departure.Add(-6*time.Hour), // < 12h ventana tardía
			false,
			policy,
		)

		assert.Equal(t, domain.RefundTypePassengerLate, res.RefundType)
		// Base: 40% pasajero ($4,000), 40% conductor ($4,000), 20% plataforma ($2,000) + $1,500 fee = $3,500 plataforma
		assert.Equal(t, domain.MoneyFromCents(400000, domain.CurrencyARS), res.PassengerRefund)
		assert.Equal(t, domain.MoneyFromCents(400000, domain.CurrencyARS), res.DriverCompensation)
		assert.Equal(t, domain.MoneyFromCents(350000, domain.CurrencyARS), res.PlatformRetained)

		sum := res.PassengerRefund.Amount + res.DriverCompensation.Amount + res.PlatformRetained.Amount
		assert.Equal(t, totalAmount.Amount, sum)
	})

	t.Run("conserva suma exacta ante centavos impares en reparto 3-vías", func(t *testing.T) {
		policy := domain.DefaultPlatformPricingConfig()
		policy.LateCancelPassengerRefundBps = 3334
		policy.LateCancelDriverCompBps = 3333
		policy.LateCancelPlatformRetentionBps = 3333

		oddBase := domain.MoneyFromCents(1001, domain.CurrencyARS)
		oddFee := domain.MoneyFromCents(151, domain.CurrencyARS)
		res := domain.CalculateCancellationRefundWithPolicy(
			oddBase,
			oddFee,
			departure,
			departure.Add(-1*time.Hour),
			false,
			policy,
		)

		sum := res.PassengerRefund.Amount + res.DriverCompensation.Amount + res.PlatformRetained.Amount
		assert.Equal(t, oddBase.Amount+oddFee.Amount, sum)
	})

	t.Run("DriverPayoutAmount distingue escrow con desglose de escrow legado", func(t *testing.T) {
		escrowWithFee, err := domain.NewEscrowTransactionWithBreakdown(
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			baseAmount,
			platformFee,
			"pi_breakdown_1",
		)
		require.NoError(t, err)
		assert.Equal(t, totalAmount, escrowWithFee.Amount)
		assert.Equal(t, baseAmount, escrowWithFee.DriverPayoutAmount())

		legacyEscrow, err := domain.NewEscrowTransaction(
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			domain.NewUUIDv7(),
			baseAmount,
			"pi_legacy_1",
		)
		require.NoError(t, err)
		assert.Equal(t, baseAmount, legacyEscrow.DriverPayoutAmount())
	})
}

func TestBooking_WithPlatformFee(t *testing.T) {
	b, err := domain.NewBooking(domain.BookingParams{
		ID:          domain.NewUUIDv7(),
		TripID:      domain.NewUUIDv7(),
		PassengerID: domain.NewUUIDv7(),
		DriverID:    domain.NewUUIDv7(),
		SeatsBooked: 2,
		Currency:    domain.CurrencyARS,
		UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		PlatformFee: domain.MoneyFromCents(45000, domain.CurrencyARS),
	})
	require.NoError(t, err)
	assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), b.BaseAmount)
	assert.Equal(t, domain.MoneyFromCents(45000, domain.CurrencyARS), b.PlatformFee)
	assert.Equal(t, domain.MoneyFromCents(345000, domain.CurrencyARS), b.TotalPrice)
}
