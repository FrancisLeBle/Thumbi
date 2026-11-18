package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestPricing_CalculateCapPrice(t *testing.T) {
	t.Run("calcula correctamente el Cap Price según fórmula EARS RF-06 con 10% margen", func(t *testing.T) {
		fuelCost := domain.MoneyFromCents(3000000, domain.CurrencyARS) // $30000.00 ARS
		tollCost := domain.MoneyFromCents(300000, domain.CurrencyARS)  // $3000.00 ARS
		seats := 3

		// Total = 33000.00
		// CostPerSeat = 33000.00 / 3 = 11000.00
		// CapPrice = 11000.00 * 1.10 = 12100.00
		matrix := domain.CalculateCapPrice(fuelCost, tollCost, seats)

		assert.Equal(t, domain.CurrencyARS, matrix.Currency)
		assert.Equal(t, domain.MoneyFromCents(3300000, domain.CurrencyARS), matrix.TotalSharedCost)
		assert.Equal(t, domain.MoneyFromCents(1100000, domain.CurrencyARS), matrix.CostPerSeat)
		assert.Equal(t, domain.MoneyFromCents(1210000, domain.CurrencyARS), matrix.CapPricePerSeat)
		assert.Equal(t, 0.10, matrix.ContingencyMargin)

		// Precios iguales o menores al Cap Price deben permitirse
		assert.True(t, matrix.IsPricePermitted(domain.MoneyFromCents(1210000, domain.CurrencyARS)))
		assert.True(t, matrix.IsPricePermitted(domain.MoneyFromCents(1000000, domain.CurrencyARS)))
		assert.True(t, matrix.IsPricePermitted(domain.MoneyFromCents(850000, domain.CurrencyARS)))

		// Precios superiores al Cap Price o en otra moneda deben rechazarse
		assert.False(t, matrix.IsPricePermitted(domain.MoneyFromCents(1210100, domain.CurrencyARS)))
		assert.False(t, matrix.IsPricePermitted(domain.MoneyFromCents(1500000, domain.CurrencyARS)))
		assert.False(t, matrix.IsPricePermitted(domain.MoneyFromCents(1000000, domain.CurrencyUSD)))
	})

	t.Run("previene división por cero si asientos ofertados es cero o negativo", func(t *testing.T) {
		matrix := domain.CalculateCapPrice(domain.MoneyFromCents(1000000, domain.CurrencyARS), domain.ZeroMoney(domain.CurrencyARS), 0)
		assert.Equal(t, 1, matrix.SeatsOffered)
		assert.Equal(t, domain.MoneyFromCents(1100000, domain.CurrencyARS), matrix.CapPricePerSeat)
	})
}

func TestGeo_CoordinatesAndLineString(t *testing.T) {
	t.Run("valida coordenadas geográficas dentro del rango geodésico", func(t *testing.T) {
		valid, err := domain.NewCoordinates(-34.603722, -58.381592) // Obelisco Buenos Aires
		require.NoError(t, err)
		assert.True(t, valid.IsValid())
		assert.InDelta(t, -34.603722, valid.Latitude, 0.00001)
		assert.InDelta(t, -58.381592, valid.Longitude, 0.00001)

		_, errInvalidLat := domain.NewCoordinates(95.0, -58.0)
		assert.ErrorIs(t, errInvalidLat, domain.ErrInvalidRouteCoordinates)

		_, errInvalidLon := domain.NewCoordinates(-34.0, 190.0)
		assert.ErrorIs(t, errInvalidLon, domain.ErrInvalidRouteCoordinates)
	})

	t.Run("valida Point y LineString correctamente", func(t *testing.T) {
		c1, _ := domain.NewCoordinates(-34.60, -58.38)
		c2, _ := domain.NewCoordinates(-33.80, -59.50)
		c3, _ := domain.NewCoordinates(-32.95, -60.64)

		pt, errPt := domain.NewPoint("  Obelisco  ", c1)
		require.NoError(t, errPt)
		assert.Equal(t, "Obelisco", pt.Title)
		assert.Equal(t, c1, pt.Coords)

		_, errEmptyTitle := domain.NewPoint("   ", c1)
		assert.ErrorIs(t, errEmptyTitle, domain.ErrInvalidRouteCoordinates)

		ls, err := domain.NewLineString([]domain.Coordinates{c1, c2, c3})
		require.NoError(t, err)
		assert.Len(t, ls.Coordinates, 3)

		_, errInsufficient := domain.NewLineString([]domain.Coordinates{c1})
		assert.ErrorIs(t, errInsufficient, domain.ErrInsufficientRoutePoints)
	})
}

