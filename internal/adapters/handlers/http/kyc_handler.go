package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type KYCHandler struct {
	kycService ports.KYCService
}

func NewKYCHandler(kycService ports.KYCService) *KYCHandler {
	return &KYCHandler{kycService: kycService}
}

type KYCSubmissionRequest struct {
	DocumentType     string `json:"document_type" binding:"required"` // DNI, Cédula, Pasaporte
	DocumentNumber   string `json:"document_number" binding:"required"`
	FrontImageBase64 string `json:"front_image_base64" binding:"required"`
	BackImageBase64  string `json:"back_image_base64" binding:"required"`
	Selfie3DBase64   string `json:"selfie_3d_base64" binding:"required"`
}

// Submit procesa la validación de identidad con DNI y selfie 3D (RF-07, RF-08, RF-09)
// POST /v1/kyc/submit
func (h *KYCHandler) Submit(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	var req KYCSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "datos de verificación biométrica incompletos",
			"details": err.Error(),
		})
		return
	}

	kyc, err := h.kycService.SubmitVerification(c.Request.Context(), ports.KYCSubmissionInput{
		UserID:           userID,
		DocumentType:     req.DocumentType,
		DocumentNumber:   req.DocumentNumber,
		FrontImageBase64: req.FrontImageBase64,
		BackImageBase64:  req.BackImageBase64,
		Selfie3DBase64:   req.Selfie3DBase64,
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrKYCAlreadyApproved):
			c.JSON(http.StatusConflict, gin.H{
				"error":  err.Error(),
				"status": domain.KYCStatusApproved,
			})
			return

		case errors.Is(err, domain.ErrMaxRetriesExceeded):
			// RF-09: Tras superar 3 reintentos, estado PENDING_MANUAL_REVIEW
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":          "Se ha superado el máximo de 3 reintentos automatizados. Tu solicitud ha sido derivada a revisión manual por nuestra mesa de control.",
				"code":           "PENDING_MANUAL_REVIEW",
				"status":         domain.KYCStatusPendingManualReview,
				"retry_count":    kyc.RetryCount,
				"liveness_score": kyc.LivenessScore,
				"can_retry":      false,
			})
			return

		case errors.Is(err, domain.ErrKYCInManualReview):
			c.JSON(http.StatusLocked, gin.H{
				"error":  "Tu trámite se encuentra actualmente en revisión manual. Te notificaremos cuando se complete el análisis.",
				"status": domain.KYCStatusPendingManualReview,
			})
			return

		case errors.Is(err, domain.ErrLivenessCheckFailed):
			// RF-08: Tolerancia de reintento (< 3)
			remaining := domain.MaxKYCRetries - kyc.RetryCount
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":             "La validación biométrica no fue satisfactoria.",
				"reason":            kyc.RejectionReason,
				"status":            kyc.Status,
				"retry_count":       kyc.RetryCount,
				"remaining_retries": remaining,
				"can_retry":         kyc.CanRetry(),
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	// Éxito
	c.JSON(http.StatusOK, gin.H{
		"message":        "Verificación de identidad aprobada exitosamente.",
		"status":         kyc.Status,
		"liveness_score": kyc.LivenessScore,
		"kyc_id":         kyc.ID,
	})
}

// Status consulta el estado de la verificación KYC del usuario autenticado
// GET /v1/kyc/status
func (h *KYCHandler) Status(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	kyc, err := h.kycService.GetStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"status":    domain.KYCStatusNotStarted,
				"can_retry": true,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	remaining := domain.MaxKYCRetries - kyc.RetryCount
	if remaining < 0 {
		remaining = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"kyc_id":            kyc.ID,
		"status":            kyc.Status,
		"retry_count":       kyc.RetryCount,
		"remaining_retries": remaining,
		"can_retry":         kyc.CanRetry(),
		"rejection_reason":  kyc.RejectionReason,
		"updated_at":        kyc.UpdatedAt,
	})
}
