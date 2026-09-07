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

type BookingHandler struct {
	bookingService ports.BookingService
}

// NewBookingHandler crea una nueva instancia del controlador HTTP para gestión de reservas y pagos en custodia
func NewBookingHandler(bookingService ports.BookingService) *BookingHandler {
	return &BookingHandler{bookingService: bookingService}
}

// CreateBookingRequest payload para solicitar la reserva de asientos
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

// CancelBookingRequest payload opcional con el motivo de cancelación
type CancelBookingRequest struct {
	Reason string `json:"reason"`
}

// CreateBooking procesa la solicitud de reserva con retención atómica de cupos y orden en pasarela
// POST /api/v1/bookings ó POST /v1/bookings
func (h *BookingHandler) CreateBooking(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	var req CreateBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "parámetros de reserva inválidos",
			"code":    "BAD_REQUEST",
			"details": err.Error(),
		})
		return
	}

	dto, err := h.bookingService.CreateBooking(c.Request.Context(), ports.CreateBookingInput{
		TripID:         req.TripID,
		PassengerID:    passengerID,
		SeatsRequested: req.SeatsRequested,
		PickupStopID:   req.PickupStopID,
		DropoffStopID:  req.DropoffStopID,
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrCannotBookOwnTrip):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "un conductor no puede reservar plazas en su propio viaje",
				"code":  "CANNOT_BOOK_OWN_TRIP",
			})
		case errors.Is(err, domain.ErrInsufficientSeats):
			c.JSON(http.StatusConflict, gin.H{
				"error": "no hay suficientes asientos disponibles para la cantidad solicitada",
				"code":  "INSUFFICIENT_SEATS",
			})
		case errors.Is(err, domain.ErrOverlappingTripBooking):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "el pasajero ya posee una reserva activa en este viaje",
				"code":  "OVERLAPPING_TRIP_BOOKING",
			})
		case errors.Is(err, domain.ErrInvalidSeatCount):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "la cantidad de asientos solicitados debe ser mayor a cero",
				"code":  "INVALID_SEAT_COUNT",
			})
		case errors.Is(err, domain.ErrTripNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "viaje no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
		case errors.Is(err, domain.ErrInvalidTripStatusChange):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "el viaje no se encuentra disponible para reservas",
				"code":  "TRIP_NOT_AVAILABLE",
			})
		case errors.Is(err, domain.ErrPaymentFailed):
			c.JSON(http.StatusBadGateway, gin.H{
				"error": "fallo al generar la intención de pago en la pasarela externa",
				"code":  "PAYMENT_GATEWAY_ERROR",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error interno al procesar la reserva",
				"code":  "INTERNAL_SERVER_ERROR",
			})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"booking": dto,
	})
}

// ConfirmPayment acredita la captura del pago y custodia los fondos en Escrow
// POST /api/v1/bookings/:id/confirm-payment ó POST /v1/bookings/:id/confirm-payment
func (h *BookingHandler) ConfirmPayment(c *gin.Context) {
	_, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	bookingID := strings.TrimSpace(c.Param("id"))
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "id de reserva requerido",
			"code":  "BAD_REQUEST",
		})
		return
	}

	var req ConfirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "parámetros de confirmación inválidos",
			"code":    "BAD_REQUEST",
			"details": err.Error(),
		})
		return
	}

	dto, err := h.bookingService.ConfirmBookingPayment(c.Request.Context(), ports.ConfirmBookingPaymentInput{
		BookingID:         bookingID,
		PaymentGatewayRef: req.PaymentGatewayRef,
	})

	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "reserva no encontrada",
				"code":  "BOOKING_NOT_FOUND",
			})
		case errors.Is(err, domain.ErrBookingExpired):
			c.JSON(http.StatusGone, gin.H{
				"error": "el tiempo límite de retención (15 min) ha expirado; los asientos fueron liberados",
				"code":  "BOOKING_EXPIRED",
			})
		case errors.Is(err, domain.ErrInvalidBookingStatus):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "la reserva no se encuentra en estado pendiente de pago",
				"code":  "INVALID_BOOKING_STATUS",
			})
		case errors.Is(err, domain.ErrPaymentFailed):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "la pasarela de pagos no confirmó la captura de la transacción",
				"code":  "PAYMENT_CONFIRMATION_FAILED",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error interno al confirmar el pago de la reserva",
				"code":  "INTERNAL_SERVER_ERROR",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"booking": dto,
	})
}

// CancelBooking cancela una reserva aplicando las políticas de reembolso correspondientes
// POST /api/v1/bookings/:id/cancel ó POST /v1/bookings/:id/cancel
func (h *BookingHandler) CancelBooking(c *gin.Context) {
	requestingUserID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	bookingID := strings.TrimSpace(c.Param("id"))
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "id de reserva requerido",
			"code":  "BAD_REQUEST",
		})
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
		switch {
		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "reserva no encontrada",
				"code":  "BOOKING_NOT_FOUND",
			})
		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no tienes autorización para cancelar esta reserva",
				"code":  "FORBIDDEN",
			})
		case errors.Is(err, domain.ErrBookingAlreadyCancelled):
			c.JSON(http.StatusConflict, gin.H{
				"error": "la reserva ya se encuentra cancelada",
				"code":  "ALREADY_CANCELLED",
			})
		case errors.Is(err, domain.ErrInvalidBookingStatus):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "la reserva no puede cancelarse en su estado actual",
				"code":  "INVALID_STATUS",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error interno al cancelar la reserva",
				"code":  "INTERNAL_SERVER_ERROR",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "reserva cancelada exitosamente",
		"refund":  refund,
	})
}

// GetBookingByID retorna el detalle completo de una reserva para el pasajero o conductor
// GET /api/v1/bookings/:id ó GET /v1/bookings/:id
func (h *BookingHandler) GetBookingByID(c *gin.Context) {
	requestingUserID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	bookingID := strings.TrimSpace(c.Param("id"))
	if bookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "id de reserva requerido",
			"code":  "BAD_REQUEST",
		})
		return
	}

	dto, err := h.bookingService.GetBookingByID(c.Request.Context(), bookingID, requestingUserID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrBookingNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "reserva no encontrada",
				"code":  "BOOKING_NOT_FOUND",
			})
		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "no tienes permisos para visualizar esta reserva",
				"code":  "FORBIDDEN",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "error interno al obtener la reserva",
				"code":  "INTERNAL_SERVER_ERROR",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"booking": dto,
	})
}

// MyBookings lista el historial paginado de reservas realizadas por el pasajero autenticado
// GET /api/v1/bookings/my-bookings ó GET /v1/bookings/my-bookings
func (h *BookingHandler) MyBookings(c *gin.Context) {
	passengerID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "error interno al listar reservas",
			"code":  "INTERNAL_SERVER_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bookings": bookings,
		"count":    len(bookings),
		"limit":    limit,
		"offset":   offset,
	})
}
