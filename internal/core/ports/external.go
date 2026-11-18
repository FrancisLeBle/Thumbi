package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
)

// SocialUserProfile datos de perfil extraídos tras verificar el token social
type SocialUserProfile struct {
	ProviderID    string
	Email         string
	FirstName     string
	LastName      string
	AvatarURL     string
	Provider      domain.AuthProvider
	EmailVerified bool
}

// OAuthProvider contrato para la validación de tokens de proveedores sociales (Google, Apple)
type OAuthProvider interface {
	ValidateToken(ctx context.Context, idToken string) (*SocialUserProfile, error)
	GetProviderName() domain.AuthProvider
}

// BiometricEvaluationResult resultado devuelto por el motor biométrico / OCR de identidad
type BiometricEvaluationResult struct {
	IsValidDocument bool
	LivenessScore   float64 // Escala 0.0 a 1.0 (>= 0.85 para pasar)
	FaceMatchScore  float64 // Coincidencia entre rostro del DNI y selfie 3D
	ExtractedDNI    string  // OCR del DNI
	ExtractedName   string
	RejectionReason string
}

// BiometricsProvider contrato para comunicarse con el motor de prueba de vida y validación documental
type BiometricsProvider interface {
	VerifyIdentity(ctx context.Context, docType, frontBase64, backBase64, selfie3DBase64 string) (*BiometricEvaluationResult, error)
}

// NotificationPayload contenido genérico de notificación para el usuario
type NotificationPayload struct {
	UserID   uuid.UUID
	Email    string
	Title    string
	Message  string
	Type     string // Ej: KYC_RESOLVED, VEHICLE_APPROVED, VEHICLE_REJECTED
	Metadata map[string]string
}

// Notifier contrato para emisión de notificaciones asíncronas (Push, Email, Eventos de infraestructura)
type Notifier interface {
	SendResolutionNotification(ctx context.Context, payload NotificationPayload) error
}

// RouteCalculationRequest parámetros para solicitar el trazado de una ruta al motor cartográfico
type RouteCalculationRequest struct {
	Origin      domain.Coordinates
	Destination domain.Coordinates
	Waypoints   []domain.Coordinates
}

// RouteDetails respuesta devuelta por el motor cartográfico (ej. OSRM, Valhalla o Google Maps)
type RouteDetails struct {
	RoutePath         domain.LineString
	DistanceKm        float64
	DurationMinutes   int
	EstimatedTollCost domain.Money
	TollsCount        int
	EstimatedFuelCost domain.Money
}

// RoutingProvider contrato para consultar polilínea, distancias, peajes y tiempos estimados a un motor de rutas
type RoutingProvider interface {
	CalculateRoute(ctx context.Context, req RouteCalculationRequest) (*RouteDetails, error)
}

// PaymentIntentResult resultado de la inicialización de intención de cobro con pasarela de pagos
type PaymentIntentResult struct {
	GatewayRef   string
	ClientSecret string
	Amount       domain.Money
	Currency     domain.Currency
	Status       string // Ej: requires_payment_method, succeeded, pending
}

// RefundResult resultado de la solicitud de reintegro ante la pasarela
type RefundResult struct {
	GatewayRefundRef string
	Amount           domain.Money
	Status           string
}

// PaymentGateway define el contrato desacoplado para interactuar con pasarelas de pago (Stripe, MercadoPago, etc.)
type PaymentGateway interface {
	CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*PaymentIntentResult, error)
	ConfirmPayment(ctx context.Context, gatewayRef string) (bool, error)
	ProcessRefund(ctx context.Context, gatewayRef string, amount domain.Money, reason string) (*RefundResult, error)
}

// RateLimiter define el contrato para limitación de tasa y protección contra abusos/DoS
type RateLimiter interface {
	// Allow verifica si la clave identificadora (IP o User ID) puede realizar una solicitud según el límite y ventana temporal.
	// Retorna allowed (true/false), remaining (solicitudes restantes), resetDuration (tiempo restante de la ventana), y error si lo hubiere.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetDuration time.Duration, err error)
}
