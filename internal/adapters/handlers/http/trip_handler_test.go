package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

// MockTripService implementa adapterHttp.TripService con testify/mock
type MockTripService struct {
	mock.Mock
}

func (m *MockTripService) CreateTrip(ctx context.Context, input ports.CreateTripInput) (*ports.TripDTO, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDTO), args.Error(1)
}

func (m *MockTripService) SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]ports.TripSearchResult), args.Error(1)
}

func (m *MockTripService) GetTripByID(ctx context.Context, tripID uuid.UUID) (*ports.TripDetailDTO, error) {
	args := m.Called(ctx, tripID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDetailDTO), args.Error(1)
}

func (m *MockTripService) CancelTrip(ctx context.Context, tripID, driverID uuid.UUID) error {
	args := m.Called(ctx, tripID, driverID)
	return args.Error(0)
}

func (m *MockTripService) CompleteTrip(ctx context.Context, tripID, driverID uuid.UUID) (*ports.TripDTO, error) {
	args := m.Called(ctx, tripID, driverID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*ports.TripDTO), args.Error(1)
}

// setupTripTestRouter inicializa un router Gin aislado con mock middleware de auth
func setupTripTestRouter(mockTripSvc *MockTripService, authUserID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)

	tripHandler := adapterHttp.NewTripHandler(mockTripSvc)

	// Middleware de autenticación simulado para pruebas
	fakeAuthMiddleware := func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if authUserID != uuid.Nil {
			c.Set(middleware.CtxUserIDKey, authUserID)
			c.Set(middleware.CtxUserRoleKey, domain.UserRoleDriver)
		}
		c.Next()
	}

	cfg := baseTestRouterConfig()
	cfg.TripHandler = tripHandler
	cfg.AuthMiddleware = fakeAuthMiddleware

	return mustSetupTestRouter(cfg)
}

