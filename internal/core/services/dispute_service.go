package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// DisputeService implementa los casos de uso para apertura, consulta y arbitraje de disputas
type DisputeService struct {
	disputeRepo    ports.DisputeRepository
	escrowRepo     ports.EscrowRepository
	bookingRepo    ports.BookingRepository
	tripRepo       ports.TripRepository
	paymentGateway ports.PaymentGateway
	txManager      ports.TransactionManager
	clock          func() time.Time
}

// NewDisputeService crea una instancia de DisputeService con soporte opcional de TransactionManager
func NewDisputeService(
	disputeRepo ports.DisputeRepository,
	escrowRepo ports.EscrowRepository,
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	paymentGateway ports.PaymentGateway,
	optionalDeps ...any,
) ports.DisputeService {
	svc := &DisputeService{
		disputeRepo:    disputeRepo,
		escrowRepo:     escrowRepo,
		bookingRepo:    bookingRepo,
		tripRepo:       tripRepo,
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

func (s *DisputeService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.txManager != nil {
		return s.txManager.WithinTransaction(ctx, fn)
	}
	return fn(ctx)
}

// OpenDispute valida la legitimidad del reclamo, congela el escrow en DISPUTED y persiste la disputa en una transacción
func (s *DisputeService) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	if strings.TrimSpace(input.BookingID) == "" ||
		strings.TrimSpace(input.ReporterID) == "" ||
		strings.TrimSpace(input.Description) == "" {
		return nil, domain.ErrInvalidDisputeData
	}

	booking, err := s.bookingRepo.FindByID(ctx, input.BookingID)
	if err != nil {
		return nil, err
	}

	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil {
		return nil, err
	}

	var defendantID string
	switch input.ReporterID {
	case booking.PassengerID:
		defendantID = trip.DriverID
	case trip.DriverID:
		defendantID = booking.PassengerID
	default:
		return nil, domain.ErrUnauthorized
	}

	escrow, err := s.escrowRepo.FindByBookingID(ctx, booking.ID)
	if err != nil {
		return nil, err
	}

	if escrow.Status != domain.EscrowStatusHeld && escrow.Status != domain.EscrowStatusDisputed {
		return nil, domain.ErrEscrowAlreadySettled
	}

	existing, err := s.disputeRepo.FindByEscrowID(ctx, escrow.ID)
	if err != nil && !errors.Is(err, domain.ErrDisputeNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, domain.ErrDisputeAlreadyExists
	}

	reason := input.Reason
	if strings.TrimSpace(string(reason)) == "" {
		reason = domain.DisputeReasonOther
	}

	dispute, err := domain.NewDispute(domain.DisputeParams{
		ID:                  uuid.New().String(),
		EscrowTransactionID: escrow.ID,
		BookingID:           booking.ID,
		TripID:              trip.ID,
		ReporterID:          input.ReporterID,
		DefendantID:         defendantID,
		Reason:              reason,
		Description:         input.Description,
		EvidenceURLs:        input.EvidenceURLs,
		EscrowStatus:        escrow.Status,
	})
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC()
	if escrow.Status == domain.EscrowStatusHeld {
		if err := escrow.OpenDispute(now); err != nil {
			return nil, err
		}
	}

	if err := s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, domain.EscrowStatusDisputed); err != nil {
			return err
		}
		if err := s.disputeRepo.Save(txCtx, dispute); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return toDisputeDTO(dispute), nil
}

// GetDisputeByID recupera el detalle de una disputa por su ID
func (s *DisputeService) GetDisputeByID(ctx context.Context, disputeID string) (*ports.DisputeDTO, error) {
	if strings.TrimSpace(disputeID) == "" {
		return nil, domain.ErrInvalidDisputeData
	}

	dispute, err := s.disputeRepo.FindByID(ctx, disputeID)
	if err != nil {
		return nil, err
	}
	return toDisputeDTO(dispute), nil
}

// ListDisputes consulta las disputas de forma paginada usando limit y offset
func (s *DisputeService) ListDisputes(ctx context.Context, limit, offset int) ([]*ports.DisputeDTO, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	disputes, err := s.disputeRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	result := make([]*ports.DisputeDTO, 0, len(disputes))
	for _, d := range disputes {
		result = append(result, toDisputeDTO(d))
	}
	return result, nil
}

