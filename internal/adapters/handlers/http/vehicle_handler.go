package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type VehicleHandler struct {
	vehicleService ports.VehicleService
}

func NewVehicleHandler(vehicleService ports.VehicleService) *VehicleHandler {
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

// Register registra el vehículo en estado PENDING_VERIFICATION (RF-10, RF-11)
// POST /v1/vehicles/register
func (h *VehicleHandler) Register(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
		return
	}

	var req VehicleRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "datos del vehículo o documentación incompletos",
			"details": err.Error(),
		})
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
		switch {
		case errors.Is(err, domain.ErrDriverKYCRequired):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Es obligatorio completar y aprobar la verificación de identidad biométrica (DNI y selfie 3D) antes de dar de alta un vehículo.",
				"code":  "DRIVER_KYC_REQUIRED",
			})
			return

		case errors.Is(err, domain.ErrVehicleAlreadyVerified):
			c.JSON(http.StatusConflict, gin.H{
				"error": "El dominio/patente especificado ya se encuentra registrado y verificado en la plataforma.",
				"code":  "VEHICLE_ALREADY_VERIFIED",
			})
			return

		case errors.Is(err, domain.ErrInvalidVehicleData):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Los datos del vehículo ingresados son inválidos.",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "usuario no autenticado"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_vehicle": true,
		"vehicle":     vehicle,
	})
}

// ProcessResolution simula o recibe la resolución asíncrona del trámite (RF-12, RF-13)
// POST /v1/vehicles/:id/resolution
func (h *VehicleHandler) ProcessResolution(c *gin.Context) {
	vehicleID := c.Param("id")
	if vehicleID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id de vehículo requerido"})
		return
	}

	var req AsyncResolutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cuerpo de resolución inválido"})
		return
	}

	err := h.vehicleService.ProcessAsyncResolution(c.Request.Context(), vehicleID, req.Approved, req.Reason)
	if err != nil {
		if errors.Is(err, domain.ErrVehicleNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "vehículo no encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