func TestTripHandler_CreateTrip(t *testing.T) {
	orig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	dest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{orig, dest})

	now := time.Now().UTC()
	departure := now.Add(24 * time.Hour)

	tripID1 := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	driverAuth1 := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	passengerUserID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
	vehicleID1 := uuid.MustParse("01960000-0000-7000-8000-000000000301")

	mockCreatedTrip := &ports.TripDTO{
		ID:                   tripID1,
		DriverID:             driverAuth1,
		VehicleID:            vehicleID1,
		OriginTitle:          "Buenos Aires",
		OriginCoords:         orig,
		DestinationTitle:     "Rosario",
		DestinationCoords:    dest,
		RoutePath:            routeLine,
		DepartureTime:        departure.Format(time.RFC3339),
		EstimatedArrivalTime: departure.Add(4 * time.Hour).Format(time.RFC3339),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		AvailableSeats:       3,
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(1100000, domain.CurrencyARS),
		CapPricePerSeat:      domain.MoneyFromCents(1138500, domain.CurrencyARS),
		Status:               domain.TripStatusPublished,
	}

	payload := adapterHttp.CreateTripRequest{
		VehicleID:        vehicleID1.String(),
		OriginTitle:      "Buenos Aires",
		OriginLat:        -34.6037,
		OriginLon:        -58.3816,
		DestinationTitle: "Rosario",
		DestinationLat:   -32.9468,
		DestinationLon:   -60.6393,
		DepartureTime:    departure.Format(time.RFC3339),
		SeatsOffered:     3,
		PricePerSeat:     domain.MoneyFromCents(1100000, domain.CurrencyARS),
		Currency:         domain.CurrencyARS,
	}

	t.Run("publica viaje exitosamente (201 Created)", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.MatchedBy(func(in ports.CreateTripInput) bool {
			return in.DriverID == driverAuth1 &&
				in.VehicleID == vehicleID1 &&
				in.PricePerSeat == domain.MoneyFromCents(1100000, domain.CurrencyARS) &&
				in.SeatsOffered == 3
		})).Return(mockCreatedTrip, nil)

		router := setupTripTestRouter(mockSvc, driverAuth1)

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
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

		router := setupTripTestRouter(mockSvc, passengerUserID)

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "DRIVER_NOT_ACTIVE", resp["code"])
	})

	t.Run("rechaza con 422 Unprocessable Entity si el conductor no posee teléfono registrado", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.Anything).Return(nil, domain.ErrDriverPhoneRequired)

		router := setupTripTestRouter(mockSvc, driverAuth1)

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		assert.Equal(t, "DRIVER_PHONE_REQUIRED", resp["code"])
	})

	t.Run("rechaza con 422 Unprocessable Entity si el precio supera el Cap Price", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.Anything).Return(nil, domain.ErrPriceExceedsCapPrice)

		router := setupTripTestRouter(mockSvc, driverAuth1)

		invalidPayload := payload
		invalidPayload.PricePerSeat = domain.MoneyFromCents(2500000, domain.CurrencyARS) // Supera el Cap Price

		body, _ := json.Marshal(invalidPayload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
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

		router := setupTripTestRouter(mockSvc, driverAuth1)

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
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
		router := setupTripTestRouter(mockSvc, uuid.Nil)

		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("acepta price_per_seat como cadena decimal exacta y rechaza float JSON", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CreateTrip", mock.Anything, mock.MatchedBy(func(in ports.CreateTripInput) bool {
			return in.PricePerSeat == domain.MoneyFromCents(1100050, domain.CurrencyARS)
		})).Return(mockCreatedTrip, nil)

		router := setupTripTestRouter(mockSvc, driverAuth1)

		strJSON := fmt.Sprintf(`{"vehicle_id":%q,"origin_title":"Buenos Aires","origin_lat":-34.6037,"origin_lon":-58.3816,"destination_title":"Rosario","destination_lat":-32.9468,"destination_lon":-60.6393,"departure_time":%q,"seats_offered":3,"price_per_seat":"11000.50"}`, vehicleID1.String(), departure.Format(time.RFC3339))
		reqStr, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBufferString(strJSON))
		reqStr.Header.Set("Content-Type", "application/json")
		reqStr.Header.Set("Authorization", "Bearer valid-driver-token")
		wStr := httptest.NewRecorder()
		router.ServeHTTP(wStr, reqStr)
		assert.Equal(t, http.StatusCreated, wStr.Code)

		floatJSON := fmt.Sprintf(`{"vehicle_id":%q,"origin_title":"Buenos Aires","origin_lat":-34.6037,"origin_lon":-58.3816,"destination_title":"Rosario","destination_lat":-32.9468,"destination_lon":-60.6393,"departure_time":%q,"seats_offered":3,"price_per_seat":11000.55}`, vehicleID1.String(), departure.Format(time.RFC3339))
		reqFloat, _ := http.NewRequest(http.MethodPost, "/v1/trips", bytes.NewBufferString(floatJSON))
		reqFloat.Header.Set("Content-Type", "application/json")
		reqFloat.Header.Set("Authorization", "Bearer valid-driver-token")
		wFloat := httptest.NewRecorder()
		router.ServeHTTP(wFloat, reqFloat)
		assert.Equal(t, http.StatusBadRequest, wFloat.Code)
	})
}

