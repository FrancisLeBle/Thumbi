package http

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// VehicleService define el contrato mínimo requerido por VehicleHandler (SOLID · ISP).
type VehicleService interface {
	RegisterVehicle(ctx context.Context, input ports.VehicleRegistrationInput) (*ports.VehicleDTO, error)
	GetVehicleStatus(ctx context.Context, userID uuid.UUID) (*ports.VehicleDTO, error)
	ProcessAsyncResolution(ctx context.Context, vehicleID uuid.UUID, approved bool, reason string) error
}

type VehicleHandler struct {
	vehicleService VehicleService
}

func NewVehicleHandler(vehicleService VehicleService) *VehicleHandler {
	return &VehicleHandler{vehicleService: vehicleService}
}

type VehicleRegistrationRequest struct {
	Brand               string `json:"brand" binding:"required"`
	Model               string `json:"model" binding:"required"`
	Year                int    `json:"year" binding:"required"`
	PlateNumber         string `json:"plate_number" binding:"required"`
	Color               string `json:"color" binding:"required"`
	SeatCapacity        int    `json:"seat_capacity" binding:"required,min=1,max=8"`
	DriverLicenseBase64 string `json:"driver_license_base64" binding:"required"`
	VehicleCedulaBase64 string `json:"vehicle_cedula_base64" binding:"required"`
	InsuranceBase64     string `json:"insurance_base64"`
}

type AsyncResolutionRequest struct {
	Approved bool   `json:"approved"`
	Reason   string `json:"reason"`
}

// VehicleResponse DTO HTTP con la proyección pública de un vehículo registrado
type VehicleResponse struct {
	ID                 uuid.UUID            `json:"id"`
	UserID             uuid.UUID            `json:"user_id"`
	Brand              string               `json:"brand"`
	Model              string               `json:"model"`
	Year               int                  `json:"year"`
	PlateNumber        string               `json:"plate_number"`
	Color              string               `json:"color"`
	SeatCapacity       int                  `json:"seat_capacity"`
	DriverLicenseURL   string               `json:"driver_license_url"`
	VehicleCedulaURL   string               `json:"vehicle_cedula_url"`
	InsurancePolicyURL string               `json:"insurance_policy_url,omitempty"`
	Status             domain.VehicleStatus `json:"status"`
	RejectionReason    string               `json:"rejection_reason,omitempty"`
	VerifiedAt         *string              `json:"verified_at,omitempty"`
	CreatedAt          string               `json:"created_at"`
	UpdatedAt          string               `json:"updated_at"`
}

func toVehicleResponse(v *ports.VehicleDTO) *VehicleResponse {
	if v == nil {
		return nil
	}
	return &VehicleResponse{
		ID:                 v.ID,
		UserID:             v.UserID,
		Brand:              v.Brand,
		Model:              v.Model,
		Year:               v.Year,
		PlateNumber:        v.PlateNumber,
		Color:              v.Color,
		SeatCapacity:       v.SeatCapacity,
		DriverLicenseURL:   v.DriverLicenseURL,
		VehicleCedulaURL:   v.VehicleCedulaURL,
		InsurancePolicyURL: v.InsurancePolicyURL,
		Status:             v.Status,
		RejectionReason:    v.RejectionReason,
		VerifiedAt:         v.VerifiedAt,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	}
}

// Register registra el vehículo en estado PENDING_VERIFICATION (RF-10, RF-11)
// POST /v1/vehicles/register
func (h *VehicleHandler) Register(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req VehicleRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "datos del vehículo o documentación incompletos", "INVALID_VEHICLE_PAYLOAD")
		return
	}

	vehicle, err := h.vehicleService.RegisterVehicle(c.Request.Context(), ports.VehicleRegistrationInput{
		UserID:              userID,
		Brand:               req.Brand,
		Model:               req.Model,
		Year:                req.Year,
		PlateNumber:         req.PlateNumber,
		Color:               req.Color,
		SeatCapacity:        req.SeatCapacity,
		DriverLicenseBase64: req.DriverLicenseBase64,
		VehicleCedulaBase64: req.VehicleCedulaBase64,
		InsuranceBase64:     req.InsuranceBase64,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	// RF-11: Se registra en estado PENDING_VERIFICATION para proceso asíncrono
	c.JSON(http.StatusCreated, gin.H{
		"message": "Solicitud de alta de vehículo registrada con éxito. La documentación ha ingresado en trámite de verificación asíncrono.",
		"status":  vehicle.Status, // PENDING_VERIFICATION
		"vehicle": gin.H{
			"id":            vehicle.ID,
			"brand":         vehicle.Brand,
			"model":         vehicle.Model,
			"year":          vehicle.Year,
			"plate_number":  vehicle.PlateNumber,
			"seat_capacity": vehicle.SeatCapacity,
			"status":        vehicle.Status,
		},
	})
}

// Status consulta el estado del vehículo del usuario (RF-13)
// GET /v1/vehicles/status
func (h *VehicleHandler) Status(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	vehicle, err := h.vehicleService.GetVehicleStatus(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, domain.ErrVehicleNotFound) {
			c.JSON(http.StatusOK, gin.H{
				"has_vehicle": false,
				"status":      "NONE",
			})
			return
		}
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_vehicle": true,
		"vehicle":     toVehicleResponse(vehicle),
	})
}

// ProcessResolution simula o recibe la resolución asíncrona del trámite (RF-12, RF-13)
// POST /v1/vehicles/:id/resolution (protegido por WebhookAuthMiddleware)
func (h *VehicleHandler) ProcessResolution(c *gin.Context) {
	rawVehicleID := strings.TrimSpace(c.Param("id"))
	if rawVehicleID == "" {
		RespondBadRequest(c, "id de vehículo requerido", "MISSING_VEHICLE_ID")
		return
	}
	vehicleID, err := domain.ParseUUID(rawVehicleID)
	if err != nil {
		RespondBadRequest(c, "id de vehículo con formato UUID inválido", "INVALID_VEHICLE_ID")
		return
	}

	var req AsyncResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "cuerpo de resolución inválido", "INVALID_RESOLUTION_PAYLOAD")
		return
	}

	reason := strings.TrimSpace(req.Reason)
	if !req.Approved && reason == "" {
		reason = "Documentación no aprobada en trámite de validación vehicular"
	}

	if err := h.vehicleService.ProcessAsyncResolution(c.Request.Context(), vehicleID, req.Approved, reason); err != nil {
		respondError(c, err)
		return
	}

	resolutionStatus := domain.VehicleStatusApproved
	if !req.Approved {
		resolutionStatus = domain.VehicleStatusRejected
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Resolución procesada y usuario notificado.",
		"status":  resolutionStatus,
	})
}
