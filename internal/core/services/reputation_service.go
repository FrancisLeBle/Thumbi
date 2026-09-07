package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// reputationService implementa ports.ReputationService bajo Clean Architecture
type reputationService struct {
	reviewRepo  ports.ReviewRepository
	bookingRepo ports.BookingRepository
	tripRepo    ports.TripRepository
	userRepo    ports.UserRepository
	clock       func() time.Time
}

// NewReputationService construye una nueva instancia inyectando los repositorios requeridos
func NewReputationService(
	reviewRepo ports.ReviewRepository,
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	userRepo ports.UserRepository,
) ports.ReputationService {
	return &reputationService{
		reviewRepo:  reviewRepo,
		bookingRepo: bookingRepo,
		tripRepo:    tripRepo,
		userRepo:    userRepo,
		clock:       time.Now,
	}
}

// CreateReview emite una calificación entre usuarios (conductor <-> pasajero) tras finalizar el viaje
func (s *reputationService) CreateReview(ctx context.Context, input ports.CreateReviewInput) (*ports.ReviewDTO, error) {
	reviewerID := strings.TrimSpace(input.ReviewerID)
	bookingID := strings.TrimSpace(input.BookingID)
	if reviewerID == "" || bookingID == "" {
		return nil, domain.ErrInvalidRating
	}

	if !domain.IsValidRating(input.Rating) {
		return nil, domain.ErrInvalidRating
	}

	// 1. Obtener la reserva y validar que exista
	booking, err := s.bookingRepo.FindByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	// 2. Validar que la reserva esté en estado COMPLETED (RF-06)
	if booking.Status != domain.BookingStatusCompleted {
		return nil, domain.ErrTripNotCompletedForReview
	}

	// 3. Obtener el viaje y validar que esté en estado COMPLETED (RF-06)
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}
	if trip.Status != domain.TripStatusCompleted {
		return nil, domain.ErrTripNotCompletedForReview
	}

	// 4. Determinar los roles: el evaluador debe ser parte de la reserva
	var revieweeID string
	if reviewerID == booking.PassengerID {
		// Pasajero califica al conductor
		revieweeID = trip.DriverID
	} else if reviewerID == trip.DriverID {
		// Conductor califica al pasajero
		revieweeID = booking.PassengerID
	} else {
		// El usuario no participó en esta reserva
		return nil, domain.ErrUnauthorized
	}

	// 5. Prevenir autocalificaciones (RF-10)
	if reviewerID == revieweeID {
		return nil, domain.ErrReviewSelfNotAllowed
	}

	// 6. Validar que no exista calificación previa para esta reserva por este evaluador (RF-09)
	existing, err := s.reviewRepo.FindByBookingAndReviewer(ctx, bookingID, reviewerID)
	if err == nil && existing != nil {
		return nil, domain.ErrDuplicateReview
	}

	// 7. Instanciar la entidad Review con validación de invariantes
	reviewID := uuid.New().String()
	review, err := domain.NewReview(domain.ReviewParams{
		ID:            reviewID,
		TripID:        trip.ID,
		BookingID:     booking.ID,
		ReviewerID:    reviewerID,
		RevieweeID:    revieweeID,
		Rating:        input.Rating,
		Comment:       strings.TrimSpace(input.Comment),
		TripStatus:    trip.Status,
		BookingStatus: booking.Status,
	})
	if err != nil {
		return nil, err
	}

	// 8. Persistir la reseña
	if err := s.reviewRepo.Save(ctx, review); err != nil {
		return nil, err
	}

	// 9. Construir DTO enriquecido
	dto := &ports.ReviewDTO{
		ID:         review.ID,
		TripID:     review.TripID,
		BookingID:  review.BookingID,
		ReviewerID: review.ReviewerID,
		RevieweeID: review.RevieweeID,
		Rating:     review.Rating,
		Comment:    review.Comment,
		CreatedAt:  review.CreatedAt.Format(time.RFC3339),
	}

	if s.userRepo != nil {
		if reviewer, err := s.userRepo.GetByID(ctx, reviewerID); err == nil && reviewer != nil {
			dto.ReviewerFirstName = reviewer.FirstName
			dto.ReviewerLastName = reviewer.LastName
			dto.ReviewerAvatarURL = reviewer.AvatarURL
		}
	}

	return dto, nil
}

// GetUserReviews consulta el historial paginado de opiniones y el promedio reputacional de un usuario
func (s *reputationService) GetUserReviews(ctx context.Context, userID string, limit, offset int) (*ports.UserReputationDTO, error) {
	trimmedID := strings.TrimSpace(userID)
	if trimmedID == "" {
		return nil, domain.ErrUserNotFound
	}

	// 1. Validar que el usuario exista
	user, err := s.userRepo.GetByID(ctx, trimmedID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	// 2. Ajustar paginación
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// 3. Obtener lista de reseñas
	reviews, err := s.reviewRepo.ListByUserID(ctx, trimmedID, limit, offset)
	if err != nil {
		return nil, err
	}

	// 4. Mapear DTOs de reseñas
	reviewDTOs := make([]*ports.ReviewDTO, 0, len(reviews))
	for _, r := range reviews {
		dto := &ports.ReviewDTO{
			ID:         r.ID,
			TripID:     r.TripID,
			BookingID:  r.BookingID,
			ReviewerID: r.ReviewerID,
			RevieweeID: r.RevieweeID,
			Rating:     r.Rating,
			Comment:    r.Comment,
			CreatedAt:  r.CreatedAt.Format(time.RFC3339),
		}
		if reviewer, err := s.userRepo.GetByID(ctx, r.ReviewerID); err == nil && reviewer != nil {
			dto.ReviewerFirstName = reviewer.FirstName
			dto.ReviewerLastName = reviewer.LastName
			dto.ReviewerAvatarURL = reviewer.AvatarURL
		}
		reviewDTOs = append(reviewDTOs, dto)
	}

	// 5. Obtener promedio y conteo actualizados
	avg, count, err := s.reviewRepo.GetAverageRating(ctx, trimmedID)
	ratingAvg := user.RatingAvg
	ratingCount := user.RatingCount
	if err == nil && count > 0 {
		ratingAvg = avg
		ratingCount = count
	}

	return &ports.UserReputationDTO{
		UserID:      user.ID,
		RatingAvg:   ratingAvg,
		RatingCount: ratingCount,
		Reviews:     reviewDTOs,
	}, nil
}
