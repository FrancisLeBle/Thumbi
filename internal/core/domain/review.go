package domain

import (
	"strings"
	"time"
)

// MinRating y MaxRating definen los límites de puntuación del sistema reputacional
const (
	MinRating = 1
	MaxRating = 5
)

// ReviewParams agrupa los datos necesarios para instanciar y validar una calificación
type ReviewParams struct {
	ID            string
	TripID        string
	BookingID     string
	ReviewerID    string
	RevieweeID    string
	Rating        int
	Comment       string
	TripStatus    TripStatus
	BookingStatus BookingStatus
}

// Review representa una evaluación reputacional emitida tras la finalización de un viaje
type Review struct {
	ID         string    `json:"id"`
	TripID     string    `json:"trip_id"`
	BookingID  string    `json:"booking_id"`
	ReviewerID string    `json:"reviewer_id"`
	RevieweeID string    `json:"reviewee_id"`
	Rating     int       `json:"rating"`
	Comment    string    `json:"comment,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// NewReview construye y valida una nueva calificación aplicando las reglas e invariantes de negocio
func NewReview(params ReviewParams) (*Review, error) {
	if strings.TrimSpace(params.ID) == "" ||
		strings.TrimSpace(params.TripID) == "" ||
		strings.TrimSpace(params.BookingID) == "" ||
		strings.TrimSpace(params.ReviewerID) == "" ||
		strings.TrimSpace(params.RevieweeID) == "" {
		return nil, ErrInvalidDisputeData
	}

	// RF-10: Invariante - Un usuario no puede calificarse a sí mismo
	if params.ReviewerID == params.RevieweeID {
		return nil, ErrReviewSelfNotAllowed
	}

	// RF-07 / RF-08: Invariante - Calificación debe ser un entero entre 1 y 5
	if !IsValidRating(params.Rating) {
		return nil, ErrInvalidRating
	}

	// RF-06: Invariante - Solo permitida tras la finalización del viaje y de la reserva
	if params.TripStatus != TripStatusCompleted || params.BookingStatus != BookingStatusCompleted {
		return nil, ErrTripNotCompletedForReview
	}

	nowUTC := time.Now().UTC()
	return &Review{
		ID:         params.ID,
		TripID:     params.TripID,
		BookingID:  params.BookingID,
		ReviewerID: params.ReviewerID,
		RevieweeID: params.RevieweeID,
		Rating:     params.Rating,
		Comment:    strings.TrimSpace(params.Comment),
		CreatedAt:  nowUTC,
		UpdatedAt:  nowUTC,
	}, nil
}

// IsValidRating comprueba si la puntuación numérica se encuentra en el rango permitido (1..5)
func IsValidRating(rating int) bool {
	return rating >= MinRating && rating <= MaxRating
}

// IsPositive indica si la calificación es considerada de satisfacción alta (4 o 5 estrellas)
func (r *Review) IsPositive() bool {
	return r.Rating >= 4
}

// IsNegative indica si la calificación refleja disconformidad (1 o 2 estrellas)
func (r *Review) IsNegative() bool {
	return r.Rating <= 2
}
