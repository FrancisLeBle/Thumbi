package http

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// RouterConfig agrupa las dependencias requeridas para inicializar el router HTTP de la API.
// Todos los handlers de dominio, HealthHandler y AuthMiddleware son obligatorios.
type RouterConfig struct {
	AuthHandler           *AuthHandler
	KYCHandler            *KYCHandler
	VehicleHandler        *VehicleHandler
	TripHandler           *TripHandler
	BookingHandler        *BookingHandler
	ReviewHandler         *ReviewHandler
	DisputeHandler        *DisputeHandler
	HealthHandler         *HealthHandler
	AuthMiddleware        gin.HandlerFunc
	WebhookAuthMiddleware gin.HandlerFunc
	RateLimiter           ports.RateLimiter
	WebhookSecret         string
	AllowedOrigins        []string
}

// Validate verifica que todos los handlers y middlewares requeridos estén configurados.
func (cfg RouterConfig) Validate() error {
	var missing []string
	if cfg.AuthHandler == nil {
		missing = append(missing, "AuthHandler")
	}
	if cfg.KYCHandler == nil {
		missing = append(missing, "KYCHandler")
	}
	if cfg.VehicleHandler == nil {
		missing = append(missing, "VehicleHandler")
	}
	if cfg.TripHandler == nil {
		missing = append(missing, "TripHandler")
	}
	if cfg.BookingHandler == nil {
		missing = append(missing, "BookingHandler")
	}
	if cfg.ReviewHandler == nil {
		missing = append(missing, "ReviewHandler")
	}
	if cfg.DisputeHandler == nil {
		missing = append(missing, "DisputeHandler")
	}
	if cfg.HealthHandler == nil {
		missing = append(missing, "HealthHandler")
	}
	if cfg.AuthMiddleware == nil {
		missing = append(missing, "AuthMiddleware")
	}
	if cfg.WebhookAuthMiddleware == nil && strings.TrimSpace(cfg.WebhookSecret) == "" {
		missing = append(missing, "WebhookSecret")
	}

	if len(missing) > 0 {
		return fmt.Errorf("configuración de router incompleta; faltan dependencias obligatorias: %s", strings.Join(missing, ", "))
	}
	return nil
}

const (
	// CanonicalAPIPrefix es el prefijo único y canónico de la API (alineado con los clientes activos en /v1/*).
	CanonicalAPIPrefix = "/v1"
	// DeprecatedAPIPrefix es el prefijo legado que redirige permanentemente hacia CanonicalAPIPrefix durante la transición.
	DeprecatedAPIPrefix = "/api/v1"
)

