package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

type bookingRepository struct {
	db  DBExecutor
	uow TxUnitOfWork
}

// NewBookingRepository crea una nueva instancia del repositorio PostgreSQL para reservas
func NewBookingRepository(db DBExecutor, uow ...TxUnitOfWork) ports.BookingRepository {
	return &bookingRepository{
		db:  db,
		uow: resolveTxUnitOfWork(db, uow...),
	}
}

// CreateWithHold aplica atómicamente el descuento de plazas validado por el dominio mediante UPDATE con guarda
// e inserta la reserva si la guarda se cumplió, reportando applied = true/false.
func (r *bookingRepository) CreateWithHold(ctx context.Context, booking *domain.Booking, seatHold domain.SeatTransition) (bool, error) {
	if booking == nil || booking.ID == uuid.Nil || booking.TripID == uuid.Nil {
		return false, domain.ErrBookingNotFound
	}
	if !booking.Status.IsValid() {
		return false, fmt.Errorf("%w: estado de reserva %s", domain.ErrInvalidStatus, booking.Status)
	}
	if seatHold.TripID == uuid.Nil {
		return false, domain.ErrTripNotFound
	}
	if seatHold.SeatsDelta <= 0 {
		return false, domain.ErrInvalidSeatCount
	}
	if !seatHold.ExpectedStatus.IsValid() || !seatHold.NewStatus.IsValid() {
		return false, fmt.Errorf("%w: transición de estado de viaje %s -> %s", domain.ErrInvalidStatus, seatHold.ExpectedStatus, seatHold.NewStatus)
	}

	bookingCur := booking.Currency
	if strings.TrimSpace(string(bookingCur)) == "" {
		bookingCur = booking.UnitPrice.EffectiveCurrency()
	}
	parsedCur, err := domain.ParseCurrency(string(bookingCur))
	if err != nil {
		return false, err
	}

	errHoldNotApplied := errors.New("seat hold guard not matched")
	err = runInRepoTx(ctx, r.uow, func(tx pgx.Tx) error {
		// 1. UPDATE atómico con guarda: el dominio decidió ExpectedStatus y NewStatus; el repositorio solo aplica si la guarda coincide.
		updateTripQuery := `
			UPDATE trips
			SET available_seats = available_seats - $1,
			    status = $2,
			    updated_at = NOW()
			WHERE id = $3
			  AND status = $4
			  AND available_seats = $5
			  AND available_seats >= $1
		`
		cmdTag, err := tx.Exec(
			ctx,
			updateTripQuery,
			seatHold.SeatsDelta,
			seatHold.NewStatus.String(),
			booking.TripID,
			seatHold.ExpectedStatus.String(),
			seatHold.ExpectedAvailableSeats,
		)
		if err != nil {
			return fmt.Errorf("error al actualizar asientos disponibles del viaje: %w", err)
		}
		if cmdTag.RowsAffected() == 0 {
			return errHoldNotApplied
		}

		// 2. Insertar la reserva en estado PENDING_PAYMENT con TTL y desglose base_amount + platform_fee
		insertBookingQuery := `
			INSERT INTO bookings (
				id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
				status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
			)
		`
		if _, err = tx.Exec(
			ctx,
			insertBookingQuery,
			booking.ID,
			booking.TripID,
			booking.PassengerID,
			booking.SeatsBooked,
			parsedCur.String(),
			moneyToPGNumeric(booking.UnitPrice),
			moneyToPGNumeric(booking.BaseAmount),
			moneyToPGNumeric(booking.PlatformFee),
			moneyToPGNumeric(booking.TotalPrice),
			booking.Status.String(),
			booking.PickupStopID,
			booking.DropoffStopID,
			booking.PaymentIntentID,
			booking.ExpiresAt,
			booking.CreatedAt,
			booking.UpdatedAt,
		); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
				return domain.ErrOverlappingTripBooking
			}
			return fmt.Errorf("error al insertar la reserva: %w", err)
		}

		return nil
	})
	if errors.Is(err, errHoldNotApplied) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	return true, nil
}

