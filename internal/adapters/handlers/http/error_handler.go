package http

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
)

// StandardErrorResponse estructura estandarizada y segura de error hacia el cliente.
type StandardErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id"`
}

// ErrorSpec define el mapeo de un error de dominio hacia su código HTTP, código semántico y mensaje sanitizado.
type ErrorSpec struct {
	Err        error
	StatusCode int
	Code       string
	Message    string
}

// domainErrorTable tabla centralizada y determinística de errores del dominio hacia respuestas HTTP.
var domainErrorTable = []ErrorSpec{
	// Autenticación, identidad y control de acceso
	{Err: domain.ErrUserNotFound, StatusCode: http.StatusNotFound, Code: "USER_NOT_FOUND", Message: "Usuario no encontrado"},
	{Err: domain.ErrUserAlreadyExists, StatusCode: http.StatusConflict, Code: "USER_ALREADY_EXISTS", Message: "El usuario ya se encuentra registrado"},
	{Err: domain.ErrInvalidCredentials, StatusCode: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "Credenciales de acceso inválidas"},
	{Err: domain.ErrInvalidToken, StatusCode: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Token no válido o mal formado"},
	{Err: domain.ErrTokenExpired, StatusCode: http.StatusUnauthorized, Code: "TOKEN_EXPIRED", Message: "El token de acceso ha expirado"},
	{Err: domain.ErrSessionExpired, StatusCode: http.StatusUnauthorized, Code: "SESSION_EXPIRED", Message: "La sesión ha expirado tras 20 minutos de inactividad o límite de tiempo"},
	{Err: domain.ErrUnauthorized, StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: "No tienes autorización para realizar esta operación"},
	{Err: domain.ErrForbidden, StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: "Acceso denegado"},
	{Err: domain.ErrInvalidInput, StatusCode: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Parámetros de solicitud inválidos"},
	{Err: domain.ErrUnverifiedEmail, StatusCode: http.StatusForbidden, Code: "UNVERIFIED_EMAIL", Message: "El correo electrónico provisto por la cuenta social no se encuentra verificado"},
	{Err: domain.ErrIdentityNotFound, StatusCode: http.StatusNotFound, Code: "IDENTITY_NOT_FOUND", Message: "Identidad social no encontrada"},
	{Err: domain.ErrIdentityAlreadyExists, StatusCode: http.StatusConflict, Code: "IDENTITY_ALREADY_EXISTS", Message: "La identidad social ya se encuentra vinculada a otra cuenta"},

	// KYC y biometría
	{Err: domain.ErrKYCAlreadySubmitted, StatusCode: http.StatusConflict, Code: "KYC_ALREADY_SUBMITTED", Message: "Ya existe un trámite de verificación KYC en proceso"},
	{Err: domain.ErrKYCAlreadyApproved, StatusCode: http.StatusConflict, Code: "KYC_ALREADY_APPROVED", Message: "La identidad del usuario ya ha sido verificada y aprobada previamente"},
	{Err: domain.ErrKYCInManualReview, StatusCode: http.StatusLocked, Code: "PENDING_MANUAL_REVIEW", Message: "Tu trámite se encuentra actualmente en revisión manual. Te notificaremos cuando se complete el análisis"},
	{Err: domain.ErrKYCNotFound, StatusCode: http.StatusNotFound, Code: "KYC_NOT_FOUND", Message: "Verificación de identidad no encontrada"},
	{Err: domain.ErrKYCNotApproved, StatusCode: http.StatusForbidden, Code: "KYC_NOT_APPROVED", Message: "La verificación de identidad aún no ha sido aprobada"},
	{Err: domain.ErrLivenessCheckFailed, StatusCode: http.StatusUnprocessableEntity, Code: "LIVENESS_CHECK_FAILED", Message: "La validación biométrica no fue satisfactoria"},
	{Err: domain.ErrDriverKYCRequired, StatusCode: http.StatusForbidden, Code: "DRIVER_KYC_REQUIRED", Message: "Es obligatorio completar y aprobar la verificación de identidad biométrica (DNI y selfie 3D) antes de continuar"},
	{Err: domain.ErrDriverNotActive, StatusCode: http.StatusForbidden, Code: "DRIVER_NOT_ACTIVE", Message: "El usuario no cuenta con rol de conductor activo o no ha completado la verificación KYC"},
	{Err: domain.ErrDriverPhoneRequired, StatusCode: http.StatusUnprocessableEntity, Code: "DRIVER_PHONE_REQUIRED", Message: "El conductor no posee un número de teléfono válido registrado"},
	{Err: domain.ErrMaxRetriesExceeded, StatusCode: http.StatusUnprocessableEntity, Code: "PENDING_MANUAL_REVIEW", Message: "Se ha superado el máximo de 3 reintentos automatizados. Tu solicitud ha sido derivada a revisión manual por nuestra mesa de control"},

	// Vehículos
	{Err: domain.ErrVehicleNotFound, StatusCode: http.StatusNotFound, Code: "VEHICLE_NOT_FOUND", Message: "Vehículo no encontrado"},
	{Err: domain.ErrVehicleAlreadyExists, StatusCode: http.StatusConflict, Code: "VEHICLE_ALREADY_EXISTS", Message: "El vehículo ya se encuentra registrado"},
	{Err: domain.ErrVehicleNotAvailable, StatusCode: http.StatusBadRequest, Code: "VEHICLE_NOT_APPROVED", Message: "El vehículo no se encuentra aprobado para realizar viajes"},
	{Err: domain.ErrVehicleAlreadyVerified, StatusCode: http.StatusConflict, Code: "VEHICLE_ALREADY_VERIFIED", Message: "El dominio/patente especificado ya se encuentra registrado y verificado en la plataforma"},
	{Err: domain.ErrInvalidVehicleData, StatusCode: http.StatusBadRequest, Code: "INVALID_VEHICLE_DATA", Message: "Los datos del vehículo ingresados son inválidos"},
	{Err: domain.ErrSeatsExceedCapacity, StatusCode: http.StatusBadRequest, Code: "SEATS_EXCEED_CAPACITY", Message: "La cantidad de asientos ofertados supera la capacidad del vehículo"},

	// Viajes y rutas
	{Err: domain.ErrTripNotFound, StatusCode: http.StatusNotFound, Code: "TRIP_NOT_FOUND", Message: "Viaje no encontrado"},
	{Err: domain.ErrTripFull, StatusCode: http.StatusConflict, Code: "TRIP_FULL", Message: "El viaje no cuenta con cupos disponibles"},
	{Err: domain.ErrTripAlreadyStarted, StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_STARTED", Message: "El viaje ya ha iniciado"},
	{Err: domain.ErrTripAlreadyCompleted, StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_COMPLETED", Message: "El viaje ya ha finalizado"},
	{Err: domain.ErrTripAlreadyCancelled, StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_CANCELLED", Message: "El viaje ya se encuentra cancelado"},
	{Err: domain.ErrTripCannotBeCancelled, StatusCode: http.StatusBadRequest, Code: "TRIP_CANNOT_BE_CANCELLED", Message: "El viaje no puede ser cancelado en su estado actual"},
	{Err: domain.ErrCannotCancelStartedTrip, StatusCode: http.StatusBadRequest, Code: "CANNOT_CANCEL_STARTED_TRIP", Message: "No es posible cancelar un viaje que ya se encuentra en curso o completado"},
	{Err: domain.ErrDepartureTimeMustBeFuture, StatusCode: http.StatusBadRequest, Code: "INVALID_DEPARTURE_TIME", Message: "La fecha y hora de salida debe ser estrictamente en el futuro"},
	{Err: domain.ErrInvalidTripStatusChange, StatusCode: http.StatusBadRequest, Code: "INVALID_TRIP_STATUS", Message: "El viaje no se encuentra disponible para reservas o en un estado que permita la operación"},
	{Err: domain.ErrPriceExceedsCapPrice, StatusCode: http.StatusUnprocessableEntity, Code: "PRICE_EXCEEDS_CAP_PRICE", Message: "El precio fijado excede el límite máximo permitido (Cap Price) para el trayecto según la normativa de economía colaborativa no lucrativa"},
	{Err: domain.ErrInvalidRouteCoordinates, StatusCode: http.StatusBadRequest, Code: "INVALID_COORDINATES", Message: "Las coordenadas geográficas proporcionadas son inválidas"},
	{Err: domain.ErrInsufficientRoutePoints, StatusCode: http.StatusBadRequest, Code: "INSUFFICIENT_ROUTE_POINTS", Message: "Se requieren al menos dos puntos para trazar la ruta"},

	// Reservas y pagos
	{Err: domain.ErrBookingNotFound, StatusCode: http.StatusNotFound, Code: "BOOKING_NOT_FOUND", Message: "Reserva no encontrada"},
	{Err: domain.ErrBookingAlreadyExists, StatusCode: http.StatusConflict, Code: "BOOKING_ALREADY_EXISTS", Message: "Ya existe una reserva registrada"},
	{Err: domain.ErrBookingCannotBeCancelled, StatusCode: http.StatusBadRequest, Code: "INVALID_STATUS", Message: "La reserva no puede cancelarse en su estado actual"},
	{Err: domain.ErrBookingAlreadyCancelled, StatusCode: http.StatusConflict, Code: "ALREADY_CANCELLED", Message: "La reserva ya se encuentra cancelada"},
	{Err: domain.ErrCannotBookOwnTrip, StatusCode: http.StatusBadRequest, Code: "CANNOT_BOOK_OWN_TRIP", Message: "Un conductor no puede reservar plazas en su propio viaje"},
	{Err: domain.ErrInvalidSeatCount, StatusCode: http.StatusBadRequest, Code: "INVALID_SEAT_COUNT", Message: "La cantidad de asientos solicitados debe ser mayor a cero"},
	{Err: domain.ErrInsufficientSeats, StatusCode: http.StatusConflict, Code: "INSUFFICIENT_SEATS", Message: "No hay suficientes asientos disponibles para la cantidad solicitada"},
	{Err: domain.ErrOverlappingTripBooking, StatusCode: http.StatusUnprocessableEntity, Code: "OVERLAPPING_TRIP_BOOKING", Message: "El pasajero ya posee una reserva activa en este viaje"},
	{Err: domain.ErrInvalidBookingStatus, StatusCode: http.StatusBadRequest, Code: "INVALID_BOOKING_STATUS", Message: "La reserva no se encuentra en el estado requerido para esta operación"},
	{Err: domain.ErrBookingExpired, StatusCode: http.StatusGone, Code: "BOOKING_EXPIRED", Message: "El tiempo límite de retención (15 min) ha expirado; los asientos fueron liberados"},
	{Err: domain.ErrPaymentFailed, StatusCode: http.StatusBadGateway, Code: "PAYMENT_GATEWAY_ERROR", Message: "Fallo al procesar o generar la intención de pago en la pasarela externa"},
	{Err: domain.ErrInvalidPaymentGatewayRef, StatusCode: http.StatusBadRequest, Code: "INVALID_PAYMENT_INTENT", Message: "La referencia de pago provista no coincide con la intención creada para esta reserva"},
	{Err: domain.ErrMaxPendingBookingsExceeded, StatusCode: http.StatusTooManyRequests, Code: "MAX_PENDING_BOOKINGS_EXCEEDED", Message: "Has alcanzado el límite máximo permitido de 3 reservas pendientes de pago simultáneas"},
	{Err: domain.ErrRateLimitExceeded, StatusCode: http.StatusTooManyRequests, Code: "RATE_LIMIT_EXCEEDED", Message: "Has excedido el límite de solicitudes permitidas; por favor intenta nuevamente en unos momentos"},
	{Err: domain.ErrConcurrentModification, StatusCode: http.StatusConflict, Code: "CONCURRENT_MODIFICATION", Message: "Conflicto de modificación concurrente; por favor intenta nuevamente"},

	// Escrow y custodia
	{Err: domain.ErrEscrowNotFound, StatusCode: http.StatusNotFound, Code: "ESCROW_NOT_FOUND", Message: "Transacción de custodia no encontrada"},
	{Err: domain.ErrEscrowAlreadySettled, StatusCode: http.StatusBadRequest, Code: "ESCROW_ALREADY_SETTLED", Message: "Los fondos en custodia ya han sido liquidados o reembolsados previamente"},
	{Err: domain.ErrInsufficientBalance, StatusCode: http.StatusBadRequest, Code: "INSUFFICIENT_BALANCE", Message: "Saldo insuficiente para procesar la transacción"},
	{Err: domain.ErrInvalidRefundAmount, StatusCode: http.StatusBadRequest, Code: "INVALID_REFUND_AMOUNT", Message: "El monto de reembolso calculado es inválido"},

	// Reputación y reseñas
	{Err: domain.ErrReviewNotFound, StatusCode: http.StatusNotFound, Code: "REVIEW_NOT_FOUND", Message: "Reseña no encontrada"},
	{Err: domain.ErrReviewAlreadyExists, StatusCode: http.StatusConflict, Code: "DUPLICATE_REVIEW", Message: "Ya has emitido una calificación previa para esta reserva"},
	{Err: domain.ErrDuplicateReview, StatusCode: http.StatusConflict, Code: "DUPLICATE_REVIEW", Message: "Ya has emitido una calificación previa para esta reserva"},
	{Err: domain.ErrReviewSelfNotAllowed, StatusCode: http.StatusUnprocessableEntity, Code: "SELF_REVIEW_NOT_ALLOWED", Message: "No está permitido calificarse a sí mismo"},
	{Err: domain.ErrInvalidRating, StatusCode: http.StatusBadRequest, Code: "INVALID_RATING", Message: "La calificación debe ser un valor entero entre 1 y 5 estrellas"},
	{Err: domain.ErrTripNotCompletedForReview, StatusCode: http.StatusBadRequest, Code: "TRIP_NOT_COMPLETED_FOR_REVIEW", Message: "Solo se pueden calificar viajes y reservas completadas"},

	// Disputas
	{Err: domain.ErrDisputeNotFound, StatusCode: http.StatusNotFound, Code: "DISPUTE_NOT_FOUND", Message: "Disputa no encontrada"},
	{Err: domain.ErrDisputeAlreadyExists, StatusCode: http.StatusConflict, Code: "DISPUTE_ALREADY_EXISTS", Message: "Ya existe una disputa activa sobre esta transacción de custodia"},
	{Err: domain.ErrInvalidDisputeData, StatusCode: http.StatusBadRequest, Code: "INVALID_DISPUTE_DATA", Message: "Datos de la disputa o dictamen de resolución inválidos o insuficientes"},
	{Err: domain.ErrDisputeSelfNotAllowed, StatusCode: http.StatusUnprocessableEntity, Code: "DISPUTE_SELF_NOT_ALLOWED", Message: "No es posible disputar una transacción contra uno mismo"},
	{Err: domain.ErrDisputeAlreadyResolved, StatusCode: http.StatusConflict, Code: "DISPUTE_ALREADY_RESOLVED", Message: "La disputa ya ha sido resuelta o desestimada previamente"},

	// Moneda, formato financiero y estados tipados
	{Err: domain.ErrInvalidCurrency, StatusCode: http.StatusBadRequest, Code: "INVALID_CURRENCY", Message: "Código de moneda inválido; se requiere código ISO 4217 de 3 letras"},
	{Err: domain.ErrCurrencyMismatch, StatusCode: http.StatusBadRequest, Code: "CURRENCY_MISMATCH", Message: "Las monedas de los montos involucrados no coinciden"},
	{Err: domain.ErrInvalidMoneyFormat, StatusCode: http.StatusBadRequest, Code: "INVALID_MONEY_FORMAT", Message: "Formato monetario inválido"},
	{Err: domain.ErrInvalidStatus, StatusCode: http.StatusBadRequest, Code: "INVALID_STATUS", Message: "El valor de estado proporcionado no es válido"},
}

// respondError resuelve cualquier error devuelto por la capa de servicio usando la tabla centralizada
// de errores del dominio, o registra el fallo inesperado y responde un 500 genérico sin filtrar detalles internos.
func respondError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	reqID := middleware.GetRequestID(c)

	for _, spec := range domainErrorTable {
		if errors.Is(err, spec.Err) {
			c.JSON(spec.StatusCode, StandardErrorResponse{
				Error:     spec.Message,
				Code:      spec.Code,
				RequestID: reqID,
			})
			return
		}
	}

	method := ""
	path := ""
	if c.Request != nil {
		method = c.Request.Method
		if c.Request.URL != nil {
			path = c.Request.URL.Path
		}
	}

	log.Printf("[ERROR] [RequestID: %s] [Path: %s %s] Error interno no controlado: %v\nStack trace:\n%s",
		reqID, method, path, err, debug.Stack())

	c.JSON(http.StatusInternalServerError, StandardErrorResponse{
		Error:     "Ha ocurrido un error interno al procesar su solicitud. Por favor contacte a soporte con el ID de referencia.",
		Code:      "INTERNAL_SERVER_ERROR",
		RequestID: reqID,
	})
}

// HandleError delega en respondError manteniendo compatibilidad con invocaciones existentes.
func HandleError(c *gin.Context, err error) {
	respondError(c, err)
}

func respondJSONError(c *gin.Context, statusCode int, message, code string) {
	c.JSON(statusCode, StandardErrorResponse{
		Error:     message,
		Code:      code,
		RequestID: middleware.GetRequestID(c),
	})
}

// RespondBadRequest responde a fallos de parsing de parámetros o payloads con un formato homogéneo y seguro.
func RespondBadRequest(c *gin.Context, message, code string) {
	respondJSONError(c, http.StatusBadRequest, message, code)
}

// RespondUnauthorized responde con un error 401 estructurado y seguro.
func RespondUnauthorized(c *gin.Context, message, code string) {
	respondJSONError(c, http.StatusUnauthorized, message, code)
}

// RespondForbidden responde con un error 403 estructurado y seguro.
func RespondForbidden(c *gin.Context, message, code string) {
	respondJSONError(c, http.StatusForbidden, message, code)
}
