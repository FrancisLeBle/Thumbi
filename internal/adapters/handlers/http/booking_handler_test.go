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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// MockBookingService implementa ports.BookingService con testify/mock para pruebas de endpoints HTTP
type MockBookingService struct {
	mock.Mock
}

func (m *MockBookingService) CreateBooking(ctx context.Context, input ports.CreateBookingInput) (*ports.BookingDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ConfirmBookingPayment(ctx context.Context, input ports.ConfirmBookingPaymentInput) (*ports.BookingDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*domain.RefundTransaction, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.RefundTransaction), args.Error(1)
}

func (m *MockBookingService) GetBookingByID(ctx context.Context, bookingID, requestingUserID string) (*ports.BookingDTO, error) {
	args := m.Called(ctx, bookingID, requestingUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ListPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*ports.BookingDTO, error) {
	args := m.Called(ctx, passengerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ListTripBookings(ctx context.Context, tripID, driverID string) ([]*ports.BookingDTO, error) {
	args := m.Called(ctx, tripID, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ProcessExpiredBookings(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

// setupBookingTestRouter inicializa un router Gin aislado con mock middleware de autenticación
func setupBookingTestRouter(mockService *MockBookingService, authUserID string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	bookingHandler := adapterHttp.NewBookingHandler(mockService)

	fakeAuthMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "encabezado de autorización ausente",
				"code":  "UNAUTHORIZED",
			})
			return
		}
		if authUserID != "" {
			c.Set(middleware.CtxUserIDKey, authUserID)
			c.Set(middleware.CtxUserRoleKey, domain.UserRolePassenger)
		}
		c.Next()
	}

	cfg := adapterHttp.RouterConfig{
		BookingHandler: bookingHandler,
		AuthMiddleware: fakeAuthMiddleware,
	}

	return adapterHttp.SetupRouter(cfg)
}

func TestBookingHandler_CreateBooking(t *testing.T) {
	t.Run("falla con 401 si no se provee token de autenticación", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		body := adapterHttp.CreateBookingRequest{
			TripID:         "trip-1",
			SeatsRequested: 2,
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		// No se setea Authorization header

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("falla con 400 Bad Request si el conductor intenta reservar su propio viaje", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "driver-123")

		body := adapterHttp.CreateBookingRequest{
			TripID:         "trip-own",
			SeatsRequested: 1,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         "trip-own",
			PassengerID:    "driver-123",
			SeatsRequested: 1,
		}).Return(nil, domain.ErrCannotBookOwnTrip)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "CANNOT_BOOK_OWN_TRIP", resp["code"])
	})

	t.Run("falla con 409 Conflict si los asientos solicitados superan el cupo disponible", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		body := adapterHttp.CreateBookingRequest{
			TripID:         "trip-full",
			SeatsRequested: 3,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         "trip-full",
			PassengerID:    "passenger-123",
			SeatsRequested: 3,
		}).Return(nil, domain.ErrInsufficientSeats)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INSUFFICIENT_SEATS", resp["code"])
	})

	t.Run("falla con 422 Unprocessable Entity si el pasajero ya tiene una reserva activa en el viaje", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		body := adapterHttp.CreateBookingRequest{
			TripID:         "trip-dup",
			SeatsRequested: 1,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         "trip-dup",
			PassengerID:    "passenger-123",
			SeatsRequested: 1,
		}).Return(nil, domain.ErrOverlappingTripBooking)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "OVERLAPPING_TRIP_BOOKING", resp["code"])
	})

	t.Run("crea exitosamente la reserva con 201 Created y orden de pago en pasarela", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		body := adapterHttp.CreateBookingRequest{
			TripID:         "trip-ok",
			SeatsRequested: 2,
		}
		jsonBytes, _ := json.Marshal(body)

		mockDTO := &ports.BookingDTO{
			ID:          "booking-999",
			TripID:      "trip-ok",
			PassengerID: "passenger-123",
			SeatsBooked: 2,
			UnitPrice:   1500.0,
			TotalPrice:  3000.0,
			Status:      domain.BookingStatusPendingPayment,
			ExpiresAt:   time.Now().Add(15 * time.Minute).Format(time.RFC3339),
			PaymentIntent: &ports.PaymentIntentResult{
				GatewayRef:   "pi_mock_12345",
				ClientSecret: "pi_mock_12345_secret_abc",
				Amount:       3000.0,
				Currency:     "USD",
				Status:       "requires_payment_method",
			},
		}

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         "trip-ok",
			PassengerID:    "passenger-123",
			SeatsRequested: 2,
		}).Return(mockDTO, nil)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "booking")
		bookingData := resp["booking"].(map[string]any)
		assert.Equal(t, "booking-999", bookingData["id"])
		assert.Equal(t, "PENDING_PAYMENT", bookingData["status"])
		assert.Equal(t, float64(3000), bookingData["total_price"])
	})
}

