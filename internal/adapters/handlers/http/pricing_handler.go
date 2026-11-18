package http

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// PricingService define el contrato mínimo para lectura, parametrización administrativa y cotización de tarifas (SOLID · ISP).
type PricingService interface {
	GetPricingConfig(ctx context.Context) (*ports.PlatformPricingConfigDTO, error)
	UpdatePricingConfig(ctx context.Context, input ports.UpdatePlatformPricingConfigInput) (*ports.PlatformPricingConfigDTO, error)
	QuoteBookingPrice(ctx context.Context, tripID uuid.UUID, seatsRequested int) (*ports.BookingPricingQuoteDTO, error)
}

// PricingHandler expone endpoints públicos de transparencia/cotización de tarifas y endpoints RBAC Admin de parametrización.
type PricingHandler struct {
	pricingService PricingService
}

// NewPricingHandler instancia el controlador HTTP para reglas de monetización y retenciones operativas.
func NewPricingHandler(pricingService PricingService) *PricingHandler {
	return &PricingHandler{pricingService: pricingService}
}

// UpdatePricingConfigRequest payload JSON para parametrizar % Service Fee y política de retención en cancelaciones/reembolsos.
// El administrador puede especificar porcentajes legibles (ej. 12.5) o puntos básicos exactos (ej. 1250).
type UpdatePricingConfigRequest struct {
	Currency                           *string  `json:"currency"`
	PassengerServiceFeeBps             *int64   `json:"passenger_service_fee_bps"`
	PassengerServiceFeePercent         *float64 `json:"passenger_service_fee_percent"`
	PassengerFixedFeeCents             *int64   `json:"passenger_fixed_fee_cents"`
	PassengerFixedFee                  *float64 `json:"passenger_fixed_fee"`
	EarlyCancelFeeRetentionBps         *int64   `json:"early_cancel_fee_retention_bps"`
	EarlyCancelFeeRetentionPercent     *float64 `json:"early_cancel_fee_retention_percent"`
	LateCancelWindowHours              *int     `json:"late_cancel_window_hours"`
	LateCancelFeeRetentionBps          *int64   `json:"late_cancel_fee_retention_bps"`
	LateCancelFeeRetentionPercent      *float64 `json:"late_cancel_fee_retention_percent"`
	LateCancelPassengerRefundBps       *int64   `json:"late_cancel_passenger_refund_bps"`
	LateCancelPassengerRefundPercent   *float64 `json:"late_cancel_passenger_refund_percent"`
	LateCancelDriverCompBps            *int64   `json:"late_cancel_driver_comp_bps"`
	LateCancelDriverCompPercent        *float64 `json:"late_cancel_driver_comp_percent"`
	LateCancelPlatformRetentionBps     *int64   `json:"late_cancel_platform_retention_bps"`
	LateCancelPlatformRetentionPercent *float64 `json:"late_cancel_platform_retention_percent"`
}

// PlatformPricingConfigResponse DTO HTTP con la configuración vigente de comisiones y retenciones.
type PlatformPricingConfigResponse struct {
	ID                                 uuid.UUID       `json:"id"`
	Currency                           domain.Currency `json:"currency"`
	PassengerServiceFeeBps             int64           `json:"passenger_service_fee_bps"`
	PassengerServiceFeePercent         float64         `json:"passenger_service_fee_percent"`
	PassengerFixedFee                  domain.Money    `json:"passenger_fixed_fee"`
	EarlyCancelFeeRetentionBps         int64           `json:"early_cancel_fee_retention_bps"`
	EarlyCancelFeeRetentionPercent     float64         `json:"early_cancel_fee_retention_percent"`
	LateCancelWindowHours              int             `json:"late_cancel_window_hours"`
	LateCancelFeeRetentionBps          int64           `json:"late_cancel_fee_retention_bps"`
	LateCancelFeeRetentionPercent      float64         `json:"late_cancel_fee_retention_percent"`
	LateCancelPassengerRefundBps       int64           `json:"late_cancel_passenger_refund_bps"`
	LateCancelPassengerRefundPercent   float64         `json:"late_cancel_passenger_refund_percent"`
	LateCancelDriverCompBps            int64           `json:"late_cancel_driver_comp_bps"`
	LateCancelDriverCompPercent        float64         `json:"late_cancel_driver_comp_percent"`
	LateCancelPlatformRetentionBps     int64           `json:"late_cancel_platform_retention_bps"`
	LateCancelPlatformRetentionPercent float64         `json:"late_cancel_platform_retention_percent"`
	IsActive                           bool            `json:"is_active"`
	UpdatedBy                          *uuid.UUID      `json:"updated_by,omitempty"`
	UpdatedAt                          string          `json:"updated_at"`
}

