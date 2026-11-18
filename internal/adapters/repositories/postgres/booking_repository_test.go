package postgres_test

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestBookingRepository_CreateWithHold(t *testing.T) {
	ctx := context.Background()
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000303")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	booking, err := domain.NewBooking(domain.BookingParams{
		ID:          bookingID,
		TripID:      tripID,
		PassengerID: passengerID,
		DriverID:    driverID,
		SeatsBooked: 2,
		UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		PlatformFee: domain.MoneyFromCents(45000, domain.CurrencyARS),
		TTL:         15 * time.Minute,
	})
	require.NoError(t, err)

	t.Run("retorna applied = false y ejecuta Rollback sin insertar reserva cuando la guarda del UPDATE no coincide (RowsAffected == 0)", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)
		repo := postgres.NewBookingRepository(poolDB, uow)

		seatHold := domain.SeatTransition{
			TripID:                 tripID,
			SeatsDelta:             2,
			ExpectedStatus:         domain.TripStatusPublished,
			ExpectedAvailableSeats: 4,
			NewStatus:              domain.TripStatusPublished,
			NewAvailableSeats:      2,
		}

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") &&
				strings.Contains(sql, "available_seats = available_seats - $1") &&
				strings.Contains(sql, "available_seats >= $1") &&
				!strings.Contains(sql, "CASE WHEN")
		}), []any{2, "PUBLISHED", tripID, "PUBLISHED", 4}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)
		txExec.On("Rollback", ctx).Return(nil)

		applied, err := repo.CreateWithHold(ctx, booking, seatHold)
		require.NoError(t, err)
		assert.False(t, applied)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("aplica atómicamente el UPDATE con guarda e inserta la reserva en la misma transacción reportando applied = true", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)
		repo := postgres.NewBookingRepository(poolDB, uow)

		seatHold := domain.SeatTransition{
			TripID:                 tripID,
			SeatsDelta:             2,
			ExpectedStatus:         domain.TripStatusPublished,
			ExpectedAvailableSeats: 2,
			NewStatus:              domain.TripStatusFull,
			NewAvailableSeats:      0,
		}

		beginner.On("Begin", ctx).Return(tx, nil)

		// 1. UPDATE trips con guarda y nuevo estado decidido por el dominio (FULL)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") &&
				strings.Contains(sql, "available_seats = available_seats - $1") &&
				strings.Contains(sql, "status = $2") &&
				strings.Contains(sql, "available_seats = $5") &&
				strings.Contains(sql, "available_seats >= $1")
		}), []any{2, "FULL", tripID, "PUBLISHED", 2}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		// 2. INSERT INTO bookings
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO bookings") &&
				strings.Contains(sql, "seats_booked") &&
				strings.Contains(sql, "base_amount") &&
				strings.Contains(sql, "platform_fee") &&
				strings.Contains(sql, "total_price")
		}), mock.MatchedBy(func(args []any) bool {
			unitNum, okUnit := args[5].(pgtype.Numeric)
			baseNum, okBase := args[6].(pgtype.Numeric)
			feeNum, okFee := args[7].(pgtype.Numeric)
			totalNum, okTotal := args[8].(pgtype.Numeric)
			return len(args) == 16 &&
				args[0] == bookingID &&
				args[1] == tripID &&
				args[2] == passengerID &&
				args[3] == 2 &&
				args[4] == "ARS" &&
				okUnit && unitNum.Valid && unitNum.Exp == -2 && unitNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				okBase && baseNum.Valid && baseNum.Exp == -2 && baseNum.Int.Cmp(big.NewInt(300000)) == 0 &&
				okFee && feeNum.Valid && feeNum.Exp == -2 && feeNum.Int.Cmp(big.NewInt(45000)) == 0 &&
				okTotal && totalNum.Valid && totalNum.Exp == -2 && totalNum.Int.Cmp(big.NewInt(345000)) == 0 &&
				args[9] == "PENDING_PAYMENT"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		txExec.On("Commit", ctx).Return(nil)

		applied, err := repo.CreateWithHold(ctx, booking, seatHold)
		require.NoError(t, err)
		assert.True(t, applied)

		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("hace Rollback y retorna ErrOverlappingTripBooking ante violación 23P01 de exclusión temporal", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)
		repo := postgres.NewBookingRepository(poolDB, uow)

		seatHold := domain.SeatTransition{
			TripID:                 tripID,
			SeatsDelta:             2,
			ExpectedStatus:         domain.TripStatusPublished,
			ExpectedAvailableSeats: 3,
			NewStatus:              domain.TripStatusPublished,
			NewAvailableSeats:      1,
		}

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips")
		}), []any{2, "PUBLISHED", tripID, "PUBLISHED", 3}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO bookings")
		}), mock.Anything).Return(pgconn.NewCommandTag(""), &pgconn.PgError{Code: "23P01"})
		txExec.On("Rollback", ctx).Return(nil)

		applied, err := repo.CreateWithHold(ctx, booking, seatHold)
		assert.False(t, applied)
		assert.ErrorIs(t, err, domain.ErrOverlappingTripBooking)

		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("falla explícitamente si no hay UnitOfWork ni TxBeginner configurado", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		seatHold := domain.SeatTransition{
			TripID:                 tripID,
			SeatsDelta:             2,
			ExpectedStatus:         domain.TripStatusPublished,
			ExpectedAvailableSeats: 2,
			NewStatus:              domain.TripStatusFull,
			NewAvailableSeats:      0,
		}

		applied, err := repo.CreateWithHold(ctx, booking, seatHold)
		assert.False(t, applied)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "UnitOfWork")
		mockDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestBookingRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000303")

	t.Run("retorna ErrBookingNotFound si no existe la reserva", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "WHERE id = $1")
		}), []any{missingID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		b, err := repo.FindByID(ctx, missingID)
		assert.Nil(t, b)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("retorna la reserva mapeada exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") && strings.Contains(sql, "WHERE id = $1")
		}), []any{bookingID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				intentID := "pi_mock_saved_101"
				*dest[0].(*uuid.UUID) = bookingID
				*dest[1].(*uuid.UUID) = tripID
				*dest[2].(*uuid.UUID) = passengerID
				*dest[3].(*int) = 2
				*dest[4].(*string) = "ARS"
				*dest[5].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(150000), Exp: -2, Valid: true}
				*dest[6].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(300000), Exp: -2, Valid: true}
				*dest[7].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(45000), Exp: -2, Valid: true}
				*dest[8].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(345000), Exp: -2, Valid: true}
				*dest[9].(*string) = "CONFIRMED"
				*dest[10].(**uuid.UUID) = nil
				*dest[11].(**uuid.UUID) = nil
				*dest[12].(**string) = &intentID
				*dest[13].(*time.Time) = now.Add(15 * time.Minute)
				*dest[14].(**time.Time) = &now
				*dest[15].(**time.Time) = nil
				*dest[16].(**string) = nil
				*dest[17].(*time.Time) = now
				*dest[18].(*time.Time) = now
				return nil
			},
		})

		b, err := repo.FindByID(ctx, bookingID)
		require.NoError(t, err)
		assert.NotNil(t, b)
		assert.Equal(t, bookingID, b.ID)
		assert.Equal(t, domain.BookingStatusConfirmed, b.Status)
		assert.Equal(t, domain.CurrencyARS, b.Currency)
		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), b.BaseAmount)
		assert.Equal(t, domain.MoneyFromCents(45000, domain.CurrencyARS), b.PlatformFee)
		assert.Equal(t, domain.MoneyFromCents(345000, domain.CurrencyARS), b.TotalPrice)
		assert.NotNil(t, b.ConfirmedAt)
	})
}

func TestBookingRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")
	racedID := uuid.MustParse("01933e3d-0000-7000-8000-000000000505")

	t.Run("falla si la reserva no existe (RowsAffected == 0)", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") && strings.Contains(sql, "SET status = $1")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.UpdateStatus(ctx, missingID, domain.BookingStatusCompleted, nil)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
	})

	t.Run("confirmación valida atómicamente PENDING_PAYMENT y expires_at > NOW()", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") &&
				strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), bookingID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateStatus(ctx, bookingID, domain.BookingStatusConfirmed, nil)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("confirmación retorna ErrBookingExpired si RowsAffected == 0 por expiración o carrera", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "status = 'PENDING_PAYMENT'") &&
				strings.Contains(sql, "expires_at > NOW()")
		}), []any{"CONFIRMED", (*string)(nil), racedID}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.UpdateStatus(ctx, racedID, domain.BookingStatusConfirmed, nil)
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
				args[2] == bookingID
		})).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateStatus(ctx, bookingID, domain.BookingStatusCancelledByDriver, &reason)
		require.NoError(t, err)
	})
}

func TestBookingRepository_GetActiveBookingsByTrip(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	b1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	b2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	p1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")
	p2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")

	mockDB := new(MockDBExecutor)
	repo := postgres.NewBookingRepository(mockDB)

	rows := NewMockRows([][]any{
		{b1ID, tripID, p1ID, 1, "ARS", domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(15000, domain.CurrencyARS), domain.MoneyFromCents(115000, domain.CurrencyARS), "PENDING_PAYMENT", nil, nil, now.Add(10 * time.Minute), nil, nil, nil, now, now},
		{b2ID, tripID, p2ID, 2, "ARS", domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(200000, domain.CurrencyARS), domain.MoneyFromCents(30000, domain.CurrencyARS), domain.MoneyFromCents(230000, domain.CurrencyARS), "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
	})

	mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "FROM bookings") &&
			strings.Contains(sql, "WHERE trip_id = $1") &&
			strings.Contains(sql, "PENDING_PAYMENT")
	}), []any{tripID}).Return(rows, nil)

	list, err := repo.GetActiveBookingsByTrip(ctx, tripID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, domain.BookingStatusPendingPayment, list[0].Status)
	assert.Equal(t, domain.CurrencyARS, list[0].Currency)
	assert.Equal(t, domain.MoneyFromCents(15000, domain.CurrencyARS), list[0].PlatformFee)
	assert.Equal(t, domain.BookingStatusConfirmed, list[1].Status)
}

