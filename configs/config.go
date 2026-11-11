package configs

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
)

// Config almacena toda la configuración de la aplicación cargada desde variables de entorno
type Config struct {
	// Servidor HTTP
	Port            string        `env:"PORT" envDefault:"8080"`
	Environment     string        `env:"ENV" envDefault:"development"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	// Base de datos PostgreSQL (Sin credenciales predeterminadas inseguras en código)
	DatabaseURL string `env:"DATABASE_URL"`
	DBMaxConns  int32  `env:"DB_MAX_CONNS" envDefault:"20"`
	DBMinConns  int32  `env:"DB_MIN_CONNS" envDefault:"5"`

	// Seguridad y JWT (Sin secretos predeterminados inseguros en código)
	JWTSecret  string        `env:"JWT_SECRET"`
	JWTIssuer  string        `env:"JWT_ISSUER" envDefault:"thumbi-auth-service"`
	SessionTTL time.Duration `env:"SESSION_TTL" envDefault:"20m"` // RF-04: exactamente 20 minutos

	// Proveedores OAuth
	GoogleClientID string `env:"GOOGLE_CLIENT_ID"`
	AppleClientID  string `env:"APPLE_CLIENT_ID"`

	// Biometría y Almacenamiento
	LivenessThreshold float64 `env:"LIVENESS_THRESHOLD" envDefault:"0.85"`

	// Webhooks de Revisión y Verificación de Vehículos (HMAC Signature)
	WebhookSecret string `env:"WEBHOOK_SECRET"`

	// Ruteo y Cartografía (OSRM)
	OSRMBaseURL string `env:"OSRM_BASE_URL" envDefault:"http://router.project-osrm.org"`

	// Orígenes autorizados para CORS (Web Domains y Móvil)
	AllowedOrigins string `env:"ALLOWED_ORIGINS"`
	AppURL         string `env:"APP_URL"`
}

// LoadConfig carga variables de entorno desde un archivo .env si existe,
// deserializa las variables en Config y ejecuta validaciones de seguridad estrictas.
func LoadConfig() (*Config, error) {
	return LoadConfigWithEnvFile(".env")
}

// LoadConfigWithEnvFile permite especificar la ruta al archivo .env (útil para pruebas y entornos locales).
func LoadConfigWithEnvFile(envFilePath string) (*Config, error) {
	// 1. Cargar archivo .env local si existe (sin sobreescribir variables ya establecidas en el sistema)
	if envFilePath != "" {
		if err := loadDotEnv(envFilePath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("error leyendo archivo .env (%s): %w", envFilePath, err)
		}
	}

	// 2. Parsear variables de entorno en el struct Config
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("error al parsear variables de entorno: %w", err)
	}

	// 3. Ejecutar validaciones estrictas de seguridad según el entorno
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("error en validación de seguridad de configuración: %w", err)
	}

	return cfg, nil
}

// Validate verifica que los secretos críticos cumplan con los estándares criptográficos.
// En entornos diferentes a "development", la aplicación fallará si faltan secretos o si JWT_SECRET
// tiene una longitud inferior a 32 bytes.
func (c *Config) Validate() error {
	isDev := strings.EqualFold(c.Environment, "development")

	if !isDev {
		// Validaciones obligatorias para entornos no-development (staging, production, etc.)
		if c.JWTSecret == "" {
			return errors.New("JWT_SECRET es obligatorio y no puede estar vacío en entornos distintos a 'development'")
		}
		if len(c.JWTSecret) < 32 {
			return fmt.Errorf("JWT_SECRET es demasiado corto: debe tener al menos 32 bytes para seguridad criptográfica (actual: %d bytes)", len(c.JWTSecret))
		}
		if c.DatabaseURL == "" {
			return errors.New("DATABASE_URL es obligatorio y no puede estar vacío en entornos distintos a 'development'")
		}
		if c.WebhookSecret == "" {
			return errors.New("WEBHOOK_SECRET es obligatorio para verificar firmas HMAC en entornos distintos a 'development'")
		}
	} else {
		// Modo desarrollo: se proveen valores de prueba seguros solo si no fueron provistos en .env
		if c.JWTSecret == "" {
			c.JWTSecret = "thumbi-dev-only-mock-jwt-secret-key-32b"
		}
		if c.DatabaseURL == "" {
			c.DatabaseURL = "postgres://postgres:postgres@localhost:5432/thumbi_auth_kyc?sslmode=disable"
		}
		if c.WebhookSecret == "" {
			c.WebhookSecret = "thumbi-dev-only-mock-webhook-secret-key-32b"
		}
	}

	return nil
}

// loadDotEnv parsea pares clave=valor de un archivo .env y los exporta mediante os.Setenv
// solo si la variable no ha sido ya definida en el entorno del proceso.
func loadDotEnv(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Remover comillas envolventes simples o dobles
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// Solo asignar si la variable no está ya definida en el entorno
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, val); err != nil {
				return fmt.Errorf("no se pudo asignar variable %s: %w", key, err)
			}
		}
	}

	return scanner.Err()
}
