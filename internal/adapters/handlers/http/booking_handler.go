package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// BookingService define el contrato mínimo del ciclo de vida de reservas requerido por BookingHandler (SOLID · ISP).
type BookingService interface {
	CreateBooking(ctx context.Context, input ports.CreateBookingInput) (*ports.BookingDTO, error)
	ConfirmBookingPayment(ctx context.Context, input ports.ConfirmBookingPaymentInput) (*ports.BookingDTO, error)
	CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*ports.RefundDTO, error)
	GetBookingByID(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.BookingDTO, error)
	ListPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*ports.BookingDTO, error)
}

// ContactLinkService define el contrato mínimo para generar enlaces de contacto WhatsApp (SOLID · ISP).
type ContactLinkService interface {
	GetContactLink(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.ContactLinkDTO, error)
}

type BookingHandler struct {
	bookingService     BookingService
	contactLinkService ContactLinkService
}

// NewBookingHandler crea una nueva instancia del controlador HTTP para gestión de reservas y enlaces de contacto
func NewBookingHandler(bookingService BookingService, contactLinkService ...ContactLinkService) *BookingHandler {
	var clSvc ContactLinkService
	if len(contactLinkService) > 0 {
		clSvc = contactLinkService[0]
	}
	return &BookingHandler{
		bookingService:     bookingService,
		contactLinkService: clSvc,
	}
}

// CreateBookingRequest payload para solicitar la reserva de asientos (passenger_id se deriva del token)
type CreateBookingRequest struct {
	TripID         string  `json:"trip_id" binding:"required"`
	SeatsRequested int     `json:"seats_requested" binding:"required,min=1"`
	PickupStopID   *string `json:"pickup_stop_id"`
	DropoffStopID  *string `json:"dropoff_stop_id"`
}

// ConfirmPaymentRequest payload para confirmar la captura de pago externa
type ConfirmPaymentRequest struct {
	PaymentGatewayRef string `json:"payment_gateway_ref" binding:"required"`
}

// PaymentWebhookRequest payload para la notificación asíncrona de eventos de pago de la pasarela
type PaymentWebhookRequest struct {
	Event             string       `json:"event"`
	BookingID         string       `json:"booking_id" binding:"required"`
	PaymentGatewayRef string       `json:"payment_gateway_ref" binding:"required"`
	Amount            domain.Money `json:"amount,omitempty"`
	Currency          string       `json:"currency,omitempty"`
}

// CancelBookingRequest payload opcional con el motivo de cancelación
type CancelBookingRequest struct {
	Reason string `json:"reason"`
}

// PaymentIntentResponse DTO HTTP para intención de cobro
type PaymentIntentResponse struct {
	GatewayRef   string          `json:"gateway_ref"`
	ClientSecret string          `json:"client_secret,omitempty"`
	Amount       domain.Money    `json:"amount"`
	Currency     domain.Currency `json:"currency"`
	Status       string          `json:"status"`
}

// BookingTripSummaryResponse DTO HTTP con resumen contextual del viaje
type BookingTripSummaryResponse struct {
	TripID           uuid.UUID `json:"trip_id"`
	OriginTitle      string    `json:"origin_title"`
	DestinationTitle string    `json:"destination_title"`
	DepartureTime    string    `json:"departure_time"`
	DriverID         uuid.UUID `json:"driver_id"`
	DriverFullName   string    `json:"driver_full_name"`
	VehicleBrand     string    `json:"vehicle_brand"`
	VehicleModel     string    `json:"vehicle_model"`
	VehiclePlate     string    `json:"vehicle_plate"`
}

// BookingResponse DTO HTTP enriquecido de una reserva
type BookingResponse struct {
	ID                 uuid.UUID                   `json:"id"`
	TripID             uuid.UUID                   `json:"trip_id"`
	PassengerID        uuid.UUID                   `json:"passenger_id"`
	SeatsBooked        int                         `json:"seats_booked"`
	Currency           domain.Currency             `json:"currency"`
	UnitPrice          domain.Money                `json:"unit_price"`
	TotalPrice         domain.Money                `json:"total_price"`
	Status             domain.BookingStatus        `json:"status"`
	PickupStopID       *uuid.UUID                  `json:"pickup_stop_id,omitempty"`
	DropoffStopID      *uuid.UUID                  `json:"dropoff_stop_id,omitempty"`
	PaymentIntentID    *string                     `json:"payment_intent_id,omitempty"`
	ExpiresAt          string                      `json:"expires_at"`
	ConfirmedAt        *string                     `json:"confirmed_at,omitempty"`
	CancelledAt        *string                     `json:"cancelled_at,omitempty"`
	CancellationReason *string                     `json:"cancellation_reason,omitempty"`
	CreatedAt          string                      `json:"created_at"`
	PaymentIntent      *PaymentIntentResponse      `json:"payment_intent,omitempty"`
	TripSummary        *BookingTripSummaryResponse `json:"trip_summary,omitempty"`
}

