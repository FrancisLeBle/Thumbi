package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// HoldEscrowInput DTO para constituir la retención de fondos en custodia tras acreditar un cobro.
type HoldEscrowInput struct {
	BookingID         uuid.UUID
	TripID            uuid.UUID
	PassengerID       uuid.UUID
	DriverID          uuid.UUID
	Amount            domain.Money
	PaymentGatewayRef string
}

// CancellationRefundInput DTO para calcular y liquidar reembolsos por cancelación de reserva.
type CancellationRefundInput struct {
	BookingID            uuid.UUID
	DepartureTime        time.Time
	CancelledAt          time.Time
	IsDriverCancellation bool
}

// DisputeRefundInput DTO para ejecutar el reembolso total ordenado tras la resolución de una disputa.
type DisputeRefundInput struct {
	Escrow *domain.EscrowTransaction
	Reason string
}

// SettlementService centraliza el procesamiento de pagos con la pasarela externa,
// la constitución/transición de fondos en Escrow y el registro contable de reembolsos
// tanto para cancelaciones de reservas como para arbitraje de disputas.
type SettlementService struct {
	escrowRepo     ports.EscrowRepository
	paymentGateway ports.PaymentGateway
	txManager      ports.TransactionManager
	clock          func() time.Time
}

// NewSettlementService instancia el servicio compartido de liquidación y custodia financiera.
func NewSettlementService(
	escrowRepo ports.EscrowRepository,
	paymentGateway ports.PaymentGateway,
	optionalDeps ...any,
) *SettlementService {
	svc := &SettlementService{
		escrowRepo:     escrowRepo,
		paymentGateway: paymentGateway,
		clock:          time.Now,
	}
	for _, opt := range optionalDeps {
		if tm, ok := opt.(ports.TransactionManager); ok {
			svc.txManager = tm
		}
	}
	return svc
}

func (s *SettlementService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.txManager != nil {
		return s.txManager.WithinTransaction(ctx, fn)
	}
	return fn(ctx)
}

// CreatePaymentIntent inicializa una orden/intención de cobro en la pasarela para una reserva.
func (s *SettlementService) CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*ports.PaymentIntentResult, error) {
	if bookingID == uuid.Nil || !amount.IsPositive() || s.paymentGateway == nil {
		return nil, domain.ErrPaymentFailed
	}

	result, err := s.paymentGateway.CreatePaymentIntent(ctx, bookingID, amount)
	if err != nil || result == nil || strings.TrimSpace(result.GatewayRef) == "" {
		return nil, domain.ErrPaymentFailed
	}

	return result, nil
}

// ConfirmPayment verifica y acredita la captura de fondos ante la pasarela de pagos.
func (s *SettlementService) ConfirmPayment(ctx context.Context, gatewayRef string) error {
	if strings.TrimSpace(gatewayRef) == "" || s.paymentGateway == nil {
		return domain.ErrPaymentFailed
	}

	confirmed, err := s.paymentGateway.ConfirmPayment(ctx, gatewayRef)
	if err != nil || !confirmed {
		return domain.ErrPaymentFailed
	}

	return nil
}

// HoldEscrow crea y persiste una transacción de custodia en estado HELD (participa en txCtx si existe).
func (s *SettlementService) HoldEscrow(ctx context.Context, input HoldEscrowInput) (*domain.EscrowTransaction, error) {
	if s.escrowRepo == nil {
		return nil, domain.ErrEscrowNotFound
	}

	escrow, err := domain.NewEscrowTransaction(
		domain.NewUUIDv7(),
		input.BookingID,
		input.TripID,
		input.PassengerID,
		input.DriverID,
		input.Amount,
		input.PaymentGatewayRef,
	)
	if err != nil {
		return nil, err
	}

	if err := s.escrowRepo.CreateEscrow(ctx, escrow); err != nil {
		return nil, err
	}

	return escrow, nil
}

// GetEscrowByBookingID recupera la transacción de custodia asociada a una reserva.
func (s *SettlementService) GetEscrowByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error) {
	if bookingID == uuid.Nil || s.escrowRepo == nil {
		return nil, domain.ErrEscrowNotFound
	}

	return s.escrowRepo.FindByBookingID(ctx, bookingID)
}

// FreezeEscrowForDispute congela los fondos en custodia bajo estado DISPUTED (participa en txCtx si existe).
func (s *SettlementService) FreezeEscrowForDispute(ctx context.Context, escrow *domain.EscrowTransaction) error {
	if escrow == nil || s.escrowRepo == nil {
		return domain.ErrEscrowNotFound
	}

	if escrow.Status == domain.EscrowStatusHeld {
		if err := escrow.OpenDispute(s.clock().UTC()); err != nil {
			return err
		}
	} else if escrow.Status != domain.EscrowStatusDisputed {
		return domain.ErrEscrowAlreadySettled
	}

	return s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusDisputed)
}

