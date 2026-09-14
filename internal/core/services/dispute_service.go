package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// disputeService implementa ports.DisputeService bajo Clean Architecture
type disputeService struct {
	disputeRepo    ports.DisputeRepository
	escrowRepo     ports.EscrowRepository
	bookingRepo    ports.BookingRepository
	tripRepo       ports.TripRepository
	paymentGateway ports.PaymentGateway
	clock          func() time.Time
}

// NewDisputeService construye una nueva instancia inyectando los puertos requeridos
func NewDisputeService(
	disputeRepo ports.DisputeRepository,
	escrowRepo ports.EscrowRepository,
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	paymentGateway ports.PaymentGateway,
) ports.DisputeService {
	return &disputeService{
		disputeRepo:    disputeRepo,
		escrowRepo:     escrowRepo,
		bookingRepo:    bookingRepo,
		tripRepo:       tripRepo,
		paymentGateway: paymentGateway,
		clock:          time.Now,
	}
}

// OpenDispute registra una reclamación sobre una transacción en Escrow y congela los fondos (RF-14 y RF-15)
func (s *disputeService) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	now := s.clock().UTC()

	bookingID := strings.TrimSpace(input.BookingID)
	reporterID := strings.TrimSpace(input.ReporterID)
	description := strings.TrimSpace(input.Description)

	if bookingID == "" || reporterID == "" || description == "" {
		return nil, domain.ErrInvalidDisputeData
	}

	// 1. Validar que la reserva exista
	booking, err := s.bookingRepo.FindByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	// 2. Validar que el viaje exista
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 3. Determinar la contraparte (defendant) y validar autorización del denunciante
	var defendantID string
	if reporterID == booking.PassengerID {
		defendantID = trip.DriverID
	} else if reporterID == trip.DriverID {
		defendantID = booking.PassengerID
	} else {
		return nil, domain.ErrUnauthorized
	}

	if reporterID == defendantID {
		return nil, domain.ErrDisputeSelfNotAllowed
	}

	// 4. Obtener la transacción de fondos en custodia (Escrow)
	escrow, err := s.escrowRepo.FindByBookingID(ctx, booking.ID)
	if err != nil || escrow == nil {
		return nil, domain.ErrEscrowNotFound
	}

	// 5. Validar que el Escrow no esté previamente liquidado o reembolsado (RF-17)
	if escrow.Status != domain.EscrowStatusHeld {
		return nil, domain.ErrEscrowAlreadySettled
	}

	// 6. Validar que no exista ya una disputa abierta sobre este Escrow (RF-18)
	existingDispute, err := s.disputeRepo.FindByEscrowID(ctx, escrow.ID)
	if err == nil && existingDispute != nil {
		return nil, domain.ErrDisputeAlreadyExists
	}

	// 7. Instanciar la entidad Dispute
	disputeID := uuid.New().String()
	dispute, err := domain.NewDispute(domain.DisputeParams{
		ID:                  disputeID,
		EscrowTransactionID: escrow.ID,
		BookingID:           booking.ID,
		TripID:              trip.ID,
		ReporterID:          reporterID,
		DefendantID:         defendantID,
		Reason:              input.Reason,
		Description:         description,
		EvidenceURLs:        input.EvidenceURLs,
		EscrowStatus:        escrow.Status,
	})
	if err != nil {
		return nil, err
	}

	// 8. Congelar la transacción de Escrow transicionándola a DISPUTED (RF-15)
	if err := escrow.OpenDispute(now); err != nil {
		return nil, err
	}
	if err := s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusDisputed); err != nil {
		return nil, err
	}

	// 9. Persistir la disputa
	if err := s.disputeRepo.Save(ctx, dispute); err != nil {
		return nil, err
	}

	return toDisputeDTO(dispute), nil
}

