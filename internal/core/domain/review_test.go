package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestReview_ValidCreation(t *testing.T) {
	params := domain.ReviewParams{
		ID:            "rev-1",
		TripID:        "trip-100",
		BookingID:     "book-100",
		ReviewerID:    "passenger-1",
		RevieweeID:    "driver-1",
		Rating:        5,
		Comment:       "Excelente viaje, muy puntual y conducción prudente.",
		TripStatus:    domain.TripStatusCompleted,
		BookingStatus: domain.BookingStatusCompleted,
	}

	review, err := domain.NewReview(params)
	assert.NoError(t, err)
	assert.NotNil(t, review)
	assert.Equal(t, "rev-1", review.ID)
	assert.Equal(t, 5, review.Rating)
	assert.True(t, review.IsPositive())
	assert.False(t, review.IsNegative())
	assert.Equal(t, "Excelente viaje, muy puntual y conducción prudente.", review.Comment)
}

func TestReview_RejectSelfReview(t *testing.T) {
	params := domain.ReviewParams{
		ID:            "rev-2",
		TripID:        "trip-100",
		BookingID:     "book-100",
		ReviewerID:    "user-1",
		RevieweeID:    "user-1", // Mismo usuario
		Rating:        5,
		Comment:       "Me auto califico con 5 estrellas.",
		TripStatus:    domain.TripStatusCompleted,
		BookingStatus: domain.BookingStatusCompleted,
	}

	review, err := domain.NewReview(params)
	assert.ErrorIs(t, err, domain.ErrReviewSelfNotAllowed, "RF-10: No se permite autocalificación")
	assert.Nil(t, review)
}

func TestReview_InvalidRating(t *testing.T) {
	// Puntuación 0 (menor a 1)
	paramsLow := domain.ReviewParams{
		ID:            "rev-3",
		TripID:        "trip-100",
		BookingID:     "book-100",
		ReviewerID:    "passenger-1",
		RevieweeID:    "driver-1",
		Rating:        0,
		TripStatus:    domain.TripStatusCompleted,
		BookingStatus: domain.BookingStatusCompleted,
	}
	review, err := domain.NewReview(paramsLow)
	assert.ErrorIs(t, err, domain.ErrInvalidRating)
	assert.Nil(t, review)

	// Puntuación 6 (mayor a 5)
	paramsHigh := paramsLow
	paramsHigh.Rating = 6
	review, err = domain.NewReview(paramsHigh)
	assert.ErrorIs(t, err, domain.ErrInvalidRating)
	assert.Nil(t, review)
}

func TestReview_TripNotCompleted(t *testing.T) {
	// Viaje aún en progreso
	params := domain.ReviewParams{
		ID:            "rev-4",
		TripID:        "trip-100",
		BookingID:     "book-100",
		ReviewerID:    "passenger-1",
		RevieweeID:    "driver-1",
		Rating:        4,
		TripStatus:    domain.TripStatusInProgress,
		BookingStatus: domain.BookingStatusConfirmed,
	}

	review, err := domain.NewReview(params)
	assert.ErrorIs(t, err, domain.ErrTripNotCompletedForReview, "RF-06: No se puede calificar viaje no completado")
	assert.Nil(t, review)
}

func TestUser_UpdateReputation(t *testing.T) {
	user := domain.NewUser("usr-1", "ana@example.com", "Ana", "Gomez", "", domain.ProviderGoogle, "g-1")
	assert.Equal(t, 5.00, user.RatingAvg)
	assert.Equal(t, 0, user.RatingCount)

	// Actualización tras recibir 3 reseñas con promedio 4.67
	user.UpdateReputation(4.67, 3)
	assert.Equal(t, 4.67, user.RatingAvg)
	assert.Equal(t, 3, user.RatingCount)
}
