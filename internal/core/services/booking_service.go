package services

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

const maxSeatHoldAttempts = 3

// BookingSettlementService define el subconjunto de operaciones financieras de liquidación
// requeridas por el ciclo de vida de reservas (SOLID · ISP).
type BookingSettlementService interface {
	CreatePaymentIntent(ctx context.Context, bookingID uuid.UUID, amount domain.Money) (*ports.PaymentIntentResult, error)
	ConfirmPayment(ctx context.Context, gatewayRef string) error
	HoldEscrow(ctx context.Context, input HoldEscrowInput) (*domain.EscrowTransaction, error)
	SettleCancellationRefund(ctx context.Context, input CancellationRefundInput) (*domain.RefundTransaction, error)
}

// BookingService implementa los casos de uso del ciclo de vida y consultas de reservas.
type BookingService struct {
	bookingRepo       ports.BookingRepository
	tripRepo          ports.TripRepository
	userRepo          ports.UserRepository
	vehicleRepo       ports.VehicleRepository
	settlement        BookingSettlementService
	notifier          ports.Notifier
	pricingConfigRepo ports.PricingConfigRepository
	txManager         ports.TransactionManager
	clock             func() time.Time
}

// NewBookingService instancia el servicio del ciclo de vida de reservas.
func NewBookingService(
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	userRepo ports.UserRepository,
	vehicleRepo ports.VehicleRepository,
	settlement BookingSettlementService,
	notifier ports.Notifier,
	optionalDeps ...any,
) *BookingService {
	svc := &BookingService{
		bookingRepo: bookingRepo,
		tripRepo:    tripRepo,
		userRepo:    userRepo,
		vehicleRepo: vehicleRepo,
		settlement:  settlement,
		notifier:    notifier,
		clock:       time.Now,
	}
	for _, opt := range optionalDeps {
		switch dep := opt.(type) {
		case ports.TransactionManager:
			svc.txManager = dep
		case ports.PricingConfigRepository:
			svc.pricingConfigRepo = dep
		}
	}
	return svc
}

func (s *BookingService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.txManager != nil {
		return s.txManager.WithinTransaction(ctx, fn)
	}
	return fn(ctx)
}

func (s *BookingService) releaseTripSeatsWithRetry(ctx context.Context, trip *domain.Trip, seats int, now time.Time) error {
	currentTrip := trip
	for attempt := 0; attempt < maxSeatHoldAttempts; attempt++ {
		if currentTrip == nil {
			return domain.ErrTripNotFound
		}

		transition, err := currentTrip.ReleaseSeats(seats, now)
		if err != nil {
			return err
		}

		applied, err := s.tripRepo.ReleaseSeats(ctx, transition)
		if err != nil {
			return err
		}
		if applied {
			return nil
		}

		reloaded, err := s.tripRepo.FindByID(ctx, currentTrip.ID)
		if err != nil || reloaded == nil {
			return domain.ErrTripNotFound
		}
		currentTrip = reloaded
	}
	return domain.ErrConcurrentModification
}

func (s *BookingService) rejectAndReleaseSeats(ctx context.Context, trip *domain.Trip, booking *domain.Booking, reason string) error {
	now := s.clock().UTC()
	if rejectErr := booking.Reject(reason, now); rejectErr != nil {
		return rejectErr
	}
	return s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, booking.CancellationReason); err != nil {
			return err
		}
		return s.releaseTripSeatsWithRetry(txCtx, trip, booking.SeatsBooked, now)
	})
}

