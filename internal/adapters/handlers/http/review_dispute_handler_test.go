package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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

// MockTripServiceForHandlers mock de adapterHttp.TripService para handlers
type MockTripServiceForHandlers struct {
	mock.Mock
}

func (m *MockTripServiceForHandlers) CreateTrip(ctx context.Context, input ports.CreateTripInput) (*ports.TripDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDTO), args.Error(1)
}

func (m *MockTripServiceForHandlers) SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.TripSearchResult), args.Error(1)
}

func (m *MockTripServiceForHandlers) GetTripByID(ctx context.Context, tripID uuid.UUID) (*ports.TripDetailDTO, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDetailDTO), args.Error(1)
}

func (m *MockTripServiceForHandlers) CancelTrip(ctx context.Context, tripID, driverID uuid.UUID) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

func (m *MockTripServiceForHandlers) CompleteTrip(ctx context.Context, tripID, driverID uuid.UUID) (*ports.TripDTO, error) {
	args := m.Called(ctx, tripID, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDTO), args.Error(1)
}

// MockReputationServiceForHandlers mock de adapterHttp.ReputationService
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

func (m *MockReputationServiceForHandlers) GetUserReviews(ctx context.Context, userID uuid.UUID, limit, offset int) (*ports.UserReputationDTO, error) {
	args := m.Called(ctx, userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.UserReputationDTO), args.Error(1)
}

// MockDisputeServiceForHandlers mock de adapterHttp.DisputeService
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

func (m *MockDisputeServiceForHandlers) GetDisputeByID(ctx context.Context, disputeID uuid.UUID) (*ports.DisputeDTO, error) {
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
	authUserID uuid.UUID,
	authRole domain.UserRole,
) *gin.Engine {
	gin.SetMode(gin.TestMode)

	mockAuthMiddleware := func(c *gin.Context) {
		if authUserID != uuid.Nil {
			c.Set(middleware.UserIDKey, authUserID)
			c.Set(middleware.CtxUserIDKey, authUserID)

			role := authRole
			if role == "" {
				role = domain.RolePassenger
			}
			if customRole := c.GetHeader("X-Test-Role"); customRole != "" {
				role = domain.UserRole(customRole)
			}

			c.Set(middleware.UserRoleKey, role)
			c.Set(middleware.CtxUserRoleKey, role)
			c.Next()
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
		}
	}

	cfg := baseTestRouterConfig()
	cfg.AuthMiddleware = mockAuthMiddleware

	if mockTrip != nil {
		cfg.TripHandler = adapterHttp.NewTripHandler(mockTrip)
	}
	if mockRep != nil {
		cfg.ReviewHandler = adapterHttp.NewReviewHandler(mockRep)
	}
	if mockDisp != nil {
		cfg.DisputeHandler = adapterHttp.NewDisputeHandler(mockDisp)
	}

	return mustSetupTestRouter(cfg)
}

// ---------------------------------------------------------------------------
// Tests de CompleteTrip
// ---------------------------------------------------------------------------