// ReleaseEscrow libera los fondos en custodia hacia el conductor (participa en txCtx si existe).
func (s *SettlementService) ReleaseEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error {
	if escrow == nil || s.escrowRepo == nil {
		return domain.ErrEscrowNotFound
	}

	if err := escrow.Release(s.clock().UTC()); err != nil {
		return err
	}

	return s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, escrow.Status)
}

// RestoreEscrowHeld restaura el estado HELD cuando una disputa es desestimada (participa en txCtx si existe).
func (s *SettlementService) RestoreEscrowHeld(ctx context.Context, escrow *domain.EscrowTransaction) error {
	if escrow == nil || s.escrowRepo == nil {
		return domain.ErrEscrowNotFound
	}

	if err := escrow.RestoreHeld(s.clock().UTC()); err != nil {
		return err
	}

	return s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, escrow.Status)
}

// SettleCancellationRefund calcula y liquida el reembolso de Escrow aplicable a una reserva cancelada.
// Si la reserva no tiene Escrow asociado (por ejemplo, cancelada en PENDING_PAYMENT), retorna (nil, nil).
func (s *SettlementService) SettleCancellationRefund(ctx context.Context, input CancellationRefundInput) (*domain.RefundTransaction, error) {
	if input.BookingID == uuid.Nil || s.escrowRepo == nil {
		return nil, nil
	}

	escrow, err := s.escrowRepo.FindByBookingID(ctx, input.BookingID)
	if errors.Is(err, domain.ErrEscrowNotFound) || (err == nil && escrow == nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cancelledAt := input.CancelledAt
	if cancelledAt.IsZero() {
		cancelledAt = s.clock().UTC()
	} else {
		cancelledAt = cancelledAt.UTC()
	}

	pRefund, dComp, refundType := domain.CalculateCancellationRefund(
		escrow.Amount,
		input.DepartureTime,
		cancelledAt,
		input.IsDriverCancellation,
	)

	return s.settleRefund(ctx, escrow, pRefund, dComp, refundType, refundType.String(), cancelledAt)
}

// SettleDisputeRefund ejecuta el reembolso total ordenado en una disputa y persiste atómicamente
// el estado de custodia, el libro de reembolsos y cualquier hook transaccional adicional (ej. actualizar disputa).
func (s *SettlementService) SettleDisputeRefund(
	ctx context.Context,
	input DisputeRefundInput,
	withinTxHooks ...func(txCtx context.Context) error,
) (*domain.RefundTransaction, error) {
	if input.Escrow == nil {
		return nil, domain.ErrEscrowNotFound
	}

	now := s.clock().UTC()
	refundAmount := input.Escrow.Amount
	zeroComp := domain.ZeroMoney(refundAmount.EffectiveCurrency())

	return s.settleRefund(
		ctx,
		input.Escrow,
		refundAmount,
		zeroComp,
		domain.RefundTypeAdminDispute,
		input.Reason,
		now,
		withinTxHooks...,
	)
}

func (s *SettlementService) settleRefund(
	ctx context.Context,
	escrow *domain.EscrowTransaction,
	passengerRefund domain.Money,
	driverCompensation domain.Money,
	refundType domain.RefundType,
	reason string,
	settledAt time.Time,
	withinTxHooks ...func(txCtx context.Context) error,
) (*domain.RefundTransaction, error) {
	if escrow == nil || s.escrowRepo == nil {
		return nil, domain.ErrEscrowNotFound
	}
	if escrow.Status != domain.EscrowStatusHeld && escrow.Status != domain.EscrowStatusDisputed {
		return nil, domain.ErrEscrowAlreadySettled
	}

	gatewayRefundRef := ""
	if passengerRefund.IsPositive() {
		if s.paymentGateway == nil {
			return nil, domain.ErrPaymentFailed
		}
		refundResult, err := s.paymentGateway.ProcessRefund(ctx, escrow.PaymentGatewayRef, passengerRefund, reason)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", domain.ErrPaymentFailed, err)
		}
		if refundResult != nil {
			gatewayRefundRef = refundResult.GatewayRefundRef
			if refundType == domain.RefundTypeAdminDispute && refundResult.Amount.IsPositive() {
				passengerRefund = refundResult.Amount
			}
		}
	}

	if refundType == domain.RefundTypePassengerLate || driverCompensation.IsPositive() {
		if err := escrow.RefundPartial(settledAt); err != nil {
			return nil, err
		}
	} else {
		if err := escrow.RefundFull(settledAt); err != nil {
			return nil, err
		}
	}

	refundTx, err := domain.NewRefundTransaction(
		domain.NewUUIDv7(),
		escrow.ID,
		escrow.BookingID,
		passengerRefund,
		driverCompensation,
		refundType,
		gatewayRefundRef,
	)
	if err != nil {
		return nil, err
	}

	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, escrow.Status); err != nil {
			return err
		}
		if err := s.escrowRepo.RecordRefund(txCtx, refundTx); err != nil {
			return err
		}
		for _, hook := range withinTxHooks {
			if hook != nil {
				if err := hook(txCtx); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return refundTx, nil
}
