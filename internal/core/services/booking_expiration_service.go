package services

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

const defaultExpireBookingsBatchSize = 500

// BookingExpirationService implementa el caso de uso de expiración en lote de reservas
// pendientes de pago cuyo TTL venció, restituyendo atómicamente las plazas en los viajes.
type BookingExpirationService struct {
	bookingRepo ports.BookingRepository
	tripRepo    ports.TripRepository
	txManager   ports.TransactionManager
	clock       func() time.Time
}

// NewBookingExpirationService instancia el servicio dedicado de expiración en lote de reservas.
func NewBookingExpirationService(
	bookingRepo ports.BookingRepository,
	tripRepo ports.TripRepository,
	optionalDeps ...any,
) *BookingExpirationService {
	svc := &BookingExpirationService{
		bookingRepo: bookingRepo,
		tripRepo:    tripRepo,
		clock:       time.Now,
	}
	for _, opt := range optionalDeps {
		if tm, ok := opt.(ports.TransactionManager); ok {
			svc.txManager = tm
		}
	}
	return svc
}

func (s *BookingExpirationService) runInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if s.txManager != nil {
		return s.txManager.WithinTransaction(ctx, fn)
	}
	return fn(ctx)
}

// ProcessExpiredBookings reclama un lote acotado de reservas vencidas (FOR UPDATE SKIP LOCKED LIMIT 500),
// delega al dominio la expiración y las transiciones PUBLISHED <-> FULL de los viajes afectados,
// y persiste los cambios en lote dentro de una única transacción atómica.
func (s *BookingExpirationService) ProcessExpiredBookings(ctx context.Context) (int, error) {
	now := s.clock().UTC()
	var totalExpired int

	err := s.runInTx(ctx, func(txCtx context.Context) error {
		candidates, err := s.bookingRepo.LockExpiredPendingBookings(
			txCtx,
			domain.BookingStatusPendingPayment,
			now,
			defaultExpireBookingsBatchSize,
		)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			return nil
		}

		expiredIDs := make([]uuid.UUID, 0, len(candidates))
		seatsByTrip := make(map[uuid.UUID]int)
		for _, b := range candidates {
			if b == nil {
				continue
			}
			if err := b.ExpireIfOverdue(now); err != nil {
				continue
			}
			expiredIDs = append(expiredIDs, b.ID)
			seatsByTrip[b.TripID] += b.SeatsBooked
		}
		if len(expiredIDs) == 0 {
			return nil
		}

		updatedBookings, err := s.bookingRepo.MarkBookingsExpiredBatch(
			txCtx,
			expiredIDs,
			domain.BookingStatusPendingPayment,
			domain.BookingStatusExpired,
			now,
		)
		if err != nil {
			return err
		}
		totalExpired = updatedBookings
		if updatedBookings == 0 {
			return nil
		}

		tripIDs := make([]uuid.UUID, 0, len(seatsByTrip))
		for tripID := range seatsByTrip {
			tripIDs = append(tripIDs, tripID)
		}
		sort.Slice(tripIDs, func(i, j int) bool {
			return bytes.Compare(tripIDs[i][:], tripIDs[j][:]) < 0
		})

		trips, err := s.tripRepo.FindByIDsForUpdate(txCtx, tripIDs)
		if err != nil {
			return err
		}

		tripsByID := make(map[uuid.UUID]*domain.Trip, len(trips))
		for _, t := range trips {
			if t != nil {
				tripsByID[t.ID] = t
			}
		}

		transitions := make([]domain.SeatTransition, 0, len(tripIDs))
		for _, tripID := range tripIDs {
			trip, ok := tripsByID[tripID]
			if !ok {
				return fmt.Errorf("%w: viaje %s asociado a reserva expirada", domain.ErrTripNotFound, tripID)
			}
			transition, err := trip.ReleaseSeats(seatsByTrip[tripID], now)
			if err != nil {
				return err
			}
			transitions = append(transitions, transition)
		}

		appliedCount, err := s.tripRepo.ApplySeatTransitionsBatch(txCtx, transitions, now)
		if err != nil {
			return err
		}
		if appliedCount != len(transitions) {
			return domain.ErrConcurrentModification
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return totalExpired, nil
}
