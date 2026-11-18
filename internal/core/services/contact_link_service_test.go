package services_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/services"
)

func TestContactLinkService_GetContactLink(t *testing.T) {
	ctx := context.Background()

	booking99ID := uuid.MustParse("01960000-0000-7000-8000-000000000799")
	missingBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000798")
	bookingNoPhoneID := uuid.MustParse("01960000-0000-7000-8000-000000000797")
	bookingBadPhoneID := uuid.MustParse("01960000-0000-7000-8000-000000000796")
	intruderID := uuid.MustParse("01960000-0000-7000-8000-000000000795")
	driverNoPhoneID := uuid.MustParse("01960000-0000-7000-8000-000000000794")
	driverBadPhoneID := uuid.MustParse("01960000-0000-7000-8000-000000000793")

	t.Run("genera exitosamente el link de WhatsApp con el conductor y datos del viaje", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		userRepo := new(MockUserRepository)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          booking99ID,
			TripID:      testTripID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 2,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})

		trip := createTestTrip(testDriverID, 2, 24)

		driverUser := &domain.User{
			ID:        testDriverID,
			FirstName: "Carlos",
			LastName:  "Gómez",
			Phone:     "+54 9 11 9876-5432",
		}

		passengerUser := &domain.User{
			ID:        testPassengerID,
			FirstName: "María",
			LastName:  "López",
		}

		bookingRepo.On("FindByID", ctx, booking99ID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		userRepo.On("GetByID", ctx, testDriverID).Return(driverUser, nil)
		userRepo.On("GetByID", ctx, testPassengerID).Return(passengerUser, nil)

		service := services.NewContactLinkService(bookingRepo, tripRepo, userRepo)

		linkDTO, err := service.GetContactLink(ctx, booking99ID, testPassengerID)
		require.NoError(t, err)
		require.NotNil(t, linkDTO)

		assert.Equal(t, "+54 9 11 9876-5432", linkDTO.DriverPhone)
		assert.Contains(t, linkDTO.WhatsAppURL, "https://wa.me/5491198765432?text=")
		assert.Contains(t, linkDTO.WhatsAppURL, "Reserva")
		assert.Contains(t, linkDTO.WhatsAppURL, booking99ID.String())
	})

	t.Run("rechaza con ErrUnauthorized si el solicitante no es el pasajero titular", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		userRepo := new(MockUserRepository)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          booking99ID,
			TripID:      testTripID,
			PassengerID: testPassengerID,
			DriverID:    testDriverID,
			SeatsBooked: 1,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})

		bookingRepo.On("FindByID", ctx, booking99ID).Return(booking, nil)

		service := services.NewContactLinkService(bookingRepo, tripRepo, userRepo)

		linkDTO, err := service.GetContactLink(ctx, booking99ID, intruderID)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, linkDTO)
	})

	t.Run("falla con ErrBookingNotFound si la reserva no existe", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		userRepo := new(MockUserRepository)

		bookingRepo.On("FindByID", ctx, missingBookingID).Return(nil, nil)

		service := services.NewContactLinkService(bookingRepo, tripRepo, userRepo)

		linkDTO, err := service.GetContactLink(ctx, missingBookingID, testPassengerID)
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
		assert.Nil(t, linkDTO)
	})

	t.Run("falla con ErrDriverPhoneRequired si el conductor no tiene teléfono registrado", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		userRepo := new(MockUserRepository)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingNoPhoneID,
			TripID:      testTripID,
			PassengerID: testPassengerID,
			DriverID:    driverNoPhoneID,
			SeatsBooked: 1,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})
		trip := createTestTrip(driverNoPhoneID, 2, 24)
		driverWithoutPhone := &domain.User{
			ID:        driverNoPhoneID,
			FirstName: "Lucía",
			LastName:  "Pérez",
			Phone:     "   ",
		}

		bookingRepo.On("FindByID", ctx, bookingNoPhoneID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		userRepo.On("GetByID", ctx, driverNoPhoneID).Return(driverWithoutPhone, nil)

		service := services.NewContactLinkService(bookingRepo, tripRepo, userRepo)

		linkDTO, err := service.GetContactLink(ctx, bookingNoPhoneID, testPassengerID)
		assert.ErrorIs(t, err, domain.ErrDriverPhoneRequired)
		assert.Nil(t, linkDTO)
	})

	t.Run("falla con ErrDriverPhoneRequired si el teléfono del conductor no contiene suficientes dígitos válidos", func(t *testing.T) {
		tripRepo := new(MockTripRepository)
		bookingRepo := new(MockBookingRepository)
		userRepo := new(MockUserRepository)

		booking, _ := domain.NewBooking(domain.BookingParams{
			ID:          bookingBadPhoneID,
			TripID:      testTripID,
			PassengerID: testPassengerID,
			DriverID:    driverBadPhoneID,
			SeatsBooked: 1,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(150000, domain.CurrencyARS),
		})
		trip := createTestTrip(driverBadPhoneID, 2, 24)
		driverWithInvalidPhone := &domain.User{
			ID:        driverBadPhoneID,
			FirstName: "Pedro",
			LastName:  "Sánchez",
			Phone:     "---",
		}

		bookingRepo.On("FindByID", ctx, bookingBadPhoneID).Return(booking, nil)
		tripRepo.On("FindByID", ctx, testTripID).Return(trip, nil)
		userRepo.On("GetByID", ctx, driverBadPhoneID).Return(driverWithInvalidPhone, nil)

		service := services.NewContactLinkService(bookingRepo, tripRepo, userRepo)

		linkDTO, err := service.GetContactLink(ctx, bookingBadPhoneID, testPassengerID)
		assert.ErrorIs(t, err, domain.ErrDriverPhoneRequired)
		assert.Nil(t, linkDTO)
	})
}