// ListDisputesByBooking consulta las disputas asociadas a una reserva a partir de su escrow
func (s *DisputeService) ListDisputesByBooking(ctx context.Context, bookingID string) ([]*domain.Dispute, error) {
	if strings.TrimSpace(bookingID) == "" || s.escrowRepo == nil {
		return []*domain.Dispute{}, nil
	}

	escrow, err := s.escrowRepo.FindByBookingID(ctx, bookingID)
	if err != nil {
		return []*domain.Dispute{}, nil
	}

	dispute, err := s.disputeRepo.FindByEscrowID(ctx, escrow.ID)
	if err != nil {
		return []*domain.Dispute{}, nil
	}
	return []*domain.Dispute{dispute}, nil
}

// ResolveDispute dictamina una resolución administrativa válida y ejecuta atómicamente la liquidación o reembolso del escrow
func (s *DisputeService) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	if strings.TrimSpace(input.DisputeID) == "" ||
		strings.TrimSpace(input.AdminID) == "" ||
		!domain.IsValidDisputeResolution(input.Resolution) {
		return nil, domain.ErrInvalidDisputeData
	}

	dispute, err := s.disputeRepo.FindByID(ctx, input.DisputeID)
	if err != nil {
		return nil, err
	}

	if dispute.IsClosed() {
		return nil, domain.ErrDisputeAlreadyResolved
	}

	if strings.TrimSpace(input.AdminNotes) == "" {
		return nil, domain.ErrInvalidDisputeData
	}

	escrow, err := s.escrowRepo.FindByBookingID(ctx, dispute.BookingID)
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC()

	switch input.Resolution {
	case domain.DisputeStatusResolvedPassengerRefund:
		if err := dispute.ResolvePassengerRefund(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}
		if err := escrow.RefundFull(now); err != nil {
			return nil, err
		}
		if s.paymentGateway == nil {
			return nil, domain.ErrPaymentFailed
		}

		refundResult, err := s.paymentGateway.ProcessRefund(
			ctx,
			escrow.PaymentGatewayRef,
			escrow.Amount,
			fmt.Sprintf("Resolución de disputa %s: %s", dispute.ID, strings.TrimSpace(input.AdminNotes)),
		)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", domain.ErrPaymentFailed, err)
		}

		refundAmount := escrow.Amount
		gatewayRefundRef := ""
		if refundResult != nil {
			if refundResult.Amount > 0 {
				refundAmount = refundResult.Amount
			}
			gatewayRefundRef = refundResult.GatewayRefundRef
		}

		refundTx, err := domain.NewRefundTransaction(
			uuid.New().String(),
			escrow.ID,
			escrow.BookingID,
			refundAmount,
			0,
			domain.RefundTypeAdminDispute,
			gatewayRefundRef,
		)
		if err != nil {
			return nil, err
		}

		if err := s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, escrow.Status); err != nil {
				return err
			}
			if err := s.escrowRepo.RecordRefund(txCtx, refundTx); err != nil {
				return err
			}
			if err := s.disputeRepo.Update(txCtx, dispute); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}

	case domain.DisputeStatusResolvedDriverPayout:
		if err := dispute.ResolveDriverPayout(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}
		if err := escrow.Release(now); err != nil {
			return nil, err
		}

		if err := s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, escrow.Status); err != nil {
				return err
			}
			if err := s.disputeRepo.Update(txCtx, dispute); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}

	case domain.DisputeStatusRejected:
		if err := dispute.Reject(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}
		if err := escrow.RestoreHeld(now); err != nil {
			return nil, err
		}

		if err := s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, escrow.Status); err != nil {
				return err
			}
			if err := s.disputeRepo.Update(txCtx, dispute); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}

	default:
		return nil, domain.ErrInvalidDisputeData
	}

	return toDisputeDTO(dispute), nil
}

func toDisputeDTO(d *domain.Dispute) *ports.DisputeDTO {
	if d == nil {
		return nil
	}
	var resolvedAtStr *string
	if d.ResolvedAt != nil {
		str := d.ResolvedAt.UTC().Format(time.RFC3339)
		resolvedAtStr = &str
	}
	evidence := d.EvidenceURLs
	if evidence == nil {
		evidence = []string{}
	}
	return &ports.DisputeDTO{
		ID:                  d.ID,
		EscrowTransactionID: d.EscrowTransactionID,
		BookingID:           d.BookingID,
		TripID:              d.TripID,
		ReporterID:          d.ReporterID,
		DefendantID:         d.DefendantID,
		Reason:              d.Reason,
		Description:         d.Description,
		EvidenceURLs:        evidence,
		Status:              d.Status,
		AdminNotes:          d.AdminNotes,
		ResolvedBy:          d.ResolvedBy,
		ResolvedAt:          resolvedAtStr,
		CreatedAt:           d.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           d.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
