package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adapterHttp "github.com/thumbi/backend/internal/adapters/handlers/http"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// MockPricingService implementa adapterHttp.PricingService con testify/mock
type MockPricingService struct {
	mock.Mock
}

func (m *MockPricingService) GetPricingConfig(ctx context.Context) (*ports.PlatformPricingConfigDTO, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PlatformPricingConfigDTO), args.Error(1)
}

func (m *MockPricingService) UpdatePricingConfig(ctx context.Context, input ports.UpdatePlatformPricingConfigInput) (*ports.PlatformPricingConfigDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.PlatformPricingConfigDTO), args.Error(1)
}

func (m *MockPricingService) QuoteBookingPrice(ctx context.Context, tripID uuid.UUID, seatsRequested int) (*ports.BookingPricingQuoteDTO, error) {
	args := m.Called(ctx, tripID, seatsRequested)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingPricingQuoteDTO), args.Error(1)
}

func setupPricingTestRouter(mockSvc *MockPricingService, userID uuid.UUID, role domain.UserRole) *gin.Engine {
	gin.SetMode(gin.TestMode)

	pricingHandler := adapterHttp.NewPricingHandler(mockSvc)
	fakeAuth := func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "no autenticado",
				"code":  "UNAUTHORIZED",
			})
			return
		}
		if userID != uuid.Nil {
			c.Set(middleware.CtxUserIDKey, userID)
			c.Set(middleware.CtxUserRoleKey, role)
		}
		c.Next()
	}

	cfg := baseTestRouterConfig()
	cfg.PricingHandler = pricingHandler
	cfg.AuthMiddleware = fakeAuth

	return mustSetupTestRouter(cfg)
}

