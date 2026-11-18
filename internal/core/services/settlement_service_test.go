package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func TestSettlementService(t *testing.T) {
	ctx := context.Background()
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000001101")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000001102")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000001103")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000001104")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000001105")
	amount := domain.MoneyFromCents(400000, domain.CurrencyARS)

	t.Run("CreatePaymentIntent y ConfirmPayment delegan en PaymentGateway y normalizan fallos a ErrPaymentFailed", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		svc := services.NewSettlementService(escrowRepo, gateway)

		gateway.On("CreatePaymentIntent", ctx, bookingID, amount).Return(&ports.PaymentIntentResult{
			GatewayRef: "pi_settle_1",
			Amount:     amount,
			Currency:   domain.CurrencyARS,
			Status:     "requires_payment_method",
		}, nil).Once()

		intent, err := svc.CreatePaymentIntent(ctx, bookingID, amount)
		require.NoError(t, err)
		assert.Equal(t, "pi_settle_1", intent.GatewayRef)

		gateway.On("ConfirmPayment", ctx, "pi_settle_1").Return(false, nil).Once()
		err = svc.ConfirmPayment(ctx, "pi_settle_1")
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)

		gateway.AssertExpectations(t)
	})

	t.Run("HoldEscrow crea y persiste la custodia en estado HELD", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		svc := services.NewSettlementService(escrowRepo, gateway)

		escrowRepo.On("CreateEscrow", ctx, mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == bookingID &&
				e.TripID == tripID &&
				e.PayerID == passengerID &&
				e.PayeeID == driverID &&
				e.Amount == amount &&
				e.Status == domain.EscrowStatusHeld &&
				e.PaymentGatewayRef == "pi_hold_1"
		})).Return(nil)

		escrow, err := svc.HoldEscrow(ctx, services.HoldEscrowInput{
			BookingID:         bookingID,
			TripID:            tripID,
			PassengerID:       passengerID,
			DriverID:          driverID,
			Amount:            amount,
			PaymentGatewayRef: "pi_hold_1",
		})
		require.NoError(t, err)
		assert.Equal(t, domain.EscrowStatusHeld, escrow.Status)
		escrowRepo.AssertExpectations(t)
	})

	t.Run("SettleCancellationRefund retorna (nil, nil) cuando la reserva nunca tuvo Escrow", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		svc := services.NewSettlementService(escrowRepo, gateway)

		escrowRepo.On("FindByBookingID", ctx, bookingID).Return(nil, domain.ErrEscrowNotFound)

		refund, err := svc.SettleCancellationRefund(ctx, services.CancellationRefundInput{
			BookingID:     bookingID,
			DepartureTime: time.Now().UTC().Add(48 * time.Hour),
			CancelledAt:   time.Now().UTC(),
		})
		require.NoError(t, err)
		assert.Nil(t, refund)
		gateway.AssertNotCalled(t, "ProcessRefund", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("SettleCancellationRefund aplica política 50/50 en cancelación tardía y persiste dentro de TransactionManager", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		txManager := new(MockTransactionManager)
		svc := services.NewSettlementService(escrowRepo, gateway, txManager)

		now := time.Now().UTC()
		half := domain.MoneyFromCents(200000, domain.CurrencyARS)
		escrow, err := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, amount, "pi_cancel_late")
		require.NoError(t, err)

		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		escrowRepo.On("FindByBookingID", ctx, bookingID).Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_cancel_late", half, string(domain.RefundTypePassengerLate)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_late_1",
			Amount:           half,
			Status:           "succeeded",
		}, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		escrowRepo.On("UpdateEscrowStatus", mock.MatchedBy(hasTx), escrowID, domain.EscrowStatusRefundedPartial).Return(nil)
		escrowRepo.On("RecordRefund", mock.MatchedBy(hasTx), mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.RefundType == domain.RefundTypePassengerLate &&
				r.PassengerRefundAmount == half &&
				r.DriverCompensationAmount == half &&
				r.GatewayRefundRef == "re_late_1"
		})).Return(nil)

		refund, err := svc.SettleCancellationRefund(ctx, services.CancellationRefundInput{
			BookingID:            bookingID,
			DepartureTime:        now.Add(6 * time.Hour),
			CancelledAt:          now,
			IsDriverCancellation: false,
		})
		require.NoError(t, err)
		require.NotNil(t, refund)
		assert.Equal(t, domain.EscrowStatusRefundedPartial, escrow.Status)
		txManager.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})

	t.Run("SettleDisputeRefund falla antes de llamar a la pasarela si el Escrow ya estaba liquidado", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		svc := services.NewSettlementService(escrowRepo, gateway)

		escrow, err := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, amount, "pi_settled")
		require.NoError(t, err)
		require.NoError(t, escrow.Release(time.Now().UTC()))

		refund, err := svc.SettleDisputeRefund(ctx, services.DisputeRefundInput{
			Escrow: escrow,
			Reason: "Reembolso inválido",
		})
		assert.Nil(t, refund)
		assert.ErrorIs(t, err, domain.ErrEscrowAlreadySettled)
		gateway.AssertNotCalled(t, "ProcessRefund", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("SettleDisputeRefund ejecuta reembolso y hooks transaccionales atómicamente", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		txManager := new(MockTransactionManager)
		svc := services.NewSettlementService(escrowRepo, gateway, txManager)

		escrow, err := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, amount, "pi_dispute_1")
		require.NoError(t, err)
		require.NoError(t, escrow.OpenDispute(time.Now().UTC()))

		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		gateway.On("ProcessRefund", ctx, "pi_dispute_1", amount, "Disputa resuelta").Return(&ports.RefundResult{
			GatewayRefundRef: "re_disp_1",
			Amount:           amount,
			Status:           "succeeded",
		}, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		escrowRepo.On("UpdateEscrowStatus", mock.MatchedBy(hasTx), escrowID, domain.EscrowStatusRefundedFull).Return(nil)
		escrowRepo.On("RecordRefund", mock.MatchedBy(hasTx), mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.RefundType == domain.RefundTypeAdminDispute && r.PassengerRefundAmount == amount
		})).Return(nil)

		hookExecutedInTx := false
		refund, err := svc.SettleDisputeRefund(ctx, services.DisputeRefundInput{
			Escrow: escrow,
			Reason: "Disputa resuelta",
		}, func(txCtx context.Context) error {
			hookExecutedInTx = hasTx(txCtx)
			return nil
		})

		require.NoError(t, err)
		require.NotNil(t, refund)
		assert.True(t, hookExecutedInTx)
		txManager.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})

	t.Run("SettleDisputeRefund propaga error del gateway como ErrPaymentFailed sin tocar repositorio", func(t *testing.T) {
		escrowRepo := new(MockEscrowRepository)
		gateway := new(MockPaymentGateway)
		svc := services.NewSettlementService(escrowRepo, gateway)

		escrow, err := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, amount, "pi_fail_gw")
		require.NoError(t, err)
		require.NoError(t, escrow.OpenDispute(time.Now().UTC()))

		gateway.On("ProcessRefund", ctx, "pi_fail_gw", amount, "Reembolso").Return(nil, errors.New("gateway down"))

		refund, err := svc.SettleDisputeRefund(ctx, services.DisputeRefundInput{
			Escrow: escrow,
			Reason: "Reembolso",
		})

		assert.Nil(t, refund)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
		assert.Equal(t, domain.EscrowStatusDisputed, escrow.Status)
		escrowRepo.AssertNotCalled(t, "UpdateEscrowStatus", mock.Anything, mock.Anything, mock.Anything)
		escrowRepo.AssertNotCalled(t, "RecordRefund", mock.Anything, mock.Anything)
	})
}
