package http_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
)

func TestCORSMiddleware_TrustedOrigins(t *testing.T) {
	router := adapterHttp.SetupRouter(adapterHttp.RouterConfig{
		AllowedOrigins: []string{"https://thumbi.app"},
	})

	t.Run("permite preflight OPTIONS desde https://localhost y expone cabeceras seguras", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", "https://localhost")
		req.Header.Set("Access-Control-Request-Method", "GET")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "https://localhost", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Origin", w.Header().Get("Vary"))
		assert.NotEqual(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("permite preflight OPTIONS desde capacitor://localhost (app móvil)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", "capacitor://localhost")
		req.Header.Set("Access-Control-Request-Method", "POST")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "capacitor://localhost", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Origin", w.Header().Get("Vary"))
		assert.NotEqual(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("permite origen explícito configurado en AllowedOrigins (https://thumbi.app)", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "https://thumbi.app")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "https://thumbi.app", w.Header().Get("Access-Control-Allow-Origin"))
		assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
		assert.Equal(t, "Origin", w.Header().Get("Vary"))
	})

	t.Run("rechaza preflight OPTIONS de origen no autorizado con 403 Forbidden", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodOptions, "/health", nil)
		req.Header.Set("Origin", "https://malicious-website.com")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("no expone cabeceras CORS en GET de origen no autorizado", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		req.Header.Set("Origin", "https://untrusted-domain.org")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// El navegador bloqueará la lectura del payload al no recibir Access-Control-Allow-Origin
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Credentials"))
	})

	t.Run("solicitud directa sin cabecera Origin se ejecuta normalmente", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/health", nil)
		// Sin cabecera Origin (petición de CLI, app nativa, backend)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
	})
}

func TestCORSMiddleware_AppURLEnv(t *testing.T) {
	os.Setenv("APP_URL", "https://web.thumbi-production.com")
	defer os.Unsetenv("APP_URL")

	router := adapterHttp.SetupRouter(adapterHttp.RouterConfig{})

	req, _ := http.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://web.thumbi-production.com")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://web.thumbi-production.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}
