package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestReviewRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	revID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	rev2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	revDupID := uuid.MustParse("01933e3d-0000-7000-8000-000000000003")
	revErrID := uuid.MustParse("01933e3d-0000-7000-8000-000000000004")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000011")
	booking1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000021")
	booking2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000022")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000031")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000041")

	t.Run("inserta reseña exitosamente con comentario", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		review := &domain.Review{
			ID:         revID,
			TripID:     tripID,
			BookingID:  booking1ID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     5,
			Comment:    "Excelente viaje, muy puntual y seguro.",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO reviews") &&
				strings.Contains(sql, "reviewer_id") &&
				strings.Contains(sql, "reviewee_id")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 9 &&
				args[0] == revID &&
				args[1] == tripID &&
				args[2] == booking1ID &&
				args[3] == passengerID &&
				args[4] == driverID &&
				args[5] == 5 &&
				args[6] != nil && *args[6].(*string) == "Excelente viaje, muy puntual y seguro."
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err := repo.Save(ctx, review)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("inserta reseña sin comentario (pasa nil a db)", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		review := &domain.Review{
			ID:         rev2ID,
			TripID:     tripID,
			BookingID:  booking2ID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     4,
			Comment:    "",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO reviews")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 9 &&
				args[0] == rev2ID &&
				args[6] == (*string)(nil)
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err := repo.Save(ctx, review)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDuplicateReview si ya existe calificación para la reserva", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		review := &domain.Review{
			ID:         revDupID,
			TripID:     tripID,
			BookingID:  booking1ID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     5,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, pgErr)

		err := repo.Save(ctx, review)
		assert.ErrorIs(t, err, domain.ErrDuplicateReview)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna error genérico ante fallo de base de datos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		review := &domain.Review{
			ID:         revErrID,
			TripID:     tripID,
			BookingID:  booking1ID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     5,
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("db connection lost"))

		err := repo.Save(ctx, review)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al insertar reseña")
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	revID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000303")
	reviewerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000401")
	revieweeID := uuid.MustParse("01933e3d-0000-7000-8000-000000000402")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	t.Run("recupera reseña existente correctamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Muy buen viaje"
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") && strings.Contains(sql, "WHERE id = $1")
		}), []any{revID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = revID
				*dest[1].(*uuid.UUID) = tripID
				*dest[2].(*uuid.UUID) = bookingID
				*dest[3].(*uuid.UUID) = reviewerID
				*dest[4].(*uuid.UUID) = revieweeID
				*dest[5].(*int) = 5
				*dest[6].(**string) = &comment
				*dest[7].(*time.Time) = now
				*dest[8].(*time.Time) = now
				return nil
			},
		})

		rev, err := repo.FindByID(ctx, revID)
		require.NoError(t, err)
		assert.NotNil(t, rev)
		assert.Equal(t, revID, rev.ID)
		assert.Equal(t, tripID, rev.TripID)
		assert.Equal(t, bookingID, rev.BookingID)
		assert.Equal(t, reviewerID, rev.ReviewerID)
		assert.Equal(t, revieweeID, rev.RevieweeID)
		assert.Equal(t, 5, rev.Rating)
		assert.Equal(t, "Muy buen viaje", rev.Comment)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrReviewNotFound cuando la reseña no existe", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{missingID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		rev, err := repo.FindByID(ctx, missingID)
		assert.ErrorIs(t, err, domain.ErrReviewNotFound)
		assert.Nil(t, rev)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_FindByBookingAndReviewer(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	revID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000003")
	userID := uuid.MustParse("01933e3d-0000-7000-8000-000000000004")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000005")

	t.Run("recupera calificación por reserva y evaluador", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Puntual"
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") &&
				strings.Contains(sql, "booking_id = $1") &&
				strings.Contains(sql, "reviewer_id = $2")
		}), []any{bookingID, userID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = revID
				*dest[1].(*uuid.UUID) = tripID
				*dest[2].(*uuid.UUID) = bookingID
				*dest[3].(*uuid.UUID) = userID
				*dest[4].(*uuid.UUID) = driverID
				*dest[5].(*int) = 4
				*dest[6].(**string) = &comment
				*dest[7].(*time.Time) = now
				*dest[8].(*time.Time) = now
				return nil
			},
		})

		rev, err := repo.FindByBookingAndReviewer(ctx, bookingID, userID)
		require.NoError(t, err)
		assert.NotNil(t, rev)
		assert.Equal(t, revID, rev.ID)
		assert.Equal(t, 4, rev.Rating)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrReviewNotFound cuando no existe calificación previa", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{bookingID, userID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		rev, err := repo.FindByBookingAndReviewer(ctx, bookingID, userID)
		assert.ErrorIs(t, err, domain.ErrReviewNotFound)
		assert.Nil(t, rev)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_ListByUserID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	rev1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	rev2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	trip1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000011")
	trip2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000012")
	b1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000021")
	b2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000022")
	reviewer1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000031")
	reviewer2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000032")
	targetUserID := uuid.MustParse("01933e3d-0000-7000-8000-000000000099")
	newUserID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")

	t.Run("retorna lista paginada de reseñas recibidas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Excelente acompañante"
		mockRows := NewMockRows([][]any{
			{rev1ID, trip1ID, b1ID, reviewer1ID, targetUserID, 5, &comment, now, now},
			{rev2ID, trip2ID, b2ID, reviewer2ID, targetUserID, 4, nil, now, now},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") &&
				strings.Contains(sql, "WHERE reviewee_id = $1") &&
				strings.Contains(sql, "ORDER BY created_at DESC") &&
				strings.Contains(sql, "LIMIT $2 OFFSET $3")
		}), []any{targetUserID, 10, 0}).Return(mockRows, nil)

		reviews, err := repo.ListByUserID(ctx, targetUserID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, reviews, 2)
		assert.Equal(t, rev1ID, reviews[0].ID)
		assert.Equal(t, "Excelente acompañante", reviews[0].Comment)
		assert.Equal(t, rev2ID, reviews[1].ID)
		assert.Equal(t, "", reviews[1].Comment)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna lista vacía si el usuario no tiene reseñas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.Anything, []any{newUserID, 10, 0}).Return(mockRows, nil)

		reviews, err := repo.ListByUserID(ctx, newUserID, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, reviews)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_GetAverageRating(t *testing.T) {
	ctx := context.Background()
	activeUserID := uuid.MustParse("01933e3d-0000-7000-8000-000000000501")
	newUserID := uuid.MustParse("01933e3d-0000-7000-8000-000000000502")

	t.Run("calcula promedio y conteo para usuario con reseñas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "AVG(rating)") &&
				strings.Contains(sql, "COUNT(*)") &&
				strings.Contains(sql, "WHERE reviewee_id = $1")
		}), []any{activeUserID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*float64) = 4.85
				*dest[1].(*int) = 14
				return nil
			},
		})

		avg, count, err := repo.GetAverageRating(ctx, activeUserID)
		require.NoError(t, err)
		assert.Equal(t, 4.85, avg)
		assert.Equal(t, 14, count)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna 5.00 de reputación inicial si count es 0", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{newUserID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*float64) = 5.00
				*dest[1].(*int) = 0
				return nil
			},
		})

		avg, count, err := repo.GetAverageRating(ctx, newUserID)
		require.NoError(t, err)
		assert.Equal(t, 5.00, avg)
		assert.Equal(t, 0, count)
		mockDB.AssertExpectations(t)
	})
}
