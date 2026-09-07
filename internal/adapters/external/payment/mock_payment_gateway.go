package payment

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type mockPaymentGateway struct{}

// NewMockPaymentGateway crea una instancia simulada de pasarela de pagos (Stripe / MercadoPago)
func NewMockPaymentGateway() ports.PaymentGateway {
	return &mockPaymentGateway{}
}

// CreatePaymentIntent genera una orden o intención de cobro simulada para la reserva
func (g *mockPaymentGateway) CreatePaymentIntent(ctx context.Context, bookingID string, amount float64, currency string) (*ports.PaymentIntentResult, error) {
	if strings.TrimSpace(bookingID) == "" {
		return nil, domain.ErrBookingNotFound
	}
	if amount <= 0 {
		return nil, domain.ErrPriceExceedsCapPrice
	}
	if strings.TrimSpace(currency) == "" {
		currency = "USD"
	}

	// Simulación de fallo intencional para tests de error
	if strings.Contains(bookingID, "fail_gateway") {
		return nil, domain.ErrPaymentFailed
	}

	intentID := fmt.Sprintf("pi_mock_%s", uuid.New().String()[:12])
	clientSecret := fmt.Sprintf("%s_secret_%s", intentID, uuid.New().String()[:8])

	return &ports.PaymentIntentResult{
		GatewayRef:   intentID,
		ClientSecret: clientSecret,
		Amount:       amount,
		Currency:     strings.ToUpper(currency),
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
func (g *mockPaymentGateway) ProcessRefund(ctx context.Context, gatewayRef string, amount float64, reason string) (*ports.RefundResult, error) {
	if strings.TrimSpace(gatewayRef) == "" {
		return nil, domain.ErrPaymentFailed
	}
	if amount <= 0 {
		return nil, domain.ErrInvalidRefundAmount
	}

	if strings.Contains(gatewayRef, "refund_error") {
		return nil, domain.ErrPaymentFailed
	}

	refundID := fmt.Sprintf("re_mock_%s", uuid.New().String()[:12])

	return &ports.RefundResult{
		GatewayRefundRef: refundID,
		Amount:           amount,
		Status:           "succeeded",
	}, nil
}
