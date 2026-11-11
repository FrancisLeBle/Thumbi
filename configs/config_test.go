package configs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_ProductionValidation_FailsOnMissingJWTSecret(t *testing.T) {
	// Limpiar variables de entorno para la prueba
	os.Unsetenv("JWT_SECRET")
	os.Setenv("ENV", "production")
	os.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb")
	os.Setenv("WEBHOOK_SECRET", "this-is-a-valid-webhook-secret-at-least-32-chars")
	defer func() {
		os.Unsetenv("ENV")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("WEBHOOK_SECRET")
	}()

	cfg, err := LoadConfigWithEnvFile("")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "JWT_SECRET")
}

func TestConfig_ProductionValidation_FailsOnShortJWTSecret(t *testing.T) {
	os.Setenv("ENV", "production")
	os.Setenv("JWT_SECRET", "too-short-secret") // < 32 bytes
	os.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb")
	os.Setenv("WEBHOOK_SECRET", "this-is-a-valid-webhook-secret-at-least-32-chars")
	defer func() {
		os.Unsetenv("ENV")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("WEBHOOK_SECRET")
	}()

	cfg, err := LoadConfigWithEnvFile("")
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "debe tener al menos 32 bytes")
}

func TestConfig_ProductionValidation_SuccessWhenValid(t *testing.T) {
	os.Setenv("ENV", "production")
	os.Setenv("JWT_SECRET", "super-secure-production-jwt-key-with-at-least-32-chars")
	os.Setenv("DATABASE_URL", "postgres://user:pass@db:5432/mydb")
	os.Setenv("WEBHOOK_SECRET", "super-secure-webhook-secret-key-with-at-least-32-chars")
	defer func() {
		os.Unsetenv("ENV")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("WEBHOOK_SECRET")
	}()

	cfg, err := LoadConfigWithEnvFile("")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "production", cfg.Environment)
	assert.Equal(t, "super-secure-production-jwt-key-with-at-least-32-chars", cfg.JWTSecret)
}

func TestConfig_DevelopmentMode_AllowsDefaults(t *testing.T) {
	os.Setenv("ENV", "development")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("WEBHOOK_SECRET")
	defer func() {
		os.Unsetenv("ENV")
	}()

	cfg, err := LoadConfigWithEnvFile("")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "development", cfg.Environment)
	assert.NotEmpty(t, cfg.JWTSecret)
	assert.GreaterOrEqual(t, len(cfg.JWTSecret), 32)
	assert.NotEmpty(t, cfg.DatabaseURL)
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

	os.Unsetenv("TEST_CUSTOM_VAR")
	os.Unsetenv("TEST_EXPORTED_VAR")
	defer func() {
		os.Unsetenv("TEST_CUSTOM_VAR")
		os.Unsetenv("TEST_EXPORTED_VAR")
	}()

	err = loadDotEnv(envPath)
	require.NoError(t, err)

	assert.Equal(t, "loaded_from_file", os.Getenv("TEST_CUSTOM_VAR"))
	assert.Equal(t, "unquoted_value", os.Getenv("TEST_EXPORTED_VAR"))
}
