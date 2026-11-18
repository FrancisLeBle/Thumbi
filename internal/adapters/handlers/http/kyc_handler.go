package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// KYCService define el contrato mínimo requerido por KYCHandler (SOLID · ISP).
type KYCService interface {
	SubmitVerification(ctx context.Context, input ports.KYCSubmissionInput) (*ports.KYCDTO, error)
	GetStatus(ctx context.Context, userID uuid.UUID) (*ports.KYCDTO, error)
}

type KYCHandler struct {
	kycService KYCService
}

func NewKYCHandler(kycService KYCService) *KYCHandler {
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
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req KYCSubmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "datos de verificación biométrica incompletos o inválidos", "INVALID_KYC_PAYLOAD")
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
				"error":      "La identidad del usuario ya ha sido verificada y aprobada previamente",
				"code":       "KYC_ALREADY_APPROVED",
				"status":     domain.KYCStatusApproved,
				"request_id": middleware.GetRequestID(c),
			})
			return

		case errors.Is(err, domain.ErrMaxRetriesExceeded):
			// RF-09: Tras superar 3 reintentos, estado PENDING_MANUAL_REVIEW
			retryCount := 3
			livenessScore := 0.0
			if kyc != nil {
				retryCount = kyc.RetryCount
				livenessScore = kyc.LivenessScore
			}
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":          "Se ha superado el máximo de 3 reintentos automatizados. Tu solicitud ha sido derivada a revisión manual por nuestra mesa de control.",
				"code":           "PENDING_MANUAL_REVIEW",
				"status":         domain.KYCStatusPendingManualReview,
				"retry_count":    retryCount,
				"liveness_score": livenessScore,
				"can_retry":      false,
				"request_id":     middleware.GetRequestID(c),
			})
			return

		case errors.Is(err, domain.ErrKYCInManualReview):
			c.JSON(http.StatusLocked, gin.H{
				"error":      "Tu trámite se encuentra actualmente en revisión manual. Te notificaremos cuando se complete el análisis.",
				"code":       "PENDING_MANUAL_REVIEW",
				"status":     domain.KYCStatusPendingManualReview,
				"request_id": middleware.GetRequestID(c),
			})
			return

		case errors.Is(err, domain.ErrLivenessCheckFailed):
			// RF-08: Tolerancia de reintento (< 3)
			retryCount := 1
			rejectionReason := "Prueba de vida insuficiente"
			status := domain.KYCStatusRejected
			canRetry := true
			remaining := domain.MaxKYCRetries - retryCount
			if kyc != nil {
				retryCount = kyc.RetryCount
				rejectionReason = kyc.RejectionReason
				status = kyc.Status
				canRetry = kyc.CanRetry
				remaining = kyc.RemainingRetries
			}
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error":             "La validación biométrica no fue satisfactoria.",
				"code":              "LIVENESS_CHECK_FAILED",
				"reason":            rejectionReason,
				"status":            status,
				"retry_count":       retryCount,
				"remaining_retries": remaining,
				"can_retry":         canRetry,
				"request_id":        middleware.GetRequestID(c),
			})
			return

		default:
			respondError(c, err)
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
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
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
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"kyc_id":            kyc.ID,
		"status":            kyc.Status,
		"retry_count":       kyc.RetryCount,
		"remaining_retries": kyc.RemainingRetries,
		"can_retry":         kyc.CanRetry,
		"rejection_reason":  kyc.RejectionReason,
		"updated_at":        kyc.UpdatedAt,
	})
}