// ResolveDispute arbitra administrativamente una disputa resolviendo la custodia y pasarela de pagos (RF-19 y RF-20)
func (s *disputeService) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	now := s.clock().UTC()

	disputeID := strings.TrimSpace(input.DisputeID)
	adminID := strings.TrimSpace(input.AdminID)
	notes := strings.TrimSpace(input.AdminNotes)

	if disputeID == "" || adminID == "" {
		return nil, domain.ErrInvalidDisputeData
	}

	// 1. Obtener la disputa
	dispute, err := s.disputeRepo.FindByID(ctx, disputeID)
	if err != nil || dispute == nil {
		return nil, domain.ErrDisputeNotFound
	}

	if dispute.IsClosed() {
		return nil, domain.ErrDisputeAlreadyResolved
	}

	// 2. Obtener el Escrow asociado
	escrow, err := s.escrowRepo.FindByBookingID(ctx, dispute.BookingID)
	if err != nil || escrow == nil {
		return nil, domain.ErrEscrowNotFound
	}

	// 3. Procesar resolución vinculante según dictamen
	switch input.Resolution {
	case domain.DisputeStatusResolvedPassengerRefund:
		// Dictamen a favor del pasajero: Reembolso 100%
		if err := dispute.ResolvePassengerRefund(adminID, notes, now); err != nil {
			return nil, err
		}
		if err := escrow.RefundFull(now); err != nil {
			return nil, err
		}
		if err := s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusRefundedFull); err != nil {
			return nil, err
		}

		// Reintegro en la pasarela de pagos
		gatewayRefundRef := ""
		if s.paymentGateway != nil && escrow.PaymentGatewayRef != "" {
			refundResult, _ := s.paymentGateway.ProcessRefund(ctx, escrow.PaymentGatewayRef, escrow.Amount, "Resolución de disputa favorable al pasajero")
			if refundResult != nil {
				gatewayRefundRef = refundResult.GatewayRefundRef
			}
		}

		// Registrar auditoría de reembolso
		refundTx, err := domain.NewRefundTransaction(
			dispute.ReporterID,
			dispute.ID,
			dispute.BookingID,
			dispute.Amount,
			0.0,
			domain.RefundTypeFull,
			input.AdminNotes,
		)
		if err != nil {
			return nil, err
		}
		_ = s.escrowRepo.RecordRefund(ctx, refundTx)

	case domain.DisputeStatusResolvedDriverPayout:
		// Dictamen a favor del conductor: Liberación de fondos
		if err := dispute.ResolveDriverPayout(adminID, notes, now); err != nil {
			return nil, err
		}
		if err := escrow.Release(now); err != nil {
			return nil, err
		}
		if err := s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusReleased); err != nil {
			return nil, err
		}

	case domain.DisputeStatusRejected:
		// Dictamen desestimado: Se devuelve la custodia al estado HELD
		if err := dispute.Reject(adminID, notes, now); err != nil {
			return nil, err
		}
		escrow.Status = domain.EscrowStatusHeld
		if err := s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusHeld); err != nil {
			return nil, err
		}

	default:
		return nil, domain.ErrInvalidDisputeData
	}

	// 4. Persistir cambios en la disputa
	if err := s.disputeRepo.Update(ctx, dispute); err != nil {
		return nil, err
	}

	return toDisputeDTO(dispute), nil
}

// GetDisputeByID consulta el detalle de una disputa por su ID
func (s *disputeService) GetDisputeByID(ctx context.Context, disputeID string) (*ports.DisputeDTO, error) {
	trimmedID := strings.TrimSpace(disputeID)
	if trimmedID == "" {
		return nil, domain.ErrDisputeNotFound
	}

	dispute, err := s.disputeRepo.FindByID(ctx, trimmedID)
	if err != nil || dispute == nil {
		return nil, domain.ErrDisputeNotFound
	}

	return toDisputeDTO(dispute), nil
}

// ListDisputes lista las disputas con soporte para paginación
func (s *disputeService) ListDisputes(ctx context.Context, limit, offset int) ([]*ports.DisputeDTO, error) {
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

	dtos := make([]*ports.DisputeDTO, 0, len(disputes))
	for _, d := range disputes {
		dtos = append(dtos, toDisputeDTO(d))
	}
	return dtos, nil
}

// toDisputeDTO función auxiliar de mapeo
func toDisputeDTO(d *domain.Dispute) *ports.DisputeDTO {
	dto := &ports.DisputeDTO{
		ID:                  d.ID,
		EscrowTransactionID: d.EscrowTransactionID,
		BookingID:           d.BookingID,
		TripID:              d.TripID,
		ReporterID:          d.ReporterID,
		DefendantID:         d.DefendantID,
		Reason:              d.Reason,
		Description:         d.Description,
		EvidenceURLs:        d.EvidenceURLs,
		Status:              d.Status,
		AdminNotes:          d.AdminNotes,
		ResolvedBy:          d.ResolvedBy,
		CreatedAt:           d.CreatedAt.Format(time.RFC3339),
		UpdatedAt:           d.UpdatedAt.Format(time.RFC3339),
	}
	if d.ResolvedAt != nil {
		formatted := d.ResolvedAt.Format(time.RFC3339)
		dto.ResolvedAt = &formatted
	}
	return dto
}
