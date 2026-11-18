package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/services"
)

func TestBookingExpirationService_ProcessExpiredBookings(t *testing.T) {
	ctx := context.Background()

	t.Run("expiración masiva delega reglas al dominio y aplica cambios en lote", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		txManager := new(MockTransactionManager)

		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		tripFull := createTestTrip(testDriverID, 0, 48) // Status == FULL, AvailableSeats == 0
		expiredBooking1, _ := domain.NewBooking(domain.BookingParams{
			ID:          uuid.MustParse("01960000-0000-7000-8000-000000000801"),
			TripID:      tripFull.ID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
			TTL:         1 * time.Nanosecond,
		})
		expiredBooking2, _ := domain.NewBooking(domain.BookingParams{
			ID:          uuid.MustParse("01960000-0000-7000-8000-000000000802"),
			TripID:      tripFull.ID,
			PassengerID: uuid.MustParse("01960000-0000-7000-8000-000000000405"),
			DriverID:    testDriverID,
			SeatsBooked: 1,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
			TTL:         1 * time.Nanosecond,
		})
		time.Sleep(5 * time.Millisecond)

		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("LockExpiredPendingBookings", mock.MatchedBy(hasTx), domain.BookingStatusPendingPayment, mock.AnythingOfType("time.Time"), 500).
			Return([]*domain.Booking{expiredBooking1, expiredBooking2}, nil)
		bookingRepo.On("MarkBookingsExpiredBatch", mock.MatchedBy(hasTx), []uuid.UUID{expiredBooking1.ID, expiredBooking2.ID}, domain.BookingStatusPendingPayment, domain.BookingStatusExpired, mock.AnythingOfType("time.Time")).
			Return(2, nil)
		tripRepo.On("FindByIDsForUpdate", mock.MatchedBy(hasTx), []uuid.UUID{tripFull.ID}).
			Return([]*domain.Trip{tripFull}, nil)
		tripRepo.On("ApplySeatTransitionsBatch", mock.MatchedBy(hasTx), mock.MatchedBy(func(transitions []domain.SeatTransition) bool {
			return len(transitions) == 1 &&
				transitions[0].TripID == tripFull.ID &&
				transitions[0].SeatsDelta == 3 &&
				transitions[0].ExpectedStatus == domain.TripStatusFull &&
				transitions[0].NewStatus == domain.TripStatusPublished
		}), mock.AnythingOfType("time.Time")).Return(1, nil)

		service := services.NewBookingExpirationService(bookingRepo, tripRepo, txManager)

		expiredCount, err := service.ProcessExpiredBookings(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, expiredCount)
		assert.Equal(t, domain.TripStatusPublished, tripFull.Status)
		assert.Equal(t, 3, tripFull.AvailableSeats)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		tripRepo.AssertExpectations(t)
	})
}
