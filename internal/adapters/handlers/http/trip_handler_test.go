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

// MockTripService implementa ports.TripService con testify/mock
type MockTripService struct {
	mock.Mock
}

func (m *MockTripService) CreateTrip(ctx context.Context, input ports.CreateTripInput) (*domain.Trip, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Trip), args.Error(1)
}

func (m *MockTripService) SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.TripSearchResult), args.Error(1)
}

func (m *MockTripService) GetTripByID(ctx context.Context, tripID string) (*ports.TripDetailDTO, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDetailDTO), args.Error(1)
}

func (m *MockTripService) CancelTrip(ctx context.Context, tripID, driverID string) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

// setupTripTestRouter inicializa un router Gin aislado con mock middleware de auth
func setupTripTestRouter(mockTripSvc *MockTripService, authUserID string) *gin.Engine {
	gin.SetMode(gin.TestMode)

	tripHandler := adapterHttp.NewTripHandler(mockTripSvc)

	// Middleware de autenticación simulado para pruebas
	fakeAuthMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if authUserID != "" {
			c.Set(middleware.CtxUserIDKey, authUserID)
			c.Set(middleware.CtxUserRoleKey, domain.UserRoleDriver)
		}
		c.Next()
	}

	cfg := adapterHttp.RouterConfig{
		TripHandler:    tripHandler,
		AuthMiddleware: fakeAuthMiddleware,
	}

	return adapterHttp.SetupRouter(cfg)
}

