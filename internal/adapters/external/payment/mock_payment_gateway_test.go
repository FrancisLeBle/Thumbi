package payment_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/external/payment"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestMockPaymentGateway_CreatePaymentIntent(t *testing.T) {
	ctx := context.Background()
	gateway := payment.NewMockPaymentGateway()
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000123")

	t.Run("crea exitosamente una intención de pago", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, bookingID, domain.MoneyFromCents(250000, domain.CurrencyARS))
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.GatewayRef, "pi_mock_")
		assert.Contains(t, res.ClientSecret, "secret_")
		assert.Equal(t, domain.MoneyFromCents(250000, domain.CurrencyARS), res.Amount)
		assert.Equal(t, domain.CurrencyARS, res.Currency)
		assert.Equal(t, "requires_payment_method", res.Status)
	})

	t.Run("falla con bookingID vacío", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, uuid.Nil, domain.MoneyFromCents(250000, domain.CurrencyARS))
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("falla con monto menor o igual a cero", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, bookingID, domain.MoneyFromCents(0, domain.CurrencyARS))
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPriceExceedsCapPrice)
	})

	t.Run("falla con bookingID simulando error de pasarela", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, payment.MockFailGatewayBookingID, domain.MoneyFromCents(100000, domain.CurrencyARS))
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
	})
}

func TestMockPaymentGateway_ConfirmPayment(t *testing.T) {
	ctx := context.Background()
	gateway := payment.NewMockPaymentGateway()

	t.Run("confirma exitosamente un gatewayRef válido", func(t *testing.T) {
		ok, err := gateway.ConfirmPayment(ctx, "pi_mock_valid_123")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("rechaza con gatewayRef vacío", func(t *testing.T) {
		ok, err := gateway.ConfirmPayment(ctx, "")
		assert.False(t, ok)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
	})

	t.Run("simula rechazo bancario de tarjeta", func(t *testing.T) {
		ok, err := gateway.ConfirmPayment(ctx, "pi_declined_card")
		assert.False(t, ok)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
	})
}

func TestMockPaymentGateway_ProcessRefund(t *testing.T) {
	ctx := context.Background()
	gateway := payment.NewMockPaymentGateway()

	t.Run("procesa reembolso exitosamente", func(t *testing.T) {
		res, err := gateway.ProcessRefund(ctx, "pi_mock_123", domain.MoneyFromCents(150000, domain.CurrencyARS), "PASSENGER_EARLY")
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.GatewayRefundRef, "re_mock_")
		assert.Equal(t, domain.MoneyFromCents(150000, domain.CurrencyARS), res.Amount)
		assert.Equal(t, "succeeded", res.Status)
	})

	t.Run("falla con monto menor o igual a cero", func(t *testing.T) {
		res, err := gateway.ProcessRefund(ctx, "pi_mock_123", domain.MoneyFromCents(0, domain.CurrencyARS), "PASSENGER_EARLY")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidRefundAmount)
	})

	t.Run("falla con gatewayRef de error", func(t *testing.T) {
		res, err := gateway.ProcessRefund(ctx, "pi_refund_error", domain.MoneyFromCents(100000, domain.CurrencyARS), "PASSENGER_EARLY")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
	})
}
