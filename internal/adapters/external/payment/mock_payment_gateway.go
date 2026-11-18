package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// MockFailGatewayBookingID es un UUID centinela utilizado en pruebas para simular un rechazo en pasarela.
var MockFailGatewayBookingID = uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")

type mockPaymentGateway struct{}

// NewMockPaymentGateway crea una instancia simulada de pasarela de pagos (Stripe / MercadoPago)
func NewMockPaymentGateway() ports.PaymentGateway {
	return &mockPaymentGateway{}
}

// CreatePaymentIntent genera una orden o intención de cobro simulada para la reserva utilizando exactamente la moneda de Money.
func (g *mockPaymentGateway) CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*ports.PaymentIntentResult, error) {
	if bookingID == uuid.Nil {
		return nil, domain.ErrBookingNotFound
	}
	if amount.Amount <= 0 {
		return nil, domain.ErrPriceExceedsCapPrice
	}
	cur, err := domain.ParseCurrency(string(amount.Currency))
	if err != nil {
		return nil, err
	}

	// Simulación de fallo intencional para tests de error
	if bookingID == MockFailGatewayBookingID {
		return nil, domain.ErrPaymentFailed
	}

	intentID := fmt.Sprintf("pi_mock_%s", domain.NewUUIDv7().String()[:12])
	clientSecret := fmt.Sprintf("%s_secret_%s", intentID, domain.NewUUIDv7().String()[:8])

	normalizedAmount := domain.MoneyFromCents(amount.Amount, cur)

	return &ports.PaymentIntentResult{
		GatewayRef:   intentID,
		ClientSecret: clientSecret,
		Amount:       normalizedAmount,
		Currency:     cur,
		Status:       "requires_payment_method",
	}, nil
}

// ConfirmPayment valida y captura la transacción con la pasarela externa
func (g *mockPaymentGateway) ConfirmPayment(ctx context.Context, gatewayRef string) (bool, error) {
	if strings.TrimSpace(gatewayRef) == "" {
		return false, domain.ErrPaymentFailed
	}

	// Simulación de rechazo de pago (fondos insuficientes, tarjeta rechazada, etc.)
	if strings.Contains(gatewayRef, "declined") || strings.Contains(gatewayRef, "failed") {
		return false, domain.ErrPaymentFailed
	}

	return true, nil
}

// ProcessRefund liquida la devolución o reintegro total/parcial del depósito
func (g *mockPaymentGateway) ProcessRefund(ctx context.Context, gatewayRef string, amount domain.Money, reason string) (*ports.RefundResult, error) {
	if strings.TrimSpace(gatewayRef) == "" {
		return nil, domain.ErrPaymentFailed
	}
	if amount.Amount <= 0 {
		return nil, domain.ErrInvalidRefundAmount
	}
	cur, err := domain.ParseCurrency(string(amount.Currency))
	if err != nil {
		return nil, err
	}

	if strings.Contains(gatewayRef, "refund_error") {
		return nil, domain.ErrPaymentFailed
	}

	refundID := fmt.Sprintf("re_mock_%s", domain.NewUUIDv7().String()[:12])

	return &ports.RefundResult{
		GatewayRefundRef: refundID,
		Amount:           domain.MoneyFromCents(amount.Amount, cur),
		Status:           "succeeded",
	}, nil
}
