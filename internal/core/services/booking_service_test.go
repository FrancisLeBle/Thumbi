package services_test

import (
	"context"
	"errors"
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

var (
	testTripID      = uuid.MustParse("01960000-0000-7000-8000-000000000100")
	testVehicleID   = uuid.MustParse("01960000-0000-7000-8000-000000000200")
	testDriverID    = uuid.MustParse("01960000-0000-7000-8000-000000000300")
	testPassengerID = uuid.MustParse("01960000-0000-7000-8000-000000000400")
)

func createTestTrip(driverID uuid.UUID, availableSeats int, departureInHours int) *domain.Trip {
	now := time.Now().UTC()
	departure := now.Add(time.Duration(departureInHours) * time.Hour)
	arrival := departure.Add(4 * time.Hour)

	cOrig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	cDest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routePath, _ := domain.NewLineString([]domain.Coordinates{cOrig, cDest})

	trip, _ := domain.NewTrip(domain.TripCreationParams{
		ID:                   testTripID,
		DriverID:             driverID,
		VehicleID:            testVehicleID,
		OriginTitle:          "Buenos Aires",
		OriginCoords:         cOrig,
		DestinationTitle:     "Rosario",
		DestinationCoords:    cDest,
		RoutePath:            routePath,
		DepartureTime:        departure,
		EstimatedArrivalTime: arrival,
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    4,
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(150000, domain.CurrencyARS),
		EstimatedFuelCost:    domain.MoneyFromCents(1000000, domain.CurrencyARS),
		EstimatedTollCost:    domain.MoneyFromCents(200000, domain.CurrencyARS),
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

		sameDriverID := uuid.MustParse("01960000-0000-7000-8000-000000000301")
		trip := createTestTrip(sameDriverID, 3, 48)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    sameDriverID,
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

		trip := createTestTrip(testDriverID, 1, 48) // Solo 1 asiento disponible
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
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

		trip := createTestTrip(testDriverID, 3, 48)
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)

		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, testPassengerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.TripID == testTripID &&
				st.SeatsDelta == 2 &&
				st.ExpectedStatus == domain.TripStatusPublished &&
				st.ExpectedAvailableSeats == 3 &&
				st.NewStatus == domain.TripStatusPublished &&
				st.NewAvailableSeats == 1
		})).Return(true, nil)

		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("uuid.UUID"), expectedAmount).Return(&ports.PaymentIntentResult{
			GatewayRef:   "pi_test_123456",
			ClientSecret: "secret_test_xyz",
			Amount:       expectedAmount,
			Currency:     domain.CurrencyARS,
			Status:       "requires_payment_method",
		}, nil)

		userRepo.On("GetByID", ctx, testDriverID).Return(&domain.User{
			FirstName: "Carlos",
			LastName:  "Gomez",
		}, nil)
		vehicleRepo.On("GetByID", ctx, testVehicleID).Return(&domain.Vehicle{
			Brand:       "Toyota",
			Model:       "Corolla",
			PlateNumber: "AA123BB",
		}, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
			SeatsRequested: 2,
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.BookingStatusPendingPayment, res.Status)
		assert.Equal(t, 2, res.SeatsBooked)
		assert.Equal(t, domain.CurrencyARS, res.Currency)
		assert.Equal(t, expectedAmount, res.TotalPrice)
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

		hoggerID := uuid.MustParse("01960000-0000-7000-8000-000000000402")
		trip := createTestTrip(testDriverID, 4, 48)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)

		bookingRepo.On("CheckPassengerBookingConstraints", ctx, hoggerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 3, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    hoggerID,
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

		overlapPassengerID := uuid.MustParse("01960000-0000-7000-8000-000000000403")
		trip := createTestTrip(testDriverID, 4, 48)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, overlapPassengerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(true, 1, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    overlapPassengerID,
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrOverlappingTripBooking)
		bookingRepo.AssertNotCalled(t, "CreateWithHold", mock.Anything, mock.Anything, mock.Anything)
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

		errPassengerID := uuid.MustParse("01960000-0000-7000-8000-000000000404")
		trip := createTestTrip(testDriverID, 4, 48)
		dbErr := errors.New("connection refused")
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, errPassengerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, dbErr)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    errPassengerID,
			SeatsRequested: 1,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, dbErr)
		bookingRepo.AssertNotCalled(t, "CreateWithHold", mock.Anything, mock.Anything, mock.Anything)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("reintenta con decisión actualizada del dominio cuando CreateWithHold reporta conflicto optimista (applied = false)", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		initialTrip := createTestTrip(testDriverID, 3, 48)
		updatedTrip := createTestTrip(testDriverID, 2, 48) // Otro pasajero tomó 1 mientras tanto; quedan 2
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)

		tripRepo.On("FindByID", ctx, testTripID).Return(initialTrip, nil).Once()
		tripRepo.On("FindByID", ctx, testTripID).Return(updatedTrip, nil).Once()
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, testPassengerID, testTripID, initialTrip.DepartureTime, initialTrip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil).Once()

		// Intento 1: espera 3 asientos -> FULL no aplica (false)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.ExpectedAvailableSeats == 3 && st.NewStatus == domain.TripStatusPublished
		})).Return(false, nil).Once()

		// Intento 2: recarga viaje con 2 asientos -> dominio decide transición PUBLISHED -> FULL -> aplica (true)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.ExpectedAvailableSeats == 2 && st.NewStatus == domain.TripStatusFull && st.NewAvailableSeats == 0
		})).Return(true, nil).Once()

		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("uuid.UUID"), expectedAmount).Return(&ports.PaymentIntentResult{
			GatewayRef: "pi_retry_ok",
			Amount:     expectedAmount,
			Currency:   domain.CurrencyARS,
		}, nil)
		userRepo.On("GetByID", ctx, testDriverID).Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, testVehicleID).Return(nil, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
			SeatsRequested: 2,
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, "pi_retry_ok", res.PaymentIntent.GatewayRef)
		tripRepo.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
	})

	t.Run("retorna ErrConcurrentModification si se agotan los reintentos de CreateWithHold por contención", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		tripRepo.On("FindByID", ctx, testTripID).Return(createTestTrip(testDriverID, 3, 48), nil).Times(4)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, testPassengerID, testTripID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(false, 0, nil).Once()
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.AnythingOfType("domain.SeatTransition")).Return(false, nil).Times(3)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
			SeatsRequested: 2,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, domain.ErrConcurrentModification)
		bookingRepo.AssertExpectations(t)
	})
}