func TestBookingRepository_GetPassengerBookings(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	b1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")

	mockDB := new(MockDBExecutor)
	repo := postgres.NewBookingRepository(mockDB)

	rows := NewMockRows([][]any{
		{b1ID, tripID, passengerID, 1, "ARS", domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(15000, domain.CurrencyARS), domain.MoneyFromCents(115000, domain.CurrencyARS), "CONFIRMED", nil, nil, now.Add(10 * time.Minute), &now, nil, nil, now, now},
	})

	mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
		return strings.Contains(sql, "FROM bookings") &&
			strings.Contains(sql, "WHERE passenger_id = $1") &&
			strings.Contains(sql, "LIMIT $2 OFFSET $3")
	}), []any{passengerID, 20, 0}).Return(rows, nil)

	list, err := repo.GetPassengerBookings(ctx, passengerID, 20, 0)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, b1ID, list[0].ID)
	assert.Equal(t, domain.CurrencyARS, list[0].Currency)
	assert.Equal(t, domain.MoneyFromCents(15000, domain.CurrencyARS), list[0].PlatformFee)
}

func TestBookingRepository_CheckPassengerBookingConstraints(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dep := now.Add(24 * time.Hour)
	arr := dep.Add(3 * time.Hour)
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")
	targetTripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000999")

	t.Run("ejecuta única consulta JOIN con tstzrange && y retorna solapamiento y conteo pendiente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings b") &&
				strings.Contains(sql, "INNER JOIN trips t ON t.id = b.trip_id") &&
				strings.Contains(sql, "tstzrange") &&
				strings.Contains(sql, "&&") &&
				strings.Contains(sql, "active_pending_count")
		}), []any{passengerID, targetTripID, dep.UTC(), arr.UTC(), now.UTC()}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*bool) = true
				*dest[1].(*int) = 2
				return nil
			},
		})

		hasOverlap, pendingCount, err := repo.CheckPassengerBookingConstraints(ctx, passengerID, targetTripID, dep, arr, now)
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

		hasOverlap, pendingCount, err := repo.CheckPassengerBookingConstraints(ctx, passengerID, targetTripID, dep, arr, now)
		assert.False(t, hasOverlap)
		assert.Equal(t, 0, pendingCount)
		assert.ErrorIs(t, err, dbErr)
		mockDB.AssertExpectations(t)
	})
}

func TestEscrowRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	escrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000301")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000401")
	refID := uuid.MustParse("01933e3d-0000-7000-8000-000000000501")

	t.Run("CreateEscrow inserta la retención en custodia con base_amount y platform_fee", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		escrow, err := domain.NewEscrowTransactionWithBreakdown(
			escrowID,
			bookingID,
			tripID,
			passengerID,
			driverID,
			domain.MoneyFromCents(300000, domain.CurrencyARS),
			domain.MoneyFromCents(45000, domain.CurrencyARS),
			"pi_test_123",
		)
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO escrow_transactions") &&
				strings.Contains(sql, "base_amount") &&
				strings.Contains(sql, "platform_fee") &&
				strings.Contains(sql, "payment_gateway_ref")
		}), mock.MatchedBy(func(args []any) bool {
			amtNum, okAmt := args[5].(pgtype.Numeric)
			baseNum, okBase := args[6].(pgtype.Numeric)
			feeNum, okFee := args[7].(pgtype.Numeric)
			return len(args) == 14 &&
				args[0] == escrowID &&
				args[1] == bookingID &&
				okAmt && amtNum.Valid && amtNum.Exp == -2 && amtNum.Int.Cmp(big.NewInt(345000)) == 0 &&
				okBase && baseNum.Valid && baseNum.Exp == -2 && baseNum.Int.Cmp(big.NewInt(300000)) == 0 &&
				okFee && feeNum.Valid && feeNum.Exp == -2 && feeNum.Int.Cmp(big.NewInt(45000)) == 0 &&
				args[8] == "ARS" &&
				args[10] == "HELD"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err = repo.CreateEscrow(ctx, escrow)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("FindByBookingID recupera el escrow con desglose", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM escrow_transactions") && strings.Contains(sql, "WHERE booking_id = $1")
		}), []any{bookingID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = escrowID
				*dest[1].(*uuid.UUID) = bookingID
				*dest[2].(*uuid.UUID) = tripID
				*dest[3].(*uuid.UUID) = passengerID
				*dest[4].(*uuid.UUID) = driverID
				*dest[5].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(345000), Exp: -2, Valid: true}
				*dest[6].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(300000), Exp: -2, Valid: true}
				*dest[7].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(45000), Exp: -2, Valid: true}
				*dest[8].(*string) = "ARS"
				*dest[9].(*string) = "pi_test_123"
				*dest[10].(*string) = "HELD"
				*dest[11].(*time.Time) = now
				*dest[12].(**time.Time) = nil
				*dest[13].(**time.Time) = nil
				*dest[14].(*time.Time) = now
				*dest[15].(*time.Time) = now
				return nil
			},
		})

		res, err := repo.FindByBookingID(ctx, bookingID)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, escrowID, res.ID)
		assert.Equal(t, domain.EscrowStatusHeld, res.Status)
		assert.Equal(t, domain.CurrencyARS, res.Currency)
		assert.Equal(t, domain.MoneyFromCents(345000, domain.CurrencyARS), res.Amount)
		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), res.BaseAmount)
		assert.Equal(t, domain.MoneyFromCents(45000, domain.CurrencyARS), res.PlatformFee)
	})

	t.Run("UpdateEscrowStatus actualiza el estado", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE escrow_transactions") &&
				strings.Contains(sql, "SET status = $1")
		}), []any{"RELEASED", escrowID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateEscrowStatus(ctx, escrowID, domain.EscrowStatusReleased)
		require.NoError(t, err)
	})

	t.Run("RecordRefund inserta la transacción de reembolso inmutable con retención de plataforma", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewEscrowRepository(mockDB)

		refund, err := domain.NewRefundTransactionWithRetention(
			refID,
			escrowID,
			bookingID,
			domain.MoneyFromCents(150000, domain.CurrencyARS),
			domain.MoneyFromCents(150000, domain.CurrencyARS),
			domain.MoneyFromCents(45000, domain.CurrencyARS),
			domain.RefundTypePassengerLate,
			"re_test_999",
		)
		require.NoError(t, err)

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO refund_transactions") &&
				strings.Contains(sql, "passenger_refund_amount") &&
				strings.Contains(sql, "platform_retained_amount")
		}), mock.MatchedBy(func(args []any) bool {
			pNum, okP := args[3].(pgtype.Numeric)
			dNum, okD := args[4].(pgtype.Numeric)
			rNum, okR := args[5].(pgtype.Numeric)
			return len(args) == 10 &&
				args[0] == refID &&
				args[1] == escrowID &&
				args[2] == bookingID &&
				okP && pNum.Valid && pNum.Exp == -2 && pNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				okD && dNum.Valid && dNum.Exp == -2 && dNum.Int.Cmp(big.NewInt(150000)) == 0 &&
				okR && rNum.Valid && rNum.Exp == -2 && rNum.Int.Cmp(big.NewInt(45000)) == 0 &&
				args[6] == "PASSENGER_LATE" &&
				args[7] == "re_test_999"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err = repo.RecordRefund(ctx, refund)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

type MockTx struct {
	pgx.Tx
	*MockDBExecutor
	closed bool
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
	err := args.Error(0)
	if err == nil {
		m.closed = true
	}
	return err
}

func (m *MockTx) Rollback(ctx context.Context) error {
	if m.closed {
		return pgx.ErrTxClosed
	}
	m.closed = true
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
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000200")

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
		}), []any{2, "PUBLISHED", tripID, "FULL"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Commit", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		tripRepo := postgres.NewTripRepository(poolDB)

		transition := domain.SeatTransition{
			TripID:         tripID,
			SeatsDelta:     2,
			ExpectedStatus: domain.TripStatusFull,
			NewStatus:      domain.TripStatusPublished,
		}

		reason := "Cancelación"
		err := txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, bookingID, domain.BookingStatusCancelledByPassenger, &reason); err != nil {
				return err
			}
			applied, err := tripRepo.ReleaseSeats(txCtx, transition)
			if err != nil {
				return err
			}
			assert.True(t, applied)
			return nil
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
		}), []any{2, "PUBLISHED", tripID, "FULL"}).Return(pgconn.NewCommandTag("UPDATE 0"), dbErr)
		txExec.On("Rollback", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		tripRepo := postgres.NewTripRepository(poolDB)

		transition := domain.SeatTransition{
			TripID:         tripID,
			SeatsDelta:     2,
			ExpectedStatus: domain.TripStatusFull,
			NewStatus:      domain.TripStatusPublished,
		}

		err := txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, bookingID, domain.BookingStatusCancelledByPassenger, nil); err != nil {
				return err
			}
			_, err := tripRepo.ReleaseSeats(txCtx, transition)
			return err
		})

		assert.ErrorIs(t, err, dbErr)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		txExec.AssertExpectations(t)
	})
}

