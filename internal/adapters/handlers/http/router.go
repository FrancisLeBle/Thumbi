package http

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

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
	WebhookSecret         string
	AllowedOrigins        []string
}

// SetupRouter configura el engine de Gin, middlewares globales y agrupamiento de rutas bajo /v1 y /api/v1
func SetupRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()

	// Middlewares estándar
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(corsMiddleware(cfg.AllowedOrigins...))

	// Endpoints de salud y readiness (Liveness & Readiness probes)
	if cfg.HealthHandler != nil {
		r.GET("/health", cfg.HealthHandler.Health)
		r.GET("/ready", cfg.HealthHandler.Ready)
	} else {
		r.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "UP",
				"service": "thumbi-api",
			})
		})
		r.GET("/ready", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "UP",
				"ready":   true,
				"service": "thumbi-api",
			})
		})
	}

	// Configuración de autenticación para Webhook de revisión de vehículos
	webhookAuth := cfg.WebhookAuthMiddleware
	if webhookAuth == nil {
		secret := cfg.WebhookSecret
		if secret == "" {
			secret = os.Getenv("WEBHOOK_SECRET")
		}
		if secret == "" {
			secret = middleware.DefaultWebhookSecret
		}
		webhookAuth = middleware.WebhookAuthMiddleware(secret)
	}

	registerV1Routes := func(group *gin.RouterGroup) {
		// Rutas públicas de Autenticación
		auth := group.Group("/auth")
		{
			auth.POST("/social-login", cfg.AuthHandler.SocialLogin)
			auth.POST("/refresh", cfg.AuthHandler.Refresh)
			auth.POST("/logout", cfg.AuthHandler.Logout)

			// Rutas protegidas por JWT (20 min TTL)
			authProtected := auth.Group("")
			authProtected.Use(cfg.AuthMiddleware)
			{
				authProtected.GET("/me", cfg.AuthHandler.Me)
			}
		}

		// Rutas de Verificación KYC y Biometría
		kyc := group.Group("/kyc")
		kyc.Use(cfg.AuthMiddleware)
		{
			kyc.POST("/submit", cfg.KYCHandler.Submit)
			kyc.GET("/status", cfg.KYCHandler.Status)
		}

		// Rutas de Gestión de Vehículos
		vehicles := group.Group("/vehicles")
		vehicles.Use(cfg.AuthMiddleware)
		{
			vehicles.POST("/register", cfg.VehicleHandler.Register)
			vehicles.GET("/status", cfg.VehicleHandler.Status)
		}

		// Webhook / Callback para resoluciones asíncronas de trámite vehicular
		// Protegido contra ejecuciones anónimas mediante HMAC-SHA256 o secreto compartido
		group.POST("/vehicles/:id/resolution", webhookAuth, cfg.VehicleHandler.ProcessResolution)

		// Rutas del Módulo 2: Core Match, Publicación y Búsqueda de Viajes
		if cfg.TripHandler != nil {
			trips := group.Group("/trips")
			{
				// Búsqueda geoespacial y detalle (públicas)
				trips.GET("/search", cfg.TripHandler.SearchTrips)
				trips.GET("/:id", cfg.TripHandler.GetTripByID)

				// Publicación, finalización y cancelación de viajes (requieren JWT de conductor autenticado)
				tripsProtected := trips.Group("")
				tripsProtected.Use(cfg.AuthMiddleware)
				{
					tripsProtected.POST("", cfg.TripHandler.CreateTrip)
					tripsProtected.POST("/:id/complete", cfg.TripHandler.CompleteTrip)
					tripsProtected.DELETE("/:id", cfg.TripHandler.CancelTrip)
				}
			}
		}

		// Rutas del Módulo 3: Reservas, Gestión de Asientos y Pagos en Escrow
		if cfg.BookingHandler != nil {
			// Webhooks firmados para confirmación autoritativa de pagos de la pasarela (HMAC-SHA256)
			group.POST("/payments/webhook", webhookAuth, cfg.BookingHandler.ProcessPaymentWebhook)
			group.POST("/bookings/payment-webhook", webhookAuth, cfg.BookingHandler.ProcessPaymentWebhook)

			bookings := group.Group("/bookings")
			bookings.Use(cfg.AuthMiddleware)
			{
				bookings.POST("", cfg.BookingHandler.CreateBooking)
				bookings.POST("/:id/confirm-payment", cfg.BookingHandler.ConfirmPayment)
				bookings.POST("/:id/cancel", cfg.BookingHandler.CancelBooking)
				bookings.GET("/my-bookings", cfg.BookingHandler.MyBookings)
				bookings.GET("/:id", cfg.BookingHandler.GetBookingByID)
				bookings.GET("/:id/contact-link", cfg.BookingHandler.GetContactLink)
			}
		}

		// Rutas del Módulo 4: Calificaciones y Reputación
		if cfg.ReviewHandler != nil {
			group.GET("/users/:user_id/reviews", cfg.ReviewHandler.GetUserReviews)

			reviewsProtected := group.Group("/reviews")
			reviewsProtected.Use(cfg.AuthMiddleware)
			{
				reviewsProtected.POST("", cfg.ReviewHandler.CreateReview)
			}
		}

		// Rutas del Módulo 4: Gestión y Mediación de Disputas
		if cfg.DisputeHandler != nil {
			disputesProtected := group.Group("/disputes")
			disputesProtected.Use(cfg.AuthMiddleware)
			{
				disputesProtected.POST("", cfg.DisputeHandler.OpenDispute)
				disputesProtected.GET("/:id", cfg.DisputeHandler.GetDisputeByID)
				disputesProtected.POST("/:id/resolve", middleware.RequireRole(domain.RoleAdmin), cfg.DisputeHandler.ResolveDispute)
				disputesProtected.GET("", middleware.RequireRole(domain.RoleAdmin), cfg.DisputeHandler.ListDisputes)
			}
		}

		// Rutas de Observabilidad y Monitoreo (Módulo 5)
		if cfg.HealthHandler != nil {
			group.GET("/health", cfg.HealthHandler.Health)
			group.GET("/ready", cfg.HealthHandler.Ready)
		}
	}

	// Registrar rutas bajo prefijos /v1 y /api/v1 para compatibilidad
	registerV1Routes(r.Group("/v1"))
	registerV1Routes(r.Group("/api/v1"))

	return r
}

// DefaultTrustedOrigins define los orígenes seguros permitidos por defecto para la app móvil y desarrollo local
var DefaultTrustedOrigins = []string{
	"https://localhost",
	"http://localhost",
	"http://localhost:3000",
	"http://localhost:5173",
	"http://localhost:8080",
	"capacitor://localhost",
	"ionic://localhost",
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

	// 1. Orígenes base por defecto (Capacitor móvil y entornos locales de desarrollo)
	for _, origin := range DefaultTrustedOrigins {
		addOrigin(origin)
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
