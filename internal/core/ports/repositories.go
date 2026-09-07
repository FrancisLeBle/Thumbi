package ports

import (
	"context"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// UserRepository define el contrato de persistencia para la entidad User
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id string) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

// SessionRepository define el contrato para almacenar, consultar e invalidar sesiones activas en caché/Redis con TTL
type SessionRepository interface {
	Save(ctx context.Context, session *domain.Session, ttl time.Duration) error
	GetByID(ctx context.Context, sessionID string) (*domain.Session, error)
	GetByToken(ctx context.Context, token string) (*domain.Session, error)
	Delete(ctx context.Context, sessionID string) error
	DeleteByUserID(ctx context.Context, userID string) error
}

// KYCRepository define el contrato para el historial y seguimiento de verificaciones KYC
type KYCRepository interface {
	Create(ctx context.Context, kyc *domain.KYCVerification) error
	GetByID(ctx context.Context, id string) (*domain.KYCVerification, error)
	GetLatestByUserID(ctx context.Context, userID string) (*domain.KYCVerification, error)
	Update(ctx context.Context, kyc *domain.KYCVerification) error
}

// VehicleRepository define el contrato de persistencia para vehículos y trámites asíncronos
type VehicleRepository interface {
	Create(ctx context.Context, vehicle *domain.Vehicle) error
	GetByID(ctx context.Context, id string) (*domain.Vehicle, error)
	GetByUserID(ctx context.Context, userID string) (*domain.Vehicle, error)
	GetByPlateNumber(ctx context.Context, plateNumber string) (*domain.Vehicle, error)
	Update(ctx context.Context, vehicle *domain.Vehicle) error
	ListPendingVerification(ctx context.Context, limit, offset int) ([]*domain.Vehicle, error)
}

// SearchTripsParams contiene los criterios de búsqueda geoespacial para la consulta en PostGIS
type SearchTripsParams struct {
	OriginCoords    domain.Coordinates `json:"origin_coords"`
	DestCoords      domain.Coordinates `json:"dest_coords"`
	ToleranceMeters float64            `json:"tolerance_meters"`
	DepartureFrom   time.Time          `json:"departure_from"`
	DepartureTo     time.Time          `json:"departure_to"`
	RequiredSeats   int                `json:"required_seats"`
	Limit           int                `json:"limit"`
	Offset          int                `json:"offset"`
}

// MatchedTripRecord representa la proyección de un viaje coincidente desde la consulta de PostGIS
type MatchedTripRecord struct {
	Trip                 domain.Trip `json:"trip"`
	DriverFirstName      string      `json:"driver_first_name"`
	DriverLastName       string      `json:"driver_last_name"`
	DriverAvatarURL      string      `json:"driver_avatar_url"`
	VehicleBrand         string      `json:"vehicle_brand"`
	VehicleModel         string      `json:"vehicle_model"`
	VehiclePlate         string      `json:"vehicle_plate"`
	PickupWalkDistanceM  float64     `json:"pickup_walk_distance_m"`
	DropoffWalkDistanceM float64     `json:"dropoff_walk_distance_m"`
}

// TripRepository define el contrato de persistencia y consultas geoespaciales en PostGIS
type TripRepository interface {
	Save(ctx context.Context, trip *domain.Trip) error
	FindByID(ctx context.Context, id string) (*domain.Trip, error)
	SearchMatchingTrips(ctx context.Context, params SearchTripsParams) ([]MatchedTripRecord, error)
	UpdateStatus(ctx context.Context, tripID string, newStatus domain.TripStatus) error
	ListByDriverID(ctx context.Context, driverID string, limit, offset int) ([]*domain.Trip, error)
}

// BookingRepository define el contrato para operaciones transaccionales y consultas de reservas
type BookingRepository interface {
	// CreateWithHold descuenta atómicamente los asientos y crea la reserva en estado PENDING_PAYMENT
	CreateWithHold(ctx context.Context, booking *domain.Booking) error
	FindByID(ctx context.Context, id string) (*domain.Booking, error)
	UpdateStatus(ctx context.Context, bookingID string, newStatus domain.BookingStatus, reason *string) error
	GetActiveBookingsByTrip(ctx context.Context, tripID string) ([]*domain.Booking, error)
	GetPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*domain.Booking, error)
	// ExpirePendingBookings transiciona reservas vencidas y reintegra los asientos al viaje
	ExpirePendingBookings(ctx context.Context, now time.Time) (int, error)
}

// EscrowRepository define el contrato para el registro y liquidación de fondos en custodia
type EscrowRepository interface {
	CreateEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error
	FindByBookingID(ctx context.Context, bookingID string) (*domain.EscrowTransaction, error)
	UpdateEscrowStatus(ctx context.Context, escrowID string, newStatus domain.EscrowStatus) error
	RecordRefund(ctx context.Context, refund *domain.RefundTransaction) error
}

// ReviewRepository define el contrato de persistencia y consultas para el sistema reputacional
type ReviewRepository interface {
	Save(ctx context.Context, review *domain.Review) error
	FindByID(ctx context.Context, id string) (*domain.Review, error)
	FindByBookingAndReviewer(ctx context.Context, bookingID, reviewerID string) (*domain.Review, error)
	ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Review, error)
	GetAverageRating(ctx context.Context, userID string) (float64, int, error)
}

// DisputeRepository define el contrato de persistencia para el seguimiento de reclamos y disputas
type DisputeRepository interface {
	Save(ctx context.Context, dispute *domain.Dispute) error
	FindByID(ctx context.Context, id string) (*domain.Dispute, error)
	FindByEscrowID(ctx context.Context, escrowID string) (*domain.Dispute, error)
	Update(ctx context.Context, dispute *domain.Dispute) error
	List(ctx context.Context, limit, offset int) ([]*domain.Dispute, error)
}
