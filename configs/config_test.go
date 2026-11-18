package configs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setValidProdEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL", "postgres://produser:StrongRdsSecret#928@db.internal:5432/thumbi?sslmode=require")
	t.Setenv("WEBHOOK_SECRET", "super-secure-webhook-secret-key-with-at-least-32-chars")
	t.Setenv("RATE_LIMIT_STORE", "postgres")
	t.Setenv("GOOGLE_CLIENT_ID", "1234567890-abc.apps.googleusercontent.com")
	t.Setenv("APPLE_CLIENT_ID", "app.thumbi.carpooling.client")
	t.Setenv("OSRM_BASE_URL", DefaultOSRMBaseURL)
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"ENV",
		"DATABASE_URL",
		"DATABASE_URL_FILE",
		"WEBHOOK_SECRET",
		"WEBHOOK_SECRET_FILE",
		"RATE_LIMIT_STORE",
		"GOOGLE_CLIENT_ID",
		"APPLE_CLIENT_ID",
		"OSRM_BASE_URL",
	} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

func TestConfig_FailFastOnMissingEnv(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := LoadConfigWithEnvFile("")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "ENV")
}

func TestConfig_ProductionValidation_FailsOnMissingSecretsOrDeps(t *testing.T) {
	t.Run("falla sin DATABASE_URL", func(t *testing.T) {
		setValidProdEnv(t)
		_ = os.Unsetenv("DATABASE_URL")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("falla con sslmode=disable en production", func(t *testing.T) {
		setValidProdEnv(t)
		t.Setenv("DATABASE_URL", "postgres://produser:StrongRdsSecret#928@db.internal:5432/thumbi?sslmode=disable")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "sslmode=disable")
	})

	t.Run("falla sin WEBHOOK_SECRET", func(t *testing.T) {
		setValidProdEnv(t)
		_ = os.Unsetenv("WEBHOOK_SECRET")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "WEBHOOK_SECRET")
	})

	t.Run("falla con WEBHOOK_SECRET demasiado corto", func(t *testing.T) {
		setValidProdEnv(t)
		t.Setenv("WEBHOOK_SECRET", "short-secret")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "32")
	})

	t.Run("falla con RATE_LIMIT_STORE inválido", func(t *testing.T) {
		setValidProdEnv(t)
		t.Setenv("RATE_LIMIT_STORE", "redis")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "RATE_LIMIT_STORE")
	})

	t.Run("falla sin GOOGLE_CLIENT_ID o APPLE_CLIENT_ID", func(t *testing.T) {
		setValidProdEnv(t)
		_ = os.Unsetenv("GOOGLE_CLIENT_ID")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "GOOGLE_CLIENT_ID")

		setValidProdEnv(t)
		_ = os.Unsetenv("APPLE_CLIENT_ID")

		cfg, err = LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "APPLE_CLIENT_ID")
	})

	t.Run("falla sin OSRM_BASE_URL en production", func(t *testing.T) {
		setValidProdEnv(t)
		_ = os.Unsetenv("OSRM_BASE_URL")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "OSRM_BASE_URL")
	})

	t.Run("entorno desconocido distinto a local/development aplica fail-fast estricto", func(t *testing.T) {
		clearConfigEnv(t)
		t.Setenv("ENV", "staging")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})
}

func TestConfig_ProductionValidation_SuccessWhenValid(t *testing.T) {
	clearConfigEnv(t)
	setValidProdEnv(t)

	cfg, err := LoadConfigWithEnvFile("")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "production", cfg.Environment)
	assert.True(t, cfg.IsProduction())
	assert.False(t, cfg.IsLocalOrDev())
	assert.Equal(t, DefaultOSRMBaseURL, cfg.OSRMBaseURL)
}

