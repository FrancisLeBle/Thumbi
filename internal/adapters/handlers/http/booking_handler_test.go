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

// MockBookingService implementa adapterHttp.BookingService con testify/mock para pruebas de endpoints HTTP
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

func (m *MockBookingService) CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*ports.RefundDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.RefundDTO), args.Error(1)
}

func (m *MockBookingService) GetBookingByID(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.BookingDTO, error) {
	args := m.Called(ctx, bookingID, requestingUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.BookingDTO), args.Error(1)
}

func (m *MockBookingService) ListPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*ports.BookingDTO, error) {
	args := m.Called(ctx, passengerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.BookingDTO), args.Error(1)
}

// MockContactLinkService implementa adapterHttp.ContactLinkService con testify/mock
type MockContactLinkService struct {
	mock.Mock
}

func (m *MockContactLinkService) GetContactLink(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.ContactLinkDTO, error) {
	args := m.Called(ctx, bookingID, requestingUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ContactLinkDTO), args.Error(1)
}

// setupBookingTestRouter inicializa un router Gin aislado con mock middleware de autenticación
func setupBookingTestRouter(mockService *MockBookingService, authUserID uuid.UUID, contactLinkServices ...adapterHttp.ContactLinkService) *gin.Engine {
	gin.SetMode(gin.TestMode)

	bookingHandler := adapterHttp.NewBookingHandler(mockService, contactLinkServices...)

	fakeAuthMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "encabezado de autorización ausente",
				"code":  "UNAUTHORIZED",
			})
			return
		}
		if authUserID != uuid.Nil {
			c.Set(middleware.CtxUserIDKey, authUserID)
			c.Set(middleware.CtxUserRoleKey, domain.UserRolePassenger)
		}
		c.Next()
	}

	cfg := baseTestRouterConfig()
	cfg.BookingHandler = bookingHandler
	cfg.AuthMiddleware = fakeAuthMiddleware

	return mustSetupTestRouter(cfg)
}

func TestBookingHandler_CreateBooking(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000999")

	t.Run("falla con 401 si no se provee token de autenticación", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 2,
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		// No se setea Authorization header

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("falla con 400 Bad Request si el conductor intenta reservar su propio viaje", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, driverID)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 1,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    driverID,
			SeatsRequested: 1,
		}).Return(nil, domain.ErrCannotBookOwnTrip)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
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
		router := setupBookingTestRouter(mockSvc, passengerID)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 3,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    passengerID,
			SeatsRequested: 3,
		}).Return(nil, domain.ErrInsufficientSeats)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
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
		router := setupBookingTestRouter(mockSvc, passengerID)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 1,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    passengerID,
			SeatsRequested: 1,
		}).Return(nil, domain.ErrOverlappingTripBooking)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
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

	t.Run("falla con 429 Too Many Requests si el pasajero excede el límite de reservas pendientes activas", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 1,
		}
		jsonBytes, _ := json.Marshal(body)

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    passengerID,
			SeatsRequested: 1,
		}).Return(nil, domain.ErrMaxPendingBookingsExceeded)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusTooManyRequests, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "MAX_PENDING_BOOKINGS_EXCEEDED", resp["code"])
	})

	t.Run("crea exitosamente la reserva con 201 Created y orden de pago en pasarela", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockDTO := &ports.BookingDTO{
			ID:          bookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
			TotalPrice:  domain.MoneyFromCents(300000, domain.CurrencyARS),
			Status:      domain.BookingStatusPendingPayment,
			ExpiresAt:   time.Now().Add(15 * time.Minute).Format(time.RFC3339),
			PaymentIntent: &ports.PaymentIntentResult{
				GatewayRef:   "pi_mock_12345",
				ClientSecret: "pi_mock_12345_secret_abc",
				Amount:       domain.MoneyFromCents(300000, domain.CurrencyARS),
				Currency:     domain.CurrencyARS,
				Status:       "requires_payment_method",
			},
		}

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    passengerID,
			SeatsRequested: 2,
		}).Return(mockDTO, nil)

		body := adapterHttp.CreateBookingRequest{
			TripID:         tripID.String(),
			SeatsRequested: 2,
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(jsonBytes))
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
		assert.Equal(t, bookingID.String(), bookingData["id"])
		assert.Equal(t, "PENDING_PAYMENT", bookingData["status"])
		assert.Equal(t, map[string]any{
			"amount":   float64(300000),
			"currency": "ARS",
		}, bookingData["total_price"])
	})

	t.Run("ignora passenger_id inyectado en el cuerpo JSON y usa exclusivamente el del token autenticado", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)
		spoofedVictimID := uuid.MustParse("01960000-0000-7000-8000-000000000991")

		mockDTO := &ports.BookingDTO{
			ID:          bookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 1,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
			TotalPrice:  domain.MoneyFromCents(150000, domain.CurrencyARS),
			Status:      domain.BookingStatusPendingPayment,
			ExpiresAt:   time.Now().Add(15 * time.Minute).Format(time.RFC3339),
		}

		mockSvc.On("CreateBooking", mock.Anything, ports.CreateBookingInput{
			TripID:         tripID,
			PassengerID:    passengerID,
			SeatsRequested: 1,
		}).Return(mockDTO, nil)

		spoofedPayload, _ := json.Marshal(map[string]any{
			"trip_id":         tripID.String(),
			"passenger_id":    spoofedVictimID.String(),
			"seats_requested": 1,
		})

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings", bytes.NewBuffer(spoofedPayload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockSvc.AssertExpectations(t)
	})
}