func TestTripHandler_CompleteTrip(t *testing.T) {
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	otherUserID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	tripID100 := uuid.MustParse("01960000-0000-7000-8000-000000000100")
	nonExistentTripID := uuid.MustParse("01960000-0000-7000-8000-000000000199")

	t.Run("concluye viaje exitosamente (200 OK)", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, driverID, domain.RoleDriver)

		completedTrip := &ports.TripDTO{
			ID:       tripID100,
			DriverID: driverID,
			Status:   domain.TripStatusCompleted,
		}
		mockTrip.On("CompleteTrip", mock.Anything, tripID100, driverID).Return(completedTrip, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips/"+tripID100.String()+"/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "COMPLETED")
		mockTrip.AssertExpectations(t)
	})

	t.Run("retorna 403 Forbidden si no es el conductor titular", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, otherUserID, domain.RolePassenger)

		mockTrip.On("CompleteTrip", mock.Anything, tripID100, otherUserID).Return(nil, domain.ErrUnauthorized)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips/"+tripID100.String()+"/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockTrip.AssertExpectations(t)
	})

	t.Run("retorna 404 Not Found si el viaje no existe", func(t *testing.T) {
		mockTrip := new(MockTripServiceForHandlers)
		router := setupModule4Router(mockTrip, nil, nil, driverID, domain.RoleDriver)

		mockTrip.On("CompleteTrip", mock.Anything, nonExistentTripID, driverID).Return(nil, domain.ErrTripNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips/"+nonExistentTripID.String()+"/complete", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockTrip.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Tests de ReviewHandler
// ---------------------------------------------------------------------------

func TestReviewHandler_CreateReview(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	userID1 := uuid.MustParse("01960000-0000-7000-8000-000000000203")
	revID := uuid.MustParse("01960000-0000-7000-8000-000000000801")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000901")

	t.Run("crea calificación exitosamente (201 Created)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, passengerID, domain.RolePassenger)

		expectedDTO := &ports.ReviewDTO{
			ID:         revID,
			TripID:     tripID,
			BookingID:  bookingID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     5,
			Comment:    "Excelente servicio",
			CreatedAt:  "2026-09-07T12:00:00Z",
		}

		mockRep.On("CreateReview", mock.Anything, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: passengerID,
			Rating:     5,
			Comment:    "Excelente servicio",
		}).Return(expectedDTO, nil)

		payload := map[string]any{
			"booking_id": bookingID.String(),
			"rating":     5,
			"comment":    "Excelente servicio",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var res adapterHttp.ReviewResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, revID, res.ID)
		assert.Equal(t, 5, res.Rating)
		mockRep.AssertExpectations(t)
	})

	t.Run("ignora reviewer_id enviado en el cuerpo JSON y utiliza el del token autenticado", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, passengerID, domain.RolePassenger)
		spoofedReviewerID := uuid.MustParse("01960000-0000-7000-8000-000000000992")

		expectedDTO := &ports.ReviewDTO{
			ID:         revID,
			TripID:     tripID,
			BookingID:  bookingID,
			ReviewerID: passengerID,
			RevieweeID: driverID,
			Rating:     5,
			Comment:    "Excelente",
			CreatedAt:  "2026-09-07T12:00:00Z",
		}

		mockRep.On("CreateReview", mock.Anything, ports.CreateReviewInput{
			BookingID:  bookingID,
			ReviewerID: passengerID,
			Rating:     5,
			Comment:    "Excelente",
		}).Return(expectedDTO, nil)

		payload := map[string]any{
			"booking_id":  bookingID.String(),
			"reviewer_id": spoofedReviewerID.String(),
			"rating":      5,
			"comment":     "Excelente",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockRep.AssertExpectations(t)
	})

	t.Run("retorna 400 Bad Request si el viaje no está completado (RF-06)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, passengerID, domain.RolePassenger)

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrTripNotCompletedForReview)

		payload := map[string]any{
			"booking_id": bookingID.String(),
			"rating":     4,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "TRIP_NOT_COMPLETED_FOR_REVIEW")
	})

	t.Run("retorna 409 Conflict si ya existe reseña previa (RF-09)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, passengerID, domain.RolePassenger)

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrDuplicateReview)

		payload := map[string]any{
			"booking_id": bookingID.String(),
			"rating":     5,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "DUPLICATE_REVIEW")
	})

	t.Run("retorna 422 Unprocessable Entity si intenta autocalificarse (RF-10)", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, userID1, domain.RolePassenger)

		mockRep.On("CreateReview", mock.Anything, mock.Anything).Return(nil, domain.ErrReviewSelfNotAllowed)

		payload := map[string]any{
			"booking_id": bookingID.String(),
			"rating":     5,
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/reviews", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "SELF_REVIEW_NOT_ALLOWED")
	})
}