func TestConfig_DevelopmentAndLocalMode_RequireDotEnv_NoHardcodedFallbacks(t *testing.T) {
	t.Run("development sin DATABASE_URL ni WEBHOOK_SECRET falla sin usar fallbacks hardcodeados", func(t *testing.T) {
		clearConfigEnv(t)
		t.Setenv("ENV", "development")

		cfg, err := LoadConfigWithEnvFile("")
		assert.Error(t, err)
		assert.Nil(t, cfg)
		assert.Contains(t, err.Error(), "DATABASE_URL")
	})

	t.Run("local y development cargan credenciales desde archivo .env local", func(t *testing.T) {
		for _, envName := range []string{"local", "development"} {
			clearConfigEnv(t)
			tempDir := t.TempDir()
			envPath := filepath.Join(tempDir, ".env")

			content := []byte("ENV=" + envName + "\n" +
				"DATABASE_URL=postgres://devuser:localpass@localhost:5432/thumbi_dev?sslmode=disable\n" +
				"WEBHOOK_SECRET=local-dev-webhook-secret-from-dotenv-file\n")
			require.NoError(t, os.WriteFile(envPath, content, 0600))

			cfg, err := LoadConfigWithEnvFile(envPath)
			require.NoError(t, err)
			assert.NotNil(t, cfg)
			assert.True(t, cfg.IsLocalOrDev())
			assert.Equal(t, envName, cfg.Environment)
			assert.Equal(t, "memory", cfg.RateLimitStore)
			assert.Equal(t, int64(1200), int64(cfg.SessionTTL.Seconds()))
			assert.Equal(t, DefaultOSRMBaseURL, cfg.OSRMBaseURL)
		}
	})
}

func TestConfig_SecretFilesInjection(t *testing.T) {
	clearConfigEnv(t)
	tempDir := t.TempDir()

	dbFile := filepath.Join(tempDir, "db_url")
	webhookFile := filepath.Join(tempDir, "webhook_secret")
	require.NoError(t, os.WriteFile(dbFile, []byte("postgres://prod:SecFile#99@db.internal:5432/thumbi?sslmode=require\n"), 0600))
	require.NoError(t, os.WriteFile(webhookFile, []byte("file-mounted-webhook-secret-at-least-32-chars\n"), 0600))

	t.Setenv("ENV", "production")
	t.Setenv("DATABASE_URL_FILE", dbFile)
	t.Setenv("WEBHOOK_SECRET_FILE", webhookFile)
	t.Setenv("RATE_LIMIT_STORE", "postgres")
	t.Setenv("GOOGLE_CLIENT_ID", "1234567890-abc.apps.googleusercontent.com")
	t.Setenv("APPLE_CLIENT_ID", "app.thumbi.carpooling.client")
	t.Setenv("OSRM_BASE_URL", DefaultOSRMBaseURL)

	cfg, err := LoadConfigWithEnvFile("")
	require.NoError(t, err)
	assert.Equal(t, "file-mounted-webhook-secret-at-least-32-chars", cfg.WebhookSecret)
	assert.Equal(t, "postgres", cfg.RateLimitStore)
}

func TestConfig_LoadDotEnvFile(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".test.env")

	content := []byte(`
# Test comment
TEST_CUSTOM_VAR="loaded_from_file"
TEST_EXPORTED_VAR=unquoted_value
`)
	err := os.WriteFile(envPath, content, 0600)
	require.NoError(t, err)

	_ = os.Unsetenv("TEST_CUSTOM_VAR")
	_ = os.Unsetenv("TEST_EXPORTED_VAR")
	defer func() {
		_ = os.Unsetenv("TEST_CUSTOM_VAR")
		_ = os.Unsetenv("TEST_EXPORTED_VAR")
	}()

	err = loadDotEnv(envPath)
	require.NoError(t, err)

	assert.Equal(t, "loaded_from_file", os.Getenv("TEST_CUSTOM_VAR"))
	assert.Equal(t, "unquoted_value", os.Getenv("TEST_EXPORTED_VAR"))
}

func TestConfig_OSRMValidation_RejectsPlainHTTPPublicServer(t *testing.T) {
	for _, rawURL := range []string{
		"http://router.project-osrm.org",
		"http://routing.example.com",
		"ftp://router.project-osrm.org",
	} {
		err := validateOSRMURL(rawURL, false)
		assert.Error(t, err, "expected error for %s", rawURL)
	}
	assert.Error(t, validateOSRMURL("http://router.project-osrm.org", true))
}

func TestConfig_OSRMValidation_AllowsHTTPSAndInternalSelfHostedOSRM(t *testing.T) {
	for _, rawURL := range []string{
		"https://router.project-osrm.org",
		"https://osrm.thumbi.app",
		"http://osrm:5000",
		"http://10.0.1.25:5000",
		"http://localhost:5000",
		"http://127.0.0.1:5000",
	} {
		assert.NoError(t, validateOSRMURL(rawURL, false), "expected valid URL: %s", rawURL)
	}
}
