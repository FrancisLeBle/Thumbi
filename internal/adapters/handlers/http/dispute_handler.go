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

// DisputeHandler gestiona los endpoints relacionados con la apertura y arbitraje de disputas
type DisputeHandler struct {
	disputeService ports.DisputeService
}

// NewDisputeHandler crea una nueva instancia de DisputeHandler
func NewDisputeHandler(disputeService ports.DisputeService) *DisputeHandler {
	return &DisputeHandler{disputeService: disputeService}
}

// OpenDisputeRequest payload para registrar un nuevo reclamo formal
type OpenDisputeRequest struct {
	BookingID    string               `json:"booking_id" binding:"required"`
	Reason       domain.DisputeReason `json:"reason" binding:"required"`
	Description  string               `json:"description" binding:"required"`
	EvidenceURLs []string             `json:"evidence_urls"`
}

// OpenDispute registra una reclamación formal y congela la transacción de custodia en DISPUTED
// POST /v1/disputes ó POST /api/v1/disputes
func (h *DisputeHandler) OpenDispute(c *gin.Context) {
	reporterID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	var req OpenDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cuerpo de solicitud incompleto: " + err.Error(),
			"code":  "INVALID_REQUEST_BODY",
		})
		return
	}

	input := ports.OpenDisputeInput{
		BookingID:    req.BookingID,
		ReporterID:   reporterID,
		Reason:       req.Reason,
		Description:  req.Description,
		EvidenceURLs: req.EvidenceURLs,
	}

	dispute, err := h.disputeService.OpenDispute(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDisputeAlreadyExists):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Ya existe una disputa activa sobre esta transacción de custodia",
				"code":  "DISPUTE_ALREADY_EXISTS",
			})
			return

		case errors.Is(err, domain.ErrEscrowAlreadySettled):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Los fondos en custodia ya han sido liquidados o reembolsados previamente",
				"code":  "ESCROW_ALREADY_SETTLED",
			})
			return

		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Reserva no encontrada",
				"code":  "BOOKING_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrEscrowNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Transacción de custodia no encontrada",
				"code":  "ESCROW_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrTripNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Viaje asociado no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No perteneces a la reserva o viaje para presentar este reclamo",
				"code":  "FORBIDDEN",
			})
			return

		case errors.Is(err, domain.ErrDisputeSelfNotAllowed):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "No es posible disputar una transacción contra uno mismo",
				"code":  "DISPUTE_SELF_NOT_ALLOWED",
			})
			return

		case errors.Is(err, domain.ErrInvalidDisputeData):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Datos de la disputa inválidos o insuficientes",
				"code":  "INVALID_DISPUTE_DATA",
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

	c.JSON(http.StatusCreated, dispute)
}

// GetDisputeByID consulta el detalle de una disputa por su ID
// GET /v1/disputes/:id ó GET /api/v1/disputes/:id
func (h *DisputeHandler) GetDisputeByID(c *gin.Context) {
	disputeID := strings.TrimSpace(c.Param("id"))
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de disputa no especificado",
			"code":  "MISSING_DISPUTE_ID",
		})
		return
	}

	dispute, err := h.disputeService.GetDisputeByID(c.Request.Context(), disputeID)
	if err != nil {
		if errors.Is(err, domain.ErrDisputeNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Disputa no encontrada",
				"code":  "DISPUTE_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_SERVER_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, dispute)
}

// ResolveDisputeRequest payload para dictaminar una resolución sobre la disputa
type ResolveDisputeRequest struct {
	Resolution domain.DisputeStatus `json:"resolution" binding:"required"`
	AdminNotes string               `json:"admin_notes" binding:"required"`
}

// ResolveDispute procesa la decisión administrativa y acciona contablemente sobre el Escrow
// POST /v1/disputes/:id/resolve ó POST /api/v1/disputes/:id/resolve
func (h *DisputeHandler) ResolveDispute(c *gin.Context) {
	adminID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	disputeID := strings.TrimSpace(c.Param("id"))
	if disputeID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de disputa no especificado",
			"code":  "MISSING_DISPUTE_ID",
		})
		return
	}

	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "cuerpo de solicitud inválido: " + err.Error(),
			"code":  "INVALID_REQUEST_BODY",
		})
		return
	}

	input := ports.ResolveDisputeInput{
		DisputeID:  disputeID,
		AdminID:    adminID,
		Resolution: req.Resolution,
		AdminNotes: req.AdminNotes,
	}

	resolved, err := h.disputeService.ResolveDispute(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDisputeNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Disputa no encontrada",
				"code":  "DISPUTE_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrDisputeAlreadyResolved):
			c.JSON(http.StatusConflict, gin.H{
				"error": "La disputa ya ha sido resuelta o desestimada previamente",
				"code":  "DISPUTE_ALREADY_RESOLVED",
			})
			return

		case errors.Is(err, domain.ErrInvalidDisputeData):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Dictamen de resolución inválido",
				"code":  "INVALID_RESOLUTION",
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

	c.JSON(http.StatusOK, resolved)
}

// ListDisputes lista las disputas con soporte de paginación para soporte y mediación
// GET /v1/disputes ó GET /api/v1/disputes
func (h *DisputeHandler) ListDisputes(c *gin.Context) {
	limit := 20
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

	disputes, err := h.disputeService.ListDisputes(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_SERVER_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, disputes)
}
