package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type RouterConfig struct {
	AuthHandler    *AuthHandler
	KYCHandler     *KYCHandler
	VehicleHandler *VehicleHandler
	TripHandler    *TripHandler
	BookingHandler *BookingHandler
	ReviewHandler  *ReviewHandler
	DisputeHandler *DisputeHandler
	HealthHandler  *HealthHandler
	AuthMiddleware gin.HandlerFunc
}

// SetupRouter configura el engine de Gin, middlewares globales y agrupamiento de rutas bajo /v1 y /api/v1
func SetupRouter(cfg RouterConfig) *gin.Engine {
	r := gin.New()

	// Middlewares estándar
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(corsMiddleware())

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

		// Webhook / Callback para resoluciones asíncronas de trámite
		group.POST("/vehicles/:id/resolution", cfg.VehicleHandler.ProcessResolution)

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
			bookings := group.Group("/bookings")
			bookings.Use(cfg.AuthMiddleware)
			{
				bookings.POST("", cfg.BookingHandler.CreateBooking)
				bookings.POST("/:id/confirm-payment", cfg.BookingHandler.ConfirmPayment)
				bookings.POST("/:id/cancel", cfg.BookingHandler.CancelBooking)
				bookings.GET("/my-bookings", cfg.BookingHandler.MyBookings)
				bookings.GET("/:id", cfg.BookingHandler.GetBookingByID)
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
				disputesProtected.POST("/:id/resolve", cfg.DisputeHandler.ResolveDispute)
				disputesProtected.GET("", cfg.DisputeHandler.ListDisputes)
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

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, PATCH, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
