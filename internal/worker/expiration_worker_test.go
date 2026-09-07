package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/worker"
)

type MockBookingService struct {
	mock.Mock
}

func (m *MockBookingService) CreateBooking(ctx context.Context, input ports.CreateBookingInput) (*ports.BookingDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ConfirmBookingPayment(ctx context.Context, input ports.ConfirmBookingPaymentInput) (*ports.BookingDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*domain.RefundTransaction, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefundTransaction), args.Error(1)
}

func (m *MockBookingService) GetBookingByID(ctx context.Context, bookingID, requestingUserID string) (*ports.BookingDTO, error) {
	args := m.Called(ctx, bookingID, requestingUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ListPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*ports.BookingDTO, error) {
	args := m.Called(ctx, passengerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ListTripBookings(ctx context.Context, tripID, driverID string) ([]*ports.BookingDTO, error) {
	args := m.Called(ctx, tripID, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ProcessExpiredBookings(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestExpirationWorker_Lifecycle(t *testing.T) {
	mockService := new(MockBookingService)
	mockService.On("ProcessExpiredBookings", mock.Anything).Return(3, nil)

	w := worker.NewExpirationWorker(mockService, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Start(ctx)
		close(done)
	}()

	<-done
	assert.True(t, len(mockService.Calls) >= 1)
}