type BookingEnvelopeResponse struct {
	Booking *BookingResponse `json:"booking"`
}

type MyBookingsResponse struct {
	Bookings []*BookingResponse `json:"bookings"`
	Count    int                `json:"count"`
	Limit    int                `json:"limit"`
	Offset   int                `json:"offset"`
}

type PaymentWebhookResponse struct {
	Message string           `json:"message"`
	Booking *BookingResponse `json:"booking,omitempty"`
	Status  string           `json:"status,omitempty"`
}

type RefundResponse struct {
	ID                       uuid.UUID         `json:"id"`
	EscrowTransactionID      uuid.UUID         `json:"escrow_transaction_id"`
	BookingID                uuid.UUID         `json:"booking_id"`
	PassengerRefundAmount    domain.Money      `json:"passenger_refund_amount"`
	DriverCompensationAmount domain.Money      `json:"driver_compensation_amount"`
	RefundType               domain.RefundType `json:"refund_type"`
	GatewayRefundRef         string            `json:"gateway_refund_ref,omitempty"`
	ProcessedAt              string            `json:"processed_at"`
	CreatedAt                string            `json:"created_at"`
}

type CancelBookingResponse struct {
	Message string          `json:"message"`
	Refund  *RefundResponse `json:"refund"`
}

type ContactLinkResponse struct {
	WhatsAppURL string `json:"whatsapp_url"`
	DriverPhone string `json:"driver_phone"`
}

func toBookingResponse(dto *ports.BookingDTO) *BookingResponse {
	if dto == nil {
		return nil
	}

	var paymentIntentResp *PaymentIntentResponse
	if dto.PaymentIntent != nil {
		paymentIntentResp = &PaymentIntentResponse{
			GatewayRef:   dto.PaymentIntent.GatewayRef,
			ClientSecret: dto.PaymentIntent.ClientSecret,
			Amount:       dto.PaymentIntent.Amount,
			Currency:     dto.PaymentIntent.Currency,
			Status:       dto.PaymentIntent.Status,
		}
	}

	var tripSummaryResp *BookingTripSummaryResponse
	if dto.TripSummary != nil {
		tripSummaryResp = &BookingTripSummaryResponse{
			TripID:           dto.TripSummary.TripID,
			OriginTitle:      dto.TripSummary.OriginTitle,
			DestinationTitle: dto.TripSummary.DestinationTitle,
			DepartureTime:    dto.TripSummary.DepartureTime,
			DriverID:         dto.TripSummary.DriverID,
			DriverFullName:   dto.TripSummary.DriverFullName,
			VehicleBrand:     dto.TripSummary.VehicleBrand,
			VehicleModel:     dto.TripSummary.VehicleModel,
			VehiclePlate:     dto.TripSummary.VehiclePlate,
		}
	}

	return &BookingResponse{
		ID:                 dto.ID,
		TripID:             dto.TripID,
		PassengerID:        dto.PassengerID,
		SeatsBooked:        dto.SeatsBooked,
		Currency:           dto.Currency,
		UnitPrice:          dto.UnitPrice,
		TotalPrice:         dto.TotalPrice,
		Status:             dto.Status,
		PickupStopID:       dto.PickupStopID,
		DropoffStopID:      dto.DropoffStopID,
		PaymentIntentID:    dto.PaymentIntentID,
		ExpiresAt:          dto.ExpiresAt,
		ConfirmedAt:        dto.ConfirmedAt,
		CancelledAt:        dto.CancelledAt,
		CancellationReason: dto.CancellationReason,
		CreatedAt:          dto.CreatedAt,
		PaymentIntent:      paymentIntentResp,
		TripSummary:        tripSummaryResp,
	}
}

