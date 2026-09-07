package configs

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v10"
)

// Config almacena toda la configuración de la aplicación cargada desde variables de entorno
type Config struct {
	// Servidor HTTP
	Port            string        `env:"PORT" envDefault:"8080"`
	Environment     string        `env:"ENV" envDefault:"development"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	// Base de datos PostgreSQL
	DatabaseURL     string `env:"DATABASE_URL" envDefault:"postgres://postgres:postgres@localhost:5432/thumbi_auth_kyc?sslmode=disable"`
	DBMaxConns     int32  `env:"DB_MAX_CONNS" envDefault:"20"`
	DBMinConns     int32  `env:"DB_MIN_CONNS" envDefault:"5"`

	// Seguridad y JWT
	JWTSecret       string        `env:"JWT_SECRET" envDefault:"thumbi-super-secure-production-ready-jwt-secret-key-32-chars"`
	JWTIssuer       string        `env:"JWT_ISSUER" envDefault:"thumbi-auth-service"`
	SessionTTL      time.Duration `env:"SESSION_TTL" envDefault:"20m"` // RF-04: exactamente 20 minutos

	// Proveedores OAuth
	GoogleClientID  string `env:"GOOGLE_CLIENT_ID" envDefault:"thumbi-google-client-id.apps.googleusercontent.com"`
	AppleClientID   string `env:"APPLE_CLIENT_ID" envDefault:"app.thumbi.carpooling.client"`

	// Biometría y Almacenamiento
	LivenessThreshold float64 `env:"LIVENESS_THRESHOLD" envDefault:"0.85"`

	// Ruteo y Cartografía (OSRM)
	OSRMBaseURL string `env:"OSRM_BASE_URL" envDefault:"http://router.project-osrm.org"`
}

// LoadConfig lee y deserializa las variables de entorno en el struct Config
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("error al parsear variables de entorno: %w", err)
	}

	return cfg, nil
}
