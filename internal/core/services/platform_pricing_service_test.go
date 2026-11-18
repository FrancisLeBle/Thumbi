package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func TestPlatformPricingService(t *testing.T) {
	ctx := context.Background()
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000009001")

	t.Run("GetPricingConfig retorna configuración activa del repositorio", func(t *testing.T) {
		pricingRepo := new(MockPricingConfigRepository)
		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 1250

		pricingRepo.On("GetActive", ctx).Return(&cfg, nil)
		svc := services.NewPlatformPricingService(pricingRepo)

		dto, err := svc.GetPricingConfig(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1250), dto.PassengerServiceFeeBps)
		assert.InDelta(t, 12.5, dto.PassengerServiceFeePercent, 0.0001)
		pricingRepo.AssertExpectations(t)
	})

	t.Run("UpdatePricingConfig permite parametrizar % fee y retención operativa", func(t *testing.T) {
		pricingRepo := new(MockPricingConfigRepository)
		currentCfg := domain.DefaultPlatformPricingConfig()

		pricingRepo.On("GetActive", ctx).Return(&currentCfg, nil)
		pricingRepo.On("UpdateActive", ctx, mock.MatchedBy(func(c *domain.PlatformPricingConfig) bool {
			return c.PassengerServiceFeeBps == 1800 &&
				c.PassengerFixedFee.Amount == 5000 &&
				c.EarlyCancelFeeRetentionBps == 5000 &&
				c.LateCancelWindowHours == 12 &&
				c.LateCancelPassengerRefundBps == 4000 &&
				c.LateCancelDriverCompBps == 4000 &&
				c.LateCancelPlatformRetentionBps == 2000 &&
				c.UpdatedBy != nil && *c.UpdatedBy == adminID
		})).Return(nil)

		svc := services.NewPlatformPricingService(pricingRepo)

		newFeeBps := int64(1800)
		newFixedCents := int64(5000)
		newEarlyRetBps := int64(5000)
		newLateWindow := 12
		newLatePassBps := int64(4000)
		newLateDriverBps := int64(4000)
		newLatePlatBps := int64(2000)

		dto, err := svc.UpdatePricingConfig(ctx, ports.UpdatePlatformPricingConfigInput{
			AdminID:                        adminID,
			PassengerServiceFeeBps:         &newFeeBps,
			PassengerFixedFeeAmount:        &newFixedCents,
			EarlyCancelFeeRetentionBps:     &newEarlyRetBps,
			LateCancelWindowHours:          &newLateWindow,
			LateCancelPassengerRefundBps:   &newLatePassBps,
			LateCancelDriverCompBps:        &newLateDriverBps,
			LateCancelPlatformRetentionBps: &newLatePlatBps,
		})

		require.NoError(t, err)
		assert.Equal(t, int64(1800), dto.PassengerServiceFeeBps)
		assert.InDelta(t, 18.0, dto.PassengerServiceFeePercent, 0.0001)
		assert.Equal(t, int64(5000), dto.PassengerFixedFee.Amount)
		assert.Equal(t, 12, dto.LateCancelWindowHours)
		assert.Equal(t, int64(2000), dto.LateCancelPlatformRetentionBps)
		pricingRepo.AssertExpectations(t)
	})

	t.Run("UpdatePricingConfig rechaza admin nulo con ErrUnauthorized", func(t *testing.T) {
		pricingRepo := new(MockPricingConfigRepository)
		svc := services.NewPlatformPricingService(pricingRepo)

		dto, err := svc.UpdatePricingConfig(ctx, ports.UpdatePlatformPricingConfigInput{
			AdminID: uuid.Nil,
		})

		assert.Nil(t, dto)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("UpdatePricingConfig rechaza reparto por cancelación tardía que no sume 100%", func(t *testing.T) {
		pricingRepo := new(MockPricingConfigRepository)
		currentCfg := domain.DefaultPlatformPricingConfig()
		pricingRepo.On("GetActive", ctx).Return(&currentCfg, nil)

		svc := services.NewPlatformPricingService(pricingRepo)
		invalidPlatBps := int64(3000) // 5000 + 5000 + 3000 != 10000

		dto, err := svc.UpdatePricingConfig(ctx, ports.UpdatePlatformPricingConfigInput{
			AdminID:                        adminID,
			LateCancelPlatformRetentionBps: &invalidPlatBps,
		})

		assert.Nil(t, dto)
		assert.ErrorIs(t, err, domain.ErrInvalidPricingConfig)
		pricingRepo.AssertNotCalled(t, "UpdateActive", mock.Anything, mock.Anything)
	})

	t.Run("QuoteBookingPrice calcula el desglose transparente para checkout", func(t *testing.T) {
		pricingRepo := new(MockPricingConfigRepository)
		tripRepo := new(MockTripRepository)

		trip := createTestTrip(testDriverID, 3, 24)
		cfg := domain.DefaultPlatformPricingConfig() // 15% service fee

		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		pricingRepo.On("GetActive", ctx).Return(&cfg, nil)

		svc := services.NewPlatformPricingService(pricingRepo, tripRepo)
		quote, err := svc.QuoteBookingPrice(ctx, testTripID, 2)

		require.NoError(t, err)
		assert.Equal(t, testTripID, quote.TripID)
		assert.Equal(t, 2, quote.SeatsRequested)
		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), quote.BaseAmount)
		assert.Equal(t, domain.MoneyFromCents(45000, domain.CurrencyARS), quote.PlatformFee)
		assert.Equal(t, domain.MoneyFromCents(345000, domain.CurrencyARS), quote.TotalPrice)
		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), quote.DriverPayout)
		assert.Equal(t, int64(1500), quote.ServiceFeeBps)
		assert.InDelta(t, 15.0, quote.ServiceFeePercent, 0.0001)
		tripRepo.AssertExpectations(t)
		pricingRepo.AssertExpectations(t)
	})
}
