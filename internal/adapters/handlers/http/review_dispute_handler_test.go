package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// MockTripServiceForHandlers mock de ports.TripService para handlers
type MockTripServiceForHandlers struct {
	mock.Mock
}

func (m *MockTripServiceForHandlers) CreateTrip(ctx context.Context, input ports.CreateTripInput) (*domain.Trip, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Trip), args.Error(1)
}

func (m *MockTripServiceForHandlers) SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.TripSearchResult), args.Error(1)
}

func (m *MockTripServiceForHandlers) GetTripByID(ctx context.Context, tripID string) (*ports.TripDetailDTO, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDetailDTO), args.Error(1)
}

func (m *MockTripServiceForHandlers) CancelTrip(ctx context.Context, tripID, driverID string) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

func (m *MockTripServiceForHandlers) CompleteTrip(ctx context.Context, tripID, driverID string) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

// MockReputationServiceForHandlers mock de ports.ReputationService
type MockReputationServiceForHandlers struct {
	mock.Mock
}

func (m *MockReputationServiceForHandlers) CreateReview(ctx context.Context, input ports.CreateReviewInput) (*ports.ReviewDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.ReviewDTO), args.Error(1)
}

func (m *MockReputationServiceForHandlers) GetUserReviews(ctx context.Context, userID string, limit, offset int) (*ports.UserReputationDTO, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UserReputationDTO), args.Error(1)
}

// MockDisputeServiceForHandlers mock de ports.DisputeService
type MockDisputeServiceForHandlers struct {
	mock.Mock
}

func (m *MockDisputeServiceForHandlers) OpenDispute(ctx context.Context, input ports.OpenDisputeInput) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeServiceForHandlers) ResolveDispute(ctx context.Context, input ports.ResolveDisputeInput) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeServiceForHandlers) GetDisputeByID(ctx context.Context, disputeID string) (*ports.DisputeDTO, error) {
	args := m.Called(ctx, disputeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.DisputeDTO), args.Error(1)
}

func (m *MockDisputeServiceForHandlers) ListDisputes(ctx context.Context, limit, offset int) ([]*ports.DisputeDTO, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*ports.DisputeDTO), args.Error(1)
}

func setupModule4Router(
	mockTrip *MockTripServiceForHandlers,
	mockRep *MockReputationServiceForHandlers,
	mockDisp *MockDisputeServiceForHandlers,
	authUserID string,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockAuthMiddleware := func(c *gin.Context) {
		if authUserID != "" {
			c.Set(middleware.UserIDKey, authUserID)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
		}
	}

	var tripHandler *adapterHttp.TripHandler
	if mockTrip != nil {
		tripHandler = adapterHttp.NewTripHandler(mockTrip)
	}

	var reviewHandler *adapterHttp.ReviewHandler
	if mockRep != nil {
		reviewHandler = adapterHttp.NewReviewHandler(mockRep)
	}

	var disputeHandler *adapterHttp.DisputeHandler
	if mockDisp != nil {
		disputeHandler = adapterHttp.NewDisputeHandler(mockDisp)
	}

	return adapterHttp.SetupRouter(adapterHttp.RouterConfig{
		TripHandler:    tripHandler,
		ReviewHandler:  reviewHandler,
		DisputeHandler: disputeHandler,
		AuthMiddleware: mockAuthMiddleware,
	})
}

// ---------------------------------------------------------------------------
// Tests de CompleteTrip
// ---------------------------------------------------------------------------

func TestTripHandler_CompleteTrip(t *testing.T) {
	t.Run("concluye viaje exitosamente (200 OK)", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, "driver-1")

		mockTrip.On("CompleteTrip", mock.Anything, "trip-100", "driver-1").Return(nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips/trip-100/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "COMPLETED")
		mockTrip.AssertExpectations(t)
	})

	t.Run("retorna 403 Forbidden si no es el conductor titular", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, "other-user")

		mockTrip.On("CompleteTrip", mock.Anything, "trip-100", "other-user").Return(domain.ErrUnauthorized)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips/trip-100/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockTrip.AssertExpectations(t)
	})

	t.Run("retorna 404 Not Found si el viaje no existe", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, "driver-1")

		mockTrip.On("CompleteTrip", mock.Anything, "non-existent", "driver-1").Return(domain.ErrTripNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips/non-existent/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockTrip.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Tests de ReviewHandler
// ---------------------------------------------------------------------------

func TestReviewHandler_CreateReview(t *testing.T) {
	t.Run("crea calificación exitosamente (201 Created)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, "passenger-1")

		expectedDTO := &ports.ReviewDTO{
			ID:         "rev-1",
			TripID:     "trip-1",
			BookingID:  "booking-1",
			ReviewerID: "passenger-1",
			RevieweeID: "driver-1",
			Rating:     5,
			Comment:    "Excelente servicio",
			CreatedAt:  "2026-09-07T12:00:00Z",
		}

		mockRep.On("CreateReview", mock.Anything, ports.CreateReviewInput{
			BookingID:  "booking-1",
			ReviewerID: "passenger-1",
			Rating:     5,
			Comment:    "Excelente servicio",
		}).Return(expectedDTO, nil)

		payload := map[string]any{
			"booking_id": "booking-1",
			"rating":     5,
			"comment":    "Excelente servicio",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var res ports.ReviewDTO
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, "rev-1", res.ID)
		assert.Equal(t, 5, res.Rating)
		mockRep.AssertExpectations(t)
	})

	t.Run("retorna 400 Bad Request si el viaje no está completado (RF-06)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, "passenger-1")

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrTripNotCompletedForReview)

		payload := map[string]any{
			"booking_id": "booking-1",
			"rating":     4,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "TRIP_NOT_COMPLETED_FOR_REVIEW")
	})

	t.Run("retorna 409 Conflict si ya existe reseña previa (RF-09)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, "passenger-1")

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrDuplicateReview)

		payload := map[string]any{
			"booking_id": "booking-1",
			"rating":     5,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "DUPLICATE_REVIEW")
	})

	t.Run("retorna 422 Unprocessable Entity si intenta autocalificarse (RF-10)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, "user-1")

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrReviewSelfNotAllowed)

		payload := map[string]any{
			"booking_id": "booking-1",
			"rating":     5,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "SELF_REVIEW_NOT_ALLOWED")
	})
}

