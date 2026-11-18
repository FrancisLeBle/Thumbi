package http_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	adapterHttp "github.com/thumbi/backend/internal/adapters/handlers/http"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestHandleError_DomainErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		inputErr       error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "ErrUserNotFound maps to 404 USER_NOT_FOUND",
			inputErr:       domain.ErrUserNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "USER_NOT_FOUND",
		},
		{
			name:           "ErrInvalidCredentials maps to 401 INVALID_CREDENTIALS",
			inputErr:       domain.ErrInvalidCredentials,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "INVALID_CREDENTIALS",
		},
		{
			name:           "ErrUnauthorized maps to 403 FORBIDDEN",
			inputErr:       domain.ErrUnauthorized,
			expectedStatus: http.StatusForbidden,
			expectedCode:   "FORBIDDEN",
		},
		{
			name:           "ErrTripNotFound maps to 404 TRIP_NOT_FOUND",
			inputErr:       domain.ErrTripNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "TRIP_NOT_FOUND",
		},
		{
			name:           "ErrBookingNotFound maps to 404 BOOKING_NOT_FOUND",
			inputErr:       domain.ErrBookingNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "BOOKING_NOT_FOUND",
		},
		{
			name:           "ErrMaxPendingBookingsExceeded maps to 429 MAX_PENDING_BOOKINGS_EXCEEDED",
			inputErr:       domain.ErrMaxPendingBookingsExceeded,
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "MAX_PENDING_BOOKINGS_EXCEEDED",
		},
		{
			name:           "ErrRateLimitExceeded maps to 429 RATE_LIMIT_EXCEEDED",
			inputErr:       domain.ErrRateLimitExceeded,
			expectedStatus: http.StatusTooManyRequests,
			expectedCode:   "RATE_LIMIT_EXCEEDED",
		},
		{
			name:           "Wrapped ErrPaymentFailed maps to 502 PAYMENT_GATEWAY_ERROR",
			inputErr:       fmt.Errorf("stripe timeout: %w", domain.ErrPaymentFailed),
			expectedStatus: http.StatusBadGateway,
			expectedCode:   "PAYMENT_GATEWAY_ERROR",
		},
		{
			name:           "ErrLivenessCheckFailed maps to 422 LIVENESS_CHECK_FAILED",
			inputErr:       domain.ErrLivenessCheckFailed,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   "LIVENESS_CHECK_FAILED",
		},
		{
			name:           "ErrInvalidRefundAmount maps to 400 INVALID_REFUND_AMOUNT",
			inputErr:       domain.ErrInvalidRefundAmount,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_REFUND_AMOUNT",
		},
		{
			name:           "ErrConcurrentModification maps to 409 CONCURRENT_MODIFICATION",
			inputErr:       domain.ErrConcurrentModification,
			expectedStatus: http.StatusConflict,
			expectedCode:   "CONCURRENT_MODIFICATION",
		},
		{
			name:           "ErrInvalidPricingConfig maps to 400 INVALID_PRICING_CONFIG",
			inputErr:       domain.ErrInvalidPricingConfig,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "INVALID_PRICING_CONFIG",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(middleware.RequestIDMiddleware())
			r.GET("/test-error", func(c *gin.Context) {
				adapterHttp.HandleError(c, tc.inputErr)
			})

			req, _ := http.NewRequest(http.MethodGet, "/test-error", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedStatus, w.Code)
			var resp map[string]any
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedCode, resp["code"])
			assert.NotEmpty(t, resp["request_id"])
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
		})
	}
}

func TestHandleError_UnexpectedInternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("never exposes raw DB or driver error strings to client and includes request_id", func(t *testing.T) {
		r := gin.New()
		r.Use(middleware.RequestIDMiddleware())
		r.GET("/test-db-error", func(c *gin.Context) {
			// Simular un error interno de base de datos con detalles de tablas, sintaxis y credenciales
			dbErr := errors.New("pq: relation \"users_secret_table\" does not exist at character 42: SELECT id, password_hash FROM users_secret_table WHERE id = 'xyz'")
			adapterHttp.HandleError(c, dbErr)
		})

		req, _ := http.NewRequest(http.MethodGet, "/test-db-error", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)

		// Comprobar que NINGÚN detalle del error de DB se filtra al cliente
		assert.NotContains(t, w.Body.String(), "users_secret_table")
		assert.NotContains(t, w.Body.String(), "password_hash")
		assert.NotContains(t, w.Body.String(), "SELECT")
		assert.NotContains(t, w.Body.String(), "pq:")

		// Comprobar que el mensaje devuelto es genérico, limpio y con request_id
		assert.Equal(t, "INTERNAL_SERVER_ERROR", resp["code"])
		assert.Contains(t, resp["error"], "error interno al procesar su solicitud")
		assert.NotEmpty(t, resp["request_id"])
		assert.Equal(t, resp["request_id"], w.Header().Get("X-Request-ID"))
	})
}
