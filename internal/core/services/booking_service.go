package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/pkg/whatsapp"
)

// bookingService implementa ports.BookingService bajo Clean Architecture
type bookingService struct {
	bookingRepo    ports.BookingRepository
	escrowRepo     ports.EscrowRepository
	tripRepo       ports.TripRepository
	userRepo       ports.UserRepository
	vehicleRepo    ports.VehicleRepository
	paymentGateway ports.PaymentGateway
	notifier       ports.Notifier
	clock          func() time.Time
}

// NewBookingService instancia el servicio de reservas y pagos en custodia
func NewBookingService(
	bookingRepo ports.BookingRepository,
	escrowRepo ports.EscrowRepository,
	tripRepo ports.TripRepository,
	userRepo ports.UserRepository,
	vehicleRepo ports.VehicleRepository,
	paymentGateway ports.PaymentGateway,
	notifier ports.Notifier,
) ports.BookingService {
	return &bookingService{
		bookingRepo:    bookingRepo,
		escrowRepo:     escrowRepo,
		tripRepo:       tripRepo,
		userRepo:       userRepo,
		vehicleRepo:    vehicleRepo,
		paymentGateway: paymentGateway,
		notifier:       notifier,
		clock:          time.Now,
	}
}

// CreateBooking procesa la solicitud de reserva con verificación y decremento atómico de asientos
func (s *bookingService) CreateBooking(ctx context.Context, input ports.CreateBookingInput) (*ports.BookingDTO, error) {
	if strings.TrimSpace(input.TripID) == "" || strings.TrimSpace(input.PassengerID) == "" {
		return nil, domain.ErrBookingNotFound
	}
	if input.SeatsRequested <= 0 {
		return nil, domain.ErrInvalidSeatCount
	}

	// 1. Obtener viaje y validar estado inicial
	trip, err := s.tripRepo.FindByID(ctx, input.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 2. Invariante RF-02: El pasajero no puede ser el mismo conductor
	if trip.DriverID == input.PassengerID {
		return nil, domain.ErrCannotBookOwnTrip
	}

	// 3. El viaje debe estar publicado
	if trip.Status != domain.TripStatusPublished {
		return nil, domain.ErrInvalidTripStatusChange
	}

	// 4. Verificación previa de asientos disponibles
	if trip.AvailableSeats < input.SeatsRequested {
		return nil, domain.ErrInsufficientSeats
	}

	// 5. Invariante RF-03: Detección de solapamiento horario de viajes para el pasajero
	existingBookings, _ := s.bookingRepo.GetPassengerBookings(ctx, input.PassengerID, 20, 0)
	for _, eb := range existingBookings {
		if eb.IsActive() && eb.TripID != trip.ID {
			existingTrip, err := s.tripRepo.FindByID(ctx, eb.TripID)
			if err == nil && existingTrip != nil {
				// Evaluar solapamiento entre departureTime y estimatedArrivalTime
				if trip.DepartureTime.Before(existingTrip.EstimatedArrivalTime) && trip.EstimatedArrivalTime.After(existingTrip.DepartureTime) {
					return nil, domain.ErrOverlappingTripBooking
				}
			}
		}
	}

	// 6. Instanciar entidad de dominio Booking en estado PENDING_PAYMENT con TTL de 15 minutos
	bookingID := uuid.New().String()
	booking, err := domain.NewBooking(domain.BookingParams{
		ID:            bookingID,
		TripID:        trip.ID,
		PassengerID:   input.PassengerID,
		DriverID:      trip.DriverID,
		SeatsBooked:   input.SeatsRequested,
		UnitPrice:     trip.PricePerSeat,
		PickupStopID:  input.PickupStopID,
		DropoffStopID: input.DropoffStopID,
		TTL:           domain.DefaultBookingTTL,
	})
	if err != nil {
		return nil, err
	}

	// 7. Persistir atómicamente la reserva con bloqueo pesimista en base de datos (SELECT ... FOR UPDATE)
	if err := s.bookingRepo.CreateWithHold(ctx, booking); err != nil {
		return nil, err
	}

	// 8. Crear intención de pago con la pasarela externa
	currency := "USD"
	paymentIntent, err := s.paymentGateway.CreatePaymentIntent(ctx, booking.ID, booking.TotalPrice, currency)
	if err != nil {
		// Si la pasarela falla, liberar los asientos y marcar como rechazada
		_ = booking.Reject("Fallo al inicializar intención de pago en pasarela", s.clock().UTC())
		_ = s.bookingRepo.UpdateStatus(ctx, booking.ID, booking.Status, booking.CancellationReason)
		_ = trip.UpdateAvailableSeats(trip.AvailableSeats + booking.SeatsBooked)
		_ = s.tripRepo.Save(ctx, trip)
		return nil, domain.ErrPaymentFailed
	}

	// 9. Enriquecer resumen del viaje para el DTO
	tripSummary := s.buildTripSummary(ctx, trip)

	return s.toBookingDTO(booking, tripSummary, paymentIntent), nil
}

// ConfirmBookingPayment acredita el cobro, transiciona a CONFIRMED y retiene fondos en Escrow
func (s *bookingService) ConfirmBookingPayment(ctx context.Context, input ports.ConfirmBookingPaymentInput) (*ports.BookingDTO, error) {
	if strings.TrimSpace(input.BookingID) == "" || strings.TrimSpace(input.PaymentGatewayRef) == "" {
		return nil, domain.ErrBookingNotFound
	}

	// 1. Obtener la reserva
	booking, err := s.bookingRepo.FindByID(ctx, input.BookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	now := s.clock().UTC()

	// 2. Verificar estado y expiración por TTL
	if booking.IsExpired(now) {
		return nil, domain.ErrBookingExpired
	}
	if booking.Status != domain.BookingStatusPendingPayment {
		return nil, domain.ErrInvalidBookingStatus
	}

	// 3. Confirmar la transacción con la pasarela de pagos
	confirmed, err := s.paymentGateway.ConfirmPayment(ctx, input.PaymentGatewayRef)
	if err != nil || !confirmed {
		return nil, domain.ErrPaymentFailed
	}

	// 4. Transicionar estado a CONFIRMED
	if err := booking.Confirm(now); err != nil {
		return nil, err
	}
	if err := s.bookingRepo.UpdateStatus(ctx, booking.ID, booking.Status, nil); err != nil {
		return nil, err
	}

	// 5. Obtener datos del viaje para vincular al conductor en Escrow
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 6. Registrar fondos en custodia (EscrowTransaction) en estado HELD
	escrowID := uuid.New().String()
	escrow, err := domain.NewEscrowTransaction(
		escrowID,
		booking.ID,
		trip.ID,
		booking.PassengerID,
		trip.DriverID,
		booking.TotalPrice,
		"USD",
		input.PaymentGatewayRef,
	)
	if err != nil {
		return nil, err
	}

	if err := s.escrowRepo.CreateEscrow(ctx, escrow); err != nil {
		return nil, err
	}

	// 7. Notificar a las partes si el notificador está configurado
	if s.notifier != nil {
		_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
			UserID:  booking.PassengerID,
			Title:   "Reserva Confirmada",
			Message: "Tu pago ha sido custodiado exitosamente. ¡Buen viaje!",
		})
	}

	tripSummary := s.buildTripSummary(ctx, trip)
	return s.toBookingDTO(booking, tripSummary, nil), nil
}

