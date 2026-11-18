package configs

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/caarlos0/env/v10"
)

// DefaultOSRMBaseURL utiliza HTTPS por defecto para proteger las coordenadas de los usuarios en tránsito
const DefaultOSRMBaseURL = "https://router.project-osrm.org"

// Config almacena toda la configuración de la aplicación cargada desde variables de entorno
type Config struct {
	// Servidor HTTP
	Port            string        `env:"PORT" envDefault:"8080"`
	Environment     string        `env:"ENV"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"10s"`

	// Base de datos PostgreSQL (Sin credenciales predeterminadas inseguras en código)
	DatabaseURL string `env:"DATABASE_URL"`
	DBMaxConns  int32  `env:"DB_MAX_CONNS" envDefault:"20"`
	DBMinConns  int32  `env:"DB_MIN_CONNS" envDefault:"5"`

	// Redis para Rate Limiting y Caché Distribuido (configurado explícitamente en .env / entorno)
	RedisAddr string `env:"REDIS_ADDR"`
	RedisURL  string `env:"REDIS_URL"`

	// Sesiones opacas (TTL estricto de 20 minutos)
	SessionTTL time.Duration `env:"SESSION_TTL" envDefault:"20m"` // RF-04: exactamente 20 minutos

	// Proveedores OAuth
	GoogleClientID string `env:"GOOGLE_CLIENT_ID"`
	AppleClientID  string `env:"APPLE_CLIENT_ID"`

	// Biometría y Almacenamiento
	LivenessThreshold float64 `env:"LIVENESS_THRESHOLD" envDefault:"0.85"`

	// Webhooks de Revisión y Verificación de Vehículos (HMAC Signature)
	WebhookSecret string `env:"WEBHOOK_SECRET"`

	// Ruteo y Cartografía (OSRM): usar HTTPS o una instancia propia en red privada
	OSRMBaseURL string `env:"OSRM_BASE_URL"`

	// Orígenes autorizados para CORS (Web Domains y Móvil)
	AllowedOrigins string `env:"ALLOWED_ORIGINS"`
	AppURL         string `env:"APP_URL"`
}

// IsLocalOrDev indica si la aplicación corre explícitamente en entorno local o de desarrollo.
// Cualquier otro valor (production, staging, vacío, desconocido) aplica validaciones estrictas fail-fast.
func (c *Config) IsLocalOrDev() bool {
	return isLocalOrDevEnv(c.Environment)
}

// IsProduction indica si el entorno configurado corresponde explícitamente a producción.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(strings.TrimSpace(c.Environment), "production")
}

func isLocalOrDevEnv(envName string) bool {
	normalized := strings.ToLower(strings.TrimSpace(envName))
	return normalized == "local" || normalized == "development"
}

// secretFileMappings lista variables que pueden inyectarse en tiempo de ejecución desde archivos montados
// (ej. Docker Secrets en /run/secrets/* o Kubernetes Secret volumes) mediante <VAR>_FILE.
var secretFileMappings = []string{
	"DATABASE_URL",
	"WEBHOOK_SECRET",
	"REDIS_URL",
	"REDIS_ADDR",
	"GOOGLE_CLIENT_ID",
	"APPLE_CLIENT_ID",
}

// LoadConfig carga variables de entorno desde un archivo .env si existe,
// lee secretos montados mediante *_FILE si están definidos, deserializa Config
// y ejecuta validaciones de seguridad fail-fast.
func LoadConfig() (*Config, error) {
	return LoadConfigWithEnvFile(".env")
}

// LoadConfigWithEnvFile permite especificar la ruta al archivo .env (útil para pruebas y entornos locales).
func LoadConfigWithEnvFile(envFilePath string) (*Config, error) {
	// 1. Cargar secretos desde archivos montados (<VAR>_FILE) si están configurados en el entorno
	if err := loadSecretFiles(); err != nil {
		return nil, err
	}

	// 2. Cargar archivo .env local si existe (sin sobreescribir variables ya establecidas en el sistema)
	if envFilePath != "" {
		if err := loadDotEnv(envFilePath); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("error leyendo archivo .env (%s): %w", envFilePath, err)
		}
	}

	// 3. Parsear variables de entorno en el struct Config
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("error al parsear variables de entorno: %w", err)
	}

	// 4. Ejecutar validaciones estrictas fail-fast según el entorno
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("error en validación de seguridad de configuración: %w", err)
	}

	return cfg, nil
}