// SetupRouter valida todas las dependencias requeridas, configura middlewares globales,
// registra de forma incondicional todas las rutas bajo el prefijo canónico /v1 y monta la redirección de transición desde /api/v1.
func SetupRouter(cfg RouterConfig) (*gin.Engine, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	r := gin.New()

	// Middlewares estándar
	r.Use(gin.Recovery())
	r.Use(middleware.RequestIDMiddleware())
	r.Use(gin.Logger())
	r.Use(corsMiddleware(cfg.AllowedOrigins...))

	// Endpoints de salud y readiness (Liveness & Readiness probes)
	r.GET("/health", cfg.HealthHandler.Health)
	r.GET("/ready", cfg.HealthHandler.Ready)

	// Configuración de autenticación para Webhooks
	webhookAuth := cfg.WebhookAuthMiddleware
	if webhookAuth == nil {
		webhookAuth = middleware.WebhookAuthMiddleware(cfg.WebhookSecret)
	}

	// Registrar todas las rutas incondicionalmente bajo el prefijo canónico /v1
	v1 := r.Group(CanonicalAPIPrefix)
	{
		// Rutas públicas de Autenticación protegidas con limitación de tasa por IP
		auth := v1.Group("/auth")
		{
			auth.POST("/social-login", middleware.RateLimitByIP(cfg.RateLimiter, 10, time.Minute, "login"), cfg.AuthHandler.SocialLogin)
			auth.POST("/refresh", middleware.RateLimitByIP(cfg.RateLimiter, 20, time.Minute, "refresh"), cfg.AuthHandler.Refresh)
			auth.POST("/logout", cfg.AuthHandler.Logout)

			// Rutas protegidas por sesión activa (token opaco con 20 min TTL)
			authProtected := auth.Group("")
			authProtected.Use(cfg.AuthMiddleware)
			{
				authProtected.GET("/me", cfg.AuthHandler.Me)
			}
		}

		// Rutas de Verificación KYC y Biometría
		kyc := v1.Group("/kyc")
		kyc.Use(cfg.AuthMiddleware)
		{
			kyc.POST("/submit", cfg.KYCHandler.Submit)
			kyc.GET("/status", cfg.KYCHandler.Status)
		}

		// Rutas de Gestión de Vehículos
		vehicles := v1.Group("/vehicles")
		vehicles.Use(cfg.AuthMiddleware)
		{
			vehicles.POST("/register", cfg.VehicleHandler.Register)
			vehicles.GET("/status", cfg.VehicleHandler.Status)
		}

		// Webhook / Callback para resoluciones asíncronas de trámite vehicular
		v1.POST("/vehicles/:id/resolution", webhookAuth, cfg.VehicleHandler.ProcessResolution)

		// Rutas del Módulo 2: Core Match, Publicación y Búsqueda de Viajes
		trips := v1.Group("/trips")
		{
			// Búsqueda geoespacial y detalle (públicas; búsqueda protegida por rate limit por IP: 30 req/min)
			trips.GET("/search", middleware.RateLimitByIP(cfg.RateLimiter, 30, time.Minute, "search"), cfg.TripHandler.SearchTrips)
			trips.GET("/:id", cfg.TripHandler.GetTripByID)

			// Publicación, finalización y cancelación de viajes (requieren sesión activa de conductor autenticado)
			tripsProtected := trips.Group("")
			tripsProtected.Use(cfg.AuthMiddleware)
			{
				tripsProtected.POST("", cfg.TripHandler.CreateTrip)
				tripsProtected.POST("/:id/complete", cfg.TripHandler.CompleteTrip)
				tripsProtected.DELETE("/:id", cfg.TripHandler.CancelTrip)
			}
		}

		// Rutas del Módulo 3: Reservas, Gestión de Asientos y Pagos en Escrow
		// Webhooks firmados para confirmación autoritativa de pagos de la pasarela (HMAC-SHA256)
		v1.POST("/payments/webhook", webhookAuth, cfg.BookingHandler.ProcessPaymentWebhook)
		v1.POST("/bookings/payment-webhook", webhookAuth, cfg.BookingHandler.ProcessPaymentWebhook)

		bookings := v1.Group("/bookings")
		bookings.Use(cfg.AuthMiddleware)
		{
			// Creación de reserva protegida por limitación por usuario (5 req/min) para evitar DoS y bloqueo masivo de cupos
			bookings.POST("", middleware.RateLimitByUser(cfg.RateLimiter, 5, time.Minute, "create_booking"), cfg.BookingHandler.CreateBooking)
			bookings.POST("/:id/confirm-payment", cfg.BookingHandler.ConfirmPayment)
			bookings.POST("/:id/cancel", cfg.BookingHandler.CancelBooking)
			bookings.GET("/my-bookings", cfg.BookingHandler.MyBookings)
			bookings.GET("/:id", cfg.BookingHandler.GetBookingByID)
			bookings.GET("/:id/contact-link", cfg.BookingHandler.GetContactLink)
		}

		// Rutas del Módulo 4: Calificaciones y Reputación
		v1.GET("/users/:user_id/reviews", cfg.ReviewHandler.GetUserReviews)

		reviewsProtected := v1.Group("/reviews")
		reviewsProtected.Use(cfg.AuthMiddleware)
		{
			reviewsProtected.POST("", cfg.ReviewHandler.CreateReview)
		}

		// Rutas del Módulo 4: Gestión y Mediación de Disputas
		disputesProtected := v1.Group("/disputes")
		disputesProtected.Use(cfg.AuthMiddleware)
		{
			disputesProtected.POST("", cfg.DisputeHandler.OpenDispute)
			disputesProtected.GET("/:id", cfg.DisputeHandler.GetDisputeByID)
			disputesProtected.POST("/:id/resolve", middleware.RequireRole(domain.RoleAdmin), cfg.DisputeHandler.ResolveDispute)
			disputesProtected.GET("", middleware.RequireRole(domain.RoleAdmin), cfg.DisputeHandler.ListDisputes)
		}

		// Rutas de Observabilidad y Monitoreo (Módulo 5)
		v1.GET("/health", cfg.HealthHandler.Health)
		v1.GET("/ready", cfg.HealthHandler.Ready)
	}

	// Redirección de transición desde el prefijo legado /api/v1 -> /v1 con HTTP 308 (preserva método y body)
	redirectLegacyV1 := RedirectDeprecatedAPIPrefix(DeprecatedAPIPrefix, CanonicalAPIPrefix)
	r.Any(DeprecatedAPIPrefix, redirectLegacyV1)
	r.Any(DeprecatedAPIPrefix+"/*path", redirectLegacyV1)

	return r, nil
}

