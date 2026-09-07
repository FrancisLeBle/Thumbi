package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestBooking_CreationAndInvariants(t *testing.T) {
	t.Run("crea exitosamente una reserva en estado PENDING_PAYMENT con TTL de 15 minutos", func(t *testing.T) {
		now := time.Now().UTC()
		params := domain.BookingParams{
			ID:          "booking-123",
			TripID:      "trip-456",
			PassengerID: "passenger-789",
			DriverID:    "driver-001",
			SeatsBooked: 2,
			UnitPrice:   1500.0,
			TTL:         15 * time.Minute,
		}

		b, err := domain.NewBooking(params)
		require.NoError(t, err)
		assert.Equal(t, "booking-123", b.ID)
		assert.Equal(t, "trip-456", b.TripID)
		assert.Equal(t, "passenger-789", b.PassengerID)
		assert.Equal(t, 2, b.SeatsBooked)
		assert.Equal(t, 1500.0, b.UnitPrice)
		assert.Equal(t, 3000.0, b.TotalPrice)
		assert.Equal(t, domain.BookingStatusPendingPayment, b.Status)
		assert.True(t, b.IsActive())
		assert.WithinDuration(t, now.Add(15*time.Minute), b.ExpiresAt, 2*time.Second)
	})

	t.Run("falla si el pasajero es el mismo conductor del viaje (Invariante RF-02)", func(t *testing.T) {
		params := domain.BookingParams{
			ID:          "booking-123",
			TripID:      "trip-456",
			PassengerID: "user-same",
			DriverID:    "user-same",
			SeatsBooked: 1,
			UnitPrice:   1500.0,
		}

		b, err := domain.NewBooking(params)
		assert.Nil(t, b)
		assert.ErrorIs(t, err, domain.ErrCannotBookOwnTrip)
	})

	t.Run("falla si la cantidad de asientos solicitada es menor o igual a cero", func(t *testing.T) {
		params := domain.BookingParams{
			ID:          "booking-123",
			TripID:      "trip-456",
			PassengerID: "passenger-789",
			DriverID:    "driver-001",
			SeatsBooked: 0,
			UnitPrice:   1500.0,
		}

		b, err := domain.NewBooking(params)
		assert.Nil(t, b)
		assert.ErrorIs(t, err, domain.ErrInvalidSeatCount)
	})

	t.Run("falla si el precio unitario es negativo", func(t *testing.T) {
		params := domain.BookingParams{
			ID:          "booking-123",
			TripID:      "trip-456",
			PassengerID: "passenger-789",
			DriverID:    "driver-001",
			SeatsBooked: 1,
			UnitPrice:   -100.0,
		}

		b, err := domain.NewBooking(params)
		assert.Nil(t, b)
		assert.ErrorIs(t, err, domain.ErrPriceExceedsCapPrice)
	})
}

func TestBooking_LifeCycleTransitions(t *testing.T) {
	createValidBooking := func(ttl time.Duration) *domain.Booking {
		b, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-1",
			TripID:      "trip-1",
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 1,
			UnitPrice:   2000.0,
			TTL:         ttl,
		})
		return b
	}

	t.Run("confirma exitosamente la reserva dentro del plazo TTL", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		now := time.Now().UTC()

		assert.True(t, b.CanBeConfirmed(now))
		err := b.Confirm(now)
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusConfirmed, b.Status)
		assert.NotNil(t, b.ConfirmedAt)
		assert.True(t, b.IsActive())
	})

	t.Run("rechaza confirmación si la reserva expiró por tiempo límite", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		pastExpiry := time.Now().UTC().Add(16 * time.Minute)

		assert.True(t, b.IsExpired(pastExpiry))
		assert.False(t, b.CanBeConfirmed(pastExpiry))
		err := b.Confirm(pastExpiry)
		assert.ErrorIs(t, err, domain.ErrBookingExpired)
	})

	t.Run("expiración manual de la reserva", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		now := time.Now().UTC()

		err := b.Expire(now)
		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusExpired, b.Status)
		assert.False(t, b.IsActive())

		// No puede confirmarse una reserva expirada
		errConfirm := b.Confirm(now)
		assert.ErrorIs(t, errConfirm, domain.ErrInvalidBookingStatus)
	})

	t.Run("cancelación por pasajero de reserva confirmada", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		now := time.Now().UTC()
		require.NoError(t, b.Confirm(now))

		errCancel := b.CancelByPassenger("Cambio de planes de viaje", now.Add(5*time.Minute))
		require.NoError(t, errCancel)
		assert.Equal(t, domain.BookingStatusCancelledByPassenger, b.Status)
		assert.NotNil(t, b.CancelledAt)
		assert.Equal(t, "Cambio de planes de viaje", *b.CancellationReason)
		assert.False(t, b.IsActive())
	})

	t.Run("cancelación por conductor ante baja del viaje", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		now := time.Now().UTC()
		require.NoError(t, b.Confirm(now))

		errCancel := b.CancelByDriver("Avería mecánica imprevista", now.Add(10*time.Minute))
		require.NoError(t, errCancel)
		assert.Equal(t, domain.BookingStatusCancelledByDriver, b.Status)
		assert.NotNil(t, b.CancelledAt)
		assert.Equal(t, "Avería mecánica imprevista", *b.CancellationReason)
	})

	t.Run("finalización exitosa tras concluir el viaje", func(t *testing.T) {
		b := createValidBooking(15 * time.Minute)
		now := time.Now().UTC()
		require.NoError(t, b.Confirm(now))

		errComplete := b.MarkCompleted(now.Add(2 * time.Hour))
		require.NoError(t, errComplete)
		assert.Equal(t, domain.BookingStatusCompleted, b.Status)

		// No puede cancelarse una reserva ya completada
		errCancel := b.CancelByPassenger("Intento cancelar después del viaje", now.Add(3*time.Hour))
		assert.ErrorIs(t, errCancel, domain.ErrBookingCannotBeCancelled)
	})
}