// BookingPricingQuoteResponse DTO HTTP con el desglose transparente para checkout previo a la reserva.
type BookingPricingQuoteResponse struct {
	TripID            uuid.UUID       `json:"trip_id"`
	SeatsRequested    int             `json:"seats_requested"`
	Currency          domain.Currency `json:"currency"`
	UnitPrice         domain.Money    `json:"unit_price"`
	BaseAmount        domain.Money    `json:"base_amount"`
	PlatformFee       domain.Money    `json:"platform_fee"`
	TotalPrice        domain.Money    `json:"total_price"`
	DriverPayout      domain.Money    `json:"driver_payout"`
	ServiceFeeBps     int64           `json:"service_fee_bps"`
	ServiceFeePercent float64         `json:"service_fee_percent"`
	FixedFee          domain.Money    `json:"fixed_fee"`
}

func toPlatformPricingConfigResponse(dto *ports.PlatformPricingConfigDTO) *PlatformPricingConfigResponse {
	if dto == nil {
		return nil
	}
	return &PlatformPricingConfigResponse{
		ID:                                 dto.ID,
		Currency:                           dto.Currency,
		PassengerServiceFeeBps:             dto.PassengerServiceFeeBps,
		PassengerServiceFeePercent:         dto.PassengerServiceFeePercent,
		PassengerFixedFee:                  dto.PassengerFixedFee,
		EarlyCancelFeeRetentionBps:         dto.EarlyCancelFeeRetentionBps,
		EarlyCancelFeeRetentionPercent:     dto.EarlyCancelFeeRetentionPercent,
		LateCancelWindowHours:              dto.LateCancelWindowHours,
		LateCancelFeeRetentionBps:          dto.LateCancelFeeRetentionBps,
		LateCancelFeeRetentionPercent:      dto.LateCancelFeeRetentionPercent,
		LateCancelPassengerRefundBps:       dto.LateCancelPassengerRefundBps,
		LateCancelPassengerRefundPercent:   dto.LateCancelPassengerRefundPercent,
		LateCancelDriverCompBps:            dto.LateCancelDriverCompBps,
		LateCancelDriverCompPercent:        dto.LateCancelDriverCompPercent,
		LateCancelPlatformRetentionBps:     dto.LateCancelPlatformRetentionBps,
		LateCancelPlatformRetentionPercent: dto.LateCancelPlatformRetentionPercent,
		IsActive:                           dto.IsActive,
		UpdatedBy:                          dto.UpdatedBy,
		UpdatedAt:                          dto.UpdatedAt,
	}
}

func toBookingPricingQuoteResponse(dto *ports.BookingPricingQuoteDTO) *BookingPricingQuoteResponse {
	if dto == nil {
		return nil
	}
	return &BookingPricingQuoteResponse{
		TripID:            dto.TripID,
		SeatsRequested:    dto.SeatsRequested,
		Currency:          dto.Currency,
		UnitPrice:         dto.UnitPrice,
		BaseAmount:        dto.BaseAmount,
		PlatformFee:       dto.PlatformFee,
		TotalPrice:        dto.TotalPrice,
		DriverPayout:      dto.DriverPayout,
		ServiceFeeBps:     dto.ServiceFeeBps,
		ServiceFeePercent: dto.ServiceFeePercent,
		FixedFee:          dto.FixedFee,
	}
}

// GetPricingConfig retorna las reglas activas de comisión al pasajero y retenciones por cancelación.
// GET /v1/pricing/config ó GET /v1/admin/pricing-config
func (h *PricingHandler) GetPricingConfig(c *gin.Context) {
	if h.pricingService == nil {
		RespondInternalError(c, "servicio de tarifas no disponible")
		return
	}

	dto, err := h.pricingService.GetPricingConfig(c.Request.Context())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toPlatformPricingConfigResponse(dto))
}

// QuoteBookingPrice calcula en tiempo real el desglose Base + Service Fee = Total para un viaje y asientos dados.
// GET /v1/pricing/quote?trip_id=<uuid>&seats=<int>
func (h *PricingHandler) QuoteBookingPrice(c *gin.Context) {
	if h.pricingService == nil {
		RespondInternalError(c, "servicio de tarifas no disponible")
		return
	}

	rawTripID := strings.TrimSpace(c.Query("trip_id"))
	if rawTripID == "" {
		RespondBadRequest(c, "el parámetro trip_id es obligatorio", "MISSING_TRIP_ID")
		return
	}
	tripID, err := domain.ParseUUID(rawTripID)
	if err != nil {
		RespondBadRequest(c, "trip_id con formato UUID inválido", "INVALID_TRIP_ID")
		return
	}

	seats := 1
	if rawSeats := strings.TrimSpace(c.Query("seats")); rawSeats != "" {
		parsedSeats, convErr := strconv.Atoi(rawSeats)
		if convErr != nil || parsedSeats <= 0 {
			RespondBadRequest(c, "el parámetro seats debe ser un entero positivo", "INVALID_SEAT_COUNT")
			return
		}
		seats = parsedSeats
	}

	dto, err := h.pricingService.QuoteBookingPrice(c.Request.Context(), tripID, seats)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toBookingPricingQuoteResponse(dto))
}

