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

// DisputeService define el contrato mínimo requerido por DisputeHandler (SOLID · ISP).
type DisputeService interface {
	OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error)
	ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error)
	GetDisputeByID(ctx context.Context, disputeID uuid.UUID) (*ports.DisputeDTO, error)
	ListDisputes(ctx context.Context, limit, offset int) ([]*ports.DisputeDTO, error)
}

// DisputeHandler gestiona los endpoints relacionados con la apertura y arbitraje de disputas
type DisputeHandler struct {
	disputeService DisputeService
}

// NewDisputeHandler crea una nueva instancia de DisputeHandler
func NewDisputeHandler(disputeService DisputeService) *DisputeHandler {
	return &DisputeHandler{disputeService: disputeService}
}

// OpenDisputeRequest payload para registrar un nuevo reclamo formal (reporter_id se extrae del token)
type OpenDisputeRequest struct {
	BookingID    string               `json:"booking_id" binding:"required"`
	Reason       domain.DisputeReason `json:"reason" binding:"required"`
	Description  string               `json:"description" binding:"required"`
	EvidenceURLs []string             `json:"evidence_urls"`
}

// ResolveDisputeRequest payload para dictaminar una resolución sobre la disputa (admin_id se extrae del token)
type ResolveDisputeRequest struct {
	Resolution domain.DisputeStatus `json:"resolution" binding:"required"`
	AdminNotes string               `json:"admin_notes" binding:"required"`
}

// DisputeResponse DTO HTTP de una disputa para clientes y mesa de control
type DisputeResponse struct {
	ID                  uuid.UUID            `json:"id"`
	EscrowTransactionID uuid.UUID            `json:"escrow_transaction_id"`
	BookingID           uuid.UUID            `json:"booking_id"`
	TripID              uuid.UUID            `json:"trip_id"`
	ReporterID          uuid.UUID            `json:"reporter_id"`
	DefendantID         uuid.UUID            `json:"defendant_id"`
	Reason              domain.DisputeReason `json:"reason"`
	Description         string               `json:"description"`
	EvidenceURLs        []string             `json:"evidence_urls"`
	Status              domain.DisputeStatus `json:"status"`
	AdminNotes          *string              `json:"admin_notes,omitempty"`
	ResolvedBy          *uuid.UUID           `json:"resolved_by,omitempty"`
	ResolvedAt          *string              `json:"resolved_at,omitempty"`
	CreatedAt           string               `json:"created_at"`
	UpdatedAt           string               `json:"updated_at"`
}

func toDisputeResponse(dto *ports.DisputeDTO) *DisputeResponse {
	if dto == nil {
		return nil
	}
	return &DisputeResponse{
		ID:                  dto.ID,
		EscrowTransactionID: dto.EscrowTransactionID,
		BookingID:           dto.BookingID,
		TripID:              dto.TripID,
		ReporterID:          dto.ReporterID,
		DefendantID:         dto.DefendantID,
		Reason:              dto.Reason,
		Description:         dto.Description,
		EvidenceURLs:        dto.EvidenceURLs,
		Status:              dto.Status,
		AdminNotes:          dto.AdminNotes,
		ResolvedBy:          dto.ResolvedBy,
		ResolvedAt:          dto.ResolvedAt,
		CreatedAt:           dto.CreatedAt,
		UpdatedAt:           dto.UpdatedAt,
	}
}

func toDisputeResponseList(dtos []*ports.DisputeDTO) []*DisputeResponse {
	out := make([]*DisputeResponse, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, toDisputeResponse(d))
	}
	return out
}

// OpenDispute registra una reclamación formal y congela la transacción de custodia en DISPUTED
// POST /v1/disputes ó POST /api/v1/disputes
func (h *DisputeHandler) OpenDispute(c *gin.Context) {
	reporterID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req OpenDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "cuerpo de solicitud incompleto o inválido", "INVALID_REQUEST_BODY")
		return
	}

	bookingID, err := domain.ParseUUID(req.BookingID)
	if err != nil {
		RespondBadRequest(c, "ID de reserva con formato UUID inválido", "INVALID_BOOKING_ID")
		return
	}

	if !req.Reason.IsValid() {
		RespondBadRequest(c, "motivo de disputa inválido", "INVALID_DISPUTE_DATA")
		return
	}

	input := ports.OpenDisputeInput{
		BookingID:    bookingID,
		ReporterID:   reporterID,
		Reason:       req.Reason,
		Description:  req.Description,
		EvidenceURLs: req.EvidenceURLs,
	}

	dispute, err := h.disputeService.OpenDispute(c.Request.Context(), input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, toDisputeResponse(dispute))
}

