package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestPricing_CalculateCapPrice(t *testing.T) {
	t.Run("calcula correctamente el Cap Price según fórmula EARS RF-06 con 10% margen", func(t *testing.T) {
		fuelCost := 30000.0 // ARS
		tollCost := 3000.0  // ARS
		seats := 3

		// Total = 33000
		// CostPerSeat = 33000 / 3 = 11000
		// CapPrice = 11000 * 1.10 = 12100
		matrix := domain.CalculateCapPrice(fuelCost, tollCost, seats)

		assert.Equal(t, 33000.0, matrix.TotalSharedCost)
		assert.Equal(t, 11000.0, matrix.CostPerSeat)
		assert.Equal(t, 12100.0, matrix.CapPricePerSeat)
		assert.Equal(t, 0.10, matrix.ContingencyMargin)

		// Precios iguales o menores al Cap Price deben permitirse
		assert.True(t, matrix.IsPricePermitted(12100.0))
		assert.True(t, matrix.IsPricePermitted(10000.0))
		assert.True(t, matrix.IsPricePermitted(8500.0))

		// Precios superiores al Cap Price deben rechazarse
		assert.False(t, matrix.IsPricePermitted(12101.0))
		assert.False(t, matrix.IsPricePermitted(15000.0))
	})

	t.Run("previene división por cero si asientos ofertados es cero o negativo", func(t *testing.T) {
		matrix := domain.CalculateCapPrice(10000.0, 0.0, 0)
		assert.Equal(t, 1, matrix.SeatsOffered)
		assert.Equal(t, 11000.0, matrix.CapPricePerSeat)
	})
}

func TestGeo_CoordinatesAndWKT(t *testing.T) {
	t.Run("valida coordenadas geográficas dentro del rango geodésico", func(t *testing.T) {
		valid, err := domain.NewCoordinates(-34.603722, -58.381592) // Obelisco Buenos Aires
		require.NoError(t, err)
		assert.True(t, valid.IsValid())
		assert.Equal(t, "POINT(-58.381592 -34.603722)", valid.ToWKTPoint())

		parsedPt, errPt := domain.ParseWKTPoint(valid.ToWKTPoint())
		require.NoError(t, errPt)
		assert.InDelta(t, -34.603722, parsedPt.Latitude, 0.00001)
		assert.InDelta(t, -58.381592, parsedPt.Longitude, 0.00001)

		_, errInvalidLat := domain.NewCoordinates(95.0, -58.0)
		assert.ErrorIs(t, errInvalidLat, domain.ErrInvalidRouteCoordinates)

		_, errInvalidLon := domain.NewCoordinates(-34.0, 190.0)
		assert.ErrorIs(t, errInvalidLon, domain.ErrInvalidRouteCoordinates)
	})

	t.Run("calcula distancia Haversine aproximada entre Buenos Aires y Rosario (~280km)", func(t *testing.T) {
		ba, _ := domain.NewCoordinates(-34.6037, -58.3816)
		rosario, _ := domain.NewCoordinates(-32.9468, -60.6393)

		distance := ba.DistanceHaversineKm(rosario)
		assert.InDelta(t, 280.0, distance, 25.0)
	})

	t.Run("genera y parsea WKT LineString correctamente", func(t *testing.T) {
		c1, _ := domain.NewCoordinates(-34.60, -58.38)
		c2, _ := domain.NewCoordinates(-33.80, -59.50)
		c3, _ := domain.NewCoordinates(-32.95, -60.64)

		ls, err := domain.NewLineString([]domain.Coordinates{c1, c2, c3})
		require.NoError(t, err)

		wkt := ls.ToWKTLineString()
		assert.Contains(t, wkt, "LINESTRING(-58.380000 -34.600000, -59.500000 -33.800000, -60.640000 -32.950000)")

		parsed, err := domain.ParseWKTLineString(wkt)
		require.NoError(t, err)
		assert.Equal(t, 3, len(parsed.Coordinates))
		assert.InDelta(t, -34.60, parsed.Coordinates[0].Latitude, 0.0001)
	})
}

func TestTrip_DomainInvariants(t *testing.T) {
	now := time.Now().UTC()
	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	route, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	validParams := domain.TripCreationParams{
		ID:                   "trip-101",
		DriverID:             "driver-001",
		VehicleID:            "vehicle-001",
		OriginTitle:          "Buenos Aires (Obelisco)",
		OriginCoords:         cOrig,
		DestinationTitle:     "Rosario (Monumento a la Bandera)",
		DestinationCoords:    cDest,
		RoutePath:            route,
		DepartureTime:        now.Add(24 * time.Hour),
		EstimatedArrivalTime: now.Add(28 * time.Hour),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		MaxVehicleSeats:      4,
		PricePerSeat:         12000.0,
		EstimatedFuelCost:    30000.0,
		EstimatedTollCost:    3000.0,
	}

	t.Run("crea un viaje exitosamente cuando cumple todas las invariantes", func(t *testing.T) {
		trip, err := domain.NewTrip(validParams, now)
		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
		assert.Equal(t, 3, trip.AvailableSeats)
		assert.Equal(t, 12100.0, trip.CapPricePerSeat)
	})

	t.Run("rechaza publicación si el precio supera el Cap Price (RF-07)", func(t *testing.T) {
		invalidParams := validParams
		invalidParams.PricePerSeat = 12500.0 // Supera el Cap Price de 12100.0

		trip, err := domain.NewTrip(invalidParams, now)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrPriceExceedsCapPrice)
	})

	t.Run("rechaza publicación con fecha de salida pasada", func(t *testing.T) {
		invalidParams := validParams
		invalidParams.DepartureTime = now.Add(-1 * time.Hour)

		trip, err := domain.NewTrip(invalidParams, now)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrDepartureTimeMustBeFuture)
	})

	t.Run("rechaza si los asientos ofertados exceden la capacidad del vehículo", func(t *testing.T) {
		invalidParams := validParams
		invalidParams.TotalSeatsOffered = 5 // Vehículo soporta 4

		trip, err := domain.NewTrip(invalidParams, now)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrInvalidSeatCount)
	})

	t.Run("gestiona ciclo de vida y actualización de asientos", func(t *testing.T) {
		trip, err := domain.NewTrip(validParams, now)
		require.NoError(t, err)

		// Ocupar 3 asientos -> pasa a FULL
		err = trip.UpdateAvailableSeats(0)
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusFull, trip.Status)

		// Liberar 1 asiento -> vuelve a PUBLISHED
		err = trip.UpdateAvailableSeats(1)
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)

		// Cancelación por el conductor
		err = trip.Cancel()
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusCancelled, trip.Status)

		// No puede cancelarse dos veces
		assert.ErrorIs(t, trip.Cancel(), domain.ErrTripCannotBeCancelled)
	})
}
