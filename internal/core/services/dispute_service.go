package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type DisputeService struct {
	disputeRepo    ports.DisputeRepository
	escrowRepo     ports.EscrowRepository
	bookingRepo    ports.BookingRepository
	tripRepo       ports.TripRepository
	paymentGateway ports.PaymentGateway
}

func NewDisputeService(
	disputeRepo ports.DisputeRepository,
	escrowRepo ports.EscrowRepository,
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	paymentGateway ports.PaymentGateway,
) ports.DisputeService {
	return &DisputeService{
		disputeRepo:    disputeRepo,
		escrowRepo:     escrowRepo,
		bookingRepo:    bookingRepo,
		tripRepo:       tripRepo,
		paymentGateway: paymentGateway,
	}
}

func (s *DisputeService) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	now := time.Now()
	dispute := &domain.Dispute{
		ID:          uuid.New().String(),
		BookingID:   input.BookingID,
		Reason:      input.Reason,
		Description: input.Description,
		Status:      "OPEN",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.disputeRepo.Save(ctx, dispute); err != nil {
		return nil, err
	}

	return &ports.DisputeDTO{
		ID:          dispute.ID,
		BookingID:   dispute.BookingID,
		Reason:      dispute.Reason,
		Description: dispute.Description,
		Status:      dispute.Status,
		CreatedAt:   dispute.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dispute.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *DisputeService) GetDisputeByID(ctx context.Context, disputeID string) (*ports.DisputeDTO, error) {
	dispute, err := s.disputeRepo.FindByID(ctx, disputeID)
	if err != nil {
		return nil, err
	}
	return &ports.DisputeDTO{
		ID:          dispute.ID,
		BookingID:   dispute.BookingID,
		Reason:      dispute.Reason,
		Description: dispute.Description,
		Status:      dispute.Status,
		CreatedAt:   dispute.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dispute.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *DisputeService) ListDisputes(ctx context.Context, page int, limit int) ([]*ports.DisputeDTO, error) {
	return nil, nil
}

func (s *DisputeService) ListDisputesByBooking(ctx context.Context, bookingID string) ([]*domain.Dispute, error) {
	dispute, err := s.disputeRepo.FindByID(ctx, bookingID)
	if err != nil {
		return []*domain.Dispute{}, nil
	}
	return []*domain.Dispute{dispute}, nil
}

func (s *DisputeService) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	dispute, err := s.disputeRepo.FindByID(ctx, input.DisputeID)
	if err != nil {
		return nil, err
	}

	if dispute.Status == "RESOLVED" || dispute.Status == "REJECTED" {
		return nil, domain.ErrDisputeAlreadyResolved
	}

	now := time.Now()
	dispute.Status = input.Resolution
	dispute.ResolvedAt = &now
	dispute.UpdatedAt = now

	if err := s.disputeRepo.Save(ctx, dispute); err != nil {
		return nil, err
	}

	return &ports.DisputeDTO{
		ID:          dispute.ID,
		BookingID:   dispute.BookingID,
		Reason:      dispute.Reason,
		Description: dispute.Description,
		Status:      dispute.Status,
		CreatedAt:   dispute.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   dispute.UpdatedAt.Format(time.RFC3339),
	}, nil
}
