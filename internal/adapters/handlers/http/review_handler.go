package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// ReviewHandler gestiona las peticiones HTTP relativas a calificaciones y reputación
type ReviewHandler struct {
	reputationService ports.ReputationService
}

// NewReviewHandler crea un nuevo handler para el subsistema de reputación
func NewReviewHandler(reputationService ports.ReputationService) *ReviewHandler {
	return &ReviewHandler{reputationService: reputationService}
}

// CreateReviewRequest payload para emitir una calificación
type CreateReviewRequest struct {
	BookingID string `json:"booking_id" binding:"required"`
	Rating    int    `json:"rating" binding:"required,min=1,max=5"`
	Comment   string `json:"comment"`
}

// CreateReview procesa la creación de una nueva reseña reputacional
// POST /v1/reviews ó POST /api/v1/reviews
func (h *ReviewHandler) CreateReview(c *gin.Context) {
	reviewerID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	var req CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cuerpo de solicitud inválido o incompleto: " + err.Error(),
			"code":  "INVALID_REQUEST_BODY",
		})
		return
	}

	input := ports.CreateReviewInput{
		BookingID:  req.BookingID,
		ReviewerID: reviewerID,
		Rating:     req.Rating,
		Comment:    req.Comment,
	}

	review, err := h.reputationService.CreateReview(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTripNotCompletedForReview):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Solo se pueden calificar viajes y reservas completadas",
				"code":  "TRIP_NOT_COMPLETED_FOR_REVIEW",
			})
			return

		case errors.Is(err, domain.ErrInvalidRating):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "La calificación debe ser un valor entero entre 1 y 5 estrellas",
				"code":  "INVALID_RATING",
			})
			return

		case errors.Is(err, domain.ErrReviewSelfNotAllowed):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "No está permitido calificarse a sí mismo",
				"code":  "SELF_REVIEW_NOT_ALLOWED",
			})
			return

		case errors.Is(err, domain.ErrDuplicateReview):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Ya has emitido una calificación previa para esta reserva",
				"code":  "DUPLICATE_REVIEW",
			})
			return

		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Reserva no encontrada",
				"code":  "BOOKING_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrTripNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Viaje no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No estás autorizado para calificar esta reserva",
				"code":  "FORBIDDEN",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
				"code":  "INTERNAL_SERVER_ERROR",
			})
			return
		}
	}

	c.JSON(http.StatusCreated, review)
}

// GetUserReviews consulta las opiniones y el puntaje reputacional acumulado de un usuario
// GET /v1/users/:user_id/reviews ó GET /api/v1/users/:user_id/reviews
func (h *ReviewHandler) GetUserReviews(c *gin.Context) {
	userID := strings.TrimSpace(c.Param("user_id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de usuario no especificado",
			"code":  "MISSING_USER_ID",
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_SERVER_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, profile)
}