func TestEscrow_Lifecycle(t *testing.T) {
	t.Run("crea transacción de custodia en estado HELD y liquida fondos a RELEASED", func(t *testing.T) {
		now := time.Now().UTC()
		escrow, err := domain.NewEscrowTransaction(
			"escrow-1",
			"booking-1",
			"trip-1",
			"passenger-1",
			"driver-1",
			5000.0,
			"ARS",
			"pay_intent_999",
		)
		require.NoError(t, err)
		assert.Equal(t, domain.EscrowStatusHeld, escrow.Status)
		assert.Equal(t, 5000.0, escrow.Amount)

		// Liberar fondos al conductor al finalizar viaje
		errRelease := escrow.Release(now.Add(3 * time.Hour))
		require.NoError(t, errRelease)
		assert.Equal(t, domain.EscrowStatusReleased, escrow.Status)
		assert.NotNil(t, escrow.ReleasedAt)

		// Intentar liberar nuevamente debe arrojar error
		errSecondRelease := escrow.Release(now.Add(4 * time.Hour))
		assert.ErrorIs(t, errSecondRelease, domain.ErrEscrowAlreadySettled)
	})

	t.Run("reembolso total y apertura de disputa", func(t *testing.T) {
		now := time.Now().UTC()
		escrow, err := domain.NewEscrowTransaction(
			"escrow-2",
			"booking-2",
			"trip-2",
			"passenger-2",
			"driver-2",
			3500.0,
			"USD",
			"pay_intent_888",
		)
		require.NoError(t, err)

		// Abrir disputa congela fondos
		errDispute := escrow.OpenDispute(now)
		require.NoError(t, errDispute)
		assert.Equal(t, domain.EscrowStatusDisputed, escrow.Status)

		// Desde disputa se puede dictaminar reembolso total
		errRefund := escrow.RefundFull(now.Add(1 * time.Hour))
		require.NoError(t, errRefund)
		assert.Equal(t, domain.EscrowStatusRefundedFull, escrow.Status)
	})
}

func TestEscrow_CalculateCancellationRefund(t *testing.T) {
	departureTime := time.Date(2026, 9, 10, 10, 0, 0, 0, time.UTC)
	totalAmount := 10000.0 // ARS

	t.Run("cancelación por conductor otorga 100% de reembolso al pasajero", func(t *testing.T) {
		cancellationTime := departureTime.Add(-2 * time.Hour) // Menos de 24hs pero es el conductor quien cancela
		passengerRefund, driverComp, refundType := domain.CalculateCancellationRefund(
			totalAmount,
			departureTime,
			cancellationTime,
			true, // isDriverCancellation
		)

		assert.Equal(t, 10000.0, passengerRefund)
		assert.Equal(t, 0.0, driverComp)
		assert.Equal(t, domain.RefundTypeDriverCancellation, refundType)
		assert.Equal(t, totalAmount, passengerRefund+driverComp)
	})

	t.Run("cancelación anticipada por pasajero con > 24hs otorga 100% de reembolso", func(t *testing.T) {
		cancellationTime := departureTime.Add(-30 * time.Hour) // 30 horas antes
		passengerRefund, driverComp, refundType := domain.CalculateCancellationRefund(
			totalAmount,
			departureTime,
			cancellationTime,
			false, // passenger
		)

		assert.Equal(t, 10000.0, passengerRefund)
		assert.Equal(t, 0.0, driverComp)
		assert.Equal(t, domain.RefundTypePassengerEarly, refundType)
		assert.Equal(t, totalAmount, passengerRefund+driverComp)
	})

	t.Run("cancelación tardía por pasajero con < 24hs aplica retención del 50% como compensación al conductor", func(t *testing.T) {
		cancellationTime := departureTime.Add(-5 * time.Hour) // 5 horas antes de la salida
		passengerRefund, driverComp, refundType := domain.CalculateCancellationRefund(
			totalAmount,
			departureTime,
			cancellationTime,
			false, // passenger
		)

		assert.Equal(t, 5000.0, passengerRefund)
		assert.Equal(t, 5000.0, driverComp)
		assert.Equal(t, domain.RefundTypePassengerLate, refundType)
		assert.Equal(t, totalAmount, passengerRefund+driverComp)
	})

	t.Run("creación inmutable de RefundTransaction con validación de montos positivos", func(t *testing.T) {
		refund, err := domain.NewRefundTransaction(
			"refund-1",
			"escrow-1",
			"booking-1",
			5000.0,
			5000.0,
			domain.RefundTypePassengerLate,
			"re_gateway_123",
		)
		require.NoError(t, err)
		assert.Equal(t, 5000.0, refund.PassengerRefundAmount)
		assert.Equal(t, 5000.0, refund.DriverCompensationAmount)
		assert.Equal(t, domain.RefundTypePassengerLate, refund.RefundType)

		// Falla si los montos de reembolso son negativos o cero
		_, errZero := domain.NewRefundTransaction("r-2", "e-2", "b-2", 0, 0, domain.RefundTypePassengerEarly, "")
		assert.ErrorIs(t, errZero, domain.ErrInvalidRefundAmount)
	})
}
