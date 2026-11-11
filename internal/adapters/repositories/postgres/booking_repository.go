package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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

	if t, ok := r.db.(transactor); ok {
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
		booking.UnitPrice,
		booking.TotalPrice,
		string(booking.Status),
		booking.PickupStopID,
		booking.DropoffStopID,
		booking.PaymentIntentID,
		booking.ExpiresAt,
		booking.CreatedAt,
		booking.UpdatedAt,
	)
	if err != nil {
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

	var b domain.Booking
	var statusStr string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&b.UnitPrice,
		&b.TotalPrice,
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

	var b domain.Booking
	var statusStr string

	err := r.db.QueryRow(ctx, query, intentID).Scan(
		&b.ID,
		&b.TripID,
		&b.PassengerID,
		&b.SeatsBooked,
		&b.UnitPrice,
		&b.TotalPrice,
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

	cmd, err := r.db.Exec(ctx, query, intentID, bookingID)
	if err != nil {
		return fmt.Errorf("error actualizando payment_intent_id de reserva %s: %w", bookingID, err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrBookingNotFound
	}

	return nil
}

// ExpirePendingBookings transiciona reservas vencidas y reintegra los asientos al viaje atómicamente
func (r *bookingRepository) ExpirePendingBookings(ctx context.Context, now time.Time) (int, error) {
	type transactor interface {
		Begin(context.Context) (pgx.Tx, error)
	}

	exec := r.db
	var tx pgx.Tx
	var err error

	if t, ok := r.db.(transactor); ok {
		tx, err = t.Begin(ctx)
		if err != nil {
			return 0, fmt.Errorf("error iniciando transacción para expirar reservas: %w", err)
		}
		defer func() {
			if tx != nil {
				_ = tx.Rollback(ctx)
			}
		}()
		exec = tx
	}

	// 1. Bloquear y obtener reservas pendientes vencidas
	findExpiredQuery := `
		SELECT id, trip_id, seats_booked
		FROM bookings
		WHERE status = 'PENDING_PAYMENT' AND expires_at < $1
		FOR UPDATE
	`
	rows, err := exec.Query(ctx, findExpiredQuery, now)
	if err != nil {
		return 0, fmt.Errorf("error consultando reservas vencidas: %w", err)
	}

	type expiredItem struct {
		bookingID   string
		tripID      string
		seatsBooked int
	}
	var expiredList []expiredItem

	for rows.Next() {
		var item expiredItem
		if err := rows.Scan(&item.bookingID, &item.tripID, &item.seatsBooked); err != nil {
			rows.Close()
			return 0, fmt.Errorf("error escaneando reserva vencida: %w", err)
		}
		expiredList = append(expiredList, item)
	}
	rows.Close()

	if len(expiredList) == 0 {
		return 0, nil
	}

	// 2. Reintegrar asientos a cada viaje afectado
	restoreTripQuery := `
		UPDATE trips
		SET available_seats = available_seats + $1,
		    status = CASE WHEN status = 'FULL' THEN 'PUBLISHED'::trip_status_enum ELSE status END,
		    updated_at = NOW()
		WHERE id = $2
	`
	for _, item := range expiredList {
		_, err := exec.Exec(ctx, restoreTripQuery, item.seatsBooked, item.tripID)
		if err != nil {
			return 0, fmt.Errorf("error restituyendo asientos al viaje %s: %w", item.tripID, err)
		}
	}

	// 3. Actualizar estado de las reservas a EXPIRED
	expireBookingsQuery := `
		UPDATE bookings
		SET status = 'EXPIRED',
		    updated_at = NOW()
		WHERE status = 'PENDING_PAYMENT' AND expires_at < $1
	`
	cmd, err := exec.Exec(ctx, expireBookingsQuery, now)
	if err != nil {
		return 0, fmt.Errorf("error actualizando reservas a EXPIRED: %w", err)
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return 0, fmt.Errorf("error al commitear expiración de reservas: %w", err)
		}
		tx = nil
	}

	return int(cmd.RowsAffected()), nil
}