// FindByID recupera una reserva por su identificador único
func (r *bookingRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Booking, error) {
	if id == uuid.Nil {
		return nil, domain.ErrBookingNotFound
	}
	query := `
		SELECT 
			id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE id = $1
	`

	var (
		b              domain.Booking
		currencyStr    string
		unitPriceNum   pgtype.Numeric
		baseAmountNum  pgtype.Numeric
		platformFeeNum pgtype.Numeric
		totalPriceNum  pgtype.Numeric
		statusStr      string
	)

	err := executor(ctx, r.db).QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&currencyStr,
		&unitPriceNum,
		&baseAmountNum,
		&platformFeeNum,
		&totalPriceNum,
		&statusStr,
		&b.PickupStopID,
		&b.DropoffStopID,
		&b.PaymentIntentID,
		&b.ExpiresAt,
		&b.ConfirmedAt,
		&b.CancelledAt,
		&b.CancellationReason,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("error consultando reserva con id %s: %w", id, err)
	}

	cur, err := domain.ParseCurrency(currencyStr)
	if err != nil {
		return nil, fmt.Errorf("error parseando currency de reserva: %w", err)
	}
	b.Currency = cur

	if b.UnitPrice, err = pgNumericToMoney(unitPriceNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
	}
	if b.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo base_amount: %w", err)
	}
	if b.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo platform_fee: %w", err)
	}
	if b.TotalPrice, err = pgNumericToMoney(totalPriceNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
	}

	b.Status = domain.BookingStatus(statusStr)
	if !b.Status.IsValid() {
		return nil, fmt.Errorf("%w: estado de reserva inválido %s", domain.ErrInvalidStatus, statusStr)
	}
	return &b, nil
}

// FindByPaymentIntentID busca una reserva por el ID de la intención de pago emitida por la pasarela
func (r *bookingRepository) FindByPaymentIntentID(ctx context.Context, intentID string) (*domain.Booking, error) {
	query := `
		SELECT 
			id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE payment_intent_id = $1
	`

	var (
		b              domain.Booking
		currencyStr    string
		unitPriceNum   pgtype.Numeric
		baseAmountNum  pgtype.Numeric
		platformFeeNum pgtype.Numeric
		totalPriceNum  pgtype.Numeric
		statusStr      string
	)

	err := executor(ctx, r.db).QueryRow(ctx, query, intentID).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&currencyStr,
		&unitPriceNum,
		&baseAmountNum,
		&platformFeeNum,
		&totalPriceNum,
		&statusStr,
		&b.PickupStopID,
		&b.DropoffStopID,
		&b.PaymentIntentID,
		&b.ExpiresAt,
		&b.ConfirmedAt,
		&b.CancelledAt,
		&b.CancellationReason,
		&b.CreatedAt,
		&b.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrBookingNotFound
		}
		return nil, fmt.Errorf("error consultando reserva por payment_intent_id %s: %w", intentID, err)
	}

	cur, err := domain.ParseCurrency(currencyStr)
	if err != nil {
		return nil, fmt.Errorf("error parseando currency de reserva: %w", err)
	}
	b.Currency = cur

	if b.UnitPrice, err = pgNumericToMoney(unitPriceNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
	}
	if b.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo base_amount: %w", err)
	}
	if b.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo platform_fee: %w", err)
	}
	if b.TotalPrice, err = pgNumericToMoney(totalPriceNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
	}

	b.Status = domain.BookingStatus(statusStr)
	if !b.Status.IsValid() {
		return nil, fmt.Errorf("%w: estado de reserva inválido %s", domain.ErrInvalidStatus, statusStr)
	}
	return &b, nil
}

