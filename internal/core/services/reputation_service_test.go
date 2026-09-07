package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
)

func TestReputationService_CreateReview(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	completedTrip := &domain.Trip{
		ID:       "trip-999",
		DriverID: "driver-1",
		Status:   domain.TripStatusCompleted,
	}

	completedBooking := &domain.Booking{
		ID:          "booking-888",
		TripID:      "trip-999",
		PassengerID: "passenger-1",
		Status:      domain.BookingStatusCompleted,
	}

	t.Run("falla si la reserva no existe", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-nonexistent").Return(nil, domain.ErrBookingNotFound)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-nonexistent",
			ReviewerID: "passenger-1",
			Rating:     5,
			Comment:    "Todo ok",
		})

		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
		assert.Nil(t, res)
	})

	t.Run("falla si la reserva no está completada (RF-06)", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		pendingBooking := &domain.Booking{
			ID:          "booking-pending",
			TripID:      "trip-999",
			PassengerID: "passenger-1",
			Status:      domain.BookingStatusConfirmed, // No completada aún
		}
		mockBookingRepo.On("FindByID", ctx, "booking-pending").Return(pendingBooking, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-pending",
			ReviewerID: "passenger-1",
			Rating:     5,
			Comment:    "Viaje aún no concluido",
		})

		assert.ErrorIs(t, err, domain.ErrTripNotCompletedForReview)
		assert.Nil(t, res)
	})

	t.Run("falla si el usuario evaluador no pertenece a la reserva", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-888").Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-999").Return(completedTrip, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-888",
			ReviewerID: "intruder-user",
			Rating:     5,
		})

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si la calificación ya existe para esta reserva (RF-09)", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-888").Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-999").Return(completedTrip, nil)

		existingReview := &domain.Review{
			ID:         "rev-prev",
			BookingID:  "booking-888",
			ReviewerID: "passenger-1",
			Rating:     4,
		}
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, "booking-888", "passenger-1").Return(existingReview, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-888",
			ReviewerID: "passenger-1",
			Rating:     5,
		})

		assert.ErrorIs(t, err, domain.ErrDuplicateReview)
		assert.Nil(t, res)
	})

	t.Run("falla si el puntaje es inválido (fuera de rango 1..5)", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-888",
			ReviewerID: "passenger-1",
			Rating:     6, // Inválido
		})

		assert.ErrorIs(t, err, domain.ErrInvalidRating)
		assert.Nil(t, res)
	})

	t.Run("crea exitosamente reseña del pasajero hacia el conductor", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-888").Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-999").Return(completedTrip, nil)
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, "booking-888", "passenger-1").Return(nil, nil)
		mockReviewRepo.On("Save", ctx, mock.AnythingOfType("*domain.Review")).Return(nil)

		passengerUser := &domain.User{
			ID:        "passenger-1",
			FirstName: "Carlos",
			LastName:  "Mendez",
			AvatarURL: "https://avatar.com/carlos.jpg",
		}
		mockUserRepo.On("GetByID", ctx, "passenger-1").Return(passengerUser, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, mockUserRepo)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-888",
			ReviewerID: "passenger-1",
			Rating:     5,
			Comment:    "Conductor muy educado y viaje muy cómodo.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 5, res.Rating)
		assert.Equal(t, "driver-1", res.RevieweeID)
		assert.Equal(t, "passenger-1", res.ReviewerID)
		assert.Equal(t, "Carlos", res.ReviewerFirstName)
		assert.Equal(t, "Conductor muy educado y viaje muy cómodo.", res.Comment)

		mockReviewRepo.AssertExpectations(t)
	})

	t.Run("crea exitosamente reseña del conductor hacia el pasajero", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-888").Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-999").Return(completedTrip, nil)
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, "booking-888", "driver-1").Return(nil, nil)
		mockReviewRepo.On("Save", ctx, mock.AnythingOfType("*domain.Review")).Return(nil)

		driverUser := &domain.User{
			ID:        "driver-1",
			FirstName: "Mariana",
			LastName:  "Lopez",
		}
		mockUserRepo.On("GetByID", ctx, "driver-1").Return(driverUser, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, mockUserRepo)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  "booking-888",
			ReviewerID: "driver-1",
			Rating:     5,
			Comment:    "Excelente pasajero, muy puntual.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 5, res.Rating)
		assert.Equal(t, "passenger-1", res.RevieweeID)
		assert.Equal(t, "driver-1", res.ReviewerID)
		assert.Equal(t, "Mariana", res.ReviewerFirstName)

		mockReviewRepo.AssertExpectations(t)
	})

	_ = now
}

func TestReputationService_GetUserReviews(t *testing.T) {
	ctx := context.Background()

	targetUser := &domain.User{
		ID:          "target-user-1",
		FirstName:   "Sofia",
		LastName:    "Perez",
		RatingAvg:   4.85,
		RatingCount: 12,
	}

	t.Run("falla si el usuario no existe", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockUserRepo := new(MockUserRepository)

		mockUserRepo.On("GetByID", ctx, "unknown").Return(nil, domain.ErrUserNotFound)

		svc := services.NewReputationService(mockReviewRepo, nil, nil, mockUserRepo)
		res, err := svc.GetUserReviews(ctx, "unknown", 10, 0)

		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		assert.Nil(t, res)
	})

	t.Run("retorna perfil reputacional con reseñas", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockUserRepo := new(MockUserRepository)

		mockUserRepo.On("GetByID", ctx, "target-user-1").Return(targetUser, nil)

		rev1 := &domain.Review{
			ID:         "rev-1",
			TripID:     "trip-1",
			BookingID:  "book-1",
			ReviewerID: "rev-user-1",
			RevieweeID: "target-user-1",
			Rating:     5,
			Comment:    "Todo perfecto",
			CreatedAt:  time.Now().UTC(),
		}
		mockReviewRepo.On("ListByUserID", ctx, "target-user-1", 20, 0).Return([]*domain.Review{rev1}, nil)
		mockReviewRepo.On("GetAverageRating", ctx, "target-user-1").Return(4.85, 12, nil)

		reviewer := &domain.User{
			ID:        "rev-user-1",
			FirstName: "Lucas",
			LastName:  "Gomez",
		}
		mockUserRepo.On("GetByID", ctx, "rev-user-1").Return(reviewer, nil)

		svc := services.NewReputationService(mockReviewRepo, nil, nil, mockUserRepo)
		res, err := svc.GetUserReviews(ctx, "target-user-1", 0, 0)

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "target-user-1", res.UserID)
		assert.Equal(t, 4.85, res.RatingAvg)
		assert.Equal(t, 12, res.RatingCount)
		assert.Len(t, res.Reviews, 1)
		assert.Equal(t, "Lucas", res.Reviews[0].ReviewerFirstName)
	})
}
