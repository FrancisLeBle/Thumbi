package services

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	txManager      ports.TransactionManager
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
	optionalDeps ...any,
) ports.BookingService {
	svc := &bookingService{
		bookingRepo:    bookingRepo,
		escrowRepo:     escrowRepo,
		tripRepo:       tripRepo,
		userRepo:       userRepo,
		vehicleRepo:    vehicleRepo,
		paymentGateway: paymentGateway,
		notifier:       notifier,
		clock:          time.Now,
	}
	for _, opt := range optionalDeps {
		if tm, ok := opt.(ports.TransactionManager); ok {
			svc.txManager = tm
		}
	}
	return svc
}

func (s *bookingService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.txManager != nil {
		return s.txManager.WithinTransaction(ctx, fn)
	}
	return fn(ctx)
}

func (s *bookingService) rejectAndReleaseSeats(ctx context.Context, booking *domain.Booking, reason string) error {
	if rejectErr := booking.Reject(reason, s.clock().UTC()); rejectErr != nil {
		return rejectErr
	}
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, booking.CancellationReason); err != nil {
			return err
		}
		if err := s.tripRepo.ReleaseSeats(txCtx, booking.TripID, booking.SeatsBooked); err != nil {
			return err
		}
		return nil
	})
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

	// 5. Invariante RF-03: Detección de solapamiento horario (tstzrange &&) y control de reservas pendientes en una sola consulta JOIN
	now := s.clock().UTC()
	hasOverlap, pendingCount, err := s.bookingRepo.CheckPassengerBookingConstraints(
		ctx,
		input.PassengerID,
		trip.ID,
		trip.DepartureTime,
		trip.EstimatedArrivalTime,
		now,
	)
	if err != nil {
		return nil, err
	}
	if hasOverlap {
		return nil, domain.ErrOverlappingTripBooking
	}

	// Mitigación de DoS/Abuso: Limitar el número de reservas simultáneas reteniendo asientos sin pagar
	if pendingCount >= domain.MaxPendingBookingsPerPassenger {
		return nil, domain.ErrMaxPendingBookingsExceeded
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
		// Si la pasarela falla, rechazar la reserva y liberar los asientos atómicamente en la misma transacción
		if txErr := s.rejectAndReleaseSeats(ctx, booking, "Fallo al inicializar intención de pago en pasarela"); txErr != nil {
			return nil, fmt.Errorf("%w: error al revertir reserva y restituir asientos: %w", domain.ErrPaymentFailed, txErr)
		}
		return nil, domain.ErrPaymentFailed
	}

	// Vincular y persistir la referencia de la orden/intención creada en la pasarela
	booking.SetPaymentIntentID(paymentIntent.GatewayRef)
	if err := s.bookingRepo.UpdatePaymentIntent(ctx, booking.ID, paymentIntent.GatewayRef); err != nil {
		if txErr := s.rejectAndReleaseSeats(ctx, booking, "Fallo al vincular intención de pago en base de datos"); txErr != nil {
			return nil, fmt.Errorf("error vinculando payment_intent_id (%v) y al revertir reserva: %w", err, txErr)
		}
		return nil, fmt.Errorf("error vinculando payment_intent_id de la reserva: %w", err)
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

	// 2. Control de Autorización: Si es llamada directa de cliente (no webhook), validar que quien confirma sea el pasajero de la reserva
	if !input.IsWebhook {
		if strings.TrimSpace(input.RequestingUserID) == "" || booking.PassengerID != strings.TrimSpace(input.RequestingUserID) {
			return nil, domain.ErrUnauthorized
		}
	}

	// 3. Validación de Payment Intent: La referencia de pago debe coincidir exactamente con el intent generado para esta reserva
	if booking.PaymentIntentID == nil || strings.TrimSpace(*booking.PaymentIntentID) == "" || *booking.PaymentIntentID != input.PaymentGatewayRef {
		return nil, domain.ErrInvalidPaymentGatewayRef
	}

	now := s.clock().UTC()

	// 4. Verificar estado y expiración por TTL
	if booking.IsExpired(now) {
		return nil, domain.ErrBookingExpired
	}
	if booking.Status != domain.BookingStatusPendingPayment {
		return nil, domain.ErrInvalidBookingStatus
	}

	// 5. Confirmar la transacción con la pasarela de pagos
	confirmed, err := s.paymentGateway.ConfirmPayment(ctx, input.PaymentGatewayRef)
	if err != nil || !confirmed {
		return nil, domain.ErrPaymentFailed
	}

	// 6. Obtener datos del viaje para vincular al conductor en Escrow
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 7. Transicionar estado a CONFIRMED y construir registro de custodia en memoria
	if err := booking.Confirm(now); err != nil {
		return nil, err
	}

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

	// 8. Persistir confirmación atómica (validando PENDING_PAYMENT y expires_at > NOW() en BD) y Escrow en la misma transacción
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, nil); err != nil {
			return err
		}
		if err := s.escrowRepo.CreateEscrow(txCtx, escrow); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 9. Notificar a las partes si el notificador está configurado (best-effort fuera de la transacción de BD)
	if s.notifier != nil {
		if notifyErr := s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
			UserID:  booking.PassengerID,
			Title:   "Reserva Confirmada",
			Message: "Tu pago ha sido custodiado exitosamente. ¡Buen viaje!",
		}); notifyErr != nil {
			log.Printf("[WARN] Error enviando notificación de confirmación para reserva %s: %v", booking.ID, notifyErr)
		}
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

	// 2. Actualizar estado de la reserva y restituir atómicamente los asientos en la misma transacción de base de datos
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, booking.CancellationReason); err != nil {
			return err
		}
		if err := s.tripRepo.ReleaseSeats(txCtx, booking.TripID, booking.SeatsBooked); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 3. Si la reserva estaba confirmada, procesar el reembolso en Escrow
	if booking.Status == domain.BookingStatusCancelledByPassenger || booking.Status == domain.BookingStatusCancelledByDriver {
		escrow, err := s.escrowRepo.FindByBookingID(ctx, booking.ID)
		if errors.Is(err, domain.ErrEscrowNotFound) || (err == nil && escrow == nil) {
			// Si no hay transacción de custodia (ej. canceló en PENDING_PAYMENT), no hay reembolso
			return nil, nil
		}
		if err != nil {
			return nil, err
		}

		// Calcular importes de reembolso según regla de tiempo
		pRefund, dComp, refundType := domain.CalculateCancellationRefund(escrow.Amount, trip.DepartureTime, now, isDriver)

		// Invocar pasarela de pago para devolución
		gatewayRefundRef := ""
		if pRefund > 0 {
			refundResult, err := s.paymentGateway.ProcessRefund(ctx, escrow.PaymentGatewayRef, pRefund, string(refundType))
			if err != nil {
				return nil, fmt.Errorf("%w: %w", domain.ErrPaymentFailed, err)
			}
			if refundResult != nil {
				gatewayRefundRef = refundResult.GatewayRefundRef
			}
		}

		// Actualizar estado del Escrow
		if refundType == domain.RefundTypePassengerLate {
			if err := escrow.RefundPartial(now); err != nil {
				return nil, err
			}
		} else {
			if err := escrow.RefundFull(now); err != nil {
				return nil, err
			}
		}

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

		err = s.runInTx(ctx, func(txCtx context.Context) error {
			if err := s.escrowRepo.UpdateEscrowStatus(txCtx, escrow.ID, escrow.Status); err != nil {
				return err
			}
			if err := s.escrowRepo.RecordRefund(txCtx, refundTx); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
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
		Status:             b.Status,
		PickupStopID:       b.PickupStopID,
		DropoffStopID:      b.DropoffStopID,
		PaymentIntentID:    b.PaymentIntentID,
		ExpiresAt:          b.ExpiresAt.Format(time.RFC3339),
		ConfirmedAt:        confirmedAtStr,
		CancelledAt:        cancelledAtStr,
		CancellationReason: b.CancellationReason,
		CreatedAt:          b.CreatedAt.Format(time.RFC3339),
		PaymentIntent:      paymentIntent,
		TripSummary:        tripSummary,
	}
}
