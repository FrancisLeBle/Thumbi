package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
)

func TestTripService_CreateTrip(t *testing.T) {
	ctx := context.Background()
	fixedNow := time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816) // Buenos Aires
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393) // Rosario
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	routeDetails := &ports.RouteDetails{
		RoutePath:            routeLine,
		DistanceKm:           300.0,
		DurationMinutes:      240,
		EstimatedTollCost:    3000.0,
		TollsCount:           2,
		EstimatedFuelCost:    28050.0, // 300km * 8.5L/100km * 1100 ARS
	}

	validInput := ports.CreateTripInput{
		DriverID:          "driver-123",
		VehicleID:         "vehicle-123",
		OriginTitle:       "Buenos Aires Obelisco",
		OriginCoords:      cOrig,
		DestinationTitle:  "Rosario Monumento",
		DestinationCoords: cDest,
		DepartureTime:     fixedNow.Add(24 * time.Hour).Format(time.RFC3339),
		SeatsOffered:      3,
		PricePerSeat:      11000.0,
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
			ID:             "driver-123",
			Email:          "driver@test.com",
			IsDriverActive: false,
		}
		mockUserRepo.On("GetByID", ctx, "driver-123").Return(inactiveDriver, nil)

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
			ID:             "driver-123",
			Email:          "driver@test.com",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, "driver-123").Return(activeDriver, nil)

		pendingVehicle := &domain.Vehicle{
			ID:           "vehicle-123",
			UserID:       "driver-123",
			Status:       domain.VehicleStatusPendingVerification,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, "vehicle-123").Return(pendingVehicle, nil)

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
			ID:             "driver-123",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, "driver-123").Return(activeDriver, nil)

		approvedVeh := &domain.Vehicle{
			ID:           "vehicle-123",
			UserID:       "driver-123",
			Status:       domain.VehicleStatusApproved,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, "vehicle-123").Return(approvedVeh, nil)

		mockRouting.On("CalculateRoute", ctx, mock.Anything).Return(routeDetails, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		// Input con precio de $12000 que supera el Cap Price de $11385
		invalidInput := validInput
		invalidInput.PricePerSeat = 12000.0

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
			ID:             "driver-123",
			IsDriverActive: true,
		}
		mockUserRepo.On("GetByID", ctx, "driver-123").Return(activeDriver, nil)

		approvedVeh := &domain.Vehicle{
			ID:           "vehicle-123",
			UserID:       "driver-123",
			Status:       domain.VehicleStatusApproved,
			SeatCapacity: 4,
		}
		mockVehRepo.On("GetByID", ctx, "vehicle-123").Return(approvedVeh, nil)

		mockRouting.On("CalculateRoute", ctx, mock.Anything).Return(routeDetails, nil)

		mockTripRepo.On("Save", ctx, mock.MatchedBy(func(trip *domain.Trip) bool {
			return trip.DriverID == "driver-123" &&
				trip.VehicleID == "vehicle-123" &&
				trip.AvailableSeats == 3 &&
				trip.PricePerSeat == 11000.0 &&
				trip.CapPricePerSeat == 11385.0 &&
				trip.Status == domain.TripStatusPublished
		})).Return(nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		trip, err := svc.CreateTrip(ctx, validInput)
		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
		assert.Equal(t, 11000.0, trip.PricePerSeat)
		assert.Equal(t, 11385.0, trip.CapPricePerSeat)

		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_SearchTrips(t *testing.T) {
	ctx := context.Background()

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	now := time.Now().UTC()
	departure := now.Add(48 * time.Hour)

	mockTrip := domain.Trip{
		ID:                   "trip-match-1",
		DriverID:             "driver-101",
		VehicleID:            "vehicle-101",
		OriginTitle:          "Buenos Aires",
		DestinationTitle:     "Rosario",
		RoutePath:            routeLine,
		DepartureTime:        departure,
		EstimatedArrivalTime: departure.Add(4 * time.Hour),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		AvailableSeats:       2,
		PricePerSeat:         10500.0,
		CapPricePerSeat:      11385.0,
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
		assert.Equal(t, "trip-match-1", res.ID)
		assert.Equal(t, "Carlos Gómez", res.DriverFullName)
		assert.Equal(t, "Toyota", res.VehicleBrand)
		assert.Equal(t, 450.0, res.PickupWalkDistanceM)
		assert.Equal(t, 820.0, res.DropoffWalkDistanceM)
		assert.Equal(t, 10500.0, res.PricePerSeat)

		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_CancelTrip(t *testing.T) {
	ctx := context.Background()

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	activeTrip := &domain.Trip{
		ID:        "trip-to-cancel",
		DriverID:  "driver-owner",
		RoutePath: routeLine,
		Status:    domain.TripStatusPublished,
	}

	t.Run("falla si otro usuario que no es el conductor intenta cancelar el viaje", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockRouting := new(MockRoutingProvider)

		mockTripRepo.On("FindByID", ctx, "trip-to-cancel").Return(activeTrip, nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		err := svc.CancelTrip(ctx, "trip-to-cancel", "impostor-user")
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("cancela exitosamente el viaje por el conductor dueño", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockUserRepo := new(MockUserRepository)
		mockVehRepo := new(MockVehicleRepository)
		mockRouting := new(MockRoutingProvider)

		tripCopy := *activeTrip
		mockTripRepo.On("FindByID", ctx, "trip-to-cancel").Return(&tripCopy, nil)
		mockTripRepo.On("UpdateStatus", ctx, "trip-to-cancel", domain.TripStatusCancelled).Return(nil)

		svc := services.NewTripService(mockTripRepo, mockUserRepo, mockVehRepo, mockRouting, nil)

		err := svc.CancelTrip(ctx, "trip-to-cancel", "driver-owner")
		require.NoError(t, err)

		mockTripRepo.AssertExpectations(t)
	})
}

func TestTripService_CompleteTrip(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	activeTrip := &domain.Trip{
		ID:                 "trip-complete-1",
		DriverID:           "driver-owner",
		VehicleID:          "vehicle-123",
		Status:             domain.TripStatusInProgress,
		TotalSeatsOffered:  3,
		AvailableSeats:     1,
		PricePerSeat:       10000.0,
		DepartureTime:      now.Add(-2 * time.Hour),
		EstimatedArrivalTime: now.Add(-10 * time.Minute),
	}

	t.Run("falla si el viaje no existe", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		mockTripRepo.On("FindByID", ctx, "non-existent").Return(nil, domain.ErrTripNotFound)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil, mockBookingRepo, mockEscrowRepo)
		res, err := svc.CompleteTrip(ctx, "non-existent", "driver-owner")

		assert.ErrorIs(t, err, domain.ErrTripNotFound)
		assert.Nil(t, res)
	})

	t.Run("falla si el usuario no es el conductor del viaje (RF-01)", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockTripRepo.On("FindByID", ctx, "trip-complete-1").Return(activeTrip, nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil)
		res, err := svc.CompleteTrip(ctx, "trip-complete-1", "impostor-driver")

		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si el viaje ya estaba cancelado", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		cancelledTrip := *activeTrip
		cancelledTrip.Status = domain.TripStatusCancelled
		mockTripRepo.On("FindByID", ctx, "trip-complete-1").Return(&cancelledTrip, nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil)
		res, err := svc.CompleteTrip(ctx, "trip-complete-1", "driver-owner")

		assert.ErrorIs(t, err, domain.ErrInvalidTripStatusChange)
		assert.Nil(t, res)
	})

	t.Run("finaliza exitosamente el viaje, reservas confirmadas y libera el Escrow", func(t *testing.T) {
		mockTripRepo := new(MockTripRepository)
		mockBookingRepo := new(MockBookingRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		tripCopy := *activeTrip
		mockTripRepo.On("FindByID", ctx, "trip-complete-1").Return(&tripCopy, nil)
		mockTripRepo.On("UpdateStatus", ctx, "trip-complete-1", domain.TripStatusCompleted).Return(nil)

		confirmedBooking := &domain.Booking{
			ID:          "booking-101",
			TripID:      "trip-complete-1",
			PassengerID: "passenger-55",
			SeatsBooked: 2,
			TotalPrice:  20000.0,
			Status:      domain.BookingStatusConfirmed,
		}

		mockBookingRepo.On("GetActiveBookingsByTrip", ctx, "trip-complete-1").Return([]*domain.Booking{confirmedBooking}, nil)
		mockBookingRepo.On("UpdateStatus", ctx, "booking-101", domain.BookingStatusCompleted, (*string)(nil)).Return(nil)

		escrowTx, _ := domain.NewEscrowTransaction("escrow-101", "booking-101", "trip-complete-1", "passenger-55", "driver-owner", 20000.0, "ARS", "pi_test_101")
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-101").Return(escrowTx, nil)
		mockEscrowRepo.On("UpdateEscrowStatus", ctx, "escrow-101", domain.EscrowStatusReleased).Return(nil)

		svc := services.NewTripService(mockTripRepo, nil, nil, nil, nil, mockBookingRepo, mockEscrowRepo)
		res, err := svc.CompleteTrip(ctx, "trip-complete-1", "driver-owner")

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

