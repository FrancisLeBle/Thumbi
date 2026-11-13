package http

import (
	"errors"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// StandardErrorResponse estructura estandarizada y segura de error hacia el cliente
type StandardErrorResponse struct {
	Error     string `json:"error"`
	Code      string `json:"code,omitempty"`
	RequestID string `json:"request_id"`
}

// ErrorSpec define el código HTTP, código semántico y mensaje sanitizado para errores conocidos
type ErrorSpec struct {
	StatusCode int
	Code       string
	Message    string
}

// domainErrorSpecs mapa centralizado de errores del dominio hacia especificaciones HTTP seguras
var domainErrorSpecs = map[error]ErrorSpec{
	// Errores de Autenticación y Credenciales
	domain.ErrUserNotFound:       {StatusCode: http.StatusNotFound, Code: "USER_NOT_FOUND", Message: "Usuario no encontrado"},
	domain.ErrUserAlreadyExists:  {StatusCode: http.StatusConflict, Code: "USER_ALREADY_EXISTS", Message: "El usuario ya se encuentra registrado"},
	domain.ErrInvalidCredentials: {StatusCode: http.StatusUnauthorized, Code: "INVALID_CREDENTIALS", Message: "Credenciales de acceso inválidas"},
	domain.ErrInvalidToken:       {StatusCode: http.StatusUnauthorized, Code: "INVALID_TOKEN", Message: "Token no válido o mal formado"},
	domain.ErrTokenExpired:       {StatusCode: http.StatusUnauthorized, Code: "TOKEN_EXPIRED", Message: "El token de acceso ha expirado"},
	domain.ErrSessionExpired:     {StatusCode: http.StatusUnauthorized, Code: "SESSION_EXPIRED", Message: "La sesión ha expirado tras 20 minutos de inactividad o límite de tiempo"},
	domain.ErrUnauthorized:       {StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: "No tienes autorización para realizar esta operación"},
	domain.ErrForbidden:          {StatusCode: http.StatusForbidden, Code: "FORBIDDEN", Message: "Acceso denegado"},
	domain.ErrInvalidInput:       {StatusCode: http.StatusBadRequest, Code: "BAD_REQUEST", Message: "Parámetros de solicitud inválidos"},
	domain.ErrUnverifiedEmail:    {StatusCode: http.StatusForbidden, Code: "UNVERIFIED_EMAIL", Message: "El correo electrónico provisto por la cuenta social no se encuentra verificado"},
	domain.ErrIdentityNotFound:   {StatusCode: http.StatusNotFound, Code: "IDENTITY_NOT_FOUND", Message: "Identidad social no encontrada"},
	domain.ErrIdentityAlreadyExists: {StatusCode: http.StatusConflict, Code: "IDENTITY_ALREADY_EXISTS", Message: "La identidad social ya se encuentra vinculada a otra cuenta"},

	// Errores de KYC y Biometría
	domain.ErrKYCAlreadySubmitted: {StatusCode: http.StatusConflict, Code: "KYC_ALREADY_SUBMITTED", Message: "Ya existe un trámite de verificación KYC en proceso"},
	domain.ErrKYCAlreadyApproved:  {StatusCode: http.StatusConflict, Code: "KYC_ALREADY_APPROVED", Message: "La identidad del usuario ya ha sido verificada y aprobada previamente"},
	domain.ErrKYCInManualReview:   {StatusCode: http.StatusLocked, Code: "PENDING_MANUAL_REVIEW", Message: "Tu trámite se encuentra actualmente en revisión manual. Te notificaremos cuando se complete el análisis"},
	domain.ErrKYCNotFound:         {StatusCode: http.StatusNotFound, Code: "KYC_NOT_FOUND", Message: "Verificación de identidad no encontrada"},
	domain.ErrKYCNotApproved:      {StatusCode: http.StatusForbidden, Code: "KYC_NOT_APPROVED", Message: "La verificación de identidad aún no ha sido aprobada"},
	domain.ErrDriverKYCRequired:   {StatusCode: http.StatusForbidden, Code: "DRIVER_KYC_REQUIRED", Message: "Es obligatorio completar y aprobar la verificación de identidad biométrica (DNI y selfie 3D) antes de continuar"},
	domain.ErrDriverNotActive:     {StatusCode: http.StatusForbidden, Code: "DRIVER_NOT_ACTIVE", Message: "El usuario no cuenta con rol de conductor activo o no ha completado la verificación KYC"},
	domain.ErrDriverPhoneRequired: {StatusCode: http.StatusUnprocessableEntity, Code: "DRIVER_PHONE_REQUIRED", Message: "El conductor no posee un número de teléfono válido registrado"},
	domain.ErrMaxRetriesExceeded:  {StatusCode: http.StatusUnprocessableEntity, Code: "PENDING_MANUAL_REVIEW", Message: "Se ha superado el máximo de 3 reintentos automatizados. Tu solicitud ha sido derivada a revisión manual por nuestra mesa de control"},

	// Errores de Vehículos
	domain.ErrVehicleNotFound:        {StatusCode: http.StatusNotFound, Code: "VEHICLE_NOT_FOUND", Message: "Vehículo no encontrado"},
	domain.ErrVehicleAlreadyExists:   {StatusCode: http.StatusConflict, Code: "VEHICLE_ALREADY_EXISTS", Message: "El vehículo ya se encuentra registrado"},
	domain.ErrVehicleNotAvailable:    {StatusCode: http.StatusBadRequest, Code: "VEHICLE_NOT_APPROVED", Message: "El vehículo no se encuentra aprobado para realizar viajes"},
	domain.ErrVehicleAlreadyVerified: {StatusCode: http.StatusConflict, Code: "VEHICLE_ALREADY_VERIFIED", Message: "El dominio/patente especificado ya se encuentra registrado y verificado en la plataforma"},
	domain.ErrInvalidVehicleData:     {StatusCode: http.StatusBadRequest, Code: "INVALID_VEHICLE_DATA", Message: "Los datos del vehículo ingresados son inválidos"},
	domain.ErrSeatsExceedCapacity:    {StatusCode: http.StatusBadRequest, Code: "SEATS_EXCEED_CAPACITY", Message: "La cantidad de asientos ofertados supera la capacidad del vehículo"},

	// Errores de Viajes
	domain.ErrTripNotFound:              {StatusCode: http.StatusNotFound, Code: "TRIP_NOT_FOUND", Message: "Viaje no encontrado"},
	domain.ErrTripFull:                  {StatusCode: http.StatusConflict, Code: "TRIP_FULL", Message: "El viaje no cuenta con cupos disponibles"},
	domain.ErrTripAlreadyStarted:        {StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_STARTED", Message: "El viaje ya ha iniciado"},
	domain.ErrTripAlreadyCompleted:      {StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_COMPLETED", Message: "El viaje ya ha finalizado"},
	domain.ErrTripAlreadyCancelled:      {StatusCode: http.StatusBadRequest, Code: "TRIP_ALREADY_CANCELLED", Message: "El viaje ya se encuentra cancelado"},
	domain.ErrTripCannotBeCancelled:     {StatusCode: http.StatusBadRequest, Code: "TRIP_CANNOT_BE_CANCELLED", Message: "El viaje no puede ser cancelado en su estado actual"},
	domain.ErrCannotCancelStartedTrip:   {StatusCode: http.StatusBadRequest, Code: "CANNOT_CANCEL_STARTED_TRIP", Message: "No es posible cancelar un viaje que ya se encuentra en curso o completado"},
	domain.ErrDepartureTimeMustBeFuture: {StatusCode: http.StatusBadRequest, Code: "INVALID_DEPARTURE_TIME", Message: "La fecha y hora de salida debe ser estrictamente en el futuro"},
	domain.ErrInvalidTripStatusChange:   {StatusCode: http.StatusBadRequest, Code: "INVALID_TRIP_STATUS", Message: "El viaje no se encuentra disponible para reservas o en un estado que permita la operación"},
	domain.ErrPriceExceedsCapPrice:      {StatusCode: http.StatusUnprocessableEntity, Code: "PRICE_EXCEEDS_CAP_PRICE", Message: "El precio fijado excede el límite máximo permitido (Cap Price) para el trayecto según la normativa de economía colaborativa no lucrativa"},
	domain.ErrInvalidRouteCoordinates:   {StatusCode: http.StatusBadRequest, Code: "INVALID_COORDINATES", Message: "Las coordenadas geográficas proporcionadas son inválidas"},
	domain.ErrInsufficientRoutePoints:   {StatusCode: http.StatusBadRequest, Code: "INSUFFICIENT_ROUTE_POINTS", Message: "Se requieren al menos dos puntos para trazar la ruta"},

	// Errores de Reservas y Pagos
	domain.ErrBookingNotFound:            {StatusCode: http.StatusNotFound, Code: "BOOKING_NOT_FOUND", Message: "Reserva no encontrada"},
	domain.ErrBookingAlreadyExists:       {StatusCode: http.StatusConflict, Code: "BOOKING_ALREADY_EXISTS", Message: "Ya existe una reserva registrada"},
	domain.ErrBookingCannotBeCancelled:   {StatusCode: http.StatusBadRequest, Code: "INVALID_STATUS", Message: "La reserva no puede cancelarse en su estado actual"},
	domain.ErrBookingAlreadyCancelled:    {StatusCode: http.StatusConflict, Code: "ALREADY_CANCELLED", Message: "La reserva ya se encuentra cancelada"},
	domain.ErrCannotBookOwnTrip:          {StatusCode: http.StatusBadRequest, Code: "CANNOT_BOOK_OWN_TRIP", Message: "Un conductor no puede reservar plazas en su propio viaje"},
	domain.ErrInvalidSeatCount:           {StatusCode: http.StatusBadRequest, Code: "INVALID_SEAT_COUNT", Message: "La cantidad de asientos solicitados debe ser mayor a cero"},
	domain.ErrInsufficientSeats:          {StatusCode: http.StatusConflict, Code: "INSUFFICIENT_SEATS", Message: "No hay suficientes asientos disponibles para la cantidad solicitada"},
	domain.ErrOverlappingTripBooking:     {StatusCode: http.StatusUnprocessableEntity, Code: "OVERLAPPING_TRIP_BOOKING", Message: "El pasajero ya posee una reserva activa en este viaje"},
	domain.ErrInvalidBookingStatus:       {StatusCode: http.StatusBadRequest, Code: "INVALID_BOOKING_STATUS", Message: "La reserva no se encuentra en el estado requerido para esta operación"},
	domain.ErrBookingExpired:             {StatusCode: http.StatusGone, Code: "BOOKING_EXPIRED", Message: "El tiempo límite de retención (15 min) ha expirado; los asientos fueron liberados"},
	domain.ErrPaymentFailed:              {StatusCode: http.StatusBadGateway, Code: "PAYMENT_GATEWAY_ERROR", Message: "Fallo al procesar o generar la intención de pago en la pasarela externa"},
	domain.ErrInvalidPaymentGatewayRef:   {StatusCode: http.StatusBadRequest, Code: "INVALID_PAYMENT_INTENT", Message: "La referencia de pago provista no coincide con la intención creada para esta reserva"},
	domain.ErrMaxPendingBookingsExceeded: {StatusCode: http.StatusTooManyRequests, Code: "MAX_PENDING_BOOKINGS_EXCEEDED", Message: "Has alcanzado el límite máximo permitido de 3 reservas pendientes de pago simultáneas"},
	domain.ErrRateLimitExceeded:          {StatusCode: http.StatusTooManyRequests, Code: "RATE_LIMIT_EXCEEDED", Message: "Has excedido el límite de solicitudes permitidas; por favor intenta nuevamente en unos momentos"},

	// Errores de Escrow y Custodia
	domain.ErrEscrowNotFound:       {StatusCode: http.StatusNotFound, Code: "ESCROW_NOT_FOUND", Message: "Transacción de custodia no encontrada"},
	domain.ErrEscrowAlreadySettled: {StatusCode: http.StatusBadRequest, Code: "ESCROW_ALREADY_SETTLED", Message: "Los fondos en custodia ya han sido liquidados o reembolsados previamente"},
	domain.ErrInsufficientBalance:  {StatusCode: http.StatusBadRequest, Code: "INSUFFICIENT_BALANCE", Message: "Saldo insuficiente para procesar la transacción"},

	// Errores de Reputación y Reseñas
	domain.ErrReviewNotFound:            {StatusCode: http.StatusNotFound, Code: "REVIEW_NOT_FOUND", Message: "Reseña no encontrada"},
	domain.ErrReviewAlreadyExists:       {StatusCode: http.StatusConflict, Code: "DUPLICATE_REVIEW", Message: "Ya has emitido una calificación previa para esta reserva"},
	domain.ErrDuplicateReview:           {StatusCode: http.StatusConflict, Code: "DUPLICATE_REVIEW", Message: "Ya has emitido una calificación previa para esta reserva"},
	domain.ErrReviewSelfNotAllowed:      {StatusCode: http.StatusUnprocessableEntity, Code: "SELF_REVIEW_NOT_ALLOWED", Message: "No está permitido calificarse a sí mismo"},
	domain.ErrInvalidRating:             {StatusCode: http.StatusBadRequest, Code: "INVALID_RATING", Message: "La calificación debe ser un valor entero entre 1 y 5 estrellas"},
	domain.ErrTripNotCompletedForReview: {StatusCode: http.StatusBadRequest, Code: "TRIP_NOT_COMPLETED_FOR_REVIEW", Message: "Solo se pueden calificar viajes y reservas completadas"},

	// Errores de Disputas
	domain.ErrDisputeNotFound:        {StatusCode: http.StatusNotFound, Code: "DISPUTE_NOT_FOUND", Message: "Disputa no encontrada"},
	domain.ErrDisputeAlreadyExists:   {StatusCode: http.StatusConflict, Code: "DISPUTE_ALREADY_EXISTS", Message: "Ya existe una disputa activa sobre esta transacción de custodia"},
	domain.ErrInvalidDisputeData:     {StatusCode: http.StatusBadRequest, Code: "INVALID_DISPUTE_DATA", Message: "Datos de la disputa o dictamen de resolución inválidos o insuficientes"},
	domain.ErrDisputeSelfNotAllowed:  {StatusCode: http.StatusUnprocessableEntity, Code: "DISPUTE_SELF_NOT_ALLOWED", Message: "No es posible disputar una transacción contra uno mismo"},
	domain.ErrDisputeAlreadyResolved: {StatusCode: http.StatusConflict, Code: "DISPUTE_ALREADY_RESOLVED", Message: "La disputa ya ha sido resuelta o desestimada previamente"},
}

// HandleError evalúa de manera centralizada el error retornado por un servicio o caso de uso.
// Si el error coincide con un error del dominio, responde con el código HTTP y mensaje amigable predefinido.
// Si se trata de un error desconocido, de base de datos o fallo de infraestructura, lo registra en logs
// detallados en el servidor (con stack/contexto) y devuelve un mensaje genérico inofensivo con el Request ID.
func HandleError(c *gin.Context, err error) {
	if err == nil {
		return
	}

	reqID := middleware.GetRequestID(c)

	// 1. Verificar si coincide con algún error conocido del dominio mediante errors.Is
	for domainErr, spec := range domainErrorSpecs {
		if errors.Is(err, domainErr) {
			c.JSON(spec.StatusCode, gin.H{
				"error":      spec.Message,
				"code":       spec.Code,
				"request_id": reqID,
			})
			return
		}
	}

	// 2. Si no es un error conocido del dominio, se trata de un error interno inesperado (DB, driver, IO, network).
	// REGISTRO OBLIGATORIO en el servidor con Request ID y traza de error real para debugging
	log.Printf("[ERROR] [RequestID: %s] [Path: %s %s] Error interno no controlado: %v\nStack trace:\n%s",
		reqID, c.Request.Method, c.Request.URL.Path, err, debug.Stack())

	// 3. Respuesta genérica y segura al cliente sin exponer datos de la base de datos o infraestructura
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":      "Ha ocurrido un error interno al procesar su solicitud. Por favor contacte a soporte con el ID de referencia.",
		"code":       "INTERNAL_SERVER_ERROR",
		"request_id": reqID,
	})
}

// RespondBadRequest responde a fallos de parsing de parámetros o payloads con un formato homogéneo y seguro.
func RespondBadRequest(c *gin.Context, message, code string) {
	reqID := middleware.GetRequestID(c)
	c.JSON(http.StatusBadRequest, gin.H{
		"error":      message,
		"code":       code,
		"request_id": reqID,
	})
}

// RespondUnauthorized responde con un error 401 estructurado y seguro
func RespondUnauthorized(c *gin.Context, message, code string) {
	reqID := middleware.GetRequestID(c)
	c.JSON(http.StatusUnauthorized, gin.H{
		"error":      message,
		"code":       code,
		"request_id": reqID,
	})
}

// RespondForbidden responde con un error 403 estructurado y seguro
func RespondForbidden(c *gin.Context, message, code string) {
	reqID := middleware.GetRequestID(c)
	c.JSON(http.StatusForbidden, gin.H{
		"error":      message,
		"code":       code,
		"request_id": reqID,
	})
}