func TestTransactionManager_ConfirmBookingAndCreateEscrow(t *testing.T) {
	ctx := context.Background()
	escrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	bookingRaceID := uuid.MustParse("01933e3d-0000-7000-8000-000000000102")
	bookingOKID := uuid.MustParse("01933e3d-0000-7000-8000-000000000103")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000301")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000401")

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
		}), []any{"CONFIRMED", (*string)(nil), bookingRaceID}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)
		txExec.On("Rollback", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		escrowRepo := postgres.NewEscrowRepository(poolDB)

		escrow, err := domain.NewEscrowTransaction(escrowID, bookingRaceID, tripID, passengerID, driverID, domain.MoneyFromCents(300000, domain.CurrencyARS), "pi_123")
		require.NoError(t, err)

		err = txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, bookingRaceID, domain.BookingStatusConfirmed, nil); err != nil {
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
		}), []any{"CONFIRMED", (*string)(nil), bookingOKID}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)
		txExec.On("Exec", mock.Anything, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO escrow_transactions")
		}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)
		txExec.On("Commit", ctx).Return(nil)

		txMgr := postgres.NewTransactionManager(beginner)
		bookingRepo := postgres.NewBookingRepository(poolDB)
		escrowRepo := postgres.NewEscrowRepository(poolDB)

		escrow, err := domain.NewEscrowTransaction(escrowID, bookingOKID, tripID, passengerID, driverID, domain.MoneyFromCents(300000, domain.CurrencyARS), "pi_123")
		require.NoError(t, err)

		err = txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
			if err := bookingRepo.UpdateStatus(txCtx, bookingOKID, domain.BookingStatusConfirmed, nil); err != nil {
				return err
			}
			return escrowRepo.CreateEscrow(txCtx, escrow)
		})

		require.NoError(t, err)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		txExec.AssertExpectations(t)
	})
}

