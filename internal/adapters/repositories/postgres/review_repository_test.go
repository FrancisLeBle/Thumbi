package postgres_test

import (
	"context"
	"errors"
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

func TestReviewRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("inserta reseña exitosamente con comentario", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		review := &domain.Review{
			ID:         "rev-1",
			TripID:     "trip-1",
			BookingID:  "booking-1",
			ReviewerID: "user-passenger",
			RevieweeID: "user-driver",
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
				args[0] == "rev-1" &&
				args[1] == "trip-1" &&
				args[2] == "booking-1" &&
				args[3] == "user-passenger" &&
				args[4] == "user-driver" &&
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
			ID:         "rev-2",
			TripID:     "trip-1",
			BookingID:  "booking-2",
			ReviewerID: "user-passenger",
			RevieweeID: "user-driver",
			Rating:     4,
			Comment:    "",
			CreatedAt:  now,
			UpdatedAt:  now,
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO reviews")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 9 &&
				args[0] == "rev-2" &&
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
			ID:         "rev-dup",
			TripID:     "trip-1",
			BookingID:  "booking-1",
			ReviewerID: "user-passenger",
			RevieweeID: "user-driver",
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
			ID:         "rev-err",
			TripID:     "trip-1",
			BookingID:  "booking-1",
			ReviewerID: "user-passenger",
			RevieweeID: "user-driver",
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

	t.Run("recupera reseña existente correctamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Muy buen viaje"
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") && strings.Contains(sql, "WHERE id = $1")
		}), []any{"rev-101"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "rev-101"
				*dest[1].(*string) = "trip-202"
				*dest[2].(*string) = "booking-303"
				*dest[3].(*string) = "reviewer-1"
				*dest[4].(*string) = "reviewee-2"
				*dest[5].(*int) = 5
				*dest[6].(**string) = &comment
				*dest[7].(*time.Time) = now
				*dest[8].(*time.Time) = now
				return nil
			},
		})

		rev, err := repo.FindByID(ctx, "rev-101")
		require.NoError(t, err)
		assert.NotNil(t, rev)
		assert.Equal(t, "rev-101", rev.ID)
		assert.Equal(t, "trip-202", rev.TripID)
		assert.Equal(t, "booking-303", rev.BookingID)
		assert.Equal(t, "reviewer-1", rev.ReviewerID)
		assert.Equal(t, "reviewee-2", rev.RevieweeID)
		assert.Equal(t, 5, rev.Rating)
		assert.Equal(t, "Muy buen viaje", rev.Comment)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrReviewNotFound cuando la reseña no existe", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"non-existent"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		rev, err := repo.FindByID(ctx, "non-existent")
		assert.ErrorIs(t, err, domain.ErrReviewNotFound)
		assert.Nil(t, rev)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_FindByBookingAndReviewer(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("recupera calificación por reserva y evaluador", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Puntual"
		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") &&
				strings.Contains(sql, "booking_id = $1") &&
				strings.Contains(sql, "reviewer_id = $2")
		}), []any{"booking-1", "user-1"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "rev-1"
				*dest[1].(*string) = "trip-1"
				*dest[2].(*string) = "booking-1"
				*dest[3].(*string) = "user-1"
				*dest[4].(*string) = "driver-1"
				*dest[5].(*int) = 4
				*dest[6].(**string) = &comment
				*dest[7].(*time.Time) = now
				*dest[8].(*time.Time) = now
				return nil
			},
		})

		rev, err := repo.FindByBookingAndReviewer(ctx, "booking-1", "user-1")
		require.NoError(t, err)
		assert.NotNil(t, rev)
		assert.Equal(t, "rev-1", rev.ID)
		assert.Equal(t, 4, rev.Rating)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrReviewNotFound cuando no existe calificación previa", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"booking-1", "user-1"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		rev, err := repo.FindByBookingAndReviewer(ctx, "booking-1", "user-1")
		assert.ErrorIs(t, err, domain.ErrReviewNotFound)
		assert.Nil(t, rev)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_ListByUserID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("retorna lista paginada de reseñas recibidas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		comment := "Excelente acompañante"
		mockRows := NewMockRows([][]any{
			{"rev-1", "trip-1", "b-1", "reviewer-1", "target-user", 5, &comment, now, now},
			{"rev-2", "trip-2", "b-2", "reviewer-2", "target-user", 4, nil, now, now},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM reviews") &&
				strings.Contains(sql, "WHERE reviewee_id = $1") &&
				strings.Contains(sql, "ORDER BY created_at DESC") &&
				strings.Contains(sql, "LIMIT $2 OFFSET $3")
		}), []any{"target-user", 10, 0}).Return(mockRows, nil)

		reviews, err := repo.ListByUserID(ctx, "target-user", 10, 0)
		require.NoError(t, err)
		assert.Len(t, reviews, 2)
		assert.Equal(t, "rev-1", reviews[0].ID)
		assert.Equal(t, "Excelente acompañante", reviews[0].Comment)
		assert.Equal(t, "rev-2", reviews[1].ID)
		assert.Equal(t, "", reviews[1].Comment)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna lista vacía si el usuario no tiene reseñas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.Anything, []any{"new-user", 10, 0}).Return(mockRows, nil)

		reviews, err := repo.ListByUserID(ctx, "new-user", 10, 0)
		require.NoError(t, err)
		assert.Empty(t, reviews)
		mockDB.AssertExpectations(t)
	})
}

func TestReviewRepository_GetAverageRating(t *testing.T) {
	ctx := context.Background()

	t.Run("calcula promedio y conteo para usuario con reseñas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "AVG(rating)") &&
				strings.Contains(sql, "COUNT(*)") &&
				strings.Contains(sql, "WHERE reviewee_id = $1")
		}), []any{"user-active"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*float64) = 4.85
				*dest[1].(*int) = 14
				return nil
			},
		})

		avg, count, err := repo.GetAverageRating(ctx, "user-active")
		require.NoError(t, err)
		assert.Equal(t, 4.85, avg)
		assert.Equal(t, 14, count)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna 5.00 de reputación inicial si count es 0", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewReviewRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"user-new"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*float64) = 5.00
				*dest[1].(*int) = 0
				return nil
			},
		})

		avg, count, err := repo.GetAverageRating(ctx, "user-new")
		require.NoError(t, err)
		assert.Equal(t, 5.00, avg)
		assert.Equal(t, 0, count)
		mockDB.AssertExpectations(t)
	})
}