// MustLoadConfig carga la configuración y entra en pánico inmediatamente si falta algún secreto o parámetro crítico.
func MustLoadConfig() *Config {
	cfg, err := LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("configuración inválida (fail-fast): %v", err))
	}
	return cfg
}

// Validate verifica que las configuraciones críticas cumplan con los estándares de seguridad.
// En ningún entorno existen secretos o cadenas de conexión de respaldo hardcodeadas:
// en 'local'/'development' deben proveerse explícitamente vía .env o variables de entorno;
// en 'production' (o cualquier entorno distinto a 'local'/'development') se aplican controles
// adicionales de entropía, TLS, Redis y Client IDs de OAuth.
func (c *Config) Validate() error {
	envName := strings.TrimSpace(c.Environment)
	if envName == "" {
		return errors.New("ENV es obligatorio y no puede estar vacío (use 'local'/'development' en desarrollo con .env o 'production'/'staging' en despliegue)")
	}

	isLocalOrDev := c.IsLocalOrDev()
	isProd := c.IsProduction()

	// 1. Base de datos: obligatoria en todos los entornos (cero credenciales hardcodeadas en código)
	if strings.TrimSpace(c.DatabaseURL) == "" {
		if isLocalOrDev {
			return errors.New("DATABASE_URL es obligatorio; defínalo en su archivo .env local (ej. postgres://...)")
		}
		return fmt.Errorf("DATABASE_URL es obligatorio y no puede estar vacío en el entorno %q", envName)
	}

	if err := validateDatabaseURL(c.DatabaseURL, isLocalOrDev, isProd); err != nil {
		return err
	}

	// 2. Secreto HMAC para Webhooks: obligatorio en todos los entornos (cero fallbacks en código)
	if strings.TrimSpace(c.WebhookSecret) == "" {
		if isLocalOrDev {
			return errors.New("WEBHOOK_SECRET es obligatorio; defínalo en su archivo .env local")
		}
		return fmt.Errorf("WEBHOOK_SECRET es obligatorio para verificar firmas HMAC en el entorno %q", envName)
	}

	if !isLocalOrDev {
		if len(strings.TrimSpace(c.WebhookSecret)) < 32 {
			return fmt.Errorf("WEBHOOK_SECRET debe tener al menos 32 caracteres en el entorno %q", envName)
		}
		if isKnownPlaceholderSecret(c.WebhookSecret) {
			return fmt.Errorf("WEBHOOK_SECRET contiene un valor de plantilla o desarrollo prohibido en el entorno %q", envName)
		}
	}

	// 3. Redis (Rate Limiting y caché): obligatorio fuera de local/development (en local/dev puede venir de .env)
	if strings.TrimSpace(c.RedisAddr) == "" && strings.TrimSpace(c.RedisURL) == "" {
		if !isLocalOrDev {
			return fmt.Errorf("REDIS_ADDR o REDIS_URL es obligatorio en el entorno %q", envName)
		}
		return errors.New("REDIS_ADDR o REDIS_URL es obligatorio; defínalo en su archivo .env local")
	}
	if strings.TrimSpace(c.RedisAddr) == "" && strings.TrimSpace(c.RedisURL) != "" {
		c.RedisAddr = c.RedisURL
	}

	// 4. Proveedores OAuth: obligatorios fuera de local/development para impedir omitir validación de audiencia (aud)
	if !isLocalOrDev {
		if strings.TrimSpace(c.GoogleClientID) == "" || isKnownPlaceholderSecret(c.GoogleClientID) {
			return fmt.Errorf("GOOGLE_CLIENT_ID es obligatorio y válido en el entorno %q", envName)
		}
		if strings.TrimSpace(c.AppleClientID) == "" {
			return fmt.Errorf("APPLE_CLIENT_ID es obligatorio en el entorno %q", envName)
		}
	}

	// 5. OSRM Base URL: en local/development permite default HTTPS público si no se especifica en .env;
	// fuera de local/development exige configuración explícita y HTTPS (o red privada interna).
	if strings.TrimSpace(c.OSRMBaseURL) == "" {
		if !isLocalOrDev {
			return fmt.Errorf("OSRM_BASE_URL es obligatorio en el entorno %q", envName)
		}
		c.OSRMBaseURL = DefaultOSRMBaseURL
	}

	if err := validateOSRMURL(c.OSRMBaseURL, isLocalOrDev); err != nil {
		return err
	}

	return nil
}

