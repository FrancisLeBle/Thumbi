package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestBookingRepository_CreateWithHold(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	booking, err := domain.NewBooking(domain.BookingParams{
		ID:          "booking-101",
		TripID:      "trip-202",
		PassengerID: "passenger-303",
		DriverID:    "driver-404",
		SeatsBooked: 2,
		UnitPrice:   1500.0,
		TTL:         15 * time.Minute,
	})
	require.NoError(t, err)

	t.Run("falla cuando el viaje no existe en base de datos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") && strings.Contains(sql, "FOR UPDATE")
		}), []any{"trip-202"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		err := repo.CreateWithHold(ctx, booking)
		assert.ErrorIs(t, err, domain.ErrTripNotFound)
		mockDB.AssertExpectations(t)
	})

	t.Run("falla cuando el viaje no está en estado PUBLISHED", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") && strings.Contains(sql, "FOR UPDATE")
		}), []any{"trip-202"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 3
				*dest[1].(*string) = "COMPLETED" // No está publicado
				return nil
			},
		})

		err := repo.CreateWithHold(ctx, booking)
		assert.ErrorIs(t, err, domain.ErrInvalidTripStatusChange)
		mockDB.AssertExpectations(t)
	})

	t.Run("falla cuando las plazas disponibles son insuficientes", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") && strings.Contains(sql, "FOR UPDATE")
		}), []any{"trip-202"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 1 // Solo 1 asiento, solicita 2
				*dest[1].(*string) = "PUBLISHED"
				return nil
			},
		})

		err := repo.CreateWithHold(ctx, booking)
		assert.ErrorIs(t, err, domain.ErrInsufficientSeats)
		mockDB.AssertExpectations(t)
	})

	t.Run("bloquea pesimistamente la fila, decrementa asientos y crea la reserva", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		// 1. SELECT ... FOR UPDATE
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "SELECT available_seats, status") &&
				strings.Contains(sql, "FROM trips") &&
				strings.Contains(sql, "FOR UPDATE")
		}), []any{"trip-202"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 4
				*dest[1].(*string) = "PUBLISHED"
				return nil
			},
		})

		// 2. UPDATE trips (decrementa 2 asientos -> quedan 2)
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") &&
				strings.Contains(sql, "SET available_seats = $1")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 2 && args[0] == 2 && args[1] == "trip-202"
		})).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		// 3. INSERT INTO bookings
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO bookings") &&
				strings.Contains(sql, "seats_booked") &&
				strings.Contains(sql, "total_price")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 12 &&
				args[0] == "booking-101" &&
				args[1] == "trip-202" &&
				args[2] == "passenger-303" &&
				args[3] == 2 &&
				args[4] == 1500.0 &&
				args[5] == 3000.0 &&
				args[6] == "PENDING_PAYMENT"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err := repo.CreateWithHold(ctx, booking)
		require.NoError(t, err)

		mockDB.AssertExpectations(t)
	})

	_ = now
}

func TestBookingRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("retorna ErrBookingNotFound si no existe la reserva", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "WHERE id = $1")
		}), []any{"booking-404"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		b, err := repo.FindByID(ctx, "booking-404")
		assert.Nil(t, b)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("retorna la reserva mapeada exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "WHERE id = $1")
		}), []any{"booking-101"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "booking-101"
				*dest[1].(*string) = "trip-202"
				*dest[2].(*string) = "passenger-303"
				*dest[3].(*int) = 2
				*dest[4].(*float64) = 1500.0
				*dest[5].(*float64) = 3000.0
				*dest[6].(*string) = "CONFIRMED"
				*dest[7].(**string) = nil
				*dest[8].(**string) = nil
				*dest[9].(*time.Time) = now.Add(15 * time.Minute)
				*dest[10].(**time.Time) = &now
				*dest[11].(**time.Time) = nil
				*dest[12].(**string) = nil
				*dest[13].(*time.Time) = now
				*dest[14].(*time.Time) = now
				return nil
			},
		})

		b, err := repo.FindByID(ctx, "booking-101")
		require.NoError(t, err)
		assert.NotNil(t, b)
		assert.Equal(t, "booking-101", b.ID)
		assert.Equal(t, domain.BookingStatusConfirmed, b.Status)
		assert.Equal(t, 3000.0, b.TotalPrice)
		assert.NotNil(t, b.ConfirmedAt)
	})
}

func TestBookingRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("falla si la reserva no existe (RowsAffected == 0)", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") && strings.Contains(sql, "SET status = $1")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.UpdateStatus(ctx, "booking-nonexistent", domain.BookingStatusConfirmed, nil)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("actualiza el estado exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		reason := "Cancelado por el conductor"
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 3 &&
				args[0] == "CANCELLED_BY_DRIVER" &&
				args[1] == &reason &&
				args[2] == "booking-101"
		})).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateStatus(ctx, "booking-101", domain.BookingStatusCancelledByDriver, &reason)
		require.NoError(t, err)
	})
}

