package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// ReputationService define el contrato mínimo requerido por ReviewHandler (SOLID · ISP).
type ReputationService interface {
	CreateReview(ctx context.Context, input ports.CreateReviewInput) (*ports.ReviewDTO, error)
	GetUserReviews(ctx context.Context, userID uuid.UUID, limit, offset int) (*ports.UserReputationDTO, error)
}

// ReviewHandler gestiona las peticiones HTTP relativas a calificaciones y reputación
type ReviewHandler struct {
	reputationService ReputationService
}

// NewReviewHandler crea un nuevo handler para el subsistema de reputación
func NewReviewHandler(reputationService ReputationService) *ReviewHandler {
	return &ReviewHandler{reputationService: reputationService}
}

// CreateReviewRequest payload para emitir una calificación (reviewer_id se extrae exclusivamente del token)
type CreateReviewRequest struct {
	BookingID string `json:"booking_id" binding:"required"`
	Rating    int    `json:"rating" binding:"required,min=1,max=5"`
	Comment   string `json:"comment"`
}

// ReviewResponse DTO HTTP para la respuesta de una reseña
type ReviewResponse struct {
	ID                uuid.UUID `json:"id"`
	TripID            uuid.UUID `json:"trip_id"`
	BookingID         uuid.UUID `json:"booking_id"`
	ReviewerID        uuid.UUID `json:"reviewer_id"`
	ReviewerFirstName string    `json:"reviewer_first_name,omitempty"`
	ReviewerLastName  string    `json:"reviewer_last_name,omitempty"`
	ReviewerAvatarURL string    `json:"reviewer_avatar_url,omitempty"`
	RevieweeID        uuid.UUID `json:"reviewee_id"`
	Rating            int       `json:"rating"`
	Comment           string    `json:"comment"`
	CreatedAt         string    `json:"created_at"`
}

// UserReputationResponse DTO HTTP para el perfil reputacional de un usuario
type UserReputationResponse struct {
	UserID      uuid.UUID         `json:"user_id"`
	RatingAvg   float64           `json:"rating_avg"`
	RatingCount int               `json:"rating_count"`
	Reviews     []*ReviewResponse `json:"reviews"`
}

func toReviewResponse(dto *ports.ReviewDTO) *ReviewResponse {
	if dto == nil {
		return nil
	}
	return &ReviewResponse{
		ID:                dto.ID,
		TripID:            dto.TripID,
		BookingID:         dto.BookingID,
		ReviewerID:        dto.ReviewerID,
		ReviewerFirstName: dto.ReviewerFirstName,
		ReviewerLastName:  dto.ReviewerLastName,
		ReviewerAvatarURL: dto.ReviewerAvatarURL,
		RevieweeID:        dto.RevieweeID,
		Rating:            dto.Rating,
		Comment:           dto.Comment,
		CreatedAt:         dto.CreatedAt,
	}
}

func toUserReputationResponse(dto *ports.UserReputationDTO) *UserReputationResponse {
	if dto == nil {
		return nil
	}
	reviews := make([]*ReviewResponse, 0, len(dto.Reviews))
	for _, r := range dto.Reviews {
		reviews = append(reviews, toReviewResponse(r))
	}
	return &UserReputationResponse{
		UserID:      dto.UserID,
		RatingAvg:   dto.RatingAvg,
		RatingCount: dto.RatingCount,
		Reviews:     reviews,
	}
}

// CreateReview procesa la creación de una nueva reseña reputacional
// POST /v1/reviews ó POST /api/v1/reviews
func (h *ReviewHandler) CreateReview(c *gin.Context) {
	reviewerID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "cuerpo de solicitud inválido o incompleto", "INVALID_REQUEST_BODY")
		return
	}

	bookingID, err := domain.ParseUUID(req.BookingID)
	if err != nil {
		RespondBadRequest(c, "ID de reserva con formato UUID inválido", "INVALID_BOOKING_ID")
		return
	}

	input := ports.CreateReviewInput{
		BookingID:  bookingID,
		ReviewerID: reviewerID,
		Rating:     req.Rating,
		Comment:    req.Comment,
	}

	review, err := h.reputationService.CreateReview(c.Request.Context(), input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toReviewResponse(review))
}

// GetUserReviews consulta las opiniones y el puntaje reputacional acumulado de un usuario
// GET /v1/users/:user_id/reviews ó GET /api/v1/users/:user_id/reviews
func (h *ReviewHandler) GetUserReviews(c *gin.Context) {
	rawUserID := strings.TrimSpace(c.Param("user_id"))
	if rawUserID == "" {
		RespondBadRequest(c, "ID de usuario no especificado", "MISSING_USER_ID")
		return
	}
	userID, err := domain.ParseUUID(rawUserID)
	if err != nil {
		RespondBadRequest(c, "ID de usuario con formato UUID inválido", "INVALID_USER_ID")
		return
	}

	limit := 10
	offset := 0

	if l := c.Query("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 && val <= 100 {
			limit = val
		}
	}
	if o := c.Query("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	profile, err := h.reputationService.GetUserReviews(c.Request.Context(), userID, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserReputationResponse(profile))
}