func TestReviewHandler_GetUserReviews(t *testing.T) {
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	revID := uuid.MustParse("01960000-0000-7000-8000-000000000801")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000901")

	t.Run("obtiene perfil reputacional del usuario", func(t *testing.T) {
		mockRep := new(MockReputationServiceForHandlers)
		router := setupModule4Router(nil, mockRep, nil, uuid.Nil, "")

		repDTO := &ports.UserReputationDTO{
			UserID:      driverID,
			RatingAvg:   4.90,
			RatingCount: 20,
			Reviews: []*ports.ReviewDTO{
				{
					ID:         revID,
					TripID:     tripID,
					BookingID:  bookingID,
					ReviewerID: passengerID,
					RevieweeID: driverID,
					Rating:     5,
					Comment:    "Genial",
					CreatedAt:  "2026-09-07T10:00:00Z",
				},
			},
		}

		mockRep.On("GetUserReviews", mock.Anything, driverID, 10, 0).Return(repDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/users/"+driverID.String()+"/reviews?limit=10&offset=0", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res adapterHttp.UserReputationResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, driverID, res.UserID)
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
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	dispID := uuid.MustParse("01960000-0000-7000-8000-000000000701")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000600")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000900")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000100")

	t.Run("abre disputa formal exitosamente (201 Created)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		expectedDTO := &ports.DisputeDTO{
			ID:                  dispID,
			EscrowTransactionID: escrowID,
			BookingID:           bookingID,
			TripID:              tripID,
			ReporterID:          passengerID,
			DefendantID:         driverID,
			Reason:              domain.DisputeReasonNoShow,
			Description:         "El conductor no llegó",
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           "2026-09-07T12:00:00Z",
			UpdatedAt:           "2026-09-07T12:00:00Z",
		}

		mockDisp.On("OpenDispute", mock.Anything, ports.OpenDisputeInput{
			BookingID:    bookingID,
			ReporterID:   passengerID,
			Reason:       domain.DisputeReasonNoShow,
			Description:  "El conductor no llegó",
			EvidenceURLs: []string{"https://img.com/1.png"},
		}).Return(expectedDTO, nil)

		payload := map[string]any{
			"booking_id":    bookingID.String(),
			"reason":        "NO_SHOW",
			"description":   "El conductor no llegó",
			"evidence_urls": []string{"https://img.com/1.png"},
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		var res adapterHttp.DisputeResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Equal(t, dispID, res.ID)
		assert.Equal(t, domain.DisputeStatusOpened, res.Status)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 409 Conflict si ya existe disputa previa (RF-17)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		mockDisp.On("OpenDispute", mock.Anything, mock.Anything).Return(nil, domain.ErrDisputeAlreadyExists)

		payload := map[string]any{
			"booking_id":  bookingID.String(),
			"reason":      "NO_SHOW",
			"description": "Reclamo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
		assert.Contains(t, w.Body.String(), "DISPUTE_ALREADY_EXISTS")
	})

	t.Run("retorna 400 Bad Request si el escrow ya fue liquidado (RF-17)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		mockDisp.On("OpenDispute", mock.Anything, mock.Anything).Return(nil, domain.ErrEscrowAlreadySettled)

		payload := map[string]any{
			"booking_id":  bookingID.String(),
			"reason":      "NO_SHOW",
			"description": "Reclamo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "ESCROW_ALREADY_SETTLED")
	})
}

func TestDisputeHandler_ResolveDispute(t *testing.T) {
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000299")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	dispID := uuid.MustParse("01960000-0000-7000-8000-000000000701")

	t.Run("resuelve disputa a favor del pasajero si es administrador (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, adminID, domain.RoleAdmin)

		resolvedDTO := &ports.DisputeDTO{
			ID:     dispID,
			Status: domain.DisputeStatusResolvedPassengerRefund,
		}

		mockDisp.On("ResolveDispute", mock.Anything, ports.ResolveDisputeInput{
			DisputeID:  dispID,
			AdminID:    adminID,
			Resolution: domain.DisputeStatusResolvedPassengerRefund,
			AdminNotes: "Aprobado reembolso completo",
		}).Return(resolvedDTO, nil)

		payload := map[string]any{
			"resolution":  "RESOLVED_PASSENGER_REFUND",
			"admin_notes": "Aprobado reembolso completo",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes/"+dispID.String()+"/resolve", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 403 Forbidden si un usuario no administrador intenta resolver la disputa", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		payload := map[string]any{
			"resolution":  "RESOLVED_PASSENGER_REFUND",
			"admin_notes": "Intento no autorizado de resolución",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes/"+dispID.String()+"/resolve", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "FORBIDDEN")
	})

	t.Run("retorna 400 Bad Request si el dictamen de resolución no pertenece al enum del dominio", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, adminID, domain.RoleAdmin)

		payload := map[string]any{
			"resolution":  "RESOLVED",
			"admin_notes": "Resolución con estado inválido",
		}
		body, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/v1/disputes/"+dispID.String()+"/resolve", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "INVALID_RESOLUTION")
		mockDisp.AssertNotCalled(t, "ResolveDispute", mock.Anything, mock.Anything)
	})
}

func TestDisputeHandler_GetDisputeByID(t *testing.T) {
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000299")
	intruderID := uuid.MustParse("01960000-0000-7000-8000-000000000209")
	dispID500 := uuid.MustParse("01960000-0000-7000-8000-000000000750")
	nonExistentDispID := uuid.MustParse("01960000-0000-7000-8000-000000000799")

	t.Run("consulta detalle si el usuario es el denunciante (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		disputeDTO := &ports.DisputeDTO{
			ID:          dispID500,
			ReporterID:  passengerID,
			DefendantID: driverID,
			Status:      domain.DisputeStatusInReview,
		}

		mockDisp.On("GetDisputeByID", mock.Anything, dispID500).Return(disputeDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes/"+dispID500.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("consulta detalle si el usuario es el denunciado (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, driverID, domain.RoleDriver)

		disputeDTO := &ports.DisputeDTO{
			ID:          dispID500,
			ReporterID:  passengerID,
			DefendantID: driverID,
			Status:      domain.DisputeStatusInReview,
		}

		mockDisp.On("GetDisputeByID", mock.Anything, dispID500).Return(disputeDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes/"+dispID500.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("consulta detalle si el usuario es administrador aunque no sea participante (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, adminID, domain.RoleAdmin)

		disputeDTO := &ports.DisputeDTO{
			ID:          dispID500,
			ReporterID:  passengerID,
			DefendantID: driverID,
			Status:      domain.DisputeStatusInReview,
		}

		mockDisp.On("GetDisputeByID", mock.Anything, dispID500).Return(disputeDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes/"+dispID500.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 403 Forbidden si un usuario ajeno a la disputa intenta consultarla", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, intruderID, domain.RolePassenger)

		disputeDTO := &ports.DisputeDTO{
			ID:          dispID500,
			ReporterID:  passengerID,
			DefendantID: driverID,
			Status:      domain.DisputeStatusInReview,
		}

		mockDisp.On("GetDisputeByID", mock.Anything, dispID500).Return(disputeDTO, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes/"+dispID500.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "FORBIDDEN")
	})

	t.Run("retorna 404 Not Found si la disputa no existe", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		mockDisp.On("GetDisputeByID", mock.Anything, nonExistentDispID).Return(nil, domain.ErrDisputeNotFound)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes/"+nonExistentDispID.String(), nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
		mockDisp.AssertExpectations(t)
	})
}

func TestDisputeHandler_ListDisputes(t *testing.T) {
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000299")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	dispID1 := uuid.MustParse("01960000-0000-7000-8000-000000000701")

	t.Run("lista disputas pasando limit y offset al servicio (200 OK)", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, adminID, domain.RoleAdmin)

		expected := []*ports.DisputeDTO{
			{ID: dispID1, Status: domain.DisputeStatusOpened},
		}
		mockDisp.On("ListDisputes", mock.Anything, 15, 30).Return(expected, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes?limit=15&offset=30", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res []*adapterHttp.DisputeResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
		assert.Len(t, res, 1)
		assert.Equal(t, dispID1, res[0].ID)
		mockDisp.AssertExpectations(t)
	})

	t.Run("convierte parámetro page a offset cuando offset no se envía", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, adminID, domain.RoleAdmin)

		mockDisp.On("ListDisputes", mock.Anything, 10, 20).Return([]*ports.DisputeDTO{}, nil)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes?page=3&limit=10", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockDisp.AssertExpectations(t)
	})

	t.Run("retorna 403 Forbidden para usuarios no administradores", func(t *testing.T) {
		mockDisp := new(MockDisputeServiceForHandlers)
		router := setupModule4Router(nil, nil, mockDisp, passengerID, domain.RolePassenger)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/v1/disputes", nil)
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		mockDisp.AssertNotCalled(t, "ListDisputes", mock.Anything, mock.Anything, mock.Anything)
	})
}
