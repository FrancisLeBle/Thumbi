package services_test

import (
	"context"
	"errors"
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
		PricePerSeat:         domain.Money(150000),
		EstimatedFuelCost:    domain.Money(1000000),
		EstimatedTollCost:    domain.Money(200000),
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
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-2", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking")).Return(nil)

		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("string"), domain.Money(300000), "USD").Return(&ports.PaymentIntentResult{
			GatewayRef:   "pi_test_123456",
			ClientSecret: "secret_test_xyz",
			Amount:       domain.Money(300000),
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
		assert.Equal(t, domain.Money(300000), res.TotalPrice)
		assert.NotNil(t, res.PaymentIntent)
		assert.Equal(t, "pi_test_123456", res.PaymentIntent.GatewayRef)
		assert.Equal(t, "Carlos Gomez", res.TripSummary.DriverFullName)
		assert.Equal(t, "Toyota", res.TripSummary.VehicleBrand)

		tripRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
	})

	t.Run("rechaza la reserva si el pasajero supera el límite de reservas pendientes activas (MaxPendingBookingsPerPassenger)", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-1", 4, 48)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)

		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-hogger", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 3, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-hogger",
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrMaxPendingBookingsExceeded)
		tripRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("rechaza la reserva si existe solapamiento horario con otra reserva activa del pasajero", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-1", 4, 48)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-overlap", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(true, 1, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-overlap",
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrOverlappingTripBooking)
		bookingRepo.AssertNotCalled(t, "CreateWithHold", mock.Anything, mock.Anything)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("propaga el error si falla la consulta de verificación de solapamiento", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		trip := createTestTrip("driver-1", 4, 48)
		dbErr := errors.New("connection refused")
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-err", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, dbErr)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-err",
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, dbErr)
		bookingRepo.AssertNotCalled(t, "CreateWithHold", mock.Anything, mock.Anything)
		bookingRepo.AssertExpectations(t)
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

		intentRef := "pi_gateway_ref_999"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-abc",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     2,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip("driver-1", 1, 48)

		bookingRepo.On("FindByID", ctx, "booking-abc").Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, "pi_gateway_ref_999").Return(true, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-abc", domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		escrowRepo.On("CreateEscrow", ctx, mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == "booking-abc" && e.Amount == domain.Money(300000) && e.Status == domain.EscrowStatusHeld
		})).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, "driver-1").Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, "vehicle-100").Return(nil, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-abc",
			RequestingUserID:  "passenger-1",
			PaymentGatewayRef: "pi_gateway_ref_999",
		})

		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusConfirmed, res.Status)
		assert.NotNil(t, res.ConfirmedAt)

		gateway.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("rechaza confirmación si el solicitante no es el pasajero de la reserva (ErrUnauthorized)", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		intentRef := "pi_gateway_ref_999"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-abc",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     2,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		bookingRepo.On("FindByID", ctx, "booking-abc").Return(booking, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		// Un atacante u otro usuario intenta confirmar la reserva
		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-abc",
			RequestingUserID:  "attacker-or-wrong-user",
			PaymentGatewayRef: "pi_gateway_ref_999",
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
	})

	t.Run("rechaza confirmación si la referencia de pasarela no coincide con el PaymentIntentID de la reserva", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		intentRef := "pi_legitimate_intent_123"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-abc",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     2,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		bookingRepo.On("FindByID", ctx, "booking-abc").Return(booking, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		// Envía un payment reference de otra transacción
		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-abc",
			RequestingUserID:  "passenger-1",
			PaymentGatewayRef: "pi_forged_or_different_ref",
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrInvalidPaymentGatewayRef)
	})

	t.Run("confirma exitosamente cuando la confirmación proviene de un webhook firmado por la pasarela", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		intentRef := "pi_webhook_ref_777"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-webhook-test",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     1,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip("driver-1", 1, 48)

		bookingRepo.On("FindByID", ctx, "booking-webhook-test").Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, "pi_webhook_ref_777").Return(true, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-webhook-test", domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		escrowRepo.On("CreateEscrow", ctx, mock.AnythingOfType("*domain.EscrowTransaction")).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, "driver-1").Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, "vehicle-100").Return(nil, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-webhook-test",
			PaymentGatewayRef: "pi_webhook_ref_777",
			IsWebhook:         true,
		})

		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusConfirmed, res.Status)
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
		intentRef := "pi_gateway_ref"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-expired",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     1,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             1 * time.Nanosecond, // Expirado de inmediato
		})
		time.Sleep(10 * time.Millisecond)

		bookingRepo.On("FindByID", ctx, "booking-expired").Return(booking, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-expired",
			RequestingUserID:  "passenger-1",
			PaymentGatewayRef: "pi_gateway_ref",
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrBookingExpired)
	})

	t.Run("ejecuta UpdateStatus y CreateEscrow dentro de la misma transacción y aborta si expiró concurrentemente", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		intentRef := "pi_race_ref_123"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-race-expired",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     2,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip("driver-1", 1, 48)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, "booking-race-expired").Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, intentRef).Return(true, nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		// Simula que el worker expiró la reserva en BD justo antes del UPDATE condicional
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), "booking-race-expired", domain.BookingStatusConfirmed, (*string)(nil)).Return(domain.ErrBookingExpired)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-race-expired",
			RequestingUserID:  "passenger-1",
			PaymentGatewayRef: intentRef,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrBookingExpired)
		escrowRepo.AssertNotCalled(t, "CreateEscrow", mock.Anything, mock.Anything)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("ejecuta UpdateStatus y CreateEscrow dentro de la misma transacción en confirmación exitosa", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		intentRef := "pi_tx_confirm_456"
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:              "booking-tx-confirm",
			TripID:          "trip-100",
			PassengerID:     "passenger-1",
			DriverID:        "driver-1",
			SeatsBooked:     2,
			UnitPrice:       domain.Money(150000),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip("driver-1", 1, 48)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, "booking-tx-confirm").Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, intentRef).Return(true, nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), "booking-tx-confirm", domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		escrowRepo.On("CreateEscrow", mock.MatchedBy(hasTx), mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == "booking-tx-confirm" && e.Amount == domain.Money(300000)
		})).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, "driver-1").Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, "vehicle-100").Return(nil, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         "booking-tx-confirm",
			RequestingUserID:  "passenger-1",
			PaymentGatewayRef: intentRef,
		})

		require.NoError(t, err)
		assert.Equal(t, domain.BookingStatusConfirmed, res.Status)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
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
			UnitPrice:   domain.Money(200000),
		})
		_ = booking.Confirm(now)

		escrow, _ := domain.NewEscrowTransaction("escrow-1", booking.ID, trip.ID, booking.PassengerID, trip.DriverID, domain.Money(400000), "USD", "pi_ref_123")

		bookingRepo.On("FindByID", ctx, "booking-cancel-early").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-cancel-early", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", ctx, trip.ID, 2).Return(nil)
		escrowRepo.On("FindByBookingID", ctx, "booking-cancel-early").Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_123", domain.Money(400000), string(domain.RefundTypePassengerEarly)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_test_999",
			Amount:           domain.Money(400000),
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, "escrow-1", domain.EscrowStatusRefundedFull).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == domain.Money(400000) && r.DriverCompensationAmount == domain.Money(0) && r.RefundType == domain.RefundTypePassengerEarly
		})).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-cancel-early",
			RequestingUserID: "passenger-1",
			Reason:           "Cambio de planes",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, domain.Money(400000), refund.PassengerRefundAmount)
		assert.Equal(t, domain.Money(0), refund.DriverCompensationAmount)
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
			UnitPrice:   domain.Money(200000),
		})
		_ = booking.Confirm(now)

		escrow, _ := domain.NewEscrowTransaction("escrow-2", booking.ID, trip.ID, booking.PassengerID, trip.DriverID, domain.Money(400000), "USD", "pi_ref_456")

		bookingRepo.On("FindByID", ctx, "booking-cancel-late").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, "booking-cancel-late", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", ctx, trip.ID, 2).Return(nil)
		escrowRepo.On("FindByBookingID", ctx, "booking-cancel-late").Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_456", domain.Money(200000), string(domain.RefundTypePassengerLate)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_late_456",
			Amount:           domain.Money(200000),
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, "escrow-2", domain.EscrowStatusRefundedPartial).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == domain.Money(200000) && r.DriverCompensationAmount == domain.Money(200000) && r.RefundType == domain.RefundTypePassengerLate
		})).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-cancel-late",
			RequestingUserID: "passenger-1",
			Reason:           "Imprevisto de último momento",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, domain.Money(200000), refund.PassengerRefundAmount)
		assert.Equal(t, domain.Money(200000), refund.DriverCompensationAmount)
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

	t.Run("cancelación ejecuta UpdateStatus y ReleaseSeats dentro de la misma transacción y nunca llama Save", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		trip := createTestTrip("driver-1", 0, 48)
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-tx-cancel",
			TripID:      trip.ID,
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 2,
			UnitPrice:   domain.Money(150000),
		})

		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, "booking-tx-cancel").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), "booking-tx-cancel", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), trip.ID, 2).Return(nil)
		escrowRepo.On("FindByBookingID", ctx, "booking-tx-cancel").Return(nil, domain.ErrEscrowNotFound)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-tx-cancel",
			RequestingUserID: "passenger-1",
			Reason:           "Cancelación antes de pagar",
		})

		require.NoError(t, err)
		assert.Nil(t, refund)
		tripRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		tripRepo.AssertExpectations(t)
	})

	t.Run("cancelación propaga error si ReleaseSeats falla dentro de la transacción", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		trip := createTestTrip("driver-1", 1, 48)
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-tx-err",
			TripID:      trip.ID,
			PassengerID: "passenger-1",
			DriverID:    "driver-1",
			SeatsBooked: 2,
			UnitPrice:   domain.Money(150000),
		})

		dbErr := errors.New("deadlock detected")
		bookingRepo.On("FindByID", ctx, "booking-tx-err").Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.Anything, "booking-tx-err", domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.Anything, trip.ID, 2).Return(dbErr)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        "booking-tx-err",
			RequestingUserID: "passenger-1",
			Reason:           "Error de BD",
		})

		assert.Nil(t, refund)
		assert.ErrorIs(t, err, dbErr)
		tripRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
	})

	t.Run("fallo en CreatePaymentIntent revierte reserva y libera asientos en la misma transacción", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		trip := createTestTrip("driver-1", 3, 48)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-2", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking")).Return(nil)
		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("string"), domain.Money(300000), "USD").Return(nil, errors.New("gateway timeout"))

		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), mock.AnythingOfType("string"), domain.BookingStatusRejected, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), "trip-100", 2).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-2",
			SeatsRequested: 2,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrPaymentFailed)
		tripRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		tripRepo.AssertExpectations(t)
	})

	t.Run("fallo en UpdatePaymentIntent revierte reserva y libera asientos en la misma transacción", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)
		txManager := new(MockTransactionManager)

		trip := createTestTrip("driver-1", 3, 48)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}
		intentErr := errors.New("db write error on payment_intent_id")

		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, "passenger-2", "trip-100", trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking")).Return(nil)
		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("string"), domain.Money(300000), "USD").Return(&ports.PaymentIntentResult{
			GatewayRef: "pi_intent_999",
			Amount:     domain.Money(300000),
			Currency:   "USD",
		}, nil)
		bookingRepo.On("UpdatePaymentIntent", ctx, mock.AnythingOfType("string"), "pi_intent_999").Return(intentErr)

		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), mock.AnythingOfType("string"), domain.BookingStatusRejected, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), "trip-100", 2).Return(nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier, txManager)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         "trip-100",
			PassengerID:    "passenger-2",
			SeatsRequested: 2,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, intentErr)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		tripRepo.AssertExpectations(t)
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
			UnitPrice:   domain.Money(150000),
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
			UnitPrice:   domain.Money(150000),
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

	t.Run("falla con ErrDriverPhoneRequired si el conductor no tiene teléfono registrado", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-no-phone",
			TripID:      "trip-100",
			PassengerID: "passenger-1",
			DriverID:    "driver-no-phone",
			SeatsBooked: 1,
			UnitPrice:   domain.Money(150000),
		})
		trip := createTestTrip("driver-no-phone", 2, 24)
		driverWithoutPhone := &domain.User{
			ID:        "driver-no-phone",
			FirstName: "Lucía",
			LastName:  "Pérez",
			Phone:     "   ",
		}

		bookingRepo.On("FindByID", ctx, "booking-no-phone").Return(booking, nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		userRepo.On("GetByID", ctx, "driver-no-phone").Return(driverWithoutPhone, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		linkDTO, err := service.GetContactLink(ctx, "booking-no-phone", "passenger-1")
		assert.ErrorIs(t, err, domain.ErrDriverPhoneRequired)
		assert.Nil(t, linkDTO)
	})

	t.Run("falla con ErrDriverPhoneRequired si el teléfono del conductor no contiene suficientes dígitos válidos", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          "booking-bad-phone",
			TripID:      "trip-100",
			PassengerID: "passenger-1",
			DriverID:    "driver-bad-phone",
			SeatsBooked: 1,
			UnitPrice:   domain.Money(150000),
		})
		trip := createTestTrip("driver-bad-phone", 2, 24)
		driverWithInvalidPhone := &domain.User{
			ID:        "driver-bad-phone",
			FirstName: "Pedro",
			LastName:  "Sánchez",
			Phone:     "---",
		}

		bookingRepo.On("FindByID", ctx, "booking-bad-phone").Return(booking, nil)
		tripRepo.On("FindByID", ctx, "trip-100").Return(trip, nil)
		userRepo.On("GetByID", ctx, "driver-bad-phone").Return(driverWithInvalidPhone, nil)

		service := services.NewBookingService(bookingRepo, escrowRepo, tripRepo, userRepo, vehicleRepo, gateway, notifier)

		linkDTO, err := service.GetContactLink(ctx, "booking-bad-phone", "passenger-1")
		assert.ErrorIs(t, err, domain.ErrDriverPhoneRequired)
		assert.Nil(t, linkDTO)
	})
}

