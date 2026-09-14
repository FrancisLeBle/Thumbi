package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/thumbi/auth-kyc-service/configs"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/biometrics"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/notifier"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/oauth"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/payment"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/routing"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
	"github.com/thumbi/auth-kyc-service/internal/worker"
	"github.com/thumbi/auth-kyc-service/pkg/jwt"
)

func main() {
	log.Println("[INFO] Iniciando Thumbi Auth & KYC Service (Módulo 1)...")

	// 1. Cargar Configuración desde Variables de Entorno
	cfg, err := configs.LoadConfig()
	if err != nil {
		log.Fatalf("[FATAL] Error cargando configuración: %v", err)
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
		log.Printf("[WARN] No se pudo conectar a la base de datos (se continuará para arranque local): %v", err)
	} else {
		log.Println("[INFO] Conexión a PostgreSQL establecida con éxito.")
	}

	// 3. Inicialización e Inyección de Dependencias (Wiring)
	// 3.1 Repositorios
	userRepo := postgres.NewUserRepository(dbPool)
	kycRepo := postgres.NewKYCRepository(dbPool)
	vehicleRepo := postgres.NewVehicleRepository(dbPool)
	tripRepo := postgres.NewTripRepository(dbPool)
	bookingRepo := postgres.NewBookingRepository(dbPool)
	escrowRepo := postgres.NewEscrowRepository(dbPool)
	reviewRepo := postgres.NewReviewRepository(dbPool)
	disputeRepo := postgres.NewDisputeRepository(dbPool)
	sessionRepo := postgres.NewMemorySessionRepository(1 * time.Minute)
	defer sessionRepo.Close()

	// 3.2 Seguridad y Tokens JWT (TTL de exactamente 20 minutos según RF-04)
	tokenManager := jwt.NewTokenManager(cfg.JWTSecret, cfg.JWTIssuer)

	// 3.3 Proveedores Externos (OAuth, Biometría, Notificaciones, Ruteo OSRM, Pasarela de Pagos)
	googleProvider := oauth.NewGoogleOAuthProvider(cfg.GoogleClientID)
	appleProvider := oauth.NewAppleOAuthProvider(cfg.AppleClientID)
	socialProviders := []ports.OAuthProvider{googleProvider, appleProvider}

	biometricsProvider := biometrics.NewMockBiometricsProvider()
	notifierService := notifier.NewPushNotifier()
	routingProvider := routing.NewOSRMProvider(cfg.OSRMBaseURL, &http.Client{Timeout: 6 * time.Second})
	paymentGateway := payment.NewMockPaymentGateway()

	// 3.4 Casos de Uso / Servicios del Dominio
	authService := services.NewAuthService(
		userRepo,
		sessionRepo,
		socialProviders,
		tokenManager,
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
	bookingService := services.NewBookingService(
		bookingRepo,
		escrowRepo,
		tripRepo,
		userRepo,
		vehicleRepo,
		paymentGateway,
		notifierService,
	)
	reputationService := services.NewReputationService(
		reviewRepo,
		bookingRepo,
		tripRepo,
		userRepo,
	)
	disputeService := services.NewDisputeService(
		disputeRepo,
		escrowRepo,
		bookingRepo,
		tripRepo,
		paymentGateway,
	)

	// 3.5 Handlers y Middleware HTTP
	authHandler := adapterHttp.NewAuthHandler(authService, userRepo)
	kycHandler := adapterHttp.NewKYCHandler(kycService)
	vehicleHandler := adapterHttp.NewVehicleHandler(vehicleService)
	tripHandler := adapterHttp.NewTripHandler(tripService)
	bookingHandler := adapterHttp.NewBookingHandler(bookingService)
	reviewHandler := adapterHttp.NewReviewHandler(reputationService)
	disputeHandler := adapterHttp.NewDisputeHandler(disputeService)
	healthHandler := adapterHttp.NewHealthHandler(dbPool)
	authMiddleware := middleware.AuthMiddleware(authService)

	// 3.6 Router de Gin
	router := adapterHttp.SetupRouter(adapterHttp.RouterConfig{
		AuthHandler:    authHandler,
		KYCHandler:     kycHandler,
		VehicleHandler: vehicleHandler,
		TripHandler:    tripHandler,
		BookingHandler: bookingHandler,
		ReviewHandler:  reviewHandler,
		DisputeHandler: disputeHandler,
		HealthHandler:  healthHandler,
		AuthMiddleware: authMiddleware,
	})

	// 3.7 Worker en segundo plano para expiración automática de reservas por TTL (15 min)
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	expirationWorker := worker.NewExpirationWorker(bookingService, 1*time.Minute)
	go expirationWorker.Start(workerCtx)

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
