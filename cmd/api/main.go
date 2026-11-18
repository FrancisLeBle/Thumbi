package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thumbi/backend/configs"
	"github.com/thumbi/backend/internal/adapters/external/biometrics"
	"github.com/thumbi/backend/internal/adapters/external/notifier"
	"github.com/thumbi/backend/internal/adapters/external/oauth"
	"github.com/thumbi/backend/internal/adapters/external/payment"
	"github.com/thumbi/backend/internal/adapters/external/ratelimit"
	"github.com/thumbi/backend/internal/adapters/external/routing"
	adapterHttp "github.com/thumbi/backend/internal/adapters/handlers/http"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
	"github.com/thumbi/backend/internal/worker"
)

func main() {
	log.Println("[INFO] Iniciando Thumbi Backend API...")

	// 1. Cargar Configuración desde Variables de Entorno (.env en local/dev o env/secret files en despliegue)
	cfg, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Error cargando configuración (fail-fast): %v", err)
	}

	if cfg.IsProduction() {
		log.Fatalf("[FATAL] Arranque bloqueado en ENV=production: los adaptadores de pagos, biometría y notificaciones siguen usando implementaciones simuladas (MockPaymentGateway, MockBiometricsProvider, PushNotifier). Reemplácelos por proveedores productivos con credenciales reales antes de habilitar producción.")
	}

	log.Printf("[INFO] Entorno: %s | Puerto: %s | TTL Sesión: %v", cfg.Environment, cfg.Port, cfg.SessionTTL)

	// 2. Conexión a Base de Datos PostgreSQL con pgxpool
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("[FATAL] Error parseando DATABASE_URL: %v", err)
	}

	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MinConns = cfg.DBMinConns
	poolConfig.MaxConnLifetime = 1 * time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute

	dbPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		log.Fatalf("[FATAL] Error conectando a PostgreSQL pool: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		if !cfg.IsLocalOrDev() {
			log.Fatalf("[FATAL] No se pudo verificar conexión a PostgreSQL en entorno %s: %v", cfg.Environment, err)
		}
		log.Printf("[WARN] No se pudo conectar a la base de datos (se continuará solo en entorno local/development): %v", err)
	} else {
		log.Println("[INFO] Conexión a PostgreSQL establecida con éxito.")
	}

	// 3. Inicialización e Inyección de Dependencias (Wiring)
	// 3.1 Unidad de Trabajo y Repositorios
	unitOfWork := postgres.NewUnitOfWork(dbPool)
	userRepo := postgres.NewUserRepository(dbPool)
	kycRepo := postgres.NewKYCRepository(dbPool)
	vehicleRepo := postgres.NewVehicleRepository(dbPool)
	tripRepo := postgres.NewTripRepository(dbPool, unitOfWork)
	bookingRepo := postgres.NewBookingRepository(dbPool, unitOfWork)
	escrowRepo := postgres.NewEscrowRepository(dbPool)
	reviewRepo := postgres.NewReviewRepository(dbPool)
	disputeRepo := postgres.NewDisputeRepository(dbPool)
	identityRepo := postgres.NewIdentityRepository(dbPool)
	sessionRepo := postgres.NewSessionRepository(dbPool)
	pricingConfigRepo := postgres.NewPricingConfigRepository(dbPool)

	// 3.2 Proveedores Externos (OAuth, Biometría, Notificaciones, Ruteo OSRM, Pasarela de Pagos)
	googleProvider := oauth.NewGoogleOAuthProviderWithEnv(cfg.GoogleClientID, cfg.Environment)
	appleProvider := oauth.NewAppleOAuthProviderWithEnv(cfg.AppleClientID, cfg.Environment)
	socialProviders := []ports.OAuthProvider{googleProvider, appleProvider}

	biometricsProvider := biometrics.NewMockBiometricsProvider()
	notifierService := notifier.NewPushNotifier()
	routingProvider := routing.NewOSRMProvider(cfg.OSRMBaseURL, routing.NewSecureHTTPClient(6*time.Second))
	paymentGateway := payment.NewMockPaymentGateway()

	// 3.3 Casos de Uso / Servicios del Dominio (sesiones con tokens opacos aleatorios de 20 min TTL según RF-04)
	authService := services.NewAuthService(
		userRepo,
		sessionRepo,
		socialProviders,
		identityRepo,
	)
	kycService := services.NewKYCService(
		userRepo,
		kycRepo,
		biometricsProvider,
		notifierService,
	)
	vehicleService := services.NewVehicleService(
		userRepo,
		vehicleRepo,
		notifierService,
	)
	pricingCalculator := services.NewPricingCalculator()
	tripService := services.NewTripService(
		tripRepo,
		userRepo,
		vehicleRepo,
		routingProvider,
		pricingCalculator,
		bookingRepo,
		escrowRepo,
	)
	settlementService := services.NewSettlementService(
		escrowRepo,
		paymentGateway,
		unitOfWork,
		pricingConfigRepo,
	)
	bookingService := services.NewBookingService(
		bookingRepo,
		tripRepo,
		userRepo,
		vehicleRepo,
		settlementService,
		notifierService,
		unitOfWork,
		pricingConfigRepo,
	)
	contactLinkService := services.NewContactLinkService(
		bookingRepo,
		tripRepo,
		userRepo,
	)
	bookingExpirationService := services.NewBookingExpirationService(
		bookingRepo,
		tripRepo,
		unitOfWork,
	)
	reputationService := services.NewReputationService(
		reviewRepo,
		bookingRepo,
		tripRepo,
		userRepo,
	)
	disputeService := services.NewDisputeService(
		disputeRepo,
		bookingRepo,
		tripRepo,
		settlementService,
		unitOfWork,
	)
	platformPricingService := services.NewPlatformPricingService(
		pricingConfigRepo,
		tripRepo,
	)

	// 3.5 Handlers y Middleware HTTP
	authHandler := adapterHttp.NewAuthHandler(authService)
	kycHandler := adapterHttp.NewKYCHandler(kycService)
	vehicleHandler := adapterHttp.NewVehicleHandler(vehicleService)
	tripHandler := adapterHttp.NewTripHandler(tripService)
	bookingHandler := adapterHttp.NewBookingHandler(bookingService, contactLinkService)
	reviewHandler := adapterHttp.NewReviewHandler(reputationService)
	disputeHandler := adapterHttp.NewDisputeHandler(disputeService)
	pricingHandler := adapterHttp.NewPricingHandler(platformPricingService)
	healthHandler := adapterHttp.NewHealthHandler(dbPool)
	authMiddleware := middleware.AuthMiddleware(authService)

	// 3.6 Router de Gin con Rate Limiting en memoria o PostgreSQL (cero dependencia de Redis)
	var rateLimiter ports.RateLimiter
	var startRateLimitCleanup func(context.Context)
	if cfg.RateLimitStore == "postgres" {
		pgLimiter := ratelimit.NewPostgresRateLimiter(dbPool)
		rateLimiter = pgLimiter
		startRateLimitCleanup = func(c context.Context) { pgLimiter.StartCleanup(c, 5*time.Minute) }
		log.Println("[INFO] Rate Limiter inicializado con respaldo PostgreSQL (tabla UNLOGGED rate_limits).")
	} else {
		memLimiter := ratelimit.NewMemoryRateLimiter()
		rateLimiter = memLimiter
		startRateLimitCleanup = func(c context.Context) { memLimiter.StartCleanup(c, 1*time.Minute) }
		log.Println("[INFO] Rate Limiter inicializado en memoria local thread-safe (costo cero).")
	}

	allowedOrigins := []string{}
	if cfg.AppURL != "" {
		allowedOrigins = append(allowedOrigins, cfg.AppURL)
	}
	if cfg.AllowedOrigins != "" {
		for _, o := range strings.Split(cfg.AllowedOrigins, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				allowedOrigins = append(allowedOrigins, trimmed)
			}
		}
	}

	router, err := adapterHttp.SetupRouter(adapterHttp.RouterConfig{
		AuthHandler:    authHandler,
		KYCHandler:     kycHandler,
		VehicleHandler: vehicleHandler,
		TripHandler:    tripHandler,
		BookingHandler: bookingHandler,
		ReviewHandler:  reviewHandler,
		DisputeHandler: disputeHandler,
		PricingHandler: pricingHandler,
		HealthHandler:  healthHandler,
		AuthMiddleware: authMiddleware,
		RateLimiter:    rateLimiter,
		WebhookSecret:  cfg.WebhookSecret,
		AllowedOrigins: allowedOrigins,
	})
	if err != nil {
		log.Fatalf("[FATAL] Error inicializando router HTTP: %v", err)
	}

	// 3.7 Workers en segundo plano: expiración de reservas (15 min TTL), limpieza de sesiones y rate limits expirados
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	expirationWorker := worker.NewExpirationWorker(bookingExpirationService, 1*time.Minute)
	go expirationWorker.Start(workerCtx)
	go sessionRepo.StartCleanup(workerCtx, 5*time.Minute)
	go startRateLimitCleanup(workerCtx)

	// 4. Configurar Servidor HTTP
	httpServer := &http.Server{
		Addr:           fmt.Sprintf(":%s", cfg.Port),
		Handler:        router,
		ReadTimeout:    15 * time.Second,
		WriteTimeout:   15 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	// 5. Iniciar Servidor en una Goroutine no bloqueante
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("[INFO] Servidor HTTP escuchando en el puerto %s...", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("fallo al escuchar en el puerto %s: %w", cfg.Port, err)
		}
	}()

	// 6. Configuración de Graceful Shutdown escuchando señales del sistema (SIGINT, SIGTERM)
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("[FATAL] Error crítico del servidor: %v", err)

	case sig := <-shutdownSignal:
		log.Printf("[INFO] Señal de terminación recibida (%s). Iniciando Graceful Shutdown...", sig.String())
		cancelWorker()

		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancelShutdown()

		// Detener el servidor HTTP ordenadamente
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("[ERROR] Error durante el apagado forzado del servidor HTTP: %v", err)
			_ = httpServer.Close()
		}

		log.Println("[INFO] Servidor detenido de manera limpia y segura.")
	}
}
