package services

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/pkg/whatsapp"
)

// ContactLinkService implementa el caso de uso único de generar deeplinks de WhatsApp
// para que un pasajero autenticado contacte al conductor de su reserva.
type ContactLinkService struct {
	bookingRepo ports.BookingRepository
	tripRepo    ports.TripRepository
	userRepo    ports.UserRepository
}

// NewContactLinkService instancia el servicio dedicado de enlaces de contacto.
func NewContactLinkService(
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	userRepo ports.UserRepository,
) *ContactLinkService {
	return &ContactLinkService{
		bookingRepo: bookingRepo,
		tripRepo:    tripRepo,
		userRepo:    userRepo,
	}
}

// GetContactLink genera el Deeplink de WhatsApp para que el pasajero autenticado contacte al conductor del viaje.
func (s *ContactLinkService) GetContactLink(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.ContactLinkDTO, error) {
	if bookingID == uuid.Nil || requestingUserID == uuid.Nil {
		return nil, domain.ErrBookingNotFound
	}
	if s.bookingRepo == nil || s.tripRepo == nil {
		return nil, domain.ErrBookingNotFound
	}

	// 1. Buscar la reserva
	booking, err := s.bookingRepo.FindByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	// 2. Validar que la reserva pertenezca al usuario que consulta (el pasajero autenticado)
	if booking.PassengerID != requestingUserID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Buscar el viaje correspondiente
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 4. Obtener datos del conductor y validar su número telefónico registrado
	if s.userRepo == nil {
		return nil, domain.ErrDriverPhoneRequired
	}

	driver, err := s.userRepo.GetByID(ctx, trip.DriverID)
	if err != nil {
		return nil, err
	}
	if driver == nil {
		return nil, domain.ErrUserNotFound
	}

	driverPhone := strings.TrimSpace(driver.Phone)
	if driverPhone == "" || len(whatsapp.CleanPhoneNumber(driverPhone)) < 7 {
		return nil, domain.ErrDriverPhoneRequired
	}

	passengerName := ""
	passenger, err := s.userRepo.GetByID(ctx, booking.PassengerID)
	if err == nil && passenger != nil {
		passengerName = passenger.FullName()
	}

	departureTimeStr := "18:00"
	if !trip.DepartureTime.IsZero() {
		departureTimeStr = trip.DepartureTime.Format("15:04")
	}

	whatsappURL := whatsapp.GenerateWhatsAppLink(
		driverPhone,
		booking.ID.String(),
		trip.OriginTitle,
		trip.DestinationTitle,
		departureTimeStr,
		passengerName,
	)

	return &ports.ContactLinkDTO{
		WhatsAppURL: whatsappURL,
		DriverPhone: driverPhone,
	}, nil
}