// CancelBooking procesa la cancelación, restituye asientos y liquida reembolsos según política
func (s *bookingService) CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*domain.RefundTransaction, error) {
	if strings.TrimSpace(input.BookingID) == "" || strings.TrimSpace(input.RequestingUserID) == "" {
		return nil, domain.ErrBookingNotFound
	}

	booking, err := s.bookingRepo.FindByID(ctx, input.BookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// Validar que quien cancela sea el conductor o el pasajero
	isDriver := input.RequestingUserID == trip.DriverID
	isPassenger := input.RequestingUserID == booking.PassengerID
	if !isDriver && !isPassenger {
		return nil, domain.ErrUnauthorized
	}

	now := s.clock().UTC()

	// 1. Transicionar la reserva al estado correspondiente
	if isDriver {
		err = booking.CancelByDriver(input.Reason, now)
	} else {
		err = booking.CancelByPassenger(input.Reason, now)
	}
	if err != nil {
		return nil, err
	}

	if err := s.bookingRepo.UpdateStatus(ctx, booking.ID, booking.Status, &input.Reason); err != nil {
		return nil, err
	}

	// 2. Restituir atómicamente los asientos al viaje
	newAvailable := trip.AvailableSeats + booking.SeatsBooked
	_ = trip.UpdateAvailableSeats(newAvailable)
	_ = s.tripRepo.Save(ctx, trip)

	// 3. Si la reserva estaba confirmada, procesar el reembolso en Escrow
	if booking.Status == domain.BookingStatusCancelledByPassenger || booking.Status == domain.BookingStatusCancelledByDriver {
		escrow, err := s.escrowRepo.FindByBookingID(ctx, booking.ID)
		if err != nil || escrow == nil {
			// Si no hay transacción de custodia (ej. canceló en PENDING_PAYMENT), no hay reembolso
			return nil, nil
		}

		// Calcular importes de reembolso según regla de tiempo
		pRefund, dComp, refundType := domain.CalculateCancellationRefund(escrow.Amount, trip.DepartureTime, now, isDriver)

		// Invocar pasarela de pago para devolución
		gatewayRefundRef := ""
		if pRefund > 0 {
			refundResult, err := s.paymentGateway.ProcessRefund(ctx, escrow.PaymentGatewayRef, pRefund, string(refundType))
			if err == nil && refundResult != nil {
				gatewayRefundRef = refundResult.GatewayRefundRef
			}
		}

		// Actualizar estado del Escrow
		if refundType == domain.RefundTypePassengerLate {
			_ = escrow.RefundPartial(now)
		} else {
			_ = escrow.RefundFull(now)
		}
		_ = s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, escrow.Status)

		// Registrar transacción inmutable de reembolso
		refundID := uuid.New().String()
		refundTx, err := domain.NewRefundTransaction(
			refundID,
			escrow.ID,
			booking.ID,
			pRefund,
			dComp,
			refundType,
			gatewayRefundRef,
		)
		if err != nil {
			return nil, err
		}

		if err := s.escrowRepo.RecordRefund(ctx, refundTx); err != nil {
			return nil, err
		}

		return refundTx, nil
	}

	return nil, nil
}

