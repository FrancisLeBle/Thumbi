package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func TestTripService_CreateTrip(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000123")
	vehicleID := uuid.MustParse("01960000-0000-7000-8000-000000000223")

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816) // Buenos Aires
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393) // Rosario
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	routeDetails := &ports.RouteDetails{
		RoutePath:         routeLine,
		DistanceKm:        300.0,
		DurationMinutes:   240,
		EstimatedTollCost: domain.MoneyFromCents(300000, domain.CurrencyARS),
		TollsCount:        2,
		EstimatedFuelCost: domain.MoneyFromCents(2805000, domain.CurrencyARS), // 300km * 8.5L/100km * $1100.00 ARS
	}

	validInput := ports.CreateTripInput{
		DriverID:          driverID,
		VehicleID:         vehicleID,
		OriginTitle:       "Buenos Aires Obelisco",
		OriginCoords:      cOrig,
		DestinationTitle:  "Rosario Monumento",
		DestinationCoords: cDest,
		DepartureTime:     fixedNow.Add(24 * time.Hour).Format(time.RFC3339),
		SeatsOffered:      3,
		Currency:          domain.CurrencyARS,
		PricePerSeat:      domain.MoneyFromCents(1100000, domain.CurrencyARS),
	}

	// Costo total estimado = 28050 (combustible) + 3000 (peajes) = 31050
	// Costo por asiento = 31050 / 3 = 10350
	// CapPrice = 10350 * 1.10 = 11385.00 ARS

	t.Run("falla si el usuario no tiene rol de conductor activo (RF-01)", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		// Usuario existe pero is_driver_active es false
		inactiveDriver := &domain.User{
			ID:             driverID,
			Email:          "driver@test.com",
			IsDriverActive: false,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(inactiveDriver, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrDriverNotActive)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("falla si el vehículo no está aprobado (RF-02)", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		activeDriver := &domain.User{
			ID:             driverID,
			Email:          "driver@test.com",
			Phone:          "+54 9 11 5555-1234",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(activeDriver, nil)

		pendingVehicle := &domain.Vehicle{
			ID:           vehicleID,
			UserID:       driverID,
			Status:       domain.VehicleStatusPendingVerification,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, vehicleID).Return(pendingVehicle, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrVehicleNotAvailable)

		mockVehRepo.AssertExpectations(t)
	})

	t.Run("falla si el precio fijado supera el Cap Price no lucrativo (RF-07)", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		activeDriver := &domain.User{
			ID:             driverID,
			Phone:          "+54 9 11 5555-1234",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(activeDriver, nil)

		approvedVeh := &domain.Vehicle{
			ID:           vehicleID,
			UserID:       driverID,
			Status:       domain.VehicleStatusApproved,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, vehicleID).Return(approvedVeh, nil)

		mockRouting.On("CalculateRoute", ctx, mock.Anything).Return(routeDetails, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		// Input con precio de $12000 que supera el Cap Price de $11385
		invalidInput := validInput
		invalidInput.PricePerSeat = domain.MoneyFromCents(1200000, domain.CurrencyARS)

		trip, err := svc.CreateTrip(ctx, invalidInput)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrPriceExceedsCapPrice)
	})

	t.Run("publica exitosamente el viaje con cálculo transparente de Cap Pricing", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		activeDriver := &domain.User{
			ID:             driverID,
			Phone:          "+54 9 11 5555-1234",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(activeDriver, nil)

		approvedVeh := &domain.Vehicle{
			ID:           vehicleID,
			UserID:       driverID,
			Status:       domain.VehicleStatusApproved,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, vehicleID).Return(approvedVeh, nil)

		mockRouting.On("CalculateRoute", ctx, mock.Anything).Return(routeDetails, nil)

		expectedPrice := domain.MoneyFromCents(1100000, domain.CurrencyARS)
		expectedCap := domain.MoneyFromCents(1138500, domain.CurrencyARS)

		mockTripRepo.On("Save", ctx, mock.MatchedBy(func(trip *domain.Trip) bool {
			return trip.DriverID == driverID &&
				trip.VehicleID == vehicleID &&
				trip.AvailableSeats == 3 &&
				trip.Currency == domain.CurrencyARS &&
				trip.PricePerSeat == expectedPrice &&
				trip.CapPricePerSeat == expectedCap &&
				trip.Status == domain.TripStatusPublished
		})).Return(nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
		assert.Equal(t, domain.CurrencyARS, trip.Currency)
		assert.Equal(t, expectedPrice, trip.PricePerSeat)
		assert.Equal(t, expectedCap, trip.CapPricePerSeat)

		mockTripRepo.AssertExpectations(t)
	})

	t.Run("falla con ErrDriverPhoneRequired si el conductor activo no tiene teléfono registrado", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		driverNoPhone := &domain.User{
			ID:             driverID,
			Email:          "driver@test.com",
			Phone:          "",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(driverNoPhone, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrDriverPhoneRequired)

		mockUserRepo.AssertExpectations(t)
	})

	t.Run("delega a PostGIS el cálculo geodésico de distancia cuando el proveedor de ruteo no retorna distancia", func(t *testing.T) {
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockTripRepo := new(MockTripRepository)
		mockRouting := new(MockRoutingProvider)

		activeDriver := &domain.User{
			ID:             driverID,
			Phone:          "+54 9 11 5555-1234",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, driverID).Return(activeDriver, nil)

		approvedVeh := &domain.Vehicle{
			ID:           vehicleID,
			UserID:       driverID,
			Status:       domain.VehicleStatusApproved,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, vehicleID).Return(approvedVeh, nil)

		fallbackRoute := &ports.RouteDetails{
			RoutePath: routeLine,
		}
		mockRouting.On("CalculateRoute", ctx, mock.Anything).Return(fallbackRoute, nil)
		mockTripRepo.On("CalculateRouteDistanceKm", ctx, routeLine).Return(300.0, nil)
		mockTripRepo.On("Save", ctx, mock.MatchedBy(func(trip *domain.Trip) bool {
			return trip.TotalDistanceKm == 300.0 && trip.TotalDurationMinutes == 225
		})).Return(nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, 300.0, trip.TotalDistanceKm)
		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_SearchTrips(t *testing.T) {
	ctx := context.Background()

	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000301")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000101")
	vehicleID := uuid.MustParse("01960000-0000-7000-8000-000000000201")

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	now := time.Now().UTC()
	departure := now.Add(48 * time.Hour)

	mockTrip := domain.Trip{
		ID:                   tripID,
		DriverID:             driverID,
		VehicleID:            vehicleID,
		OriginTitle:          "Buenos Aires",
		DestinationTitle:     "Rosario",
		RoutePath:            routeLine,
		DepartureTime:        departure,
		EstimatedArrivalTime: departure.Add(4 * time.Hour),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		AvailableSeats:       2,
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(1050000, domain.CurrencyARS),
		CapPricePerSeat:      domain.MoneyFromCents(1138500, domain.CurrencyARS),
		Status:               domain.TripStatusPublished,
	}

	record := ports.MatchedTripRecord{
		Trip:                 mockTrip,
		DriverFirstName:      "Carlos",
		DriverLastName:       "Gómez",
		DriverAvatarURL:      "https://example.com/avatar.jpg",
		VehicleBrand:         "Toyota",
		VehicleModel:         "Corolla",
		VehiclePlate:         "AF123JK",
		RoutePreviewPolyline: "p`~rE~lxbJc~kH|m}M",
		PickupWalkDistanceM:  450.0,
		DropoffWalkDistanceM: 820.0,
	}

	t.Run("retorna resultados coincidentes mapeados a DTOs", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockRouting := new(MockRoutingProvider)

		mockTripRepo.On("SearchMatchingTrips", ctx, mock.MatchedBy(func(p ports.SearchTripsParams) bool {
			return p.RequiredSeats == 1 && p.ToleranceMeters == 10000.0
		})).Return([]ports.MatchedTripRecord{record}, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		query := ports.SearchTripsQuery{
			OriginCoords:  cOrig,
			DestCoords:    cDest,
			DepartureDate: departure.Format("2006-01-02"),
			RequiredSeats: 1,
		}

		results, err := svc.SearchTrips(ctx, query)
		require.NoError(t, err)
		assert.Len(t, results, 1)

		res := results[0]
		assert.Equal(t, tripID, res.ID)
		assert.Equal(t, "Carlos Gómez", res.DriverFullName)
		assert.Equal(t, "Toyota", res.VehicleBrand)
		assert.Equal(t, 450.0, res.PickupWalkDistanceM)
		assert.Equal(t, 820.0, res.DropoffWalkDistanceM)
		assert.Equal(t, "p`~rE~lxbJc~kH|m}M", res.RoutePreviewPolyline)
		assert.Equal(t, domain.CurrencyARS, res.Currency)
		assert.Equal(t, domain.MoneyFromCents(1050000, domain.CurrencyARS), res.PricePerSeat)

		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_CancelTrip(t *testing.T) {
	ctx := context.Background()

	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000401")
	ownerDriverID := uuid.MustParse("01960000-0000-7000-8000-000000000402")
	impostorID := uuid.MustParse("01960000-0000-7000-8000-000000000403")

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	activeTrip := &domain.Trip{
		ID:        tripID,
		DriverID:  ownerDriverID,
		RoutePath: routeLine,
		Status:    domain.TripStatusPublished,
	}

	t.Run("falla si otro usuario que no es el conductor intenta cancelar el viaje", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockRouting := new(MockRoutingProvider)

		mockTripRepo.On("FindByID", ctx, tripID).Return(activeTrip, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		err := svc.CancelTrip(ctx, tripID, impostorID)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("cancela exitosamente el viaje por el conductor dueño", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockRouting := new(MockRoutingProvider)

		tripCopy := *activeTrip
		mockTripRepo.On("FindByID", ctx, tripID).Return(&tripCopy, nil)
		mockTripRepo.On("UpdateStatus", ctx, tripID, domain.TripStatusCancelled).Return(nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		err := svc.CancelTrip(ctx, tripID, ownerDriverID)
		require.NoError(t, err)

		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_CompleteTrip(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000501")
	missingTripID := uuid.MustParse("01960000-0000-7000-8000-000000000599")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000502")
	impostorID := uuid.MustParse("01960000-0000-7000-8000-000000000503")
	vehicleID := uuid.MustParse("01960000-0000-7000-8000-000000000504")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000505")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000506")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000507")

	activeTrip := &domain.Trip{
		ID:                   tripID,
		DriverID:             driverID,
		VehicleID:            vehicleID,
		Status:               domain.TripStatusInProgress,
		TotalSeatsOffered:    3,
		AvailableSeats:       1,
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(1000000, domain.CurrencyARS),
		DepartureTime:        now.Add(-2 * time.Hour),
		EstimatedArrivalTime: now.Add(-10 * time.Minute),
	}

	t.Run("falla si el viaje no existe", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		mockTripRepo.On("FindByID", ctx, missingTripID).Return(nil, domain.ErrTripNotFound)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil, mockBookingRepo, mockEscrowRepo)
		res, err := svc.CompleteTrip(ctx, missingTripID, driverID)

		assert.ErrorIs(t, err, domain.ErrTripNotFound)
		assert.Nil(t, res)
	})

	t.Run("falla si el usuario no es el conductor del viaje (RF-01)", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockTripRepo.On("FindByID", ctx, tripID).Return(activeTrip, nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil)
		res, err := svc.CompleteTrip(ctx, tripID, impostorID)

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si el viaje ya estaba cancelado", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		cancelledTrip := *activeTrip
		cancelledTrip.Status = domain.TripStatusCancelled
		mockTripRepo.On("FindByID", ctx, tripID).Return(&cancelledTrip, nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil)
		res, err := svc.CompleteTrip(ctx, tripID, driverID)

		assert.ErrorIs(t, err, domain.ErrInvalidTripStatusChange)
		assert.Nil(t, res)
	})

	t.Run("finaliza exitosamente el viaje, reservas confirmadas y libera el Escrow", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		tripCopy := *activeTrip
		mockTripRepo.On("FindByID", ctx, tripID).Return(&tripCopy, nil)
		mockTripRepo.On("UpdateStatus", ctx, tripID, domain.TripStatusCompleted).Return(nil)

		confirmedBooking := &domain.Booking{
			ID:          bookingID,
			TripID:      tripID,
			PassengerID: passengerID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			TotalPrice:  domain.MoneyFromCents(2000000, domain.CurrencyARS),
			Status:      domain.BookingStatusConfirmed,
		}

		mockBookingRepo.On("GetActiveBookingsByTrip", ctx, tripID).Return([]*domain.Booking{confirmedBooking}, nil)
		mockBookingRepo.On("UpdateStatus", ctx, bookingID, domain.BookingStatusCompleted, (*string)(nil)).Return(nil)

		escrowTx, _ := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, domain.MoneyFromCents(2000000, domain.CurrencyARS), "pi_test_101")
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(escrowTx, nil)
		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowID, domain.EscrowStatusReleased).Return(nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil, mockBookingRepo, mockEscrowRepo)
		res, err := svc.CompleteTrip(ctx, tripID, driverID)

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.TripStatusCompleted, res.Status)
		assert.Equal(t, domain.BookingStatusCompleted, confirmedBooking.Status)
		assert.Equal(t, domain.EscrowStatusReleased, escrowTx.Status)

		mockTripRepo.AssertExpectations(t)
		mockBookingRepo.AssertExpectations(t)
		mockEscrowRepo.AssertExpectations(t)
	})
}

