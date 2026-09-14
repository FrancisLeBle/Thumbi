package domain

import (
	"errors"
)

// Errores tipados del dominio (Clean Architecture)
var (
	// Errores de Autenticación y Sesión
	ErrInvalidCredentials     = errors.New("credenciales de autenticación inválidas")
	ErrSessionExpired         = errors.New("la sesión ha expirado por límite de tiempo (20 minutos)")
	ErrInvalidToken           = errors.New("el token de acceso proporcionado es inválido o está corrupto")
	ErrUserNotFound           = errors.New("usuario no encontrado")
	ErrUserAlreadyExists      = errors.New("el usuario ya se encuentra registrado con este correo o proveedor")
	ErrUnauthorized           = errors.New("no tiene permisos para realizar esta acción")

	// Errores de KYC y Biometría
	ErrKYCAlreadyApproved     = errors.New("la verificación de identidad ya se encuentra aprobada")
	ErrMaxRetriesExceeded     = errors.New("se ha superado el límite máximo de 3 reintentos automatizados para KYC")
	ErrInvalidDocumentData    = errors.New("los datos del DNI son ilegibles o incompletos")
	ErrLivenessCheckFailed    = errors.New("la prueba de vida biométrica (selfie 3D) no fue satisfactoria")
	ErrKYCInManualReview      = errors.New("la solicitud de verificación se encuentra bloqueada en revisión manual")

	// Errores de Vehículo y Conductor
	ErrDriverKYCRequired      = errors.New("es obligatorio completar la verificación de identidad (DNI y prueba de vida) para registrar un vehículo")
	ErrVehicleAlreadyVerified = errors.New("el vehículo ya cuenta con una verificación previa")
	ErrInvalidVehicleData     = errors.New("los datos del vehículo o de la licencia de conducir son inválidos")
	ErrVehicleNotFound        = errors.New("vehículo no encontrado")
	ErrVehicleNotApproved     = errors.New("el vehículo no ha sido aprobado por el proceso de verificación")

	// Errores de Módulo 2: Viajes y Geoespacial
	ErrDriverNotActive           = errors.New("solo los conductores con perfil activo y verificado pueden publicar viajes")
	ErrVehicleNotAvailable       = errors.New("el vehículo seleccionado no está aprobado o no se encuentra disponible")
	ErrInvalidRouteCoordinates   = errors.New("las coordenadas de origen, destino o paradas son inválidas")
	ErrInsufficientRoutePoints   = errors.New("la ruta requiere al menos 2 puntos geográficos distintos para ser trazada")
	ErrPriceExceedsCapPrice      = errors.New("el precio ingresado supera el tope máximo permitido (Cap Price)")
	ErrInvalidSeatCount          = errors.New("la cantidad de asientos ofertada debe estar entre 1 y la capacidad del vehículo")
	ErrDepartureTimeMustBeFuture = errors.New("la fecha y hora de salida del viaje debe ser estrictamente futura")
	ErrInvalidTripStatusChange   = errors.New("la transición de estado solicitada para el viaje no está permitida")
	ErrTripNotFound              = errors.New("viaje no encontrado")
	ErrTripCannotBeCancelled     = errors.New("el viaje no puede ser cancelado en su estado actual")

	// Errores de Módulo 3: Reservas, Concurrencia y Pagos en Custodia (Escrow)
	ErrBookingNotFound           = errors.New("reserva no encontrada")
	ErrCannotBookOwnTrip         = errors.New("el conductor no puede reservar asientos en su propio viaje")
	ErrInsufficientSeats         = errors.New("asientos insuficientes para completar la solicitud de reserva")
	ErrBookingExpired            = errors.New("la reserva ha expirado por haberse superado el tiempo límite de pago (15 minutos)")
	ErrInvalidBookingStatus      = errors.New("la transición de estado solicitada para la reserva no es válida")
	ErrBookingCannotBeCancelled  = errors.New("la reserva no puede cancelarse en su estado actual")
	ErrOverlappingTripBooking    = errors.New("el pasajero ya posee una reserva activa en un viaje con horario superpuesto")
	ErrEscrowNotFound            = errors.New("transacción de custodia no encontrada")
	ErrEscrowAlreadySettled      = errors.New("los fondos en custodia ya han sido liberados o reembolsados")
	ErrPaymentFailed             = errors.New("el procesamiento del pago de la reserva ha fallado")
	ErrInvalidRefundAmount       = errors.New("el monto a reembolsar excede el total custodiado en la transacción")

	// Errores de Módulo 4: Finalización de Viajes, Calificaciones y Disputas
	ErrInvalidRating             = errors.New("la calificación debe ser un valor entero entre 1 y 5")
	ErrReviewSelfNotAllowed      = errors.New("el usuario no puede calificarse a sí mismo")
	ErrTripNotCompletedForReview = errors.New("solo se pueden calificar viajes y reservas completadas")
	ErrDuplicateReview           = errors.New("ya se ha registrado una calificación previa para esta reserva")
	ErrReviewNotFound            = errors.New("reseña no encontrada")
	ErrDisputeAlreadyExists      = errors.New("ya existe una disputa abierta sobre esta transacción de custodia")
	ErrDisputeNotFound           = errors.New("disputa no encontrada")
	ErrInvalidDisputeStatus      = errors.New("la transición de estado de la disputa no es válida")
	ErrDisputeAlreadyResolved    = errors.New("la disputa ya ha sido resuelta o desestimada")
	ErrInvalidDisputeData        = errors.New("los datos de la disputa son incompletos o inválidos")
	ErrDisputeSelfNotAllowed     = errors.New("no es posible iniciar una disputa contra uno mismo")
)