// RedirectDeprecatedAPIPrefix redirige solicitudes dirigidas a un prefijo legado hacia el prefijo canónico
// utilizando HTTP 308 Permanent Redirect para preservar el método HTTP y el cuerpo en peticiones POST/PUT/PATCH/DELETE.
// Evita duplicar middlewares de autenticación, rate limiting y monitoreo sobre el prefijo obsoleto.
func RedirectDeprecatedAPIPrefix(deprecatedPrefix, canonicalPrefix string) gin.HandlerFunc {
	cleanDeprecated := "/" + strings.Trim(deprecatedPrefix, "/")
	cleanCanonical := "/" + strings.Trim(canonicalPrefix, "/")

	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		suffix := strings.TrimPrefix(reqPath, cleanDeprecated)

		var targetPath string
		switch {
		case suffix == "":
			targetPath = cleanCanonical
		case suffix == "/":
			targetPath = cleanCanonical + "/"
		default:
			// Normalizar barras iniciales múltiples para prevenir open redirects tipo "//host"
			targetPath = cleanCanonical + "/" + strings.TrimLeft(suffix, "/")
		}

		if rawQuery := c.Request.URL.RawQuery; rawQuery != "" {
			targetPath += "?" + rawQuery
		}

		c.Header("Deprecation", "true")
		c.Header("Link", "<"+cleanCanonical+">; rel=\"successor-version\"")
		c.Redirect(http.StatusPermanentRedirect, targetPath)
	}
}

// DefaultTrustedOrigins define los orígenes seguros permitidos por defecto para la app móvil y desarrollo local
var DefaultTrustedOrigins = []string{
	"https://localhost",
	"capacitor://localhost",
	"ionic://localhost",
}

// DevTrustedOrigins define los orígenes HTTP locales permitidos únicamente en entornos local/development
var DevTrustedOrigins = []string{
	"http://localhost",
	"http://localhost:3000",
	"http://localhost:5173",
	"http://localhost:8080",
}

// corsMiddleware implementa una política CORS segura basada en lista blanca estricta de orígenes.
// Elimina el comodín '*' en combinación con 'Access-Control-Allow-Credentials: true' para prevenir
// que cualquier sitio web malicioso acceda a la API y cumplir con las especificaciones de seguridad de navegadores.
func corsMiddleware(extraOrigins ...string) gin.HandlerFunc {
	// Construir conjunto de orígenes autorizados normalizados (en minúsculas y sin trailing slashes)
	allowedOrigins := make(map[string]bool)

	addOrigin := func(raw string) {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		trimmed = strings.TrimRight(trimmed, "/")
		if trimmed != "" {
			allowedOrigins[trimmed] = true
		}
	}

	// 1. Orígenes base por defecto (Capacitor móvil y HTTPS localhost)
	for _, origin := range DefaultTrustedOrigins {
		addOrigin(origin)
	}
	if envName := strings.ToLower(strings.TrimSpace(os.Getenv("ENV"))); envName == "local" || envName == "development" {
		for _, origin := range DevTrustedOrigins {
			addOrigin(origin)
		}
	}

	// 2. Orígenes configurados en variables de entorno ALLOWED_ORIGINS (separados por coma)
	if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
		for _, o := range strings.Split(envOrigins, ",") {
			addOrigin(o)
		}
	}

	// 3. Dominio web oficial de la aplicación desde variable de entorno APP_URL
	if appURL := os.Getenv("APP_URL"); appURL != "" {
		addOrigin(appURL)
	}

	// 4. Orígenes adicionales provistos programáticamente en RouterConfig
	for _, o := range extraOrigins {
		addOrigin(o)
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Si no se envía la cabecera Origin (llamadas servidor a servidor, apps nativas puras o curl), continuar
		if origin == "" {
			c.Next()
			return
		}

		normalizedOrigin := strings.ToLower(strings.TrimSpace(origin))
		normalizedOrigin = strings.TrimRight(normalizedOrigin, "/")

		// Comprobar si el origen de la solicitud pertenece a la lista blanca autorizada
		isAllowed := allowedOrigins[normalizedOrigin]

		if isAllowed {
			// Reflejar de vuelta únicamente el origen explícito verificado (NUNCA el comodín '*')
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Webhook-Signature, X-Signature-SHA256, X-Webhook-Secret")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")
			c.Writer.Header().Set("Access-Control-Max-Age", "86400")
			c.Writer.Header().Set("Vary", "Origin")
		}

		// Manejo de peticiones preflight (OPTIONS)
		if c.Request.Method == http.MethodOptions {
			if !isAllowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "origen no autorizado por política CORS",
					"code":  "CORS_FORBIDDEN",
				})
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