func TestTripHandler_SearchTrips(t *testing.T) {
	tripID101 := uuid.MustParse("01960000-0000-7000-8000-000000000102")
	driverID1 := uuid.MustParse("01960000-0000-7000-8000-000000000201")

	t.Run("devuelve resultados de búsqueda coincidentes (200 OK)", func(t *testing.T) {
		mockSvc := new(MockTripService)

		expectedResults := []ports.TripSearchResult{
			{
				ID:                   tripID101,
				DriverID:             driverID1,
				DriverFullName:       "Alejandro Sanz",
				VehicleBrand:         "Ford",
				VehicleModel:         "Focus",
				VehiclePlate:         "AA999ZZ",
				OriginTitle:          "Buenos Aires",
				DestinationTitle:     "Rosario",
				AvailableSeats:       2,
				Currency:             domain.CurrencyARS,
				PricePerSeat:         domain.MoneyFromCents(1050000, domain.CurrencyARS),
				CapPricePerSeat:      domain.MoneyFromCents(1138500, domain.CurrencyARS),
				PickupWalkDistanceM:  320.0,
				DropoffWalkDistanceM: 540.0,
				Status:               domain.TripStatusPublished,
			},
		}

		mockSvc.On("SearchTrips", mock.Anything, mock.MatchedBy(func(q ports.SearchTripsQuery) bool {
			return q.RequiredSeats == 2 && q.ToleranceMeters == 8000.0
		})).Return(expectedResults, nil)

		router := setupTripTestRouter(mockSvc, uuid.Nil)

		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/search?orig_lat=-34.6037&orig_lon=-58.3816&dest_lat=-32.9468&dest_lon=-60.6393&seats=2&radius_meters=8000", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp adapterHttp.SearchTripsResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Count)
		assert.Equal(t, tripID101, resp.Results[0].ID)
		assert.Equal(t, "Alejandro Sanz", resp.Results[0].DriverFullName)
	})

	t.Run("falla con 400 Bad Request si faltan coordenadas de búsqueda", func(t *testing.T) {
		mockSvc := new(MockTripService)
		router := setupTripTestRouter(mockSvc, uuid.Nil)

		// Falta dest_lat y dest_lon
		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/search?orig_lat=-34.6037&orig_lon=-58.3816", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestTripHandler_GetTripByID(t *testing.T) {
	tripDetailID := uuid.MustParse("01960000-0000-7000-8000-000000000103")
	nonExistentID := uuid.MustParse("01960000-0000-7000-8000-000000000199")

	t.Run("devuelve 200 OK con detalle y matriz de costos", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockDetail := &ports.TripDetailDTO{
			Trip: &ports.TripDTO{
				ID:           tripDetailID,
				OriginTitle:  "Buenos Aires",
				Currency:     domain.CurrencyARS,
				PricePerSeat: domain.MoneyFromCents(1000000, domain.CurrencyARS),
			},
			CostMatrix: domain.CostMatrix{
				TotalSharedCost: domain.MoneyFromCents(3105000, domain.CurrencyARS),
				CostPerSeat:     domain.MoneyFromCents(1035000, domain.CurrencyARS),
				CapPricePerSeat: domain.MoneyFromCents(1138500, domain.CurrencyARS),
			},
		}

		mockSvc.On("GetTripByID", mock.Anything, tripDetailID).Return(mockDetail, nil)

		router := setupTripTestRouter(mockSvc, uuid.Nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/"+tripDetailID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("devuelve 404 Not Found si el viaje no existe", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("GetTripByID", mock.Anything, nonExistentID).Return(nil, domain.ErrTripNotFound)

		router := setupTripTestRouter(mockSvc, uuid.Nil)
		req, _ := http.NewRequest(http.MethodGet, "/v1/trips/"+nonExistentID.String(), nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestTripHandler_CancelTrip(t *testing.T) {
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000104")
	driverOwnerID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
	impostorID := uuid.MustParse("01960000-0000-7000-8000-000000000209")

	t.Run("cancela viaje exitosamente (200 OK)", func(t *testing.T) {
		mockSvc := new(MockTripService)
		mockSvc.On("CancelTrip", mock.Anything, tripID, driverOwnerID).Return(nil)

		router := setupTripTestRouter(mockSvc, driverOwnerID)
		req, _ := http.NewRequest(http.MethodDelete, "/v1/trips/"+tripID.String(), nil)
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
		mockSvc.On("CancelTrip", mock.Anything, tripID, impostorID).Return(domain.ErrUnauthorized)

		router := setupTripTestRouter(mockSvc, impostorID)
		req, _ := http.NewRequest(http.MethodDelete, "/v1/trips/"+tripID.String(), nil)
		req.Header.Set("Authorization", "Bearer valid-token")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