func TestBookingService_ConfirmBookingPayment(t *testing.T) {
	ctx := context.Background()

	bookingABCID := uuid.MustParse("01960000-0000-7000-8000-000000000501")
	bookingWebhookID := uuid.MustParse("01960000-0000-7000-8000-000000000502")
	bookingExpiredID := uuid.MustParse("01960000-0000-7000-8000-000000000503")
	bookingRaceID := uuid.MustParse("01960000-0000-7000-8000-000000000504")
	bookingTxID := uuid.MustParse("01960000-0000-7000-8000-000000000505")
	attackerID := uuid.MustParse("01960000-0000-7000-8000-000000000499")

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
			ID:              bookingABCID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     2,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip(testDriverID, 1, 48)
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)

		bookingRepo.On("FindByID", ctx, bookingABCID).Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, "pi_gateway_ref_999").Return(true, nil)
		bookingRepo.On("UpdateStatus", ctx, bookingABCID, domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		escrowRepo.On("CreateEscrow", ctx, mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == bookingABCID && e.Amount == expectedAmount && e.Currency == domain.CurrencyARS && e.Status == domain.EscrowStatusHeld
		})).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, testDriverID).Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, testVehicleID).Return(nil, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingABCID,
			RequestingUserID:  testPassengerID,
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
			ID:              bookingABCID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     2,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		bookingRepo.On("FindByID", ctx, bookingABCID).Return(booking, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		// Un atacante u otro usuario intenta confirmar la reserva
		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingABCID,
			RequestingUserID:  attackerID,
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
			ID:              bookingABCID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     2,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		bookingRepo.On("FindByID", ctx, bookingABCID).Return(booking, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		// Envía un payment reference de otra transacción
		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingABCID,
			RequestingUserID:  testPassengerID,
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
			ID:              bookingWebhookID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     1,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip(testDriverID, 1, 48)

		bookingRepo.On("FindByID", ctx, bookingWebhookID).Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, "pi_webhook_ref_777").Return(true, nil)
		bookingRepo.On("UpdateStatus", ctx, bookingWebhookID, domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		escrowRepo.On("CreateEscrow", ctx, mock.AnythingOfType("*domain.EscrowTransaction")).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, testDriverID).Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, testVehicleID).Return(nil, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingWebhookID,
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
			ID:              bookingExpiredID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     1,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             1 * time.Nanosecond, // Expirado de inmediato
		})
		time.Sleep(10 * time.Millisecond)

		bookingRepo.On("FindByID", ctx, bookingExpiredID).Return(booking, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingExpiredID,
			RequestingUserID:  testPassengerID,
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
			ID:              bookingRaceID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     2,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip(testDriverID, 1, 48)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, bookingRaceID).Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, intentRef).Return(true, nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		// Simula que el worker expiró la reserva en BD justo antes del UPDATE condicional
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), bookingRaceID, domain.BookingStatusConfirmed, (*string)(nil)).Return(domain.ErrBookingExpired)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingRaceID,
			RequestingUserID:  testPassengerID,
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
			ID:              bookingTxID,
			TripID:          testTripID,
			PassengerID:     testPassengerID,
			DriverID:        testDriverID,
			SeatsBooked:     2,
			Currency:        domain.CurrencyARS,
			UnitPrice:       domain.MoneyFromCents(150000, domain.CurrencyARS),
			PaymentIntentID: &intentRef,
			TTL:             15 * time.Minute,
		})

		trip := createTestTrip(testDriverID, 1, 48)
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, bookingTxID).Return(booking, nil)
		gateway.On("ConfirmPayment", ctx, intentRef).Return(true, nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), bookingTxID, domain.BookingStatusConfirmed, (*string)(nil)).Return(nil)
		escrowRepo.On("CreateEscrow", mock.MatchedBy(hasTx), mock.MatchedBy(func(e *domain.EscrowTransaction) bool {
			return e.BookingID == bookingTxID && e.Amount == expectedAmount && e.Currency == domain.CurrencyARS
		})).Return(nil)
		notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)
		userRepo.On("GetByID", ctx, testDriverID).Return(&domain.User{FirstName: "Carlos", LastName: "Gomez"}, nil)
		vehicleRepo.On("GetByID", ctx, testVehicleID).Return(nil, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		res, err := service.ConfirmBookingPayment(ctx, ports.ConfirmBookingPaymentInput{
			BookingID:         bookingTxID,
			RequestingUserID:  testPassengerID,
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

	bookingEarlyID := uuid.MustParse("01960000-0000-7000-8000-000000000601")
	bookingLateID := uuid.MustParse("01960000-0000-7000-8000-000000000602")
	bookingTxCancelID := uuid.MustParse("01960000-0000-7000-8000-000000000603")
	bookingTxErrID := uuid.MustParse("01960000-0000-7000-8000-000000000604")
	escrow1ID := uuid.MustParse("01960000-0000-7000-8000-000000000611")
	escrow2ID := uuid.MustParse("01960000-0000-7000-8000-000000000612")

	t.Run("cancelación con > 24hs otorga 100% de reembolso al pasajero y restituye asientos", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		escrowRepo := new(MockEscrowRepository)
		userRepo := new(MockUserRepository)
		vehicleRepo := new(MockVehicleRepository)
		gateway := new(MockPaymentGateway)
		notifier := new(MockNotifier)

		now := time.Now().UTC()
		trip := createTestTrip(testDriverID, 1, 48) // Partida en 48 horas (> 24hs)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingEarlyID,
			TripID:      trip.ID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(200000, domain.CurrencyARS),
		})
		_ = booking.Confirm(now)

		totalAmount := domain.MoneyFromCents(400000, domain.CurrencyARS)
		zeroAmount := domain.ZeroMoney(domain.CurrencyARS)
		escrow, _ := domain.NewEscrowTransaction(escrow1ID, booking.ID, trip.ID, booking.PassengerID, trip.DriverID, totalAmount, "pi_ref_123")

		bookingRepo.On("FindByID", ctx, bookingEarlyID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, bookingEarlyID, domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", ctx, mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.TripID == trip.ID && st.SeatsDelta == 2 && st.ExpectedStatus == domain.TripStatusPublished && st.NewStatus == domain.TripStatusPublished
		})).Return(true, nil)
		escrowRepo.On("FindByBookingID", ctx, bookingEarlyID).Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_123", totalAmount, string(domain.RefundTypePassengerEarly)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_test_999",
			Amount:           totalAmount,
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, escrow1ID, domain.EscrowStatusRefundedFull).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == totalAmount && r.DriverCompensationAmount == zeroAmount && r.RefundType == domain.RefundTypePassengerEarly
		})).Return(nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        bookingEarlyID,
			RequestingUserID: testPassengerID,
			Reason:           "Cambio de planes",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, totalAmount, refund.PassengerRefundAmount)
		assert.Equal(t, zeroAmount, refund.DriverCompensationAmount)
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
		trip := createTestTrip(testDriverID, 1, 6) // Partida en solo 6 horas (< 24hs)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingLateID,
			TripID:      trip.ID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(200000, domain.CurrencyARS),
		})
		_ = booking.Confirm(now)

		totalAmount := domain.MoneyFromCents(400000, domain.CurrencyARS)
		halfAmount := domain.MoneyFromCents(200000, domain.CurrencyARS)
		escrow, _ := domain.NewEscrowTransaction(escrow2ID, booking.ID, trip.ID, booking.PassengerID, trip.DriverID, totalAmount, "pi_ref_456")

		bookingRepo.On("FindByID", ctx, bookingLateID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		bookingRepo.On("UpdateStatus", ctx, bookingLateID, domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", ctx, mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.TripID == trip.ID && st.SeatsDelta == 2 && st.ExpectedStatus == domain.TripStatusPublished && st.NewStatus == domain.TripStatusPublished
		})).Return(true, nil)
		escrowRepo.On("FindByBookingID", ctx, bookingLateID).Return(escrow, nil)
		gateway.On("ProcessRefund", ctx, "pi_ref_456", halfAmount, string(domain.RefundTypePassengerLate)).Return(&ports.RefundResult{
			GatewayRefundRef: "re_late_456",
			Amount:           halfAmount,
			Status:           "succeeded",
		}, nil)
		escrowRepo.On("UpdateEscrowStatus", ctx, escrow2ID, domain.EscrowStatusRefundedPartial).Return(nil)
		escrowRepo.On("RecordRefund", ctx, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.PassengerRefundAmount == halfAmount && r.DriverCompensationAmount == halfAmount && r.RefundType == domain.RefundTypePassengerLate
		})).Return(nil)

		settlement := services.NewSettlementService(escrowRepo, gateway)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        bookingLateID,
			RequestingUserID: testPassengerID,
			Reason:           "Imprevisto de último momento",
		})

		require.NoError(t, err)
		assert.NotNil(t, refund)
		assert.Equal(t, halfAmount, refund.PassengerRefundAmount)
		assert.Equal(t, halfAmount, refund.DriverCompensationAmount)
		assert.Equal(t, domain.RefundTypePassengerLate, refund.RefundType)

		tripRepo.AssertExpectations(t)
		escrowRepo.AssertExpectations(t)
		gateway.AssertExpectations(t)
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

		trip := createTestTrip(testDriverID, 0, 48)
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingTxCancelID,
			TripID:      trip.ID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})

		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		bookingRepo.On("FindByID", ctx, bookingTxCancelID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), bookingTxCancelID, domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), mock.MatchedBy(func(st domain.SeatTransition) bool {
			return st.TripID == trip.ID && st.SeatsDelta == 2 && st.ExpectedStatus == domain.TripStatusFull && st.NewStatus == domain.TripStatusPublished
		})).Return(true, nil)
		escrowRepo.On("FindByBookingID", ctx, bookingTxCancelID).Return(nil, domain.ErrEscrowNotFound)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        bookingTxCancelID,
			RequestingUserID: testPassengerID,
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

		trip := createTestTrip(testDriverID, 1, 48)
		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingTxErrID,
			TripID:      trip.ID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})

		dbErr := errors.New("deadlock detected")
		bookingRepo.On("FindByID", ctx, bookingTxErrID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, trip.ID).Return(trip, nil)
		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.Anything, bookingTxErrID, domain.BookingStatusCancelledByPassenger, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.Anything, mock.AnythingOfType("domain.SeatTransition")).Return(false, dbErr)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		refund, err := service.CancelBooking(ctx, ports.CancelBookingInput{
			BookingID:        bookingTxErrID,
			RequestingUserID: testPassengerID,
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

		trip := createTestTrip(testDriverID, 3, 48)
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}

		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, testPassengerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.AnythingOfType("domain.SeatTransition")).Return(true, nil)
		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("uuid.UUID"), expectedAmount).Return(nil, errors.New("gateway timeout"))

		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), mock.AnythingOfType("uuid.UUID"), domain.BookingStatusRejected, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), mock.AnythingOfType("domain.SeatTransition")).Return(true, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
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

		trip := createTestTrip(testDriverID, 3, 48)
		expectedAmount := domain.MoneyFromCents(300000, domain.CurrencyARS)
		hasTx := func(c context.Context) bool {
			v, _ := c.Value(mockTxCtxKey{}).(bool)
			return v
		}
		intentErr := errors.New("db write error on payment_intent_id")

		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		bookingRepo.On("CheckPassengerBookingConstraints", ctx, testPassengerID, testTripID, trip.DepartureTime, trip.EstimatedArrivalTime, mock.AnythingOfType("time.Time")).Return(false, 0, nil)
		bookingRepo.On("CreateWithHold", ctx, mock.AnythingOfType("*domain.Booking"), mock.AnythingOfType("domain.SeatTransition")).Return(true, nil)
		gateway.On("CreatePaymentIntent", ctx, mock.AnythingOfType("uuid.UUID"), expectedAmount).Return(&ports.PaymentIntentResult{
			GatewayRef: "pi_intent_999",
			Amount:     expectedAmount,
			Currency:   domain.CurrencyARS,
		}, nil)
		bookingRepo.On("UpdatePaymentIntent", ctx, mock.AnythingOfType("uuid.UUID"), "pi_intent_999").Return(intentErr)

		txManager.On("WithinTransaction", ctx).Return(nil)
		bookingRepo.On("UpdateStatus", mock.MatchedBy(hasTx), mock.AnythingOfType("uuid.UUID"), domain.BookingStatusRejected, mock.Anything).Return(nil)
		tripRepo.On("ReleaseSeats", mock.MatchedBy(hasTx), mock.AnythingOfType("domain.SeatTransition")).Return(true, nil)

		settlement := services.NewSettlementService(escrowRepo, gateway, txManager)
		service := services.NewBookingService(bookingRepo, tripRepo, userRepo, vehicleRepo, settlement, notifier, txManager)

		res, err := service.CreateBooking(ctx, ports.CreateBookingInput{
			TripID:         testTripID,
			PassengerID:    testPassengerID,
			SeatsRequested: 2,
		})

		assert.Nil(t, res)
		assert.ErrorIs(t, err, intentErr)
		txManager.AssertExpectations(t)
		bookingRepo.AssertExpectations(t)
		tripRepo.AssertExpectations(t)
	})
}

