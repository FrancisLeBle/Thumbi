package postgres_test

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
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
		UnitPrice:   domain.Money(150000),
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
			unitNum, okUnit := args[4].(pgtype.Numeric)
			totalNum, okTotal := args[5].(pgtype.Numeric)
			return len(args) == 12 &&
				args[0] == "booking-101" &&
				args[1] == "trip-202" &&
				args[2] == "passenger-303" &&
				args[3] == 2 &&
				okUnit && unitNum.Valid && unitNum.Exp == -2 && unitNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				okTotal && totalNum.Valid && totalNum.Exp == -2 && totalNum.Int.Cmp(big.NewInt(300000)) == 0 &&
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
				intentID := "pi_mock_saved_101"
				*dest[0].(*string) = "booking-101"
				*dest[1].(*string) = "trip-202"
				*dest[2].(*string) = "passenger-303"
				*dest[3].(*int) = 2
				*dest[4].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(150000), Exp: -2, Valid: true}
				*dest[5].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(300000), Exp: -2, Valid: true}
				*dest[6].(*string) = "CONFIRMED"
				*dest[7].(**string) = nil
				*dest[8].(**string) = nil
				*dest[9].(**string) = &intentID
				*dest[10].(*time.Time) = now.Add(15 * time.Minute)
				*dest[11].(**time.Time) = &now
				*dest[12].(**time.Time) = nil
				*dest[13].(**string) = nil
				*dest[14].(*time.Time) = now
				*dest[15].(*time.Time) = now
				return nil
			},
		})

		b, err := repo.FindByID(ctx, "booking-101")
		require.NoError(t, err)
		assert.NotNil(t, b)
		assert.Equal(t, "booking-101", b.ID)
		assert.Equal(t, domain.BookingStatusConfirmed, b.Status)
		assert.Equal(t, domain.Money(300000), b.TotalPrice)
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

		err := repo.UpdateStatus(ctx, "booking-nonexistent", domain.BookingStatusCompleted, nil)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("confirmación valida atómicamente PENDING_PAYMENT y expires_at > NOW()", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") &&
				strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), "booking-101"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateStatus(ctx, "booking-101", domain.BookingStatusConfirmed, nil)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("confirmación retorna ErrBookingExpired si RowsAffected == 0 por expiración o carrera", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), "booking-raced"}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.UpdateStatus(ctx, "booking-raced", domain.BookingStatusConfirmed, nil)
		assert.ErrorIs(t, err, domain.ErrBookingExpired)
		mockDB.AssertExpectations(t)
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
		{"booking-1", "trip-100", "passenger-1", 1, domain.Money(100000), domain.Money(100000), "PENDING_PAYMENT", nil, nil, now.Add(10 * time.Minute), nil, nil, nil, now, now},
		{"booking-2", "trip-100", "passenger-2", 2, domain.Money(100000), domain.Money(200000), "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
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
		{"booking-1", "trip-100", "passenger-1", 1, domain.Money(100000), domain.Money(100000), "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
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

func TestBookingRepository_CheckPassengerBookingConstraints(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dep := now.Add(24 * time.Hour)
	arr := dep.Add(3 * time.Hour)

	t.Run("ejecuta única consulta JOIN con tstzrange && y retorna solapamiento y conteo pendiente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings b") &&
				strings.Contains(sql, "INNER JOIN trips t ON t.id = b.trip_id") &&
				strings.Contains(sql, "tstzrange") &&
				strings.Contains(sql, "&&") &&
				strings.Contains(sql, "active_pending_count")
		}), []any{"passenger-1", "trip-target", dep.UTC(), arr.UTC(), now.UTC()}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*bool) = true
				*dest[1].(*int) = 2
				return nil
			},
		})

		hasOverlap, pendingCount, err := repo.CheckPassengerBookingConstraints(ctx, "passenger-1", "trip-target", dep, arr, now)
		require.NoError(t, err)
		assert.True(t, hasOverlap)
		assert.Equal(t, 2, pendingCount)
		mockDB.AssertExpectations(t)
	})

	t.Run("propaga errores de base de datos sin ignorarlos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		dbErr := errors.New("query timeout")
		mockDB.On("QueryRow", ctx, mock.Anything, mock.Anything).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return dbErr
			},
		})

		hasOverlap, pendingCount, err := repo.CheckPassengerBookingConstraints(ctx, "passenger-1", "trip-target", dep, arr, now)
		assert.False(t, hasOverlap)
		assert.Equal(t, 0, pendingCount)
		assert.ErrorIs(t, err, dbErr)
		mockDB.AssertExpectations(t)
	})
}

func TestEscrowRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("CreateEscrow inserta la retención en custodia", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		escrow, err := domain.NewEscrowTransaction("escrow-1", "booking-101", "trip-1", "passenger-1", "driver-1", domain.Money(300000), "USD", "pi_test_123")
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO escrow_transactions") &&
				strings.Contains(sql, "payment_gateway_ref")
		}), mock.MatchedBy(func(args []any) bool {
			amtNum, okAmt := args[5].(pgtype.Numeric)
			return len(args) == 12 &&
				args[0] == "escrow-1" &&
				args[1] == "booking-101" &&
				okAmt && amtNum.Valid && amtNum.Exp == -2 && amtNum.Int.Cmp(big.NewInt(300000)) == 0 &&
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
				*dest[5].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(300000), Exp: -2, Valid: true}
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
		assert.Equal(t, domain.Money(300000), res.Amount)
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
			domain.Money(150000),
			domain.Money(150000),
			domain.RefundTypePassengerLate,
			"re_test_999",
		)
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO refund_transactions") &&
				strings.Contains(sql, "passenger_refund_amount")
		}), mock.MatchedBy(func(args []any) bool {
			pNum, okP := args[3].(pgtype.Numeric)
			dNum, okD := args[4].(pgtype.Numeric)
			return len(args) == 9 &&
				args[0] == "ref-1" &&
				args[1] == "escrow-1" &&
				args[2] == "booking-101" &&
				okP && pNum.Valid && pNum.Exp == -2 && pNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				okD && dNum.Valid && dNum.Exp == -2 && dNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				args[5] == "PASSENGER_LATE" &&
				args[6] == "re_test_999"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err = repo.RecordRefund(ctx, refund)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

type MockTx struct {
	pgx.Tx
	*MockDBExecutor
}

func (m *MockTx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return m.MockDBExecutor.Exec(ctx, sql, arguments...)
}

func (m *MockTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.MockDBExecutor.Query(ctx, sql, args...)
}

func (m *MockTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return m.MockDBExecutor.QueryRow(ctx, sql, args...)
}

func (m *MockTx) Commit(ctx context.Context) error {
	args := m.MockDBExecutor.Called(ctx)
	return args.Error(0)
}

func (m *MockTx) Rollback(ctx context.Context) error {
	args := m.MockDBExecutor.Called(ctx)
	return args.Error(0)
}

type MockTxBeginner struct {
	mock.Mock
}

func (b *MockTxBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	args := b.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(pgx.Tx), args.Error(1)
}

func TestTransactionManager_BookingStatusAndReleaseSeats(t *testing.T) {
	ctx := context.Background()

	t.Run("ejecuta UpdateStatus y ReleaseSeats sobre el mismo pgx.Tx y hace Commit", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") && strings.Contains(sql, "available_seats = available_seats + $1")
		}), []any{2, "trip-100"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Commit", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		tripRepo := postgres.NewTripRepository(poolDB)

		reason := "Cancelación"
		err := txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, "booking-100", domain.BookingStatusCancelledByPassenger, &reason); err != nil {
				return err
			}
			return tripRepo.ReleaseSeats(txCtx, "trip-100", 2)
		})

		require.NoError(t, err)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("hace Rollback automático si ReleaseSeats falla tras UpdateStatus", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)

		dbErr := errors.New("check constraint chk_available_seats_valid violated")
		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips")
		}), []any{2, "trip-100"}).Return(pgconn.NewCommandTag("UPDATE 0"), dbErr)
		txExec.On("Rollback", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		tripRepo := postgres.NewTripRepository(poolDB)

		err := txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, "booking-100", domain.BookingStatusCancelledByPassenger, nil); err != nil {
				return err
			}
			return tripRepo.ReleaseSeats(txCtx, "trip-100", 2)
		})

		assert.ErrorIs(t, err, dbErr)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		txExec.AssertExpectations(t)
	})
}

func TestTransactionManager_ConfirmBookingAndCreateEscrow(t *testing.T) {
	ctx := context.Background()

	t.Run("aborta y hace Rollback si la reserva expiró concurrentemente (RowsAffected == 0)", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") &&
				strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), "booking-expired-race"}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)
		txExec.On("Rollback", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		escrowRepo := postgres.NewEscrowRepository(poolDB)

		escrow, err := domain.NewEscrowTransaction("escrow-1", "booking-expired-race", "trip-1", "p-1", "d-1", domain.Money(300000), "USD", "pi_123")
		require.NoError(t, err)

		err = txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, "booking-expired-race", domain.BookingStatusConfirmed, nil); err != nil {
				return err
			}
			return escrowRepo.CreateEscrow(txCtx, escrow)
		})

		assert.ErrorIs(t, err, domain.ErrBookingExpired)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		txExec.AssertExpectations(t)
	})

	t.Run("confirma reserva y crea Escrow en el mismo pgx.Tx cuando está vigente", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") &&
				strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), "booking-ok"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO escrow_transactions")
		}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)
		txExec.On("Commit", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		escrowRepo := postgres.NewEscrowRepository(poolDB)

		escrow, err := domain.NewEscrowTransaction("escrow-1", "booking-ok", "trip-1", "p-1", "d-1", domain.Money(300000), "USD", "pi_123")
		require.NoError(t, err)

		err = txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, "booking-ok", domain.BookingStatusConfirmed, nil); err != nil {
				return err
			}
			return escrowRepo.CreateEscrow(txCtx, escrow)
		})

		require.NoError(t, err)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		txExec.AssertExpectations(t)
	})
}

func TestBookingRepository_ExpirePendingBookings(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("usa FOR UPDATE SKIP LOCKED LIMIT 500 y UPDATE trips FROM agrupado en una sola consulta atómica", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FOR UPDATE SKIP LOCKED") &&
				strings.Contains(sql, "LIMIT $2") &&
				strings.Contains(sql, "UPDATE bookings b") &&
				strings.Contains(sql, "GROUP BY trip_id") &&
				strings.Contains(sql, "UPDATE trips t") &&
				strings.Contains(sql, "FROM seat_restores sr")
		}), []any{now.UTC(), 500}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*int) = 12
				*dest[1].(*int) = 4
				return nil
			},
		})

		expiredCount, err := repo.ExpirePendingBookings(ctx, now)
		require.NoError(t, err)
		assert.Equal(t, 12, expiredCount)
		mockDB.AssertExpectations(t)
	})

	t.Run("propaga errores de base de datos al expirar en lote", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{now.UTC(), 500}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return errors.New("deadlock detected")
			},
		})

		expiredCount, err := repo.ExpirePendingBookings(ctx, now)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error ejecutando expiración en lote")
		assert.Equal(t, 0, expiredCount)
		mockDB.AssertExpectations(t)
	})
}