func TestPricingHandler(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000009001")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000101")

	t.Run("GET /v1/pricing/config retorna configuración pública activa con 200 OK", func(t *testing.T) {
		mockSvc := new(MockPricingService)
		router := setupPricingTestRouter(mockSvc, uuid.Nil, domain.RolePassenger)

		expectedDTO := &ports.PlatformPricingConfigDTO{
			ID:                             domain.DefaultPlatformPricingConfigID,
			Currency:                       domain.CurrencyARS,
			PassengerServiceFeeBps:         1500,
			PassengerServiceFeePercent:     15.0,
			PassengerFixedFee:              domain.ZeroMoney(domain.CurrencyARS),
			EarlyCancelFeeRetentionBps:     10000,
			EarlyCancelFeeRetentionPercent: 100.0,
			LateCancelWindowHours:          24,
			LateCancelFeeRetentionBps:      10000,
			LateCancelFeeRetentionPercent:  100.0,
			LateCancelPassengerRefundBps:   5000,
			LateCancelPassengerRefundPercent: 50.0,
			LateCancelDriverCompBps:        5000,
			LateCancelDriverCompPercent:    50.0,
			IsActive:                       true,
			UpdatedAt:                      time.Now().UTC().Format(time.RFC3339),
		}

		mockSvc.On("GetPricingConfig", mock.Anything).Return(expectedDTO, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/pricing/config", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(1500), resp["passenger_service_fee_bps"])
		assert.Equal(t, float64(15), resp["passenger_service_fee_percent"])
		mockSvc.AssertExpectations(t)
	})

	t.Run("GET /v1/pricing/quote retorna el desglose transparente del viaje", func(t *testing.T) {
		mockSvc := new(MockPricingService)
		router := setupPricingTestRouter(mockSvc, uuid.Nil, domain.RolePassenger)

		expectedQuote := &ports.BookingPricingQuoteDTO{
			TripID:            tripID,
			SeatsRequested:    2,
			Currency:          domain.CurrencyARS,
			UnitPrice:         domain.MoneyFromCents(150000, domain.CurrencyARS),
			BaseAmount:        domain.MoneyFromCents(300000, domain.CurrencyARS),
			PlatformFee:       domain.MoneyFromCents(45000, domain.CurrencyARS),
			TotalPrice:        domain.MoneyFromCents(345000, domain.CurrencyARS),
			DriverPayout:      domain.MoneyFromCents(300000, domain.CurrencyARS),
			ServiceFeeBps:     1500,
			ServiceFeePercent: 15.0,
			FixedFee:          domain.ZeroMoney(domain.CurrencyARS),
		}

		mockSvc.On("QuoteBookingPrice", mock.Anything, tripID, 2).Return(expectedQuote, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/pricing/quote?trip_id="+tripID.String()+"&seats=2", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(15), resp["service_fee_percent"])
		assert.Equal(t, map[string]any{"amount": float64(45000), "currency": "ARS"}, resp["platform_fee"])
		assert.Equal(t, map[string]any{"amount": float64(345000), "currency": "ARS"}, resp["total_price"])
		mockSvc.AssertExpectations(t)
	})

	t.Run("PUT /v1/admin/pricing-config rechaza 401 sin token y 403 para pasajero no-admin", func(t *testing.T) {
		mockSvc := new(MockPricingService)
		router := setupPricingTestRouter(mockSvc, passengerID, domain.RolePassenger)

		payload := bytes.NewBufferString(`{"passenger_service_fee_percent": 12.0}`)

		reqNoAuth := httptest.NewRequest(http.MethodPut, "/v1/admin/pricing-config", payload)
		wNoAuth := httptest.NewRecorder()
		router.ServeHTTP(wNoAuth, reqNoAuth)
		assert.Equal(t, http.StatusUnauthorized, wNoAuth.Code)

		reqForbidden := httptest.NewRequest(http.MethodPut, "/v1/admin/pricing-config", bytes.NewBufferString(`{"passenger_service_fee_percent": 12.0}`))
		reqForbidden.Header.Set("Authorization", "Bearer user-token")
		reqForbidden.Header.Set("Content-Type", "application/json")
		wForbidden := httptest.NewRecorder()
		router.ServeHTTP(wForbidden, reqForbidden)
		assert.Equal(t, http.StatusForbidden, wForbidden.Code)
	})

	t.Run("PUT /v1/admin/pricing-config permite al admin parametrizar porcentaje y retención operativa", func(t *testing.T) {
		mockSvc := new(MockPricingService)
		router := setupPricingTestRouter(mockSvc, adminID, domain.RoleAdmin)

		expectedDTO := &ports.PlatformPricingConfigDTO{
			ID:                                 domain.DefaultPlatformPricingConfigID,
			Currency:                           domain.CurrencyARS,
			PassengerServiceFeeBps:             1250,
			PassengerServiceFeePercent:         12.5,
			PassengerFixedFee:                  domain.MoneyFromCents(5000, domain.CurrencyARS),
			EarlyCancelFeeRetentionBps:         10000,
			EarlyCancelFeeRetentionPercent:     100.0,
			LateCancelWindowHours:              24,
			LateCancelFeeRetentionBps:          10000,
			LateCancelFeeRetentionPercent:      100.0,
			LateCancelPassengerRefundBps:       4000,
			LateCancelPassengerRefundPercent:   40.0,
			LateCancelDriverCompBps:            4000,
			LateCancelDriverCompPercent:        40.0,
			LateCancelPlatformRetentionBps:     2000,
			LateCancelPlatformRetentionPercent: 20.0,
			IsActive:                           true,
			UpdatedBy:                          &adminID,
			UpdatedAt:                          time.Now().UTC().Format(time.RFC3339),
		}

		mockSvc.On("UpdatePricingConfig", mock.Anything, mock.MatchedBy(func(in ports.UpdatePlatformPricingConfigInput) bool {
			return in.AdminID == adminID &&
				in.PassengerServiceFeeBps != nil && *in.PassengerServiceFeeBps == 1250 &&
				in.PassengerFixedFeeAmount != nil && *in.PassengerFixedFeeAmount == 5000 &&
				in.LateCancelPassengerRefundBps != nil && *in.LateCancelPassengerRefundBps == 4000 &&
				in.LateCancelDriverCompBps != nil && *in.LateCancelDriverCompBps == 4000 &&
				in.LateCancelPlatformRetentionBps != nil && *in.LateCancelPlatformRetentionBps == 2000
		})).Return(expectedDTO, nil)

		body := `{
			"passenger_service_fee_percent": 12.5,
			"passenger_fixed_fee_cents": 5000,
			"late_cancel_passenger_refund_percent": 40,
			"late_cancel_driver_comp_percent": 40,
			"late_cancel_platform_retention_percent": 20
		}`
		req := httptest.NewRequest(http.MethodPut, "/v1/admin/pricing-config", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(1250), resp["passenger_service_fee_bps"])
		assert.Equal(t, float64(12.5), resp["passenger_service_fee_percent"])
		assert.Equal(t, float64(2000), resp["late_cancel_platform_retention_bps"])
		mockSvc.AssertExpectations(t)
	})

	t.Run("PUT /v1/admin/pricing-config retorna 400 INVALID_PRICING_CONFIG ante reglas inválidas del dominio", func(t *testing.T) {
		mockSvc := new(MockPricingService)
		router := setupPricingTestRouter(mockSvc, adminID, domain.RoleAdmin)

		mockSvc.On("UpdatePricingConfig", mock.Anything, mock.AnythingOfType("ports.UpdatePlatformPricingConfigInput")).
			Return(nil, domain.ErrInvalidPricingConfig)

		req := httptest.NewRequest(http.MethodPut, "/v1/admin/pricing-config", bytes.NewBufferString(`{"late_cancel_platform_retention_bps": 9000}`))
		req.Header.Set("Authorization", "Bearer admin-token")
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "INVALID_PRICING_CONFIG", resp["code"])
	})
}
