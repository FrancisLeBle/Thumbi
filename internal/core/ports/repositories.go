package ports

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
)

// UserRepository define el contrato de persistencia para la entidad User
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// IdentityRepository define el contrato de persistencia para identidades federadas (OAuth) en la tabla user_identities
type IdentityRepository interface {
	Create(ctx context.Context, identity *domain.UserIdentity) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.UserIdentity, error)
	GetByProviderAndID(ctx context.Context, provider domain.AuthProvider, providerUserID string) (*domain.UserIdentity, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.UserIdentity, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// SessionRepository define el contrato para almacenar, consultar e invalidar sesiones activas con TTL
type SessionRepository interface {
	Save(ctx context.Context, session *domain.Session, ttl time.Duration) error
	GetByToken(ctx context.Context, token string) (*domain.Session, error)
	Delete(ctx context.Context, sessionID uuid.UUID) error
}

// KYCRepository define el contrato para el historial y seguimiento de verificaciones KYC
type KYCRepository interface {
	Create(ctx context.Context, kyc *domain.KYCVerification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.KYCVerification, error)
	GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*domain.KYCVerification, error)
	Update(ctx context.Context, kyc *domain.KYCVerification) error
}

// VehicleRepository define el contrato de persistencia para vehículos y trámites asíncronos
type VehicleRepository interface {
	Create(ctx context.Context, vehicle *domain.Vehicle) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Vehicle, error)
	GetByPlateNumber(ctx context.Context, plateNumber string) (*domain.Vehicle, error)
	Update(ctx context.Context, vehicle *domain.Vehicle) error
}

// SearchTripsParams contiene los criterios de búsqueda geoespacial para la consulta en PostGIS
type SearchTripsParams struct {
	OriginCoords    domain.Coordinates
	DestCoords      domain.Coordinates
	ToleranceMeters float64
	DepartureFrom   time.Time
	DepartureTo     time.Time
	RequiredSeats   int
	Limit           int
	Offset          int
}

// MatchedTripRecord representa la proyección de un viaje coincidente desde la consulta de PostGIS
type MatchedTripRecord struct {
	Trip                 domain.Trip
	DriverFirstName      string
	DriverLastName       string
	DriverAvatarURL      string
	VehicleBrand         string
	VehicleModel         string
	VehiclePlate         string
	RoutePreviewPolyline string
	PickupWalkDistanceM  float64
	DropoffWalkDistanceM float64
}

// TransactionManager (UnitOfWork / TxManager) ejecuta múltiples operaciones de repositorio dentro de una única transacción atómica de base de datos
type TransactionManager interface {
	WithinTransaction(ctx context.Context, fn func(txCtx context.Context) error) error
}

// UnitOfWork es un alias semántico de TransactionManager para coordinar unidades de trabajo ACID en servicios
type UnitOfWork = TransactionManager

// TxManager es un alias abreviado de TransactionManager
type TxManager = TransactionManager

// TripRepository define el contrato de persistencia y consultas geoespaciales en PostGIS
type TripRepository interface {
	Save(ctx context.Context, trip *domain.Trip) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Trip, error)
	// FindByIDsForUpdate bloquea y retorna múltiples viajes en orden determinista de ID para coordinación en lote.
	FindByIDsForUpdate(ctx context.Context, ids []uuid.UUID) ([]*domain.Trip, error)
	SearchMatchingTrips(ctx context.Context, params SearchTripsParams) ([]MatchedTripRecord, error)
	UpdateStatus(ctx context.Context, tripID uuid.UUID, newStatus domain.TripStatus) error
	// ReleaseSeats aplica atómicamente una restitución de plazas decidida por el dominio y reporta si se aplicó.
	ReleaseSeats(ctx context.Context, transition domain.SeatTransition) (applied bool, err error)
	// ApplySeatTransitionsBatch aplica en lote restituciones de plazas y transiciones decididas por el dominio.
	ApplySeatTransitionsBatch(ctx context.Context, transitions []domain.SeatTransition, updatedAt time.Time) (int, error)
	// CalculateRouteDistanceKm calcula en PostGIS la longitud geodésica (WGS 84) de la ruta en kilómetros.
	CalculateRouteDistanceKm(ctx context.Context, routePath domain.LineString) (float64, error)
}

// BookingRepository define el contrato para operaciones transaccionales y consultas de reservas
type BookingRepository interface {
	// CreateWithHold aplica atómicamente el descuento de plazas validado por el dominio mediante UPDATE con guarda
	// e inserta la reserva si la guarda se cumplió, reportando applied = true/false.
	CreateWithHold(ctx context.Context, booking *domain.Booking, seatHold domain.SeatTransition) (applied bool, err error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Booking, error)
	FindByPaymentIntentID(ctx context.Context, intentID string) (*domain.Booking, error)
	UpdateStatus(ctx context.Context, bookingID uuid.UUID, newStatus domain.BookingStatus, reason *string) error
	UpdatePaymentIntent(ctx context.Context, bookingID uuid.UUID, intentID string) error
	GetActiveBookingsByTrip(ctx context.Context, tripID uuid.UUID) ([]*domain.Booking, error)
	GetPassengerBookings(ctx context.Context, passengerID uuid.UUID, limit, offset int) ([]*domain.Booking, error)
	// CheckPassengerBookingConstraints verifica en una sola consulta JOIN (bookings + trips) el solapamiento horario (tstzrange &&)
	// y contabiliza las reservas PENDING_PAYMENT activas sin límite de paginación.
	CheckPassengerBookingConstraints(ctx context.Context, passengerID, targetTripID uuid.UUID, departureTime, estimatedArrival, now time.Time) (hasOverlap bool, activePendingCount int, err error)
	// LockExpiredPendingBookings reclama un lote acotado de reservas vencidas mediante FOR UPDATE SKIP LOCKED.
	LockExpiredPendingBookings(ctx context.Context, status domain.BookingStatus, overdueAt time.Time, limit int) ([]*domain.Booking, error)
	// MarkBookingsExpiredBatch persiste en lote el estado EXPIRED decidido por el dominio sobre las reservas reclamadas.
	MarkBookingsExpiredBatch(ctx context.Context, bookingIDs []uuid.UUID, expectedStatus, newStatus domain.BookingStatus, updatedAt time.Time) (int, error)
}

// EscrowRepository define el contrato para el registro y liquidación de fondos en custodia
type EscrowRepository interface {
	CreateEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error
	FindByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error)
	UpdateEscrowStatus(ctx context.Context, escrowID uuid.UUID, newStatus domain.EscrowStatus) error
	RecordRefund(ctx context.Context, refund *domain.RefundTransaction) error
}

// ReviewRepository define el contrato de persistencia y consultas para el sistema reputacional
type ReviewRepository interface {
	Save(ctx context.Context, review *domain.Review) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Review, error)
	FindByBookingAndReviewer(ctx context.Context, bookingID, reviewerID uuid.UUID) (*domain.Review, error)
	ListByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.Review, error)
	GetAverageRating(ctx context.Context, userID uuid.UUID) (float64, int, error)
}

// DisputeRepository define el contrato de persistencia para el seguimiento de reclamos y disputas
type DisputeRepository interface {
	Save(ctx context.Context, dispute *domain.Dispute) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Dispute, error)
	FindByEscrowID(ctx context.Context, escrowID uuid.UUID) (*domain.Dispute, error)
	Update(ctx context.Context, dispute *domain.Dispute) error
	List(ctx context.Context, limit, offset int) ([]*domain.Dispute, error)
}