// CreateBooking procesa la solicitud de reserva delegando al dominio la decisión de invariantes/transiciones
// y aplicando el hold en base de datos mediante UPDATE atómico con guarda.
func (s *BookingService) CreateBooking(ctx context.Context, input ports.CreateBookingInput) (*ports.BookingDTO, error) {
	if input.TripID == uuid.Nil || input.PassengerID == uuid.Nil {
		return nil, domain.ErrBookingNotFound
	}
	if input.SeatsRequested <= 0 {
		return nil, domain.ErrInvalidSeatCount
	}

	now := s.clock().UTC()
	constraintsChecked := false
	var (
		trip    *domain.Trip
		booking *domain.Booking
		held    bool
	)

	for attempt := 0; attempt < maxSeatHoldAttempts; attempt++ {
		var err error
		trip, err = s.tripRepo.FindByID(ctx, input.TripID)
		if err != nil || trip == nil {
			return nil, domain.ErrTripNotFound
		}

		// El dominio valida elegibilidad, auto-reserva, estado del viaje y disponibilidad de asientos.
		if err := trip.CanBook(input.PassengerID, input.SeatsRequested); err != nil {
			return nil, err
		}

		// Invariante RF-03: Detección de solapamiento horario (tstzrange &&) y control de reservas pendientes en una sola consulta JOIN
		if !constraintsChecked {
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
			if pendingCount >= domain.MaxPendingBookingsPerPassenger {
				return nil, domain.ErrMaxPendingBookingsExceeded
			}
			constraintsChecked = true
		}

		// El dominio calcula el nuevo estado del viaje (ej. PUBLISHED -> FULL) y genera el SeatTransition con guarda.
		seatHold, err := trip.ReserveSeats(input.SeatsRequested, now)
		if err != nil {
			return nil, err
		}

		if booking == nil {
			pricingCfg := domain.DefaultPlatformPricingConfig()
			if s.pricingConfigRepo != nil {
				activeCfg, cfgErr := s.pricingConfigRepo.GetActive(ctx)
				if cfgErr != nil {
					return nil, cfgErr
				}
				if activeCfg != nil {
					pricingCfg = *activeCfg
				}
			}

			breakdown, err := pricingCfg.CalculateBookingBreakdown(trip.PricePerSeat, input.SeatsRequested)
			if err != nil {
				return nil, err
			}

			booking, err = domain.NewBooking(domain.BookingParams{
				ID:            domain.NewUUIDv7(),
				TripID:        trip.ID,
				PassengerID:   input.PassengerID,
				DriverID:      trip.DriverID,
				SeatsBooked:   input.SeatsRequested,
				Currency:      trip.Currency,
				UnitPrice:     trip.PricePerSeat,
				PlatformFee:   breakdown.PlatformFee,
				PickupStopID:  input.PickupStopID,
				DropoffStopID: input.DropoffStopID,
				TTL:           domain.DefaultBookingTTL,
			})
			if err != nil {
				return nil, err
			}
		}

		applied, err := s.bookingRepo.CreateWithHold(ctx, booking, seatHold)
		if err != nil {
			return nil, err
		}
		if applied {
			held = true
			break
		}
	}

	if !held {
		latestTrip, err := s.tripRepo.FindByID(ctx, input.TripID)
		if err != nil || latestTrip == nil {
			return nil, domain.ErrTripNotFound
		}
		if err := latestTrip.CanBook(input.PassengerID, input.SeatsRequested); err != nil {
			return nil, err
		}
		return nil, domain.ErrConcurrentModification
	}

	// Inicializar intención de pago delegando la pasarela a SettlementService
	if s.settlement == nil {
		if txErr := s.rejectAndReleaseSeats(ctx, trip, booking, "Servicio de liquidación no configurado"); txErr != nil {
			return nil, fmt.Errorf("%w: error al revertir reserva y restituir asientos: %w", domain.ErrPaymentFailed, txErr)
		}
		return nil, domain.ErrPaymentFailed
	}

	paymentIntent, err := s.settlement.CreatePaymentIntent(ctx, booking.ID, booking.TotalPrice)
	if err != nil {
		if txErr := s.rejectAndReleaseSeats(ctx, trip, booking, "Fallo al inicializar intención de pago en pasarela"); txErr != nil {
			return nil, fmt.Errorf("%w: error al revertir reserva y restituir asientos: %w", domain.ErrPaymentFailed, txErr)
		}
		return nil, domain.ErrPaymentFailed
	}

	// Vincular y persistir la referencia de la orden/intención creada en la pasarela
	booking.SetPaymentIntentID(paymentIntent.GatewayRef)
	if err := s.bookingRepo.UpdatePaymentIntent(ctx, booking.ID, paymentIntent.GatewayRef); err != nil {
		if txErr := s.rejectAndReleaseSeats(ctx, trip, booking, "Fallo al vincular intención de pago en base de datos"); txErr != nil {
			return nil, fmt.Errorf("error vinculando payment_intent_id (%v) y al revertir reserva: %w", err, txErr)
		}
		return nil, fmt.Errorf("error vinculando payment_intent_id de la reserva: %w", err)
	}

	tripSummary := s.buildTripSummary(ctx, trip)
	return s.toBookingDTO(booking, tripSummary, paymentIntent), nil
}