func toBookingResponseList(dtos []*ports.BookingDTO) []*BookingResponse {
	out := make([]*BookingResponse, 0, len(dtos))
	for _, dto := range dtos {
		out = append(out, toBookingResponse(dto))
	}
	return out
}

func toContactLinkResponse(dto *ports.ContactLinkDTO) *ContactLinkResponse {
	if dto == nil {
		return nil
	}
	return &ContactLinkResponse{
		WhatsAppURL: dto.WhatsAppURL,
		DriverPhone: dto.DriverPhone,
	}
}

func toRefundResponse(dto *ports.RefundDTO) *RefundResponse {
	if dto == nil {
		return nil
	}
	return &RefundResponse{
		ID:                       dto.ID,
		EscrowTransactionID:      dto.EscrowTransactionID,
		BookingID:                dto.BookingID,
		PassengerRefundAmount:    dto.PassengerRefundAmount,
		DriverCompensationAmount: dto.DriverCompensationAmount,
		RefundType:               dto.RefundType,
		GatewayRefundRef:         dto.GatewayRefundRef,
		ProcessedAt:              dto.ProcessedAt,
		CreatedAt:                dto.CreatedAt,
	}
}

// CreateBooking procesa la solicitud de reserva con retención atómica de cupos y orden en pasarela
// POST /api/v1/bookings ó POST /v1/bookings
func (h *BookingHandler) CreateBooking(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "parámetros de reserva inválidos", "BAD_REQUEST")
		return
	}

	tripID, err := domain.ParseUUID(req.TripID)
	if err != nil {
		RespondBadRequest(c, "id de viaje con formato UUID inválido", "INVALID_TRIP_ID")
		return
	}

	pickupStopID, err := domain.ParseOptionalUUID(req.PickupStopID)
	if err != nil {
		RespondBadRequest(c, "id de parada de origen con formato UUID inválido", "INVALID_STOP_ID")
		return
	}

	dropoffStopID, err := domain.ParseOptionalUUID(req.DropoffStopID)
	if err != nil {
		RespondBadRequest(c, "id de parada de destino con formato UUID inválido", "INVALID_STOP_ID")
		return
	}

	dto, err := h.bookingService.CreateBooking(c.Request.Context(), ports.CreateBookingInput{
		TripID:         tripID,
		PassengerID:    passengerID,
		SeatsRequested: req.SeatsRequested,
		PickupStopID:   pickupStopID,
		DropoffStopID:  dropoffStopID,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, BookingEnvelopeResponse{
		Booking: toBookingResponse(dto),
	})
}

// ConfirmPayment acredita la captura del pago y custodia los fondos en Escrow
// POST /api/v1/bookings/:id/confirm-payment ó POST /v1/bookings/:id/confirm-payment
func (h *BookingHandler) ConfirmPayment(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	bookingID, err := domain.ParseUUID(c.Param("id"))
	if err != nil {
		RespondBadRequest(c, "id de reserva requerido o con formato UUID inválido", "BAD_REQUEST")
		return
	}

	var req ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "parámetros de confirmación inválidos", "BAD_REQUEST")
		return
	}

	dto, err := h.bookingService.ConfirmBookingPayment(c.Request.Context(), ports.ConfirmBookingPaymentInput{
		BookingID:         bookingID,
		RequestingUserID:  userID,
		PaymentGatewayRef: req.PaymentGatewayRef,
		IsWebhook:         false,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, BookingEnvelopeResponse{
		Booking: toBookingResponse(dto),
	})
}