func validateDatabaseURL(rawURL string, isLocalOrDev, isProd bool) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("DATABASE_URL tiene un formato inválido")
	}

	if !isLocalOrDev {
		if parsed.User != nil {
			pwd, _ := parsed.User.Password()
			if isKnownPlaceholderSecret(pwd) {
				return errors.New("DATABASE_URL utiliza una contraseña de desarrollo o plantilla prohibida fuera de entornos locales")
			}
		}
		if isProd {
			sslMode := strings.ToLower(parsed.Query().Get("sslmode"))
			if sslMode == "disable" {
				return errors.New("DATABASE_URL no puede usar sslmode=disable en producción; habilite TLS (sslmode=require o verify-full)")
			}
		}
	}
	return nil
}

func isKnownPlaceholderSecret(val string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(val))
	switch trimmed {
	case "",
		"thumbi_secret",
		"postgres",
		"password",
		"password123",
		"choose_a_strong_database_password_here",
		"thumbi-dev-only-mock-webhook-secret-key-32b",
		"thumbi-vehicle-review-webhook-secret-key-32-chars",
		"generate_a_cryptographically_secure_webhook_secret_at_least_32_chars",
		"your_google_oauth_client_id.apps.googleusercontent.com",
		"my_gemini_api_key",
		"my_app_url":
		return true
	default:
		return false
	}
}

func loadSecretFiles() error {
	for _, key := range secretFileMappings {
		if existing := strings.TrimSpace(os.Getenv(key)); existing != "" {
			continue
		}
		filePath := strings.TrimSpace(os.Getenv(key + "_FILE"))
		if filePath == "" {
			continue
		}
		raw, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error leyendo archivo de secreto %s_FILE (%s): %w", key, filePath, err)
		}
		val := strings.TrimSpace(string(raw))
		if val == "" {
			return fmt.Errorf("el archivo de secreto %s_FILE (%s) está vacío", key, filePath)
		}
		if err := os.Setenv(key, val); err != nil {
			return fmt.Errorf("no se pudo exportar secreto %s desde archivo: %w", key, err)
		}
	}
	return nil
}

// validateOSRMURL impide llamadas en texto plano (http://) hacia servidores públicos por Internet.
// Exige HTTPS para endpoints externos y solo permite HTTP si apunta a un host interno/privado autohospedado.
func validateOSRMURL(rawURL string, isLocalOrDev bool) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("OSRM_BASE_URL inválida: %q", rawURL)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())

	if scheme == "https" {
		return nil
	}

	if scheme != "http" {
		return fmt.Errorf("OSRM_BASE_URL debe usar esquema https (o http únicamente para una instancia interna autohospedada): esquema %q no soportado", parsed.Scheme)
	}

	// Nunca permitir HTTP plano hacia el servidor público demo de OSRM
	if host == "router.project-osrm.org" {
		return errors.New("OSRM_BASE_URL no puede usar http:// con el servidor público router.project-osrm.org; use https:// o una instancia propia de OSRM")
	}

	// En entornos productivos/staging, HTTP plano solo es aceptable para servicios internos en red privada
	if !isLocalOrDev && !isInternalHost(host) {
		return fmt.Errorf("OSRM_BASE_URL (%q) debe usar https:// en entornos no locales para evitar exponer coordenadas de pasajeros en texto plano", rawURL)
	}

	if isLocalOrDev && !isInternalHost(host) {
		return fmt.Errorf("OSRM_BASE_URL (%q) debe usar https:// cuando apunta a un host público externo", rawURL)
	}

	return nil
}

// isInternalHost determina si el hostname corresponde a localhost, una IP privada/loopback o un servicio interno sin dominio público (ej. "osrm").
func isInternalHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".svc.cluster.local") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	// Nombre de servicio Docker/Kubernetes de una sola etiqueta (ej. "osrm", "osrm-routing")
	return !strings.Contains(host, ".")
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