func TestTripHandler_CreateTrip(t *testing.T) {
	orig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	dest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{orig, dest})

	now := time.Now().UTC()
	departure := now.Add(24 * time.Hour)

	mockCreatedTrip := &domain.Trip{
		ID:                   "trip-uuid-1",
		DriverID:             "driver-auth-1",
		VehicleID:            "vehicle-1",
		OriginTitle:          "Buenos Aires",
		OriginCoords:         orig,
		DestinationTitle:     "Rosario",
		DestinationCoords:    dest,
		RoutePath:            routeLine,
		DepartureTime:        departure,
		EstimatedArrivalTime: departure.Add(4 * time.Hour),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		AvailableSeats:       3,
		PricePerSeat:         11000.0,
		CapPricePerSeat:      11385.0,
		Status:               domain.TripStatusPublished,
	}

	payload := adapterHttp.CreateTripRequest{
		VehicleID:        "vehicle-1",
		OriginTitle:      "Buenos Aires",
		OriginLat:        -34.6037,
		OriginLon:        -58.3816,
		DestinationTitle: "Rosario",
		DestinationLat:   -32.9468,
		DestinationLon:   -60.6393,
		DepartureTime:    departure.Format(time.RFC3339),
		SeatsOffered:     3,
		PricePerSeat:     11000.0,
	}

	t.Run("publica viaje exitosamente (201 Created)", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.MatchedBy(func(in ports.CreateTripInput) bool {
			return in.DriverID == "driver-auth-1" &&
				in.VehicleID == "vehicle-1" &&
				in.PricePerSeat == 11000.0 &&
				in.SeatsOffered == 3
		})).Return(mockCreatedTrip, nil)

		router := setupTripTestRouter(mockSvc, "driver-auth-1")

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-driver-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]any
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "Viaje publicado con éxito", resp["message"])
		mockSvc.AssertExpectations(t)
	})

	t.Run("rechaza con 403 Forbidden si el usuario no tiene rol de conductor activo", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.Anything).Return(nil, domain.ErrDriverNotActive)

		router := setupTripTestRouter(mockSvc, "passenger-user-id")

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "DRIVER_NOT_ACTIVE", resp["code"])
	})

	t.Run("rechaza con 422 Unprocessable Entity si el precio supera el Cap Price", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.Anything).Return(nil, domain.ErrPriceExceedsCapPrice)

		router := setupTripTestRouter(mockSvc, "driver-auth-1")

		invalidPayload := payload
		invalidPayload.PricePerSeat = 25000.0 // Supera el Cap Price

		body, _ := json.Marshal(invalidPayload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "PRICE_EXCEEDS_CAP_PRICE", resp["code"])
	})

	t.Run("rechaza con 400 Bad Request si el vehículo no está aprobado", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.Anything).Return(nil, domain.ErrVehicleNotAvailable)

		router := setupTripTestRouter(mockSvc, "driver-auth-1")

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "VEHICLE_NOT_APPROVED", resp["code"])
	})

	t.Run("rechaza con 401 Unauthorized si falta el token de autorización", func(t *testing.T) {
		mockSvc := new(MockTripService)
		router := setupTripTestRouter(mockSvc, "")

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestTripHandler_SearchTrips(t *testing.T) {
	t.Run("devuelve resultados de búsqueda coincidentes (200 OK)", func(t *testing.T) {
		mockSvc := new(MockTripService)

		expectedResults := []ports.TripSearchResult{
			{
				ID:                   "trip-101",
				DriverID:             "driver-1",
				DriverFullName:       "Alejandro Sanz",
				VehicleBrand:         "Ford",
				VehicleModel:         "Focus",
				VehiclePlate:         "AA999ZZ",
				OriginTitle:          "Buenos Aires",
				DestinationTitle:     "Rosario",
				AvailableSeats:       2,
				PricePerSeat:         10500.0,
				CapPricePerSeat:      11385.0,
				PickupWalkDistanceM:  320.0,
				DropoffWalkDistanceM: 540.0,
				Status:               domain.TripStatusPublished,
			},
		}

		mockSvc.On("SearchTrips", mock.Anything, mock.MatchedBy(func(q ports.SearchTripsQuery) bool {
			return q.RequiredSeats == 2 && q.ToleranceMeters == 8000.0
		})).Return(expectedResults, nil)

		router := setupTripTestRouter(mockSvc, "")

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/trips/search?orig_lat=-34.6037&orig_lon=-58.3816&dest_lat=-32.9468&dest_lon=-60.6393&seats=2&radius_meters=8000", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp struct {
			Count   int                      `json:"count"`
			Results []ports.TripSearchResult `json:"results"`
		}
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, "trip-101", resp.Results[0].ID)
		assert.Equal(t, "Alejandro Sanz", resp.Results[0].DriverFullName)
	})

	t.Run("falla con 400 Bad Request si faltan coordenadas de búsqueda", func(t *testing.T) {
		mockSvc := new(MockTripService)
		router := setupTripTestRouter(mockSvc, "")

		// Falta dest_lat y dest_lon
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/trips/search?orig_lat=-34.6037&orig_lon=-58.3816", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTripHandler_GetTripByID(t *testing.T) {
	t.Run("devuelve 200 OK con detalle y matriz de costos", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockDetail := &ports.TripDetailDTO{
			Trip: &domain.Trip{
				ID:           "trip-detail-1",
				OriginTitle:  "Buenos Aires",
				PricePerSeat: 10000.0,
			},
			CostMatrix: domain.CostMatrix{
				TotalTripCost:   31050.0,
				CostPerSeatBase: 10350.0,
				CapPricePerSeat: 11385.0,
			},
		}

		mockSvc.On("GetTripByID", mock.Anything, "trip-detail-1").Return(mockDetail, nil)

		router := setupTripTestRouter(mockSvc, "")
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/trips/trip-detail-1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("devuelve 404 Not Found si el viaje no existe", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("GetTripByID", mock.Anything, "non-existent").Return(nil, domain.ErrTripNotFound)

		router := setupTripTestRouter(mockSvc, "")
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/trips/non-existent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestTripHandler_CancelTrip(t *testing.T) {
	t.Run("cancela viaje exitosamente (200 OK)", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CancelTrip", mock.Anything, "trip-1", "driver-owner").Return(nil)

		router := setupTripTestRouter(mockSvc, "driver-owner")
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/trips/trip-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "CANCELLED", resp["status"])
	})

	t.Run("rechaza con 403 Forbidden si el usuario no es el conductor del viaje", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CancelTrip", mock.Anything, "trip-1", "impostor").Return(domain.ErrUnauthorized)

		router := setupTripTestRouter(mockSvc, "impostor")
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/trips/trip-1", nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
