package payment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/payment"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestMockPaymentGateway_CreatePaymentIntent(t *testing.T) {
	ctx := context.Background()
	gateway := payment.NewMockPaymentGateway()

	t.Run("crea exitosamente una intención de pago", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, "booking-123", 2500.0, "USD")
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.GatewayRef, "pi_mock_")
		assert.Contains(t, res.ClientSecret, "secret_")
		assert.Equal(t, 2500.0, res.Amount)
		assert.Equal(t, "USD", res.Currency)
		assert.Equal(t, "requires_payment_method", res.Status)
	})

	t.Run("falla con bookingID vacío", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, "", 2500.0, "USD")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("falla con monto menor o igual a cero", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, "booking-123", 0, "USD")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPriceExceedsCapPrice)
	})

	t.Run("falla con bookingID simulando error de pasarela", func(t *testing.T) {
		res, err := gateway.CreatePaymentIntent(ctx, "fail_gateway_booking", 1000.0, "USD")
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
		res, err := gateway.ProcessRefund(ctx, "pi_mock_123", 1500.0, "PASSENGER_EARLY")
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.GatewayRefundRef, "re_mock_")
		assert.Equal(t, 1500.0, res.Amount)
		assert.Equal(t, "succeeded", res.Status)
	})

	t.Run("falla con monto menor o igual a cero", func(t *testing.T) {
		res, err := gateway.ProcessRefund(ctx, "pi_mock_123", 0, "PASSENGER_EARLY")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidRefundAmount)
	})

	t.Run("falla con gatewayRef de error", func(t *testing.T) {
		res, err := gateway.ProcessRefund(ctx, "pi_refund_error", 1000.0, "PASSENGER_EARLY")
		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
	})
}