func TestBookingRepository_LockAndExpireBatch(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	b1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000201")

	t.Run("LockExpiredPendingBookings usa FOR UPDATE SKIP LOCKED LIMIT $3", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		rows := NewMockRows([][]any{
			{b1ID, tripID, passengerID, 2, "ARS", domain.MoneyFromCents(100000, domain.CurrencyARS), domain.MoneyFromCents(200000, domain.CurrencyARS), domain.MoneyFromCents(30000, domain.CurrencyARS), domain.MoneyFromCents(230000, domain.CurrencyARS), "PENDING_PAYMENT", nil, nil, nil, now.Add(-1 * time.Minute), nil, nil, nil, now.Add(-20 * time.Minute), now.Add(-20 * time.Minute)},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM bookings") &&
				strings.Contains(sql, "WHERE status = $1") &&
				strings.Contains(sql, "expires_at <= $2") &&
				strings.Contains(sql, "FOR UPDATE SKIP LOCKED") &&
				strings.Contains(sql, "LIMIT $3")
		}), []any{"PENDING_PAYMENT", now.UTC(), 500}).Return(rows, nil)

		candidates, err := repo.LockExpiredPendingBookings(ctx, domain.BookingStatusPendingPayment, now, 500)
		require.NoError(t, err)
		require.Len(t, candidates, 1)
		assert.Equal(t, b1ID, candidates[0].ID)
		assert.Equal(t, 2, candidates[0].SeatsBooked)
		assert.Equal(t, domain.BookingStatusPendingPayment, candidates[0].Status)
		mockDB.AssertExpectations(t)
	})

	t.Run("MarkBookingsExpiredBatch actualiza en lote reservas reclamadas sin lógica de viajes en SQL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		ids := []uuid.UUID{b1ID}
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE bookings") &&
				strings.Contains(sql, "SET status = $1") &&
				strings.Contains(sql, "WHERE id = ANY($3)") &&
				strings.Contains(sql, "AND status = $4") &&
				!strings.Contains(sql, "UPDATE trips")
		}), []any{"EXPIRED", now.UTC(), ids, "PENDING_PAYMENT"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		updated, err := repo.MarkBookingsExpiredBatch(ctx, ids, domain.BookingStatusPendingPayment, domain.BookingStatusExpired, now)
		require.NoError(t, err)
		assert.Equal(t, 1, updated)
		mockDB.AssertExpectations(t)
	})

	t.Run("LockExpiredPendingBookings propaga errores de base de datos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewBookingRepository(mockDB)

		dbErr := errors.New("deadlock detected")
		mockDB.On("Query", ctx, mock.Anything, []any{"PENDING_PAYMENT", now.UTC(), 500}).Return(nil, dbErr)

		candidates, err := repo.LockExpiredPendingBookings(ctx, domain.BookingStatusPendingPayment, now, 500)
		assert.ErrorIs(t, err, dbErr)
		assert.Nil(t, candidates)
		mockDB.AssertExpectations(t)
	})
}

func TestUnitOfWork_WithTx(t *testing.T) {
	ctx := context.Background()

	t.Run("ejecuta Commit cuando fn retorna nil", func(t *testing.T) {
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Commit", ctx).Return(nil)

		var gotTx pgx.Tx
		err := uow.WithTx(ctx, func(activeTx pgx.Tx) error {
			gotTx = activeTx
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, tx, gotTx)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("ejecuta Rollback cuando fn retorna error", func(t *testing.T) {
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		expectedErr := errors.New("constraint failed")
		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Rollback", ctx).Return(nil)

		err := uow.WithTx(ctx, func(activeTx pgx.Tx) error {
			return expectedErr
		})

		assert.ErrorIs(t, err, expectedErr)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("ejecuta Rollback antes de propagar panic dentro de fn", func(t *testing.T) {
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Rollback", ctx).Return(nil)

		assert.Panics(t, func() {
			_ = uow.WithTx(ctx, func(activeTx pgx.Tx) error {
				panic("boom")
			})
		})

		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("falla explícitamente cuando TxBeginner es nil", func(t *testing.T) {
		uow := postgres.NewUnitOfWork(nil)

		err := uow.WithTx(ctx, func(activeTx pgx.Tx) error {
			return nil
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "TxBeginner")
	})

	t.Run("reutiliza la transacción existente en ctx sin abrir una segunda transacción", func(t *testing.T) {
		txExec := new(MockDBExecutor)
		existingTx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		txCtx := postgres.ContextWithTx(ctx, existingTx)
		var gotTx pgx.Tx
		err := uow.WithTx(txCtx, func(activeTx pgx.Tx) error {
			gotTx = activeTx
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, existingTx, gotTx)
		beginner.AssertNotCalled(t, "Begin", mock.Anything)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		txExec.AssertNotCalled(t, "Rollback", mock.Anything)
	})
}
