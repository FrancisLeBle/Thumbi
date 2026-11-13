package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type bookingRepository struct {
	db DBExecutor
}

// NewBookingRepository crea una nueva instancia del repositorio PostgreSQL para reservas
func NewBookingRepository(db DBExecutor) ports.BookingRepository {
	return &bookingRepository{db: db}
}

// CreateWithHold descuenta atómicamente los asientos y crea la reserva en estado PENDING_PAYMENT con bloqueo pesimista
func (r *bookingRepository) CreateWithHold(ctx context.Context, booking *domain.Booking) error {
	type transactor interface {
		Begin(context.Context) (pgx.Tx, error)
	}

	exec := r.db
	var tx pgx.Tx
	var err error

	if existingTx := txFromContext(ctx); existingTx != nil {
		exec = existingTx
	} else if t, ok := r.db.(transactor); ok {
		tx, err = t.Begin(ctx)
		if err != nil {
			return fmt.Errorf("error iniciando transacción para crear reserva con hold: %w", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		exec = tx
	}

	// 1. Bloqueo pesimista de fila en trips (SELECT ... FOR UPDATE)
	lockQuery := `
		SELECT available_seats, status
		FROM trips
		WHERE id = $1
		FOR UPDATE
	`
	var availableSeats int
	var tripStatus string

	err = exec.QueryRow(ctx, lockQuery, booking.TripID).Scan(&availableSeats, &tripStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrTripNotFound
		}
		return fmt.Errorf("error al obtener y bloquear viaje con id %s: %w", booking.TripID, err)
	}

	// 2. Validaciones de estado e invariantes de asientos
	if domain.TripStatus(tripStatus) != domain.TripStatusPublished {
		return domain.ErrInvalidTripStatusChange
	}

	if availableSeats < booking.SeatsBooked {
		return domain.ErrInsufficientSeats
	}

	// 3. Decrementar asientos disponibles y transicionar a FULL si llega a 0
	newSeats := availableSeats - booking.SeatsBooked
	updateTripQuery := `
		UPDATE trips
		SET available_seats = $1,
		    status = CASE WHEN $1 = 0 THEN 'FULL'::trip_status_enum ELSE status END,
		    updated_at = NOW()
		WHERE id = $2
	`
	_, err = exec.Exec(ctx, updateTripQuery, newSeats, booking.TripID)
	if err != nil {
		return fmt.Errorf("error al actualizar asientos disponibles del viaje: %w", err)
	}

	// 4. Insertar la reserva en estado PENDING_PAYMENT con TTL
	insertBookingQuery := `
		INSERT INTO bookings (
			id, trip_id, passenger_id, seats_booked, unit_price, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		)
	`
	_, err = exec.Exec(
		ctx,
		insertBookingQuery,
		booking.ID,
		booking.TripID,
		booking.PassengerID,
		booking.SeatsBooked,
		moneyToPGNumeric(booking.UnitPrice),
		moneyToPGNumeric(booking.TotalPrice),
		string(booking.Status),
		booking.PickupStopID,
		booking.DropoffStopID,
		booking.PaymentIntentID,
		booking.ExpiresAt,
		booking.CreatedAt,
		booking.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23P01" {
			return domain.ErrOverlappingTripBooking
		}
		return fmt.Errorf("error al insertar la reserva: %w", err)
	}

	// 5. Commit si se inició la transacción aquí
	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("error al commitear transacción de reserva: %w", err)
		}
		tx = nil
	}

	return nil
}