// ConfirmBookingPayment acredita el cobro vía SettlementService, transiciona a CONFIRMED y constituye la custodia en Escrow.
func (s *BookingService) ConfirmBookingPayment(ctx context.Context, input ports.ConfirmBookingPaymentInput) (*ports.BookingDTO, error) {
	if input.BookingID == uuid.Nil || strings.TrimSpace(input.PaymentGatewayRef) == "" {
		return nil, domain.ErrBookingNotFound
	}

	// 1. Obtener la reserva
	booking, err := s.bookingRepo.FindByID(ctx, input.BookingID)
	if err != nil || booking == nil {
		return nil, domain.ErrBookingNotFound
	}

	// 2. Control de Autorización: Si es llamada directa de cliente (no webhook), validar que quien confirma sea el pasajero
	if !input.IsWebhook {
		if input.RequestingUserID == uuid.Nil || booking.PassengerID != input.RequestingUserID {
			return nil, domain.ErrUnauthorized
		}
	}

	// 3. Validación de Payment Intent
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

	if s.settlement == nil {
		return nil, domain.ErrPaymentFailed
	}

	// 5. Confirmar el pago ante la pasarela vía SettlementService
	if err := s.settlement.ConfirmPayment(ctx, input.PaymentGatewayRef); err != nil {
		return nil, err
	}

	// 6. Obtener datos del viaje para vincular al conductor en Escrow
	trip, err := s.tripRepo.FindByID(ctx, booking.TripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 7. Transicionar estado a CONFIRMED
	if err := booking.Confirm(now); err != nil {
		return nil, err
	}

	// 8. Persistir confirmación atómica y constituir el Escrow en la misma transacción
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, nil); err != nil {
			return err
		}
		_, err := s.settlement.HoldEscrow(txCtx, HoldEscrowInput{
			BookingID:         booking.ID,
			TripID:            trip.ID,
			PassengerID:       booking.PassengerID,
			DriverID:          trip.DriverID,
			Amount:            booking.TotalPrice,
			BaseAmount:        booking.BaseAmount,
			PlatformFee:       booking.PlatformFee,
			PaymentGatewayRef: input.PaymentGatewayRef,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	// 9. Notificar al pasajero (best-effort fuera de la transacción de BD)
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

// CancelBooking procesa la cancelación, restituye asientos atómicamente y delega el reembolso de Escrow a SettlementService.
func (s *BookingService) CancelBooking(ctx context.Context, input ports.CancelBookingInput) (*ports.RefundDTO, error) {
	if input.BookingID == uuid.Nil || input.RequestingUserID == uuid.Nil {
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

	// 2. Actualizar estado de la reserva y restituir atómicamente los asientos en la misma transacción
	err = s.runInTx(ctx, func(txCtx context.Context) error {
		if err := s.bookingRepo.UpdateStatus(txCtx, booking.ID, booking.Status, booking.CancellationReason); err != nil {
			return err
		}
		return s.releaseTripSeatsWithRetry(txCtx, trip, booking.SeatsBooked, now)
	})
	if err != nil {
		return nil, err
	}

	// 3. Delegar la liquidación y reembolso de Escrow a SettlementService
	if s.settlement != nil && (booking.Status == domain.BookingStatusCancelledByPassenger || booking.Status == domain.BookingStatusCancelledByDriver) {
		refund, err := s.settlement.SettleCancellationRefund(ctx, CancellationRefundInput{
			BookingID:            booking.ID,
			DepartureTime:        trip.DepartureTime,
			CancelledAt:          now,
			IsDriverCancellation: isDriver,
		})
		if err != nil {
			return nil, err
		}
		return toRefundDTO(refund), nil
	}

	return nil, nil
}

// GetBookingByID consulta una reserva validando autorización del solicitante.
func (s *BookingService) GetBookingByID(ctx context.Context, bookingID, requestingUserID uuid.UUID) (*ports.BookingDTO, error) {
	if bookingID == uuid.Nil {
		return nil, domain.ErrBookingNotFound
	}
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

// ListPassengerBookings devuelve el historial de reservas de un pasajero.
func (s *BookingService) ListPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*ports.BookingDTO, error) {
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

// buildTripSummary construye el resumen contextual del viaje para el DTO.
func (s *BookingService) buildTripSummary(ctx context.Context, trip *domain.Trip) *ports.BookingTripSummary {
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

// toBookingDTO mapea la entidad de dominio a DTO de presentación.
func (s *BookingService) toBookingDTO(b *domain.Booking, tripSummary *ports.BookingTripSummary, paymentIntent *ports.PaymentIntentResult) *ports.BookingDTO {
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
		Currency:           b.Currency,
		UnitPrice:          b.UnitPrice,
		BaseAmount:         b.BaseAmount,
		PlatformFee:        b.PlatformFee,
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

func toRefundDTO(r *domain.RefundTransaction) *ports.RefundDTO {
	if r == nil {
		return nil
	}
	return &ports.RefundDTO{
		ID:                       r.ID,
		EscrowTransactionID:      r.EscrowTransactionID,
		BookingID:                r.BookingID,
		PassengerRefundAmount:    r.PassengerRefundAmount,
		DriverCompensationAmount: r.DriverCompensationAmount,
		PlatformRetainedAmount:   r.PlatformRetainedAmount,
		RefundType:               r.RefundType,
		GatewayRefundRef:         r.GatewayRefundRef,
		ProcessedAt:              formatTimeRFC3339(r.ProcessedAt),
		CreatedAt:                formatTimeRFC3339(r.CreatedAt),
	}
}
