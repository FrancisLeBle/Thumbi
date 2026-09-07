package ports

import (
	"context"

	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// SocialUserProfile datos de perfil extraídos tras verificar el token social
type SocialUserProfile struct {
	ProviderID string              `json:"provider_id"`
	Email      string              `json:"email"`
	FirstName  string              `json:"first_name"`
	LastName   string              `json:"last_name"`
	AvatarURL  string              `json:"avatar_url,omitempty"`
	Provider   domain.AuthProvider `json:"provider"`
}

// OAuthProvider contrato para la validación de tokens de proveedores sociales (Google, Apple)
type OAuthProvider interface {
	ValidateToken(ctx context.Context, idToken string) (*SocialUserProfile, error)
	GetProviderName() domain.AuthProvider
}

// BiometricEvaluationResult resultado devuelto por el motor biométrico / OCR de identidad
type BiometricEvaluationResult struct {
	IsValidDocument  bool    `json:"is_valid_document"`
	LivenessScore    float64 `json:"liveness_score"`     // Escala 0.0 a 1.0 (>= 0.85 para pasar)
	FaceMatchScore   float64 `json:"face_match_score"`   // Coincidencia entre rostro del DNI y selfie 3D
	ExtractedDNI     string  `json:"extracted_dni"`      // OCR del DNI
	ExtractedName    string  `json:"extracted_name"`
	RejectionReason  string  `json:"rejection_reason,omitempty"`
}

// BiometricsProvider contrato para comunicarse con el motor de prueba de vida y validación documental
type BiometricsProvider interface {
	VerifyIdentity(ctx context.Context, docType, frontBase64, backBase64, selfie3DBase64 string) (*BiometricEvaluationResult, error)
}

// NotificationPayload contenido genérico de notificación para el usuario
type NotificationPayload struct {
	UserID    string            `json:"user_id"`
	Email     string            `json:"email"`
	Title     string            `json:"title"`
	Message   string            `json:"message"`
	Type      string            `json:"type"` // Ej: KYC_RESOLVED, VEHICLE_APPROVED, VEHICLE_REJECTED
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Notifier contrato para emisión de notificaciones asíncronas (Push, Email, Eventos de infraestructura)
type Notifier interface {
	SendResolutionNotification(ctx context.Context, payload NotificationPayload) error
}

// RouteCalculationRequest parámetros para solicitar el trazado de una ruta al motor cartográfico
type RouteCalculationRequest struct {
	Origin      domain.Coordinates   `json:"origin"`
	Destination domain.Coordinates   `json:"destination"`
	Waypoints   []domain.Coordinates `json:"waypoints,omitempty"`
}

// RouteDetails respuesta devuelta por el motor cartográfico (ej. OSRM, Valhalla o Google Maps)
type RouteDetails struct {
	RoutePath            domain.LineString `json:"route_path"`
	DistanceKm           float64           `json:"distance_km"`
	DurationMinutes      int               `json:"duration_minutes"`
	EstimatedTollCost    float64           `json:"estimated_toll_cost"`
	TollsCount           int               `json:"tolls_count"`
	EstimatedFuelCost    float64           `json:"estimated_fuel_cost"`
}

// RoutingProvider contrato para consultar polilínea, distancias, peajes y tiempos estimados a un motor de rutas
type RoutingProvider interface {
	CalculateRoute(ctx context.Context, req RouteCalculationRequest) (*RouteDetails, error)
}

// PaymentIntentResult resultado de la inicialización de intención de cobro con pasarela de pagos
type PaymentIntentResult struct {
	GatewayRef   string  `json:"gateway_ref"`
	ClientSecret string  `json:"client_secret,omitempty"`
	Amount       float64 `json:"amount"`
	Currency     string  `json:"currency"`
	Status       string  `json:"status"` // Ej: requires_payment_method, succeeded, pending
}

// RefundResult resultado de la solicitud de reintegro ante la pasarela
type RefundResult struct {
	GatewayRefundRef string  `json:"gateway_refund_ref"`
	Amount           float64 `json:"amount"`
	Status           string  `json:"status"`
}

// PaymentGateway define el contrato desacoplado para interactuar con pasarelas de pago (Stripe, MercadoPago, etc.)
type PaymentGateway interface {
	CreatePaymentIntent(ctx context.Context, bookingID string, amount float64, currency string) (*PaymentIntentResult, error)
	ConfirmPayment(ctx context.Context, gatewayRef string) (bool, error)
	ProcessRefund(ctx context.Context, gatewayRef string, amount float64, reason string) (*RefundResult, error)
}
