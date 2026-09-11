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

func createTestTrip(driverID string, availableSeats int, departureInHours int) *domain.Trip {
	now := time.Now().UTC()
	departure := now.Add(time.Duration(departureInHours) * time.Hour)
	arrival := departure.Add(4 * time.Hour)

	trip, _ := domain.NewTrip(domain.TripParams{
		ID:                   "trip-100",
		DriverID:             driverID,
		VehicleID:            "vehicle-100",
		OriginTitle:          "Buenos Aires",
		OriginCoords:         domain.Coordinates{Latitude: -34.6037, Longitude: -58.3816},
		DestinationTitle:     "Rosario",
		DestinationCoords:    domain.Coordinates{Latitude: -32.9468, Longitude: -60.6393},
		RoutePath:            domain.RouteGeometry{Coordinates: [][]float64{{-58.3816, -34.6037}, {-60.6393, -32.9468}}},
		DepartureTime:        departure,
		EstimatedArrivalTime: arrival,
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    4,
		PricePerSeat:         1500.0,
		EstimatedFuelCost:    10000.0,
		EstimatedTollCost:    2000.0,
		MaxVehicleSeats:      5,
	}, now)
	_ = trip.UpdateAvailableSeats(availableSeats)
	return trip
}

func TestBookingService_CreateBooking(t *testing.T) {
	ctx := context.Background()

	t.Run("rechaza la reserva si el pasajero es el propio conductor del viaje (Invariante RF-02)", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-same", 3, 48)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "driver-same",
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrCannotBookOwnTrip)
		tripRepo.AssertExpectations(t)
	})

	t.Run("rechaza la reserva si las plazas solicitadas superan los asientos disponibles", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-1", 1, 48) // Solo 1 asiento disponible
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-2",
			SeatsRequested: 2, // Solicita 2 asientos
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInsufficientSeats)
		tripRepo.AssertExpectations(t)
	})

	t.Run("crea exitosamente una reserva en estado PENDING_PAYMENT con PaymentIntent vinculado", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-1", 3, 48)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		bookingRepo.On("GetPassengerBookings", ctx, "passenger-2", 20, 0).Return([]*domain.Booking{}, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking")).Return(nil)

		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("string"), 3000.0, "USD").Return(&ports.PaymentIntentResult{
			GatewayRef:   "pi_test_123456",
			ClientSecret: "secret_test_xyz",
			Amount:       3000.0,
			Currency:     "USD",
			Status:       "requires_payment_method",
		}, nil)

		userRepo.On("GetByID", ctx, "driver-1").Return(&domain.User{
			FirstName: "Carlos",
			LastName:  "Gomez",
		}, nil)
		vehicleRepo.On("GetByID", ctx, "vehicle-100").Return(&domain.Vehicle{
			Brand: "Toyota",
			Model: "Corolla",
			Plate: "AA123BB",
		}, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-2",
			SeatsRequested: 2,
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.BookingStatusPendingPayment, res.Status)
		assert.Equal(t, 2, res.SeatsBooked)
		assert.Equal(t, 3000.0, res.TotalPrice)
		assert.NotNil(t, res.PaymentIntent)
		assert.Equal(t, "pi_test_123456", res.PaymentIntent.GatewayRef)
		assert.Equal(t, "Carlos Gomez", res.TripSummary.DriverFullName)
		assert.Equal(t, "Toyota", res.TripSummary.VehicleBrand)

		tripRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})
}

func TestBookingService_ConfirmBookingPayment(t *testing.T) {
	ctx := context.Background()

	t.Run("confirma exitosamente el pago y genera la transacción de custodia Escrow en estado HELD", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-abc",
			TripID:      "trip-100",
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 2,
			UnitPrice:   1500.0,
			TTL:         15 * time.Minute,
		})

		trip := createTestTrip("driver-1", 1, 48)

		bookingRepo.On("FindByID", ctx, "booking-abc").Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, "pi_gateway_ref_999").Return(true, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-abc", domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		escrowRepo.On("CreateEscrow", ctx, mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == "booking-abc" && e.Amount == 3000.0 && e.Status == domain.EscrowStatusHeld
		})).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, "driver-1").Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, "vehicle-100").Return(nil, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-abc",
			PaymentGatewayRef: "pi_gateway_ref_999",
		})

		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusConfirmed, res.Status)
		assert.NotNil(t, res.ConfirmedAt)

		gateway.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("rechaza confirmación si la reserva expiró por exceder los 15 minutos de TTL", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		// Booking expirado
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-expired",
			TripID:      "trip-100",
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 1,
			UnitPrice:   1500.0,
			TTL:         1 * time.Nanosecond, // Expirado de inmediato
		})
		time.Sleep(10 * time.Millisecond)

		bookingRepo.On("FindByID", ctx, "booking-expired").Return(booking, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-expired",
			PaymentGatewayRef: "pi_gateway_ref",
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrBookingExpired)
	})
}