// FindByID recupera una reserva por su identificador único
func (r *bookingRepository) FindByID(ctx context.Context, id string) (*domain.Booking, error) {
	query := `
		SELECT 
			id, trip_id, passenger_id, seats_booked, unit_price, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE id = $1
	`

	var (
		b             domain.Booking
		unitPriceNum  pgtype.Numeric
		totalPriceNum pgtype.Numeric
		statusStr     string
	)

	err := executor(ctx, r.db).QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&unitPriceNum,
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

	if b.UnitPrice, err = pgNumericToMoney(unitPriceNum); err != nil {
		return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
	}
	if b.TotalPrice, err = pgNumericToMoney(totalPriceNum); err != nil {
		return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
	}

	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

// FindByPaymentIntentID busca una reserva por el ID de la intención de pago emitida por la pasarela
func (r *bookingRepository) FindByPaymentIntentID(ctx context.Context, intentID string) (*domain.Booking, error) {
	query := `
		SELECT 
			id, trip_id, passenger_id, seats_booked, unit_price, total_price,
			status, pickup_stop_id, dropoff_stop_id, payment_intent_id, expires_at, confirmed_at, cancelled_at,
			cancellation_reason, created_at, updated_at
		FROM bookings
		WHERE payment_intent_id = $1
	`

	var (
		b             domain.Booking
		unitPriceNum  pgtype.Numeric
		totalPriceNum pgtype.Numeric
		statusStr     string
	)

	err := executor(ctx, r.db).QueryRow(ctx, query, intentID).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&unitPriceNum,
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

	if b.UnitPrice, err = pgNumericToMoney(unitPriceNum); err != nil {
		return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
	}
	if b.TotalPrice, err = pgNumericToMoney(totalPriceNum); err != nil {
		return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
	}

	b.Status = domain.BookingStatus(statusStr)
	return &b, nil
}

// UpdatePaymentIntent actualiza o asocia el identificador de orden/intención de la pasarela
func (r *bookingRepository) UpdatePaymentIntent(ctx context.Context, bookingID, intentID string) error {
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
func (r *bookingRepository) UpdateStatus(ctx context.Context, bookingID string, newStatus domain.BookingStatus, reason *string) error {
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

	cmd, err := executor(ctx, r.db).Exec(ctx, query, string(newStatus), reason, bookingID)
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
func (r *bookingRepository) GetActiveBookingsByTrip(ctx context.Context, tripID string) ([]*domain.Booking, error) {
	query := `
		SELECT
			id, trip_id, passenger_id, seats_booked, unit_price, total_price,
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
			b             domain.Booking
			unitPriceNum  pgtype.Numeric
			totalPriceNum pgtype.Numeric
			statusStr     string
		)
		if err := rows.Scan(
			&b.ID,
			&b.TripID,
			&b.PassengerID,
			&b.SeatsBooked,
			&unitPriceNum,
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
		if b.UnitPrice, err = pgNumericToMoney(unitPriceNum); err != nil {
			return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
		}
		if b.TotalPrice, err = pgNumericToMoney(totalPriceNum); err != nil {
			return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
		bookings = append(bookings, &b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando reservas activas del viaje %s: %w", tripID, err)
	}

	return bookings, nil
}

// GetPassengerBookings devuelve el historial paginado de reservas de un pasajero.
func (r *bookingRepository) GetPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*domain.Booking, error) {
	query := `
		SELECT
			id, trip_id, passenger_id, seats_booked, unit_price, total_price,
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
			b             domain.Booking
			unitPriceNum  pgtype.Numeric
			totalPriceNum pgtype.Numeric
			statusStr     string
		)
		if err := rows.Scan(
			&b.ID,
			&b.TripID,
			&b.PassengerID,
			&b.SeatsBooked,
			&unitPriceNum,
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
		if b.UnitPrice, err = pgNumericToMoney(unitPriceNum); err != nil {
			return nil, fmt.Errorf("error convirtiendo unit_price: %w", err)
		}
		if b.TotalPrice, err = pgNumericToMoney(totalPriceNum); err != nil {
			return nil, fmt.Errorf("error convirtiendo total_price: %w", err)
		}
		b.Status = domain.BookingStatus(statusStr)
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
	passengerID, targetTripID string,
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

// ExpirePendingBookings transiciona en lotes de hasta 500 reservas pendientes vencidas a EXPIRED
// utilizando FOR UPDATE SKIP LOCKED para evitar contención/duplicación entre réplicas concurrentes,
// y restituye los asientos afectados mediante un único UPDATE trips ... FROM agrupado por viaje.
func (r *bookingRepository) ExpirePendingBookings(ctx context.Context, now time.Time) (int, error) {
	query := `
		WITH locked_expired AS (
			SELECT id, trip_id, seats_booked
			FROM bookings
			WHERE status = 'PENDING_PAYMENT'
			  AND expires_at <= $1
			ORDER BY expires_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		),
		expired_bookings AS (
			UPDATE bookings b
			SET status = 'EXPIRED',
			    updated_at = NOW()
			FROM locked_expired l
			WHERE b.id = l.id
			RETURNING b.id, b.trip_id, b.seats_booked
		),
		seat_restores AS (
			SELECT trip_id, SUM(seats_booked)::INT AS seats_to_restore
			FROM expired_bookings
			GROUP BY trip_id
			ORDER BY trip_id ASC
		),
		updated_trips AS (
			UPDATE trips t
			SET available_seats = t.available_seats + sr.seats_to_restore,
			    status = CASE WHEN t.status = 'FULL' THEN 'PUBLISHED'::trip_status_enum ELSE t.status END,
			    updated_at = NOW()
			FROM seat_restores sr
			WHERE t.id = sr.trip_id
			RETURNING t.id
		)
		SELECT
			(SELECT COUNT(*)::INT FROM expired_bookings) AS expired_count,
			(SELECT COUNT(*)::INT FROM updated_trips) AS updated_trips_count
	`

	var expiredCount int
	var updatedTripsCount int
	err := executor(ctx, r.db).QueryRow(
		ctx,
		query,
		now.UTC(),
		defaultExpireBookingsBatchLimit,
	).Scan(&expiredCount, &updatedTripsCount)
	if err != nil {
		return 0, fmt.Errorf("error ejecutando expiración en lote de reservas vencidas: %w", err)
	}

	return expiredCount, nil
}