// GetDisputeByID consulta el detalle de una disputa por su ID previa validación de propiedad o rol admin
// GET /v1/disputes/:id ó GET /api/v1/disputes/:id
func (h *DisputeHandler) GetDisputeByID(c *gin.Context) {
	currentUserID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	rawDisputeID := strings.TrimSpace(c.Param("id"))
	if rawDisputeID == "" {
		RespondBadRequest(c, "ID de disputa no especificado", "MISSING_DISPUTE_ID")
		return
	}
	disputeID, err := domain.ParseUUID(rawDisputeID)
	if err != nil {
		RespondBadRequest(c, "ID de disputa con formato UUID inválido", "INVALID_DISPUTE_ID")
		return
	}

	dispute, err := h.disputeService.GetDisputeByID(c.Request.Context(), disputeID)
	if err != nil {
		respondError(c, err)
		return
	}

	// Control de acceso y validación de propiedad (RF: Seguridad y Privacidad):
	// Solo el denunciante (ReporterID), el denunciado (DefendantID) o un administrador (ADMIN)
	// pueden acceder a la información de la disputa.
	userRole, _ := middleware.GetUserRole(c)
	isAdmin := userRole == domain.RoleAdmin || userRole == domain.UserRoleAdmin
	isParticipant := (dispute.ReporterID != uuid.Nil && currentUserID == dispute.ReporterID) ||
		(dispute.DefendantID != uuid.Nil && currentUserID == dispute.DefendantID)

	if !isAdmin && !isParticipant {
		RespondForbidden(c, "acceso denegado: no posees autorización para ver los detalles de esta disputa", "FORBIDDEN")
		return
	}

	c.JSON(http.StatusOK, toDisputeResponse(dispute))
}

// ResolveDispute procesa la decisión administrativa y acciona contablemente sobre el Escrow
// POST /v1/disputes/:id/resolve ó POST /api/v1/disputes/:id/resolve
func (h *DisputeHandler) ResolveDispute(c *gin.Context) {
	adminID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	// Control de acceso RBAC: Solo administradores pueden resolver disputas y dictaminar fondos
	userRole, hasRole := middleware.GetUserRole(c)
	if !hasRole || (userRole != domain.RoleAdmin && userRole != domain.UserRoleAdmin) {
		RespondForbidden(c, "acceso denegado: solo los administradores pueden dictaminar y resolver disputas", "FORBIDDEN")
		return
	}

	rawDisputeID := strings.TrimSpace(c.Param("id"))
	if rawDisputeID == "" {
		RespondBadRequest(c, "ID de disputa no especificado", "MISSING_DISPUTE_ID")
		return
	}
	disputeID, err := domain.ParseUUID(rawDisputeID)
	if err != nil {
		RespondBadRequest(c, "ID de disputa con formato UUID inválido", "INVALID_DISPUTE_ID")
		return
	}

	var req ResolveDisputeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "cuerpo de solicitud inválido", "INVALID_REQUEST_BODY")
		return
	}

	if !req.Resolution.IsValidResolution() {
		RespondBadRequest(c, "Dictamen de resolución inválido; valores permitidos: RESOLVED_PASSENGER_REFUND, RESOLVED_DRIVER_PAYOUT, REJECTED", "INVALID_RESOLUTION")
		return
	}

	if strings.TrimSpace(req.AdminNotes) == "" {
		RespondBadRequest(c, "Las notas administrativas de resolución son obligatorias", "INVALID_DISPUTE_DATA")
		return
	}

	input := ports.ResolveDisputeInput{
		DisputeID:  disputeID,
		AdminID:    adminID,
		Resolution: req.Resolution,
		AdminNotes: strings.TrimSpace(req.AdminNotes),
	}

	resolved, err := h.disputeService.ResolveDispute(c.Request.Context(), input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toDisputeResponse(resolved))
}

// ListDisputes lista las disputas con soporte de paginación (limit/offset o page) para soporte y mediación
// GET /v1/disputes ó GET /api/v1/disputes
func (h *DisputeHandler) ListDisputes(c *gin.Context) {
	// Solo administradores pueden listar el repositorio global de disputas
	userRole, hasRole := middleware.GetUserRole(c)
	if !hasRole || (userRole != domain.RoleAdmin && userRole != domain.UserRoleAdmin) {
		RespondForbidden(c, "acceso denegado: permisos insuficientes para listar disputas", "FORBIDDEN")
		return
	}

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
	} else if p := c.Query("page"); p != "" {
		if page, err := strconv.Atoi(p); err == nil && page >= 1 {
			offset = (page - 1) * limit
		}
	}

	disputes, err := h.disputeService.ListDisputes(c.Request.Context(), limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toDisputeResponseList(disputes))
}