func TestBookingRepository_GetActiveBookingsByTrip(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	mockDB := new(MockDBExecutor)
	repo := postgres.NewBookingRepository(mockDB)

	rows := NewMockRows([][]any{
		{"booking-1", "trip-100", "passenger-1", 1, 1000.0, 1000.0, "PENDING_PAYMENT", nil, nil, now.Add(10 * time.Minute), nil, nil, nil, now, now},
		{"booking-2", "trip-100", "passenger-2", 2, 1000.0, 2000.0, "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
	})

	mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "FROM bookings") &&
			strings.Contains(sql, "WHERE trip_id = $1") &&
			strings.Contains(sql, "PENDING_PAYMENT")
	}), []any{"trip-100"}).Return(rows, nil)

	list, err := repo.GetActiveBookingsByTrip(ctx, "trip-100")
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, domain.BookingStatusPendingPayment, list[0].Status)
	assert.Equal(t, domain.BookingStatusConfirmed, list[1].Status)
}

func TestBookingRepository_GetPassengerBookings(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	mockDB := new(MockDBExecutor)
	repo := postgres.NewBookingRepository(mockDB)

	rows := NewMockRows([][]any{
		{"booking-1", "trip-100", "passenger-1", 1, 1000.0, 1000.0, "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
	})

	mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "FROM bookings") &&
			strings.Contains(sql, "WHERE passenger_id = $1") &&
			strings.Contains(sql, "LIMIT $2 OFFSET $3")
	}), []any{"passenger-1", 20, 0}).Return(rows, nil)

	list, err := repo.GetPassengerBookings(ctx, "passenger-1", 20, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "booking-1", list[0].ID)
}

func TestBookingRepository_ExpirePendingBookings(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("sin reservas vencidas retorna 0", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		emptyRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "FOR UPDATE")
		}), []any{now}).Return(emptyRows, nil)

		count, err := repo.ExpirePendingBookings(ctx, now)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("encuentra reservas vencidas, restituye asientos al viaje y actualiza estado a EXPIRED", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		expiredRows := NewMockRows([][]any{
			{"b-exp-1", "trip-1", 2},
			{"b-exp-2", "trip-2", 1},
		})

		// 1. SELECT ... FOR UPDATE
		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "status = 'PENDING_PAYMENT'")
		}), []any{now}).Return(expiredRows, nil)

		// 2. Restitución de asientos para cada reserva
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") && strings.Contains(sql, "available_seats = available_seats + $1")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 2 && args[0] == 2 && args[1] == "trip-1"
		})).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") && strings.Contains(sql, "available_seats = available_seats + $1")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 2 && args[0] == 1 && args[1] == "trip-2"
		})).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		// 3. Transición de reservas a EXPIRED
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") && strings.Contains(sql, "SET status = 'EXPIRED'")
		}), []any{now}).Return(pgconn.NewCommandTag("UPDATE 2"), nil)

		count, err := repo.ExpirePendingBookings(ctx, now)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		mockDB.AssertExpectations(t)
	})
}

func TestEscrowRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("CreateEscrow inserta la retención en custodia", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		escrow, err := domain.NewEscrowTransaction("escrow-1", "booking-101", "trip-1", "passenger-1", "driver-1", 3000.0, "USD", "pi_test_123")
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO escrow_transactions") &&
				strings.Contains(sql, "payment_gateway_ref")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 12 &&
				args[0] == "escrow-1" &&
				args[1] == "booking-101" &&
				args[5] == 3000.0 &&
				args[8] == "HELD"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err = repo.CreateEscrow(ctx, escrow)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("FindByBookingID recupera el escrow", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM escrow_transactions") && strings.Contains(sql, "WHERE booking_id = $1")
		}), []any{"booking-101"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "escrow-1"
				*dest[1].(*string) = "booking-101"
				*dest[2].(*string) = "trip-1"
				*dest[3].(*string) = "passenger-1"
				*dest[4].(*string) = "driver-1"
				*dest[5].(*float64) = 3000.0
				*dest[6].(*string) = "USD"
				*dest[7].(*string) = "pi_test_123"
				*dest[8].(*string) = "HELD"
				*dest[9].(*time.Time) = now
				*dest[10].(**time.Time) = nil
				*dest[11].(**time.Time) = nil
				*dest[12].(*time.Time) = now
				*dest[13].(*time.Time) = now
				return nil
			},
		})

		res, err := repo.FindByBookingID(ctx, "booking-101")
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "escrow-1", res.ID)
		assert.Equal(t, domain.EscrowStatusHeld, res.Status)
		assert.Equal(t, 3000.0, res.Amount)
	})

	t.Run("UpdateEscrowStatus actualiza el estado", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE escrow_transactions") &&
				strings.Contains(sql, "SET status = $1")
		}), []any{"RELEASED", "escrow-1"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateEscrowStatus(ctx, "escrow-1", domain.EscrowStatusReleased)
		require.NoError(t, err)
	})

	t.Run("RecordRefund inserta la transacción de reembolso inmutable", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		refund, err := domain.NewRefundTransaction(
			"ref-1",
			"escrow-1",
			"booking-101",
			1500.0,
			1500.0,
			domain.RefundTypePassengerLate,
			"re_test_999",
		)
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO refund_transactions") &&
				strings.Contains(sql, "passenger_refund_amount")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 9 &&
				args[0] == "ref-1" &&
				args[1] == "escrow-1" &&
				args[2] == "booking-101" &&
				args[3] == 1500.0 &&
				args[4] == 1500.0 &&
				args[5] == "PASSENGER_LATE" &&
				args[6] == "re_test_999"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err = repo.RecordRefund(ctx, refund)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}