// UpdatePricingConfig permite al Administrador parametrizar el % Service Fee al pasajero y las retenciones en cancelaciones.
// PUT /v1/admin/pricing-config
func (h *PricingHandler) UpdatePricingConfig(c *gin.Context) {
	adminID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	userRole, hasRole := middleware.GetUserRole(c)
	if !hasRole || (userRole != domain.RoleAdmin && userRole != domain.UserRoleAdmin) {
		RespondForbidden(c, "acceso denegado: solo los administradores pueden parametrizar tarifas y retenciones", "FORBIDDEN")
		return
	}

	if h.pricingService == nil {
		RespondInternalError(c, "servicio de tarifas no disponible")
		return
	}

	var req UpdatePricingConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "cuerpo de solicitud inválido para parametrización de tarifas", "INVALID_REQUEST_BODY")
		return
	}

	var currencyPtr *domain.Currency
	if req.Currency != nil {
		parsedCur, err := domain.ParseCurrency(*req.Currency)
		if err != nil {
			respondError(c, err)
			return
		}
		currencyPtr = &parsedCur
	}

	passengerFeeBps, ok := resolveBpsParam(req.PassengerServiceFeeBps, req.PassengerServiceFeePercent)
	if !ok {
		RespondBadRequest(c, "passenger_service_fee_bps y passenger_service_fee_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	passengerFixedCents, ok := resolveFixedFeeCentsParam(req.PassengerFixedFeeCents, req.PassengerFixedFee)
	if !ok {
		RespondBadRequest(c, "passenger_fixed_fee_cents y passenger_fixed_fee son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	earlyRetBps, ok := resolveBpsParam(req.EarlyCancelFeeRetentionBps, req.EarlyCancelFeeRetentionPercent)
	if !ok {
		RespondBadRequest(c, "early_cancel_fee_retention_bps y early_cancel_fee_retention_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	lateFeeRetBps, ok := resolveBpsParam(req.LateCancelFeeRetentionBps, req.LateCancelFeeRetentionPercent)
	if !ok {
		RespondBadRequest(c, "late_cancel_fee_retention_bps y late_cancel_fee_retention_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	latePassRefundBps, ok := resolveBpsParam(req.LateCancelPassengerRefundBps, req.LateCancelPassengerRefundPercent)
	if !ok {
		RespondBadRequest(c, "late_cancel_passenger_refund_bps y late_cancel_passenger_refund_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	lateDriverCompBps, ok := resolveBpsParam(req.LateCancelDriverCompBps, req.LateCancelDriverCompPercent)
	if !ok {
		RespondBadRequest(c, "late_cancel_driver_comp_bps y late_cancel_driver_comp_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	latePlatRetBps, ok := resolveBpsParam(req.LateCancelPlatformRetentionBps, req.LateCancelPlatformRetentionPercent)
	if !ok {
		RespondBadRequest(c, "late_cancel_platform_retention_bps y late_cancel_platform_retention_percent son inconsistentes o inválidos", "INVALID_PRICING_CONFIG")
		return
	}

	dto, err := h.pricingService.UpdatePricingConfig(c.Request.Context(), ports.UpdatePlatformPricingConfigInput{
		AdminID:                        adminID,
		Currency:                       currencyPtr,
		PassengerServiceFeeBps:         passengerFeeBps,
		PassengerFixedFeeAmount:        passengerFixedCents,
		EarlyCancelFeeRetentionBps:     earlyRetBps,
		LateCancelWindowHours:          req.LateCancelWindowHours,
		LateCancelFeeRetentionBps:      lateFeeRetBps,
		LateCancelPassengerRefundBps:   latePassRefundBps,
		LateCancelDriverCompBps:        lateDriverCompBps,
		LateCancelPlatformRetentionBps: latePlatRetBps,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toPlatformPricingConfigResponse(dto))
}

func resolveBpsParam(bpsPtr *int64, percentPtr *float64) (*int64, bool) {
	if percentPtr != nil && (math.IsNaN(*percentPtr) || math.IsInf(*percentPtr, 0)) {
		return nil, false
	}
	if bpsPtr != nil && percentPtr != nil {
		converted := int64(math.Round(*percentPtr * 100.0))
		if converted != *bpsPtr {
			return nil, false
		}
		return bpsPtr, true
	}
	if bpsPtr != nil {
		return bpsPtr, true
	}
	if percentPtr != nil {
		converted := int64(math.Round(*percentPtr * 100.0))
		return &converted, true
	}
	return nil, true
}

func resolveFixedFeeCentsParam(centsPtr *int64, majorPtr *float64) (*int64, bool) {
	if majorPtr != nil && (math.IsNaN(*majorPtr) || math.IsInf(*majorPtr, 0)) {
		return nil, false
	}
	if centsPtr != nil && majorPtr != nil {
		converted := int64(math.Round(*majorPtr * 100.0))
		if converted != *centsPtr {
			return nil, false
		}
		return centsPtr, true
	}
	if centsPtr != nil {
		return centsPtr, true
	}
	if majorPtr != nil {
		converted := int64(math.Round(*majorPtr * 100.0))
		return &converted, true
	}
	return nil, true
}