func TestReviewHandler_GetUserReviews(t *testing.T) {
	t.Run("obtiene perfil reputacional del usuario", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, "")

		repDTO := &ports.UserReputationDTO{
			UserID:      "driver-1",
			RatingAvg:   4.90,
			RatingCount: 20,
			Reviews: []*ports.ReviewDTO{
				{
					ID:         "rev-1",
					TripID:     "t-1",
					BookingID:  "b-1",
					ReviewerID: "p-1",
					RevieweeID: "driver-1",
					Rating:     5,
					Comment:    "Genial",
					CreatedAt:  "2026-09-07T10:00:00Z",
				},
			},
		}

		mockRep.On("GetUserReviews", mock.Anything, "driver-1", 10, 0).Return(repDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/users/driver-1/reviews?limit=10&offset=0", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res ports.UserReputationDTO
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, "driver-1", res.UserID)
		assert.Equal(t, 4.90, res.RatingAvg)
		assert.Equal(t, 20, res.RatingCount)
		assert.Len(t, res.Reviews, 1)
		mockRep.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Tests de DisputeHandler
// ---------------------------------------------------------------------------

func TestDisputeHandler_OpenDispute(t *testing.T) {
	t.Run("abre disputa formal exitosamente (201 Created)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "passenger-1")

		expectedDTO := &ports.DisputeDTO{
			ID:                  "disp-1",
			EscrowTransactionID: "escrow-100",
			BookingID:           "booking-100",
			TripID:              "trip-100",
			ReporterID:          "passenger-1",
			DefendantID:         "driver-1",
			Reason:              domain.DisputeReasonNoShow,
			Description:         "El conductor no llegó",
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           "2026-09-07T12:00:00Z",
			UpdatedAt:           "2026-09-07T12:00:00Z",
		}

		mockDisp.On("OpenDispute", mock.Anything, ports.OpenDisputeInput{
			BookingID:    "booking-100",
			ReporterID:   "passenger-1",
			Reason:       domain.DisputeReasonNoShow,
			Description:  "El conductor no llegó",
			EvidenceURLs: []string{"https://img.com/1.png"},
		}).Return(expectedDTO, nil)

		payload := map[string]any{
			"booking_id":    "booking-100",
			"reason":        "NO_SHOW",
			"description":   "El conductor no llegó",
			"evidence_urls": []string{"https://img.com/1.png"},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var res ports.DisputeDTO
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, "disp-1", res.ID)
		assert.Equal(t, domain.DisputeStatusOpened, res.Status)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 409 Conflict si ya existe disputa previa (RF-17)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "passenger-1")

		mockDisp.On("OpenDispute", mock.Anything, mock.Anything).Return(nil, domain.ErrDisputeAlreadyExists)

		payload := map[string]any{
			"booking_id":  "booking-100",
			"reason":      "NO_SHOW",
			"description": "Reclamo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "DISPUTE_ALREADY_EXISTS")
	})

	t.Run("retorna 400 Bad Request si el escrow ya fue liquidado (RF-17)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "passenger-1")

		mockDisp.On("OpenDispute", mock.Anything, mock.Anything).Return(nil, domain.ErrEscrowAlreadySettled)

		payload := map[string]any{
			"booking_id":  "booking-100",
			"reason":      "NO_SHOW",
			"description": "Reclamo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "ESCROW_ALREADY_SETTLED")
	})
}

func TestDisputeHandler_ResolveDispute(t *testing.T) {
	t.Run("resuelve disputa a favor del pasajero (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "admin-user")

		resolvedDTO := &ports.DisputeDTO{
			ID:     "disp-1",
			Status: domain.DisputeStatusResolvedPassengerRefund,
		}

		mockDisp.On("ResolveDispute", mock.Anything, ports.ResolveDisputeInput{
			DisputeID:  "disp-1",
			AdminID:    "admin-user",
			Resolution: domain.DisputeStatusResolvedPassengerRefund,
			AdminNotes: "Aprobado reembolso completo",
		}).Return(resolvedDTO, nil)

		payload := map[string]any{
			"resolution":  "RESOLVED_PASSENGER_REFUND",
			"admin_notes": "Aprobado reembolso completo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/disputes/disp-1/resolve", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})
}

func TestDisputeHandler_GetDisputeByID(t *testing.T) {
	t.Run("consulta detalle de disputa existente", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "user-1")

		disputeDTO := &ports.DisputeDTO{
			ID:     "disp-500",
			Status: domain.DisputeStatusInReview,
		}

		mockDisp.On("GetDisputeByID", mock.Anything, "disp-500").Return(disputeDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/disputes/disp-500", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 404 Not Found si la disputa no existe", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, "user-1")

		mockDisp.On("GetDisputeByID", mock.Anything, "non-existent").Return(nil, domain.ErrDisputeNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/disputes/non-existent", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockDisp.AssertExpectations(t)
	})
}