func TestBookingService_CancelBooking(t *testing.T) {
	ctx := context.Background()

	t.Run("cancelación con > 24hs otorga 100% de reembolso al pasajero y restituye asientos", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		now := time.Now().UTC()
		trip := createTestTrip("driver-1", 1, 48) // Partida en 48 horas (> 24hs)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-cancel-early",
			TripID:      trip.ID,
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 2,
			UnitPrice:   2000.0,
		})
		_ = booking.Confirm(now)

		escrow, _ := domain.NewEscrowTransaction("escrow-1", booking.ID, trip.ID, booking.PassengerID, trip.DriverID, 4000.0, "USD", "pi_ref_123")

		bookingRepo.On("FindByID", ctx, "booking-cancel-early").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-cancel-early", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("Save", ctx, mock.MatchedBy(func(tr *domain.Trip) bool {
			return tr.AvailableSeats == 3 // 1 + 2 restituidos
		})).Return(nil)
		escrowRepo.On("FindByBookingID", ctx, "booking-cancel-early").Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_123", 4000.0, string(domain.RefundTypePassengerEarly)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_test_999",
			Amount:           4000.0,
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, "escrow-1", domain.EscrowStatusRefundedFull).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == 4000.0 && r.DriverCompensationAmount == 0.0 && r.RefundType == domain.RefundTypePassengerEarly
		})).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-cancel-early",
			RequestingUserID: "passenger-1",
			Reason:           "Cambio de planes",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, 4000.0, refund.PassengerRefundAmount)
		assert.Equal(t, 0.0, refund.DriverCompensationAmount)
		assert.Equal(t, domain.RefundTypePassengerEarly, refund.RefundType)

		tripRepo.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})

	t.Run("cancelación con < 24hs aplica penalización tardía con 50% al pasajero y 50% al conductor", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		now := time.Now().UTC()
		trip := createTestTrip("driver-1", 1, 6) // Partida en solo 6 horas (< 24hs)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-cancel-late",
			TripID:      trip.ID,
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 2,
			UnitPrice:   2000.0,
		})
		_ = booking.Confirm(now)

		escrow, _ := domain.NewEscrowTransaction("escrow-2", booking.ID, trip.ID, booking.PassengerID, trip.DriverID, 4000.0, "USD", "pi_ref_456")

		bookingRepo.On("FindByID", ctx, "booking-cancel-late").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-cancel-late", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("Save", ctx, mock.MatchedBy(func(tr *domain.Trip) bool {
			return tr.AvailableSeats == 3 // 1 + 2 restituidos
		})).Return(nil)
		escrowRepo.On("FindByBookingID", ctx, "booking-cancel-late").Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_456", 2000.0, string(domain.RefundTypePassengerLate)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_late_456",
			Amount:           2000.0,
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, "escrow-2", domain.EscrowStatusRefundedPartial).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == 2000.0 && r.DriverCompensationAmount == 2000.0 && r.RefundType == domain.RefundTypePassengerLate
		})).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-cancel-late",
			RequestingUserID: "passenger-1",
			Reason:           "Imprevisto de último momento",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, 2000.0, refund.PassengerRefundAmount)
		assert.Equal(t, 2000.0, refund.DriverCompensationAmount)
		assert.Equal(t, domain.RefundTypePassengerLate, refund.RefundType)

		tripRepo.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})

	t.Run("expiración masiva mediante ProcessExpiredBookings", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		bookingRepo.On("ExpirePendingBookings", ctx, mock.AnythingOfType("time.Time")).Return(3, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		expiredCount, err := service.ProcessExpiredBookings(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, expiredCount)
		bookingRepo.AssertExpectations(t)
	})
}

func TestBookingService_GetContactLink(t *testing.T) {
	ctx := context.Background()

	t.Run("genera exitosamente el link de WhatsApp con el conductor y datos del viaje", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		bookingID := "booking-99"
		passengerID := "passenger-1"
		driverID := "driver-1"

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingID,
			TripID:      "trip-100",
			PassengerID: passengerID,
			DriverID:    driverID,
			SeatsBooked: 2,
			UnitPrice:   1500.0,
		})

		trip := createTestTrip(driverID, 2, 24)

		driverUser := &domain.User{
			ID:        driverID,
			FirstName: "Carlos",
			LastName:  "Gómez",
			Phone:     "+54 9 11 9876-5432",
		}

		passengerUser := &domain.User{
			ID:        passengerID,
			FirstName: "María",
			LastName:  "López",
		}

		bookingRepo.On("FindByID", ctx, bookingID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		userRepo.On("GetByID", ctx, driverID).Return(driverUser, nil)
		userRepo.On("GetByID", ctx, passengerID).Return(passengerUser, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		linkDTO, err := service.GetContactLink(ctx, bookingID, passengerID)
		require.NoError(t, err)
		require.NotNil(t, linkDTO)

		assert.Equal(t, "+54 9 11 9876-5432", linkDTO.DriverPhone)
		assert.Contains(t, linkDTO.WhatsAppURL, "https://wa.me/5491198765432?text=")
		assert.Contains(t, linkDTO.WhatsAppURL, "Reserva")
		assert.Contains(t, linkDTO.WhatsAppURL, "booking-99")
	})

	t.Run("rechaza con ErrUnauthorized si el solicitante no es el pasajero titular", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-99",
			TripID:      "trip-100",
			PassengerID: "passenger-original",
			DriverID:    "driver-1",
			SeatsBooked: 1,
			UnitPrice:   1500.0,
		})

		bookingRepo.On("FindByID", ctx, "booking-99").Return(booking, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		linkDTO, err := service.GetContactLink(ctx, "booking-99", "intruder-user")
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, linkDTO)
	})

	t.Run("falla con ErrBookingNotFound si la reserva no existe", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		bookingRepo.On("FindByID", ctx, "non-existent").Return(nil, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		linkDTO, err := service.GetContactLink(ctx, "non-existent", "passenger-1")
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
		assert.Nil(t, linkDTO)
	})
}