func TestTrip_DomainInvariants(t *testing.T) {
	now := time.Now().UTC()
	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	route, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	validParams := domain.TripCreationParams{
		ID:                   domain.NewUUIDv7(),
		DriverID:             domain.NewUUIDv7(),
		VehicleID:            domain.NewUUIDv7(),
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
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(1200000, domain.CurrencyARS),
		EstimatedFuelCost:    domain.MoneyFromCents(3000000, domain.CurrencyARS),
		EstimatedTollCost:    domain.MoneyFromCents(300000, domain.CurrencyARS),
	}

	t.Run("crea un viaje exitosamente cuando cumple todas las invariantes", func(t *testing.T) {
		trip, err := domain.NewTrip(validParams, now)
		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
		assert.Equal(t, domain.CurrencyARS, trip.Currency)
		assert.Equal(t, 3, trip.AvailableSeats)
		assert.Equal(t, domain.MoneyFromCents(1210000, domain.CurrencyARS), trip.CapPricePerSeat)
	})

	t.Run("rechaza publicación si la moneda del precio difiere de la moneda del viaje", func(t *testing.T) {
		invalidParams := validParams
		invalidParams.PricePerSeat = domain.MoneyFromCents(1200000, domain.CurrencyUSD)

		trip, err := domain.NewTrip(invalidParams, now)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrCurrencyMismatch)
	})

	t.Run("rechaza publicación si el precio supera el Cap Price (RF-07)", func(t *testing.T) {
		invalidParams := validParams
		invalidParams.PricePerSeat = domain.MoneyFromCents(1250000, domain.CurrencyARS) // Supera el Cap Price de $12100.00

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

	t.Run("CanBook, ReserveSeats y ReleaseSeats gobiernan disponibilidad y transiciones PUBLISHED <-> FULL", func(t *testing.T) {
		trip, err := domain.NewTrip(validParams, now) // TotalSeatsOffered: 3, AvailableSeats: 3, Status: PUBLISHED
		require.NoError(t, err)

		passengerID := domain.NewUUIDv7()

		// Driver no puede reservar su propio viaje
		assert.ErrorIs(t, trip.CanBook(trip.DriverID, 1), domain.ErrCannotBookOwnTrip)
		// Cantidad de asientos <= 0 inválida
		assert.ErrorIs(t, trip.CanBook(passengerID, 0), domain.ErrInvalidSeatCount)
		// Superar disponibles falla
		assert.ErrorIs(t, trip.CanBook(passengerID, 4), domain.ErrInsufficientSeats)
		// Reserva válida de 1 asiento
		require.NoError(t, trip.CanBook(passengerID, 1))

		t1 := now.Add(5 * time.Minute)
		hold1, err := trip.ReserveSeats(1, t1)
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusPublished, hold1.ExpectedStatus)
		assert.Equal(t, 3, hold1.ExpectedAvailableSeats)
		assert.Equal(t, domain.TripStatusPublished, hold1.NewStatus)
		assert.Equal(t, 2, hold1.NewAvailableSeats)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
		assert.Equal(t, 2, trip.AvailableSeats)

		// Reservar los últimos 2 asientos transiciona a FULL
		t2 := now.Add(10 * time.Minute)
		hold2, err := trip.ReserveSeats(2, t2)
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusPublished, hold2.ExpectedStatus)
		assert.Equal(t, 2, hold2.ExpectedAvailableSeats)
		assert.Equal(t, domain.TripStatusFull, hold2.NewStatus)
		assert.Equal(t, 0, hold2.NewAvailableSeats)
		assert.Equal(t, domain.TripStatusFull, trip.Status)

		// En estado FULL no admite nuevas reservas
		assert.ErrorIs(t, trip.CanBook(passengerID, 1), domain.ErrInvalidTripStatusChange)
		_, errReserveFull := trip.ReserveSeats(1, t2)
		assert.ErrorIs(t, errReserveFull, domain.ErrInvalidTripStatusChange)

		// Restituir 2 asientos desde FULL reabre el viaje a PUBLISHED
		t3 := now.Add(15 * time.Minute)
		rel1, err := trip.ReleaseSeats(2, t3)
		require.NoError(t, err)
		assert.Equal(t, domain.TripStatusFull, rel1.ExpectedStatus)
		assert.Equal(t, domain.TripStatusPublished, rel1.NewStatus)
		assert.Equal(t, 2, rel1.NewAvailableSeats)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)

		// No puede restituir más asientos que TotalSeatsOffered (2 + 2 = 4 > 3)
		_, errExceed := trip.ReleaseSeats(2, t3)
		assert.ErrorIs(t, errExceed, domain.ErrInvalidSeatCount)
	})
}
