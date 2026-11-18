package services

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// PlatformPricingService gestiona la lectura, parametrización administrativa y cotización
// de comisiones (Passenger Service Fee) y retenciones operativas por cancelación.
type PlatformPricingService struct {
	pricingRepo ports.PricingConfigRepository
	tripRepo    ports.TripRepository
}

// NewPlatformPricingService construye el servicio de parametrización de tarifas y políticas de cancelación.
func NewPlatformPricingService(pricingRepo ports.PricingConfigRepository, optionalDeps ...any) *PlatformPricingService {
	svc := &PlatformPricingService{
		pricingRepo: pricingRepo,
	}
	for _, opt := range optionalDeps {
		if tr, ok := opt.(ports.TripRepository); ok {
			svc.tripRepo = tr
		}
	}
	return svc
}

// GetPricingConfig retorna la configuración activa de monetización y retenciones.
func (s *PlatformPricingService) GetPricingConfig(ctx context.Context) (*ports.PlatformPricingConfigDTO, error) {
	cfg, err := s.loadActiveConfig(ctx)
	if err != nil {
		return nil, err
	}
	return toPlatformPricingConfigDTO(&cfg), nil
}

// UpdatePricingConfig permite al Administrador parametrizar el % fee al pasajero y las reglas de retención en cancelaciones/reembolsos.
func (s *PlatformPricingService) UpdatePricingConfig(ctx context.Context, input ports.UpdatePlatformPricingConfigInput) (*ports.PlatformPricingConfigDTO, error) {
	if input.AdminID == uuid.Nil {
		return nil, domain.ErrUnauthorized
	}
	if s.pricingRepo == nil {
		return nil, domain.ErrInvalidPricingConfig
	}

	cfg, err := s.loadActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	if input.Currency != nil {
		parsedCur, err := domain.ParseCurrency(string(*input.Currency))
		if err != nil {
			return nil, domain.ErrInvalidPricingConfig
		}
		cfg.Currency = parsedCur
		cfg.PassengerFixedFee.Currency = parsedCur
	}
	if input.PassengerServiceFeeBps != nil {
		cfg.PassengerServiceFeeBps = *input.PassengerServiceFeeBps
	}
	if input.PassengerFixedFeeAmount != nil {
		cfg.PassengerFixedFee = domain.MoneyFromCents(*input.PassengerFixedFeeAmount, cfg.Currency)
	}
	if input.EarlyCancelFeeRetentionBps != nil {
		cfg.EarlyCancelFeeRetentionBps = *input.EarlyCancelFeeRetentionBps
	}
	if input.LateCancelWindowHours != nil {
		cfg.LateCancelWindowHours = *input.LateCancelWindowHours
	}
	if input.LateCancelFeeRetentionBps != nil {
		cfg.LateCancelFeeRetentionBps = *input.LateCancelFeeRetentionBps
	}
	if input.LateCancelPassengerRefundBps != nil {
		cfg.LateCancelPassengerRefundBps = *input.LateCancelPassengerRefundBps
	}
	if input.LateCancelDriverCompBps != nil {
		cfg.LateCancelDriverCompBps = *input.LateCancelDriverCompBps
	}
	if input.LateCancelPlatformRetentionBps != nil {
		cfg.LateCancelPlatformRetentionBps = *input.LateCancelPlatformRetentionBps
	}

	cfg.UpdatedBy = &input.AdminID

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	if err := s.pricingRepo.UpdateActive(ctx, &cfg); err != nil {
		return nil, err
	}

	return toPlatformPricingConfigDTO(&cfg), nil
}

// QuoteBookingPrice calcula en vivo el desglose financiero exacto (Base + Service Fee = Total) para un viaje y plazas dadas.
func (s *PlatformPricingService) QuoteBookingPrice(ctx context.Context, tripID uuid.UUID, seatsRequested int) (*ports.BookingPricingQuoteDTO, error) {
	if tripID == uuid.Nil || s.tripRepo == nil {
		return nil, domain.ErrTripNotFound
	}
	if seatsRequested <= 0 {
		return nil, domain.ErrInvalidSeatCount
	}

	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	cfg, err := s.loadActiveConfig(ctx)
	if err != nil {
		return nil, err
	}

	breakdown, err := cfg.CalculateBookingBreakdown(trip.PricePerSeat, seatsRequested)
	if err != nil {
		return nil, err
	}

	return &ports.BookingPricingQuoteDTO{
		TripID:            trip.ID,
		SeatsRequested:    seatsRequested,
		Currency:          breakdown.Currency,
		UnitPrice:         breakdown.UnitPrice,
		BaseAmount:        breakdown.BaseAmount,
		PlatformFee:       breakdown.PlatformFee,
		TotalPrice:        breakdown.TotalPrice,
		DriverPayout:      breakdown.DriverPayout,
		ServiceFeeBps:     breakdown.AppliedFeeBps,
		ServiceFeePercent: domain.BpsToPercent(breakdown.AppliedFeeBps),
		FixedFee:          breakdown.AppliedFixedFee,
	}, nil
}

func (s *PlatformPricingService) loadActiveConfig(ctx context.Context) (domain.PlatformPricingConfig, error) {
	if s.pricingRepo == nil {
		return domain.DefaultPlatformPricingConfig(), nil
	}
	active, err := s.pricingRepo.GetActive(ctx)
	if err != nil {
		return domain.PlatformPricingConfig{}, err
	}
	if active == nil {
		return domain.DefaultPlatformPricingConfig(), nil
	}
	return *active, nil
}

func toPlatformPricingConfigDTO(cfg *domain.PlatformPricingConfig) *ports.PlatformPricingConfigDTO {
	if cfg == nil {
		return nil
	}
	return &ports.PlatformPricingConfigDTO{
		ID:                                 cfg.ID,
		Currency:                           cfg.Currency,
		PassengerServiceFeeBps:             cfg.PassengerServiceFeeBps,
		PassengerServiceFeePercent:         domain.BpsToPercent(cfg.PassengerServiceFeeBps),
		PassengerFixedFee:                  cfg.PassengerFixedFee,
		EarlyCancelFeeRetentionBps:         cfg.EarlyCancelFeeRetentionBps,
		EarlyCancelFeeRetentionPercent:     domain.BpsToPercent(cfg.EarlyCancelFeeRetentionBps),
		LateCancelWindowHours:              cfg.LateCancelWindowHours,
		LateCancelFeeRetentionBps:          cfg.LateCancelFeeRetentionBps,
		LateCancelFeeRetentionPercent:      domain.BpsToPercent(cfg.LateCancelFeeRetentionBps),
		LateCancelPassengerRefundBps:       cfg.LateCancelPassengerRefundBps,
		LateCancelPassengerRefundPercent:   domain.BpsToPercent(cfg.LateCancelPassengerRefundBps),
		LateCancelDriverCompBps:            cfg.LateCancelDriverCompBps,
		LateCancelDriverCompPercent:        domain.BpsToPercent(cfg.LateCancelDriverCompBps),
		LateCancelPlatformRetentionBps:     cfg.LateCancelPlatformRetentionBps,
		LateCancelPlatformRetentionPercent: domain.BpsToPercent(cfg.LateCancelPlatformRetentionBps),
		IsActive:                           cfg.IsActive,
		UpdatedBy:                          cfg.UpdatedBy,
		UpdatedAt:                          cfg.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