func TestBookingHandler_ConfirmPayment(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	differentPassengerID := uuid.MustParse("01960000-0000-7000-8000-000000000203")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000999")
	expiredBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000998")
	notFoundBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000997")
	webhookBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000996")

	t.Run("confirma exitosamente el pago y fondea el Escrow (200 OK)", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		confirmedDTO := &ports.BookingDTO{
			ID:          bookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			TotalPrice:  domain.MoneyFromCents(300000, domain.CurrencyARS),
			Status:      domain.BookingStatusConfirmed,
		}

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingID,
			RequestingUserID:  passengerID,
			PaymentGatewayRef: "pi_mock_12345",
			IsWebhook:         false,
		}).Return(confirmedDTO, nil)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_12345",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+bookingID.String()+"/confirm-payment", bytes.NewBuffer(jsonBytes))
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

	t.Run("falla con 403 Forbidden si el usuario autenticado no es el pasajero de la reserva", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, differentPassengerID)

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingID,
			RequestingUserID:  differentPassengerID,
			PaymentGatewayRef: "pi_mock_12345",
			IsWebhook:         false,
		}).Return(nil, domain.ErrUnauthorized)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_12345",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+bookingID.String()+"/confirm-payment", bytes.NewBuffer(jsonBytes))
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

	t.Run("falla con 400 Bad Request si la referencia de pasarela no coincide con el intent de la reserva", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingID,
			RequestingUserID:  passengerID,
			PaymentGatewayRef: "pi_wrong_intent",
			IsWebhook:         false,
		}).Return(nil, domain.ErrInvalidPaymentGatewayRef)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_wrong_intent",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+bookingID.String()+"/confirm-payment", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "INVALID_PAYMENT_INTENT", resp["code"])
	})

	t.Run("falla con 410 Gone si la reserva ha expirado por TTL", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         expiredBookingID,
			RequestingUserID:  passengerID,
			PaymentGatewayRef: "pi_mock_expired",
			IsWebhook:         false,
		}).Return(nil, domain.ErrBookingExpired)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_expired",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+expiredBookingID.String()+"/confirm-payment", bytes.NewBuffer(jsonBytes))
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
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         notFoundBookingID,
			RequestingUserID:  passengerID,
			PaymentGatewayRef: "pi_mock_ref",
			IsWebhook:         false,
		}).Return(nil, domain.ErrBookingNotFound)

		body := adapterHttp.ConfirmPaymentRequest{
			PaymentGatewayRef: "pi_mock_ref",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+notFoundBookingID.String()+"/confirm-payment", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("procesa exitosamente confirmación autoritativa vía webhook firmado de pasarela", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, uuid.Nil)

		confirmedDTO := &ports.BookingDTO{
			ID:          webhookBookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			TotalPrice:  domain.MoneyFromCents(300000, domain.CurrencyARS),
			Status:      domain.BookingStatusConfirmed,
		}

		mockSvc.On("ConfirmBookingPayment", mock.Anything, ports.ConfirmBookingPaymentInput{
			BookingID:         webhookBookingID,
			PaymentGatewayRef: "pi_gateway_webhook_ref",
			IsWebhook:         true,
		}).Return(confirmedDTO, nil)

		webhookBody := adapterHttp.PaymentWebhookRequest{
			Event:             "payment_intent.succeeded",
			BookingID:         webhookBookingID.String(),
			PaymentGatewayRef: "pi_gateway_webhook_ref",
			Amount:            domain.MoneyFromCents(300000, domain.CurrencyARS),
			Currency:          string(domain.CurrencyARS),
		}
		jsonBytes, _ := json.Marshal(webhookBody)

		req, _ := http.NewRequest(http.MethodPost, "/v1/payments/webhook", bytes.NewBuffer(jsonBytes))
		req.Header.Set("Content-Type", "application/json")
		// Encabezado de autenticación de webhook admitido por WebhookAuthMiddleware
		req.Header.Set("X-Webhook-Secret", testWebhookSecret)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Contains(t, resp["message"], "exitosamente")
	})
}

