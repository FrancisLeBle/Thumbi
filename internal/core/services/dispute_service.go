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

// DisputeSettlementService define el subconjunto de operaciones financieras de custodia y reembolso
// requeridas por la gestión y arbitraje de disputas (SOLID · ISP).
type DisputeSettlementService interface {
	GetEscrowByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error)
	FreezeEscrowForDispute(ctx context.Context, escrow *domain.EscrowTransaction) error
	ReleaseEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error
	RestoreEscrowHeld(ctx context.Context, escrow *domain.EscrowTransaction) error
	SettleDisputeRefund(ctx context.Context, input DisputeRefundInput, withinTxHooks ...func(txCtx context.Context) error) (*domain.RefundTransaction, error)
}

// DisputeService implementa los casos de uso para apertura, consulta y arbitraje de disputas,
// delegando toda la lógica de custodia y reembolsos en SettlementService.
type DisputeService struct {
	disputeRepo ports.DisputeRepository
	bookingRepo ports.BookingRepository
	tripRepo    ports.TripRepository
	settlement  DisputeSettlementService
	txManager   ports.TransactionManager
	clock       func() time.Time
}

// NewDisputeService crea una instancia de DisputeService con soporte opcional de TransactionManager.
func NewDisputeService(
	disputeRepo ports.DisputeRepository,
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	settlement DisputeSettlementService,
	optionalDeps ...any,
) *DisputeService {
	svc := &DisputeService{
		disputeRepo: disputeRepo,
		bookingRepo: bookingRepo,
		tripRepo:    tripRepo,
		settlement:  settlement,
		clock:       time.Now,
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

// OpenDispute valida la legitimidad del reclamo, congela el escrow en DISPUTED y persiste la disputa en una transacción.
func (s *DisputeService) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	if input.BookingID == uuid.Nil ||
		input.ReporterID == uuid.Nil ||
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

	var defendantID uuid.UUID
	switch input.ReporterID {
	case booking.PassengerID:
		defendantID = trip.DriverID
	case trip.DriverID:
		defendantID = booking.PassengerID
	default:
		return nil, domain.ErrUnauthorized
	}

	if s.settlement == nil {
		return nil, domain.ErrEscrowNotFound
	}
	escrow, err := s.settlement.GetEscrowByBookingID(ctx, booking.ID)
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
	} else if !reason.IsValid() {
		return nil, domain.ErrInvalidDisputeData
	}

	dispute, err := domain.NewDispute(domain.DisputeParams{
		ID:                  domain.NewUUIDv7(),
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

	if err := s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.settlement.FreezeEscrowForDispute(txCtx, escrow); err != nil {
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

// GetDisputeByID recupera el detalle de una disputa por su ID.
func (s *DisputeService) GetDisputeByID(ctx context.Context, disputeID uuid.UUID) (*ports.DisputeDTO, error) {
	if disputeID == uuid.Nil {
		return nil, domain.ErrInvalidDisputeData
	}

	dispute, err := s.disputeRepo.FindByID(ctx, disputeID)
	if err != nil {
		return nil, err
	}
	return toDisputeDTO(dispute), nil
}

// ListDisputes consulta las disputas de forma paginada usando limit y offset.
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

// ResolveDispute dictamina una resolución administrativa válida y delega la liquidación o reembolso a SettlementService.
func (s *DisputeService) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	if input.DisputeID == uuid.Nil ||
		input.AdminID == uuid.Nil ||
		!input.Resolution.IsValidResolution() {
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

	if s.settlement == nil {
		return nil, domain.ErrEscrowNotFound
	}
	escrow, err := s.settlement.GetEscrowByBookingID(ctx, dispute.BookingID)
	if err != nil {
		return nil, err
	}

	now := s.clock().UTC()

	switch input.Resolution {
	case domain.DisputeStatusResolvedPassengerRefund:
		if err := dispute.ResolvePassengerRefund(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}

		reason := fmt.Sprintf("Resolución de disputa %s: %s", dispute.ID.String(), strings.TrimSpace(input.AdminNotes))
		if _, err := s.settlement.SettleDisputeRefund(
			ctx,
			DisputeRefundInput{Escrow: escrow, Reason: reason},
			func(txCtx context.Context) error {
				return s.disputeRepo.Update(txCtx, dispute)
			},
		); err != nil {
			return nil, err
		}

	case domain.DisputeStatusResolvedDriverPayout:
		if err := dispute.ResolveDriverPayout(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}

		if err := s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.settlement.ReleaseEscrow(txCtx, escrow); err != nil {
				return err
			}
			return s.disputeRepo.Update(txCtx, dispute)
		}); err != nil {
			return nil, err
		}

	case domain.DisputeStatusRejected:
		if err := dispute.Reject(input.AdminID, input.AdminNotes, now); err != nil {
			return nil, err
		}

		if err := s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.settlement.RestoreEscrowHeld(txCtx, escrow); err != nil {
				return err
			}
			return s.disputeRepo.Update(txCtx, dispute)
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
