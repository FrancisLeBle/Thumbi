package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thumbi/backend/internal/worker"
)

type MockBookingExpirationService struct {
	mock.Mock
}

func (m *MockBookingExpirationService) ProcessExpiredBookings(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestExpirationWorker_Lifecycle(t *testing.T) {
	mockService := new(MockBookingExpirationService)
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

func TestExpirationWorker_DrainsMultipleBatchesWhenFull(t *testing.T) {
	mockService := new(MockBookingExpirationService)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// Primera llamada retorna un lote lleno (500); segunda retorna 42, detiene el drenado y finaliza el worker
	mockService.On("ProcessExpiredBookings", mock.Anything).Return(worker.ExpirationBatchSize, nil).Once()
	mockService.On("ProcessExpiredBookings", mock.Anything).Run(func(args mock.Arguments) {
		cancel()
	}).Return(42, nil).Once()

	w := worker.NewExpirationWorker(mockService, 10*time.Millisecond)
	w.Start(ctx)

	mockService.AssertExpectations(t)
}