func TestBookingHandler_CancelBooking(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	strangerID := uuid.MustParse("01960000-0000-7000-8000-000000000209")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000901")
	refundID := uuid.MustParse("01960000-0000-7000-8000-000000000701")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000601")

	t.Run("cancela la reserva y retorna los datos de liquidación y reembolso (200 OK)", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockRefund := &ports.RefundDTO{
			ID:                       refundID,
			EscrowTransactionID:      escrowID,
			BookingID:                bookingID,
			PassengerRefundAmount:    domain.MoneyFromCents(300000, domain.CurrencyARS),
			DriverCompensationAmount: domain.MoneyFromCents(0, domain.CurrencyARS),
			RefundType:               domain.RefundTypePassengerEarly,
			GatewayRefundRef:         "re_mock_12345",
			ProcessedAt:              time.Now().UTC().Format(time.RFC3339),
		}

		mockSvc.On("CancelBooking", mock.Anything, ports.CancelBookingInput{
			BookingID:        bookingID,
			RequestingUserID: passengerID,
			Reason:           "Cambio de itinerario",
		}).Return(mockRefund, nil)

		body := adapterHttp.CancelBookingRequest{
			Reason: "Cambio de itinerario",
		}
		jsonBytes, _ := json.Marshal(body)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+bookingID.String()+"/cancel", bytes.NewBuffer(jsonBytes))
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
		router := setupBookingTestRouter(mockSvc, strangerID)

		mockSvc.On("CancelBooking", mock.Anything, ports.CancelBookingInput{
			BookingID:        bookingID,
			RequestingUserID: strangerID,
			Reason:           "",
		}).Return(nil, domain.ErrUnauthorized)

		req, _ := http.NewRequest(http.MethodPost, "/v1/bookings/"+bookingID.String()+"/cancel", bytes.NewBufferString("{}"))
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
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000901")
	notFoundBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000997")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000456")

	t.Run("obtiene exitosamente el detalle de una reserva", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockDTO := &ports.BookingDTO{
			ID:          bookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			TotalPrice:  domain.MoneyFromCents(300000, domain.CurrencyARS),
			Status:      domain.BookingStatusConfirmed,
		}

		mockSvc.On("GetBookingByID", mock.Anything, bookingID, passengerID).Return(mockDTO, nil)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+bookingID.String(), nil)
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
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockSvc.On("GetBookingByID", mock.Anything, notFoundBookingID, passengerID).Return(nil, domain.ErrBookingNotFound)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+notFoundBookingID.String(), nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestBookingHandler_MyBookings(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	bookingID1 := uuid.MustParse("01960000-0000-7000-8000-000000000901")
	bookingID2 := uuid.MustParse("01960000-0000-7000-8000-000000000902")
	tripID10 := uuid.MustParse("01960000-0000-7000-8000-000000000110")
	tripID20 := uuid.MustParse("01960000-0000-7000-8000-000000000120")

	t.Run("retorna el historial paginado de reservas del pasajero", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		router := setupBookingTestRouter(mockSvc, passengerID)

		mockList := []*ports.BookingDTO{
			{
				ID:          bookingID1,
				TripID:      tripID10,
				PassengerID: passengerID,
				SeatsBooked: 1,
				Currency:    domain.CurrencyARS,
				TotalPrice:  domain.MoneyFromCents(150000, domain.CurrencyARS),
				Status:      domain.BookingStatusConfirmed,
			},
			{
				ID:          bookingID2,
				TripID:      tripID20,
				PassengerID: passengerID,
				SeatsBooked: 2,
				Currency:    domain.CurrencyARS,
				TotalPrice:  domain.MoneyFromCents(300000, domain.CurrencyARS),
				Status:      domain.BookingStatusPendingPayment,
			},
		}

		mockSvc.On("ListPassengerBookings", mock.Anything, passengerID, 20, 0).Return(mockList, nil)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/my-bookings", nil)
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

func TestBookingHandler_GetContactLink(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	differentPassengerID := uuid.MustParse("01960000-0000-7000-8000-000000000203")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000901")
	notFoundBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000997")

	t.Run("falla con 401 si no se provee token de autenticación", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		mockContactSvc := new(MockContactLinkService)
		router := setupBookingTestRouter(mockSvc, passengerID, mockContactSvc)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+bookingID.String()+"/contact-link", nil)
		// Sin header Authorization

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("falla con 403 si la reserva no pertenece al usuario solicitante", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		mockContactSvc := new(MockContactLinkService)
		router := setupBookingTestRouter(mockSvc, differentPassengerID, mockContactSvc)

		mockContactSvc.On("GetContactLink", mock.Anything, bookingID, differentPassengerID).
			Return(nil, domain.ErrUnauthorized)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+bookingID.String()+"/contact-link", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "FORBIDDEN", resp["code"])
	})

	t.Run("falla con 404 si la reserva no existe", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		mockContactSvc := new(MockContactLinkService)
		router := setupBookingTestRouter(mockSvc, passengerID, mockContactSvc)

		mockContactSvc.On("GetContactLink", mock.Anything, notFoundBookingID, passengerID).
			Return(nil, domain.ErrBookingNotFound)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+notFoundBookingID.String()+"/contact-link", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "BOOKING_NOT_FOUND", resp["code"])
	})

	t.Run("retorna 200 OK con whatsapp_url y driver_phone para el pasajero autenticado", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		mockContactSvc := new(MockContactLinkService)
		router := setupBookingTestRouter(mockSvc, passengerID, mockContactSvc)

		expectedDTO := &ports.ContactLinkDTO{
			WhatsAppURL: "https://wa.me/5491112345678?text=%C2%A1Hola%21+Te+escribo+por+el+viaje+de+hoy...",
			DriverPhone: "+54 9 11 1234-5678",
		}

		mockContactSvc.On("GetContactLink", mock.Anything, bookingID, passengerID).
			Return(expectedDTO, nil)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+bookingID.String()+"/contact-link", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, expectedDTO.WhatsAppURL, resp["whatsapp_url"])
		assert.Equal(t, expectedDTO.DriverPhone, resp["driver_phone"])
	})

	t.Run("retorna 422 Unprocessable Entity si el conductor no posee teléfono registrado", func(t *testing.T) {
		mockSvc := new(MockBookingService)
		mockContactSvc := new(MockContactLinkService)
		router := setupBookingTestRouter(mockSvc, passengerID, mockContactSvc)

		mockContactSvc.On("GetContactLink", mock.Anything, bookingID, passengerID).
			Return(nil, domain.ErrDriverPhoneRequired)

		req, _ := http.NewRequest(http.MethodGet, "/v1/bookings/"+bookingID.String()+"/contact-link", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "DRIVER_PHONE_REQUIRED", resp["code"])
	})
}