// ProcessExpiredBookings ejecuta la expiración masiva de reservas pendientes vencidas
func (s *bookingService) ProcessExpiredBookings(ctx context.Context) (int, error) {
	now := s.clock().UTC()
	return s.bookingRepo.ExpirePendingBookings(ctx, now)
}

// GetBookingByID consulta una reserva validando autorización del solicitante
func (s *bookingService) GetBookingByID(ctx context.Context, bookingID, requestingUserID string) (*ports.BookingDTO, error) {
	booking, err := s.bookingRepo.FindByID(ctx, bookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	if requestingUserID != booking.PassengerID && requestingUserID != trip.DriverID {
		return nil, domain.ErrUnauthorized
	}

	tripSummary := s.buildTripSummary(ctx, trip)
	return s.toBookingDTO(booking, tripSummary, nil), nil
}

// GetContactLink genera el Deeplink de WhatsApp para que el pasajero autenticado contacte al conductor del viaje
func (s *bookingService) GetContactLink(ctx context.Context, bookingID, requestingUserID string) (*ports.ContactLinkDTO, error) {
	if strings.TrimSpace(bookingID) == "" || strings.TrimSpace(requestingUserID) == "" {
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

	// 4. Obtener datos del conductor y nombre del pasajero
	driverPhone := "+54 9 11 1234-5678" // Default fallback para la plataforma o si no posee teléfono configurado
	if s.userRepo != nil {
		driver, _ := s.userRepo.GetByID(ctx, trip.DriverID)
		if driver != nil && strings.TrimSpace(driver.Phone) != "" {
			driverPhone = driver.Phone
		}
	}

	passengerName := ""
	if s.userRepo != nil {
		passenger, _ := s.userRepo.GetByID(ctx, booking.PassengerID)
		if passenger != nil {
			passengerName = passenger.FullName()
		}
	}

	departureTimeStr := "18:00"
	if !trip.DepartureTime.IsZero() {
		departureTimeStr = trip.DepartureTime.Format("15:04")
	}

	whatsappURL := whatsapp.GenerateWhatsAppLink(
		driverPhone,
		booking.ID,
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

// ListPassengerBookings devuelve el historial de reservas de un pasajero
func (s *bookingService) ListPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*ports.BookingDTO, error) {
	bookings, err := s.bookingRepo.GetPassengerBookings(ctx, passengerID, limit, offset)
	if err != nil {
		return nil, err
	}

	dtos := make([]*ports.BookingDTO, 0, len(bookings))
	for _, b := range bookings {
		trip, _ := s.tripRepo.FindByID(ctx, b.TripID)
		var tripSummary *ports.BookingTripSummary
		if trip != nil {
			tripSummary = s.buildTripSummary(ctx, trip)
		}
		dtos = append(dtos, s.toBookingDTO(b, tripSummary, nil))
	}

	return dtos, nil
}

// ListTripBookings lista las reservas de un viaje, accesible únicamente por su conductor
func (s *bookingService) ListTripBookings(ctx context.Context, tripID, driverID string) ([]*ports.BookingDTO, error) {
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	if trip.DriverID != driverID {
		return nil, domain.ErrUnauthorized
	}

	bookings, err := s.bookingRepo.GetActiveBookingsByTrip(ctx, tripID)
	if err != nil {
		return nil, err
	}

	tripSummary := s.buildTripSummary(ctx, trip)
	dtos := make([]*ports.BookingDTO, 0, len(bookings))
	for _, b := range bookings {
		dtos = append(dtos, s.toBookingDTO(b, tripSummary, nil))
	}

	return dtos, nil
}

// buildTripSummary construye el resumen contextual del viaje para el DTO
func (s *bookingService) buildTripSummary(ctx context.Context, trip *domain.Trip) *ports.BookingTripSummary {
	summary := &ports.BookingTripSummary{
		TripID:           trip.ID,
		OriginTitle:      trip.OriginTitle,
		DestinationTitle: trip.DestinationTitle,
		DepartureTime:    trip.DepartureTime.Format(time.RFC3339),
		DriverID:         trip.DriverID,
	}

	if s.userRepo != nil {
		driver, _ := s.userRepo.GetByID(ctx, trip.DriverID)
		if driver != nil {
			summary.DriverFullName = driver.FullName()
		}
	}

	if s.vehicleRepo != nil {
		vehicle, _ := s.vehicleRepo.GetByID(ctx, trip.VehicleID)
		if vehicle != nil {
			summary.VehicleBrand = vehicle.Brand
			summary.VehicleModel = vehicle.Model
			summary.VehiclePlate = vehicle.PlateNumber
		}
	}

	return summary
}

// toBookingDTO mapea la entidad de dominio a DTO de presentación
func (s *bookingService) toBookingDTO(b *domain.Booking, tripSummary *ports.BookingTripSummary, paymentIntent *ports.PaymentIntentResult) *ports.BookingDTO {
	var confirmedAtStr *string
	if b.ConfirmedAt != nil {
		formatted := b.ConfirmedAt.Format(time.RFC3339)
		confirmedAtStr = &formatted
	}

	var cancelledAtStr *string
	if b.CancelledAt != nil {
		formatted := b.CancelledAt.Format(time.RFC3339)
		cancelledAtStr = &formatted
	}

	return &ports.BookingDTO{
		ID:                 b.ID,
		TripID:             b.TripID,
		PassengerID:        b.PassengerID,
		SeatsBooked:        b.SeatsBooked,
		UnitPrice:          b.UnitPrice,
		TotalPrice:         b.TotalPrice,
		PickupStopID:       b.PickupStopID,
		DropoffStopID:      b.DropoffStopID,
		ExpiresAt:          b.ExpiresAt.Format(time.RFC3339),
		ConfirmedAt:        confirmedAtStr,
		CancelledAt:        cancelledAtStr,
		CancellationReason: b.CancellationReason,
		CreatedAt:          b.CreatedAt.Format(time.RFC3339),
		PaymentIntent:      paymentIntent,
		TripSummary:        tripSummary,
	}
}
