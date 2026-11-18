package http_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adapterHttp "github.com/thumbi/backend/internal/adapters/handlers/http"
)

const testWebhookSecret = "test-only-webhook-secret-key-32-chars!"

func baseTestRouterConfig() adapterHttp.RouterConfig {
	return adapterHttp.RouterConfig{
		AuthHandler:    adapterHttp.NewAuthHandler(nil),
		KYCHandler:     adapterHttp.NewKYCHandler(nil),
		VehicleHandler: adapterHttp.NewVehicleHandler(nil),
		TripHandler:    adapterHttp.NewTripHandler(nil),
		BookingHandler: adapterHttp.NewBookingHandler(nil),
		ReviewHandler:  adapterHttp.NewReviewHandler(nil),
		DisputeHandler: adapterHttp.NewDisputeHandler(nil),
		HealthHandler:  adapterHttp.NewHealthHandler(nil),
		AuthMiddleware: func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED"})
		},
		WebhookSecret: testWebhookSecret,
	}
}

func mustSetupTestRouter(cfg adapterHttp.RouterConfig) *gin.Engine {
	r, err := adapterHttp.SetupRouter(cfg)
	if err != nil {
		panic(err)
	}
	return r
}

func TestSetupRouter_FailFastOnMissingDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("falla inmediatamente con configuración vacía listando dependencias faltantes", func(t *testing.T) {
		r, err := adapterHttp.SetupRouter(adapterHttp.RouterConfig{})
		assert.Nil(t, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "AuthHandler")
		assert.Contains(t, err.Error(), "TripHandler")
		assert.Contains(t, err.Error(), "HealthHandler")
		assert.Contains(t, err.Error(), "AuthMiddleware")
	})

	cases := []struct {
		name    string
		mutate  func(*adapterHttp.RouterConfig)
		missing string
	}{
		{"falta AuthHandler", func(c *adapterHttp.RouterConfig) { c.AuthHandler = nil }, "AuthHandler"},
		{"falta KYCHandler", func(c *adapterHttp.RouterConfig) { c.KYCHandler = nil }, "KYCHandler"},
		{"falta VehicleHandler", func(c *adapterHttp.RouterConfig) { c.VehicleHandler = nil }, "VehicleHandler"},
		{"falta TripHandler", func(c *adapterHttp.RouterConfig) { c.TripHandler = nil }, "TripHandler"},
		{"falta BookingHandler", func(c *adapterHttp.RouterConfig) { c.BookingHandler = nil }, "BookingHandler"},
		{"falta ReviewHandler", func(c *adapterHttp.RouterConfig) { c.ReviewHandler = nil }, "ReviewHandler"},
		{"falta DisputeHandler", func(c *adapterHttp.RouterConfig) { c.DisputeHandler = nil }, "DisputeHandler"},
		{"falta HealthHandler", func(c *adapterHttp.RouterConfig) { c.HealthHandler = nil }, "HealthHandler"},
		{"falta AuthMiddleware", func(c *adapterHttp.RouterConfig) { c.AuthMiddleware = nil }, "AuthMiddleware"},
		{"falta WebhookSecret", func(c *adapterHttp.RouterConfig) { c.WebhookSecret = "" }, "WebhookSecret"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseTestRouterConfig()
			tc.mutate(&cfg)

			r, err := adapterHttp.SetupRouter(cfg)
			assert.Nil(t, r)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.missing)
		})
	}
}

func TestSetupRouter_CanonicalRoutesAndDeprecatedRedirect(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authCalled := 0
	cfg := baseTestRouterConfig()
	cfg.AuthMiddleware = func(c *gin.Context) {
		authCalled++
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": "UNAUTHORIZED"})
	}

	router, err := adapterHttp.SetupRouter(cfg)
	require.NoError(t, err)

	t.Run("prefijo canónico /v1 atiende rutas directamente sin redirección", func(t *testing.T) {
		authCalled = 0
		req := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, 1, authCalled)
		assert.Empty(t, w.Header().Get("Location"))
	})

	t.Run("prefijo legado /api/v1 redirige con 308 preservando método GET y query string sin ejecutar middlewares de negocio", func(t *testing.T) {
		authCalled = 0
		req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/search?orig_lat=-34.6&orig_lon=-58.3&seats=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusPermanentRedirect, w.Code)
		assert.Equal(t, "/v1/trips/search?orig_lat=-34.6&orig_lon=-58.3&seats=2", w.Header().Get("Location"))
		assert.Equal(t, "true", w.Header().Get("Deprecation"))
		assert.Equal(t, `</v1>; rel="successor-version"`, w.Header().Get("Link"))
		assert.Equal(t, 0, authCalled)
	})

	t.Run("prefijo legado /api/v1 redirige peticiones POST y DELETE con 308 para preservar método y payload", func(t *testing.T) {
		authCalled = 0
		body := bytes.NewBufferString(`{"seats_requested":1}`)
		reqPost := httptest.NewRequest(http.MethodPost, "/api/v1/bookings", body)
		wPost := httptest.NewRecorder()
		router.ServeHTTP(wPost, reqPost)

		assert.Equal(t, http.StatusPermanentRedirect, wPost.Code)
		assert.Equal(t, "/v1/bookings", wPost.Header().Get("Location"))
		assert.Equal(t, 0, authCalled, "AuthMiddleware no debe duplicarse en el prefijo legado")

		reqDelete := httptest.NewRequest(http.MethodDelete, "/api/v1/trips/01960000-0000-7000-8000-000000000001", nil)
		wDelete := httptest.NewRecorder()
		router.ServeHTTP(wDelete, reqDelete)

		assert.Equal(t, http.StatusPermanentRedirect, wDelete.Code)
		assert.Equal(t, "/v1/trips/01960000-0000-7000-8000-000000000001", wDelete.Header().Get("Location"))
	})

	t.Run("raíz /api/v1 y barras múltiples se normalizan de forma segura evitando open redirects", func(t *testing.T) {
		reqRoot := httptest.NewRequest(http.MethodGet, "/api/v1", nil)
		wRoot := httptest.NewRecorder()
		router.ServeHTTP(wRoot, reqRoot)

		assert.Equal(t, http.StatusPermanentRedirect, wRoot.Code)
		assert.Equal(t, "/v1", wRoot.Header().Get("Location"))

		reqSlashes := httptest.NewRequest(http.MethodGet, "/api/v1//evil.com/phish", nil)
		wSlashes := httptest.NewRecorder()
		router.ServeHTTP(wSlashes, reqSlashes)

		assert.Equal(t, http.StatusPermanentRedirect, wSlashes.Code)
		assert.Equal(t, "/v1/evil.com/phish", wSlashes.Header().Get("Location"))
	})

	t.Run("preflight CORS OPTIONS sobre /api/v1 sigue respondiendo 204 sin redirección", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/bookings", nil)
		req.Header.Set("Origin", "https://localhost")
		req.Header.Set("Access-Control-Request-Method", "POST")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNoContent, w.Code)
		assert.Equal(t, "https://localhost", w.Header().Get("Access-Control-Allow-Origin"))
	})
}