// ProcessPaymentWebhook procesa la confirmación autoritativa de pago vía webhook firmado de la pasarela
// POST /api/v1/payments/webhook ó POST /v1/payments/webhook
func (h *BookingHandler) ProcessPaymentWebhook(c *gin.Context) {
	var req PaymentWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "payload de webhook de pagos inválido", "BAD_REQUEST")
		return
	}

	bookingID, err := domain.ParseUUID(req.BookingID)
	if err != nil {
		RespondBadRequest(c, "id de reserva con formato UUID inválido", "BAD_REQUEST")
		return
	}

	if strings.TrimSpace(req.Currency) != "" {
		parsedCur, errCur := domain.ParseCurrency(req.Currency)
		if errCur != nil {
			respondError(c, errCur)
			return
		}
		if req.Amount.Currency != "" && req.Amount.Currency != parsedCur {
			respondError(c, domain.ErrCurrencyMismatch)
			return
		}
	}

	dto, err := h.bookingService.ConfirmBookingPayment(c.Request.Context(), ports.ConfirmBookingPaymentInput{
		BookingID:         bookingID,
		PaymentGatewayRef: req.PaymentGatewayRef,
		IsWebhook:         true,
	})
	if err != nil {
		// Idempotencia de webhook: si ya está confirmada o procesada, responder 200 OK informativo
		if errors.Is(err, domain.ErrInvalidBookingStatus) {
			c.JSON(http.StatusOK, PaymentWebhookResponse{
				Message: "notificación de pago recibida pero la reserva ya fue procesada",
				Status:  "already_processed",
			})
			return
		}
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, PaymentWebhookResponse{
		Message: "pago confirmado exitosamente vía webhook y fondos custodiados en Escrow",
		Booking: toBookingResponse(dto),
	})
}

// CancelBooking cancela una reserva aplicando las políticas de reembolso correspondientes
// POST /api/v1/bookings/:id/cancel ó POST /v1/bookings/:id/cancel
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	requestingUserID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	bookingID, err := domain.ParseUUID(c.Param("id"))
	if err != nil {
		RespondBadRequest(c, "id de reserva requerido o con formato UUID inválido", "BAD_REQUEST")
		return
	}

	var req CancelBookingRequest
	_ = c.ShouldBindJSON(&req)

	refund, err := h.bookingService.CancelBooking(c.Request.Context(), ports.CancelBookingInput{
		BookingID:        bookingID,
		RequestingUserID: requestingUserID,
		Reason:           req.Reason,
	})
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, CancelBookingResponse{
		Message: "reserva cancelada exitosamente",
		Refund:  toRefundResponse(refund),
	})
}

// GetBookingByID retorna el detalle completo de una reserva para el pasajero o conductor
// GET /api/v1/bookings/:id ó GET /v1/bookings/:id
func (h *BookingHandler) GetBookingByID(c *gin.Context) {
	requestingUserID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	bookingID, err := domain.ParseUUID(c.Param("id"))
	if err != nil {
		RespondBadRequest(c, "id de reserva requerido o con formato UUID inválido", "BAD_REQUEST")
		return
	}

	dto, err := h.bookingService.GetBookingByID(c.Request.Context(), bookingID, requestingUserID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, BookingEnvelopeResponse{
		Booking: toBookingResponse(dto),
	})
}

// MyBookings lista el historial paginado de reservas realizadas por el pasajero autenticado
// GET /api/v1/bookings/my-bookings ó GET /v1/bookings/my-bookings
func (h *BookingHandler) MyBookings(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	limit := 20
	if limitQuery := c.Query("limit"); limitQuery != "" {
		if parsed, err := strconv.Atoi(limitQuery); err == nil && parsed > 0 {
			if parsed > 50 {
				limit = 50
			} else {
				limit = parsed
			}
		}
	}

	offset := 0
	if offsetQuery := c.Query("offset"); offsetQuery != "" {
		if parsed, err := strconv.Atoi(offsetQuery); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	bookings, err := h.bookingService.ListPassengerBookings(c.Request.Context(), passengerID, limit, offset)
	if err != nil {
		respondError(c, err)
		return
	}

	mappedBookings := toBookingResponseList(bookings)
	c.JSON(http.StatusOK, MyBookingsResponse{
		Bookings: mappedBookings,
		Count:    len(mappedBookings),
		Limit:    limit,
		Offset:   offset,
	})
}

// GetContactLink genera y devuelve el Deeplink de WhatsApp para que el pasajero contacte al conductor
// GET /api/v1/bookings/:id/contact-link ó GET /v1/bookings/:id/contact-link
func (h *BookingHandler) GetContactLink(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	bookingID, err := domain.ParseUUID(c.Param("id"))
	if err != nil {
		RespondBadRequest(c, "id de reserva requerido o con formato UUID inválido", "BAD_REQUEST")
		return
	}

	if h.contactLinkService == nil {
		RespondInternalError(c, "servicio de enlaces de contacto no disponible")
		return
	}

	dto, err := h.contactLinkService.GetContactLink(c.Request.Context(), bookingID, passengerID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toContactLinkResponse(dto))
}