func TestBookingHandler_ConfirmPayment(t *testing.T) {
	t.Run("confirma exitosamente el pago y fondea el Escrow (200 OK)", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		confirmedDTO := &ports.BookingDTO{
			ID:          "booking-999",
			TripID:      "trip-ok",
			PassengerID: "passenger-123",
			SeatsBooked: 2,
			TotalPrice:  3000.0,
			Status:      domain.BookingStatusConfirmed,
		}

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-999",
			PaymentGatewayRef: "pi_mock_12345",
		}).Return(confirmedDTO, nil)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_12345",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings/booking-999/confirm-payment", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		bookingData := resp["booking"].(map[string]any)
		assert.Equal(t, "CONFIRMED", bookingData["status"])
	})

	t.Run("falla con 410 Gone si la reserva ha expirado por TTL", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-expired",
			PaymentGatewayRef: "pi_mock_expired",
		}).Return(nil, domain.ErrBookingExpired)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_expired",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings/booking-expired/confirm-payment", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusGone, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "BOOKING_EXPIRED", resp["code"])
	})

	t.Run("falla con 404 Not Found si la reserva no existe", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-notfound",
			PaymentGatewayRef: "pi_mock_ref",
		}).Return(nil, domain.ErrBookingNotFound)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_ref",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings/booking-notfound/confirm-payment", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestBookingHandler_CancelBooking(t *testing.T) {
	t.Run("cancela la reserva y retorna los datos de liquidación y reembolso (200 OK)", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockRefund := &domain.RefundTransaction{
			ID:                       "ref-001",
			EscrowTransactionID:      "esc-001",
			BookingID:                "booking-123",
			PassengerRefundAmount:    3000.0,
			DriverCompensationAmount: 0.0,
			RefundType:               domain.RefundTypePassengerEarly,
			GatewayRefundRef:         "re_mock_12345",
			ProcessedAt:              time.Now().UTC(),
		}

		mockSvc.On("CancelBooking", mock.Anything, ports.CancelBookingInput{
			BookingID:        "booking-123",
			RequestingUserID: "passenger-123",
			Reason:           "Cambio de itinerario",
		}).Return(mockRefund, nil)

		body := adapterHttp.CancelBookingRequest{
			Reason: "Cambio de itinerario",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings/booking-123/cancel", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "reserva cancelada exitosamente", resp["message"])
		assert.Contains(t, resp, "refund")
	})

	t.Run("falla con 403 Forbidden si el solicitante no es ni pasajero ni conductor de la reserva", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "stranger-999")

		mockSvc.On("CancelBooking", mock.Anything, ports.CancelBookingInput{
			BookingID:        "booking-123",
			RequestingUserID: "stranger-999",
			Reason:           "",
		}).Return(nil, domain.ErrUnauthorized)

		req, _ := http.NewRequest(http.MethodPost, "/api/v1/bookings/booking-123/cancel", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "FORBIDDEN", resp["code"])
	})
}

func TestBookingHandler_GetBookingByID(t *testing.T) {
	t.Run("obtiene exitosamente el detalle de una reserva", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockDTO := &ports.BookingDTO{
			ID:          "booking-123",
			TripID:      "trip-456",
			PassengerID: "passenger-123",
			SeatsBooked: 2,
			TotalPrice:  3000.0,
			Status:      domain.BookingStatusConfirmed,
		}

		mockSvc.On("GetBookingByID", mock.Anything, "booking-123", "passenger-123").Return(mockDTO, nil)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/bookings/booking-123", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "booking")
	})

	t.Run("retorna 404 si la reserva no existe", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockSvc.On("GetBookingByID", mock.Anything, "booking-notfound", "passenger-123").Return(nil, domain.ErrBookingNotFound)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/bookings/booking-notfound", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestBookingHandler_MyBookings(t *testing.T) {
	t.Run("retorna el historial paginado de reservas del pasajero", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, "passenger-123")

		mockList := []*ports.BookingDTO{
			{
				ID:          "booking-1",
				TripID:      "trip-10",
				PassengerID: "passenger-123",
				SeatsBooked: 1,
				TotalPrice:  1500.0,
				Status:      domain.BookingStatusConfirmed,
			},
			{
				ID:          "booking-2",
				TripID:      "trip-20",
				PassengerID: "passenger-123",
				SeatsBooked: 2,
				TotalPrice:  3000.0,
				Status:      domain.BookingStatusPendingPayment,
			},
		}

		mockSvc.On("ListPassengerBookings", mock.Anything, "passenger-123", 20, 0).Return(mockList, nil)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/bookings/my-bookings", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, float64(2), resp["count"])
		assert.Contains(t, resp, "bookings")
	})
}