// UpdatePaymentIntent actualiza o asocia el identificador de orden/intención de la pasarela
func (r *bookingRepository) UpdatePaymentIntent(ctx context.Context, bookingID uuid.UUID, intentID string) error {
	if bookingID == uuid.Nil {
		return domain.ErrBookingNotFound
	}
	query := `
		UPDATE bookings
		SET payment_intent_id = $1,
		    updated_at = NOW()
		WHERE id = $2
	`

	cmd, err := executor(ctx, r.db).Exec(ctx, query, intentID, bookingID)
	if err != nil {
		return fmt.Errorf("error actualizando payment_intent_id de reserva %s: %w", bookingID, err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrBookingNotFound
	}

	return nil
}

// UpdateStatus actualiza el estado de una reserva y valida condiciones atómicas en BD para prevenir carreras de confirmación o doble cancelación.
func (r *bookingRepository) UpdateStatus(ctx context.Context, bookingID uuid.UUID, newStatus domain.BookingStatus, reason *string) error {
	if bookingID == uuid.Nil {
		return domain.ErrBookingNotFound
	}
	if !newStatus.IsValid() {
		return fmt.Errorf("%w: estado de reserva %s", domain.ErrInvalidStatus, newStatus)
	}
	query := `
		UPDATE bookings
		SET status = $1,
		    confirmed_at = CASE WHEN $1 = 'CONFIRMED' THEN NOW() ELSE confirmed_at END,
		    cancelled_at = CASE WHEN $1 IN ('CANCELLED_BY_PASSENGER', 'CANCELLED_BY_DRIVER') THEN NOW() ELSE cancelled_at END,
		    cancellation_reason = COALESCE($2, cancellation_reason),
		    updated_at = NOW()
		WHERE id = $3
		  AND (
		      ($1 = 'CONFIRMED' AND status = 'PENDING_PAYMENT' AND expires_at > NOW())
		      OR ($1 IN ('CANCELLED_BY_PASSENGER', 'CANCELLED_BY_DRIVER', 'REJECTED') AND status IN ('PENDING_PAYMENT', 'CONFIRMED'))
		      OR ($1 NOT IN ('CONFIRMED', 'CANCELLED_BY_PASSENGER', 'CANCELLED_BY_DRIVER', 'REJECTED'))
		  )
	`

	cmd, err := executor(ctx, r.db).Exec(ctx, query, newStatus.String(), reason, bookingID)
	if err != nil {
		return fmt.Errorf("error actualizando estado de reserva %s: %w", bookingID, err)
	}

	if cmd.RowsAffected() == 0 {
		switch newStatus {
		case domain.BookingStatusConfirmed:
			return domain.ErrBookingExpired
		case domain.BookingStatusCancelledByPassenger, domain.BookingStatusCancelledByDriver:
			return domain.ErrBookingCannotBeCancelled
		case domain.BookingStatusRejected:
			return domain.ErrInvalidBookingStatus
		default:
			return domain.ErrBookingNotFound
		}
	}

	return nil
}

// GetActiveBookingsByTrip recupera las reservas activas (PENDING_PAYMENT o CONFIRMED) asociadas a un viaje.
func (r *bookingRepository) GetActiveBookingsByTrip(ctx context.Context, tripID uuid.UUID) ([]*domain.Booking, error) {
	query := `
		SELECT
			id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
			status, pickup_stop_id, dropoff_stop_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE trip_id = $1
		  AND status IN ('PENDING_PAYMENT', 'CONFIRMED')
		ORDER BY created_at ASC
	`

	rows, err := executor(ctx, r.db).Query(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("error consultando reservas activas del viaje %s: %w", tripID, err)
	}
	defer rows.Close()

	var bookings []*domain.Booking
	for rows.Next() {
		var (
			b              domain.Booking
			currencyStr    string
			unitPriceNum   pgtype.Numeric
			baseAmountNum  pgtype.Numeric
			platformFeeNum pgtype.Numeric
			totalPriceNum  pgtype.Numeric
			statusStr      string
		)
		if err := rows.Scan(
			&b.ID,
			&b.TripID,
			&b.PassengerID,
			&b.SeatsBooked,
			&currencyStr,
			&unitPriceNum,
			&baseAmountNum,
			&platformFeeNum,
			&totalPriceNum,
			&statusStr,
			&b.PickupStopID,
			&b.DropoffStopID,
			&b.ExpiresAt,
			&b.ConfirmedAt,
			&b.CancelledAt,
			&b.CancellationReason,
			&b.CreatedAt,
			&b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando reserva activa del viaje %s: %w", tripID, err)
		}
		cur, err := domain.ParseCurrency(currencyStr)
		if err != nil {
			return nil, fmt.Errorf("error parseando currency de reserva activa: %w", err)
		}
		b.Currency = cur
		if b.UnitPrice, err = pgNumericToMoney(unitPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
		}
		if b.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo base_amount: %w", err)
		}
		if b.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo platform_fee: %w", err)
		}
		if b.TotalPrice, err = pgNumericToMoney(totalPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		if !b.Status.IsValid() {
			return nil, fmt.Errorf("%w: estado de reserva inválido %s", domain.ErrInvalidStatus, statusStr)
		}
		bookings = append(bookings, &b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando reservas activas del viaje %s: %w", tripID, err)
	}

	return bookings, nil
}

// GetPassengerBookings devuelve el historial paginado de reservas de un pasajero.
func (r *bookingRepository) GetPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*domain.Booking, error) {
	query := `
		SELECT
			id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
			status, pickup_stop_id, dropoff_stop_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE passenger_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := executor(ctx, r.db).Query(ctx, query, passengerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listando reservas del pasajero %s: %w", passengerID, err)
	}
	defer rows.Close()

	var bookings []*domain.Booking
	for rows.Next() {
		var (
			b              domain.Booking
			currencyStr    string
			unitPriceNum   pgtype.Numeric
			baseAmountNum  pgtype.Numeric
			platformFeeNum pgtype.Numeric
			totalPriceNum  pgtype.Numeric
			statusStr      string
		)
		if err := rows.Scan(
			&b.ID,
			&b.TripID,
			&b.PassengerID,
			&b.SeatsBooked,
			&currencyStr,
			&unitPriceNum,
			&baseAmountNum,
			&platformFeeNum,
			&totalPriceNum,
			&statusStr,
			&b.PickupStopID,
			&b.DropoffStopID,
			&b.ExpiresAt,
			&b.ConfirmedAt,
			&b.CancelledAt,
			&b.CancellationReason,
			&b.CreatedAt,
			&b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando reserva del pasajero %s: %w", passengerID, err)
		}
		cur, err := domain.ParseCurrency(currencyStr)
		if err != nil {
			return nil, fmt.Errorf("error parseando currency de reserva del pasajero: %w", err)
		}
		b.Currency = cur
		if b.UnitPrice, err = pgNumericToMoney(unitPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
		}
		if b.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo base_amount: %w", err)
		}
		if b.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo platform_fee: %w", err)
		}
		if b.TotalPrice, err = pgNumericToMoney(totalPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		if !b.Status.IsValid() {
			return nil, fmt.Errorf("%w: estado de reserva inválido %s", domain.ErrInvalidStatus, statusStr)
		}
		bookings = append(bookings, &b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando reservas del pasajero %s: %w", passengerID, err)
	}

	return bookings, nil
}

// CheckPassengerBookingConstraints valida en una única consulta JOIN entre bookings y trips
// el solapamiento horario mediante el operador tstzrange && y contabiliza las reservas PENDING_PAYMENT vigentes.
func (r *bookingRepository) CheckPassengerBookingConstraints(
	ctx context.Context,
	passengerID, targetTripID uuid.UUID,
	departureTime, estimatedArrival, now time.Time,
) (bool, int, error) {
	query := `
		SELECT
			COALESCE(BOOL_OR(
				b.trip_id <> $2
				AND tstzrange(
					t.departure_time,
					GREATEST(t.estimated_arrival_time, t.departure_time),
					'[)'
				) && tstzrange(
					$3::timestamptz,
					GREATEST($4::timestamptz, $3::timestamptz),
					'[)'
				)
			), FALSE) AS has_overlap,
			COUNT(*) FILTER (
				WHERE b.status = 'PENDING_PAYMENT'
				  AND b.expires_at > $5
			)::INT AS active_pending_count
		FROM bookings b
		INNER JOIN trips t ON t.id = b.trip_id
		WHERE b.passenger_id = $1
		  AND (
			  b.status = 'CONFIRMED'
			  OR (b.status = 'PENDING_PAYMENT' AND b.expires_at > $5)
		  )
	`

	var hasOverlap bool
	var activePendingCount int
	err := executor(ctx, r.db).QueryRow(
		ctx,
		query,
		passengerID,
		targetTripID,
		departureTime.UTC(),
		estimatedArrival.UTC(),
		now.UTC(),
	).Scan(&hasOverlap, &activePendingCount)
	if err != nil {
		return false, 0, fmt.Errorf("error verificando restricciones y solapamiento de reservas del pasajero %s: %w", passengerID, err)
	}

	return hasOverlap, activePendingCount, nil
}

const defaultExpireBookingsBatchLimit = 500

// LockExpiredPendingBookings reclama en lotes acotados (por defecto 500) reservas pendientes cuyo TTL expiró
// usando FOR UPDATE SKIP LOCKED para ejecución segura en múltiples réplicas concurrentes.
func (r *bookingRepository) LockExpiredPendingBookings(
	ctx context.Context,
	status domain.BookingStatus,
	overdueAt time.Time,
	limit int,
) ([]*domain.Booking, error) {
	if !status.IsValid() {
		return nil, fmt.Errorf("%w: estado de reserva %s", domain.ErrInvalidStatus, status)
	}
	if limit <= 0 {
		limit = defaultExpireBookingsBatchLimit
	}

	query := `
		SELECT
			id, trip_id, passenger_id, seats_booked, currency, unit_price, base_amount, platform_fee, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE status = $1
		  AND expires_at <= $2
		ORDER BY expires_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $3
	`

	rows, err := executor(ctx, r.db).Query(ctx, query, status.String(), overdueAt.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("error bloqueando reservas pendientes vencidas: %w", err)
	}
	defer rows.Close()

	var bookings []*domain.Booking
	for rows.Next() {
		var (
			b              domain.Booking
			currencyStr    string
			unitPriceNum   pgtype.Numeric
			baseAmountNum  pgtype.Numeric
			platformFeeNum pgtype.Numeric
			totalPriceNum  pgtype.Numeric
			statusStr      string
		)
		if err := rows.Scan(
			&b.ID,
			&b.TripID,
			&b.PassengerID,
			&b.SeatsBooked,
			&currencyStr,
			&unitPriceNum,
			&baseAmountNum,
			&platformFeeNum,
			&totalPriceNum,
			&statusStr,
			&b.PickupStopID,
			&b.DropoffStopID,
			&b.PaymentIntentID,
			&b.ExpiresAt,
			&b.ConfirmedAt,
			&b.CancelledAt,
			&b.CancellationReason,
			&b.CreatedAt,
			&b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando reserva pendiente vencida: %w", err)
		}
		cur, err := domain.ParseCurrency(currencyStr)
		if err != nil {
			return nil, fmt.Errorf("error parseando currency de reserva vencida: %w", err)
		}
		b.Currency = cur
		if b.UnitPrice, err = pgNumericToMoney(unitPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
		}
		if b.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo base_amount: %w", err)
		}
		if b.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo platform_fee: %w", err)
		}
		if b.TotalPrice, err = pgNumericToMoney(totalPriceNum, cur); err != nil {
			return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		if !b.Status.IsValid() {
			return nil, fmt.Errorf("%w: estado de reserva inválido %s", domain.ErrInvalidStatus, statusStr)
		}
		bookings = append(bookings, &b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando reservas pendientes vencidas: %w", err)
	}

	return bookings, nil
}

// MarkBookingsExpiredBatch persiste en lote el cambio de estado decidido por el dominio sobre reservas reclamadas.
func (r *bookingRepository) MarkBookingsExpiredBatch(
	ctx context.Context,
	bookingIDs []uuid.UUID,
	expectedStatus, newStatus domain.BookingStatus,
	updatedAt time.Time,
) (int, error) {
	if len(bookingIDs) == 0 {
		return 0, nil
	}
	if !expectedStatus.IsValid() || !newStatus.IsValid() {
		return 0, fmt.Errorf("%w: transición de estado de reserva %s -> %s", domain.ErrInvalidStatus, expectedStatus, newStatus)
	}

	query := `
		UPDATE bookings
		SET status = $1,
		    updated_at = $2
		WHERE id = ANY($3)
		  AND status = $4
	`

	cmdTag, err := executor(ctx, r.db).Exec(
		ctx,
		query,
		newStatus.String(),
		updatedAt.UTC(),
		bookingIDs,
		expectedStatus.String(),
	)
	if err != nil {
		return 0, fmt.Errorf("error ejecutando actualización en lote de reservas vencidas: %w", err)
	}

	return int(cmdTag.RowsAffected()), nil
}
