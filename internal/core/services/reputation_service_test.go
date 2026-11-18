package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func TestReputationService_CreateReview(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000999")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000001")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000002")
	intruderID := uuid.MustParse("01960000-0000-7000-8000-000000000003")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000888")
	missingBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000899")
	pendingBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000877")
	reviewID := uuid.MustParse("01960000-0000-7000-8000-000000000777")

	completedTrip := &domain.Trip{
		ID:       tripID,
		DriverID: driverID,
		Status:   domain.TripStatusCompleted,
	}

	completedBooking := &domain.Booking{
		ID:          bookingID,
		TripID:      tripID,
		PassengerID: passengerID,
		Status:      domain.BookingStatusCompleted,
	}

	t.Run("falla si la reserva no existe", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, missingBookingID).Return(nil, domain.ErrBookingNotFound)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  missingBookingID,
			ReviewerID: passengerID,
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
			ID:          pendingBookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			Status:      domain.BookingStatusConfirmed, // No completada aún
		}
		mockBookingRepo.On("FindByID", ctx, pendingBookingID).Return(pendingBooking, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  pendingBookingID,
			ReviewerID: passengerID,
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

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(completedTrip, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: intruderID,
			Rating:     5,
		})

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si la calificación ya existe para esta reserva (RF-09)", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(completedTrip, nil)

		existingReview := &domain.Review{
			ID:         reviewID,
			BookingID:  bookingID,
			ReviewerID: passengerID,
			Rating:     4,
		}
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, bookingID, passengerID).Return(existingReview, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: passengerID,
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
			BookingID:  bookingID,
			ReviewerID: passengerID,
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

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(completedTrip, nil)
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, bookingID, passengerID).Return(nil, nil)
		mockReviewRepo.On("Save", ctx, mock.AnythingOfType("*domain.Review")).Return(nil)

		passengerUser := &domain.User{
			ID:        passengerID,
			FirstName: "Carlos",
			LastName:  "Mendez",
			AvatarURL: "https://avatar.com/carlos.jpg",
		}
		mockUserRepo.On("GetByID", ctx, passengerID).Return(passengerUser, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, mockUserRepo)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: passengerID,
			Rating:     5,
			Comment:    "Conductor muy educado y viaje muy cómodo.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 5, res.Rating)
		assert.Equal(t, driverID, res.RevieweeID)
		assert.Equal(t, passengerID, res.ReviewerID)
		assert.Equal(t, "Carlos", res.ReviewerFirstName)
		assert.Equal(t, "Conductor muy educado y viaje muy cómodo.", res.Comment)

		mockReviewRepo.AssertExpectations(t)
	})

	t.Run("crea exitosamente reseña del conductor hacia el pasajero", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(completedBooking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(completedTrip, nil)
		mockReviewRepo.On("FindByBookingAndReviewer", ctx, bookingID, driverID).Return(nil, nil)
		mockReviewRepo.On("Save", ctx, mock.AnythingOfType("*domain.Review")).Return(nil)

		driverUser := &domain.User{
			ID:        driverID,
			FirstName: "Mariana",
			LastName:  "Lopez",
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(driverUser, nil)

		svc := services.NewReputationService(mockReviewRepo, mockBookingRepo, mockTripRepo, mockUserRepo)
		res, err := svc.CreateReview(ctx, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: driverID,
			Rating:     5,
			Comment:    "Excelente pasajero, muy puntual.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, 5, res.Rating)
		assert.Equal(t, passengerID, res.RevieweeID)
		assert.Equal(t, driverID, res.ReviewerID)
		assert.Equal(t, "Mariana", res.ReviewerFirstName)

		mockReviewRepo.AssertExpectations(t)
	})

	_ = now
}

func TestReputationService_GetUserReviews(t *testing.T) {
	ctx := context.Background()

	targetUserID := uuid.MustParse("01960000-0000-7000-8000-000000000601")
	unknownUserID := uuid.MustParse("01960000-0000-7000-8000-000000000699")
	reviewerUserID := uuid.MustParse("01960000-0000-7000-8000-000000000602")
	revID := uuid.MustParse("01960000-0000-7000-8000-000000000603")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000604")
	bookID := uuid.MustParse("01960000-0000-7000-8000-000000000605")

	targetUser := &domain.User{
		ID:          targetUserID,
		FirstName:   "Sofia",
		LastName:    "Perez",
		RatingAvg:   4.85,
		RatingCount: 12,
	}

	t.Run("falla si el usuario no existe", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockUserRepo := new(MockUserRepository)

		mockUserRepo.On("GetByID", ctx, unknownUserID).Return(nil, domain.ErrUserNotFound)

		svc := services.NewReputationService(mockReviewRepo, nil, nil, mockUserRepo)
		res, err := svc.GetUserReviews(ctx, unknownUserID, 10, 0)

		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		assert.Nil(t, res)
	})

	t.Run("retorna perfil reputacional con reseñas", func(t *testing.T) {
		mockReviewRepo := new(MockReviewRepository)
		mockUserRepo := new(MockUserRepository)

		mockUserRepo.On("GetByID", ctx, targetUserID).Return(targetUser, nil)

		rev1 := &domain.Review{
			ID:         revID,
			TripID:     tripID,
			BookingID:  bookID,
			ReviewerID: reviewerUserID,
			RevieweeID: targetUserID,
			Rating:     5,
			Comment:    "Todo perfecto",
			CreatedAt:  time.Now().UTC(),
		}
		mockReviewRepo.On("ListByUserID", ctx, targetUserID, 20, 0).Return([]*domain.Review{rev1}, nil)
		mockReviewRepo.On("GetAverageRating", ctx, targetUserID).Return(4.85, 12, nil)

		reviewer := &domain.User{
			ID:        reviewerUserID,
			FirstName: "Lucas",
			LastName:  "Gomez",
		}
		mockUserRepo.On("GetByID", ctx, reviewerUserID).Return(reviewer, nil)

		svc := services.NewReputationService(mockReviewRepo, nil, nil, mockUserRepo)
		res, err := svc.GetUserReviews(ctx, targetUserID, 0, 0)

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, targetUserID, res.UserID)
		assert.Equal(t, 4.85, res.RatingAvg)
		assert.Equal(t, 12, res.RatingCount)
		assert.Len(t, res.Reviews, 1)
		assert.Equal(t, "Lucas", res.Reviews[0].ReviewerFirstName)
	})
}
