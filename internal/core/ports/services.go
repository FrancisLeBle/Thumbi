package ports

import (
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
)

// UserDTO proyección de aplicación del perfil de usuario y sus capacidades vigentes
type UserDTO struct {
	ID              uuid.UUID
	Email           string
	FirstName       string
	LastName        string
	Phone           string
	AvatarURL       string
	Provider        domain.AuthProvider
	Role            domain.UserRole
	KYCStatus       domain.KYCStatus
	IsDriverActive  bool
	RatingAvg       float64
	RatingCount     int
	CanBookRides    bool
	CanPublishRides bool
	CreatedAt       string
	UpdatedAt       string
}

// AuthResult encapsula el resultado exitoso de autenticación
type AuthResult struct {
	User        *UserDTO
	SessionID   uuid.UUID
	AccessToken string
	ExpiresIn   int64 // Segundos hasta la expiración (1200s = 20m)
	IsNewUser   bool
}

// SocialAuthInput parámetros puros de aplicación para autenticación social
type SocialAuthInput struct {
	Provider  domain.AuthProvider
	IDToken   string
	UserAgent string
	ClientIP  string
}

// KYCSubmissionInput parámetros puros de aplicación para la carga de documentos y selfie 3D
type KYCSubmissionInput struct {
	UserID           uuid.UUID
	DocumentType     string
	DocumentNumber   string
	FrontImageBase64 string
	BackImageBase64  string
	Selfie3DBase64   string
}

// KYCDTO proyección de aplicación del estado de verificación KYC
type KYCDTO struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	DocumentType     string
	DocumentNumber   string
	LivenessScore    float64
	RetryCount       int
	RemainingRetries int
	CanRetry         bool
	Status           domain.KYCStatus
	RejectionReason  string
	CreatedAt        string
	UpdatedAt        string
}

// VehicleRegistrationInput parámetros puros de aplicación para el registro de vehículo
type VehicleRegistrationInput struct {
	UserID              uuid.UUID
	Brand               string
	Model               string
	Year                int
	PlateNumber         string
	Color               string
	SeatCapacity        int
	DriverLicenseBase64 string
	VehicleCedulaBase64 string
	InsuranceBase64     string
}

// VehicleDTO proyección de aplicación del estado y datos de un vehículo registrado
type VehicleDTO struct {
	ID                 uuid.UUID
	UserID             uuid.UUID
	Brand              string
	Model              string
	Year               int
	PlateNumber        string
	Color              string
	SeatCapacity       int
	DriverLicenseURL   string
	VehicleCedulaURL   string
	InsurancePolicyURL string
	Status             domain.VehicleStatus
	RejectionReason    string
	VerifiedAt         *string
	CreatedAt          string
	UpdatedAt          string
}

// CreateTripStopInput parámetros puros de aplicación para ingresar una parada intermedia al crear un viaje
type CreateTripStopInput struct {
	StopOrder            int
	LocationTitle        string
	Coords               domain.Coordinates
	EstimatedArrivalTime string
}

// CreateTripInput parámetros puros de aplicación para la publicación de un nuevo viaje por un conductor verificado
type CreateTripInput struct {
	DriverID          uuid.UUID
	VehicleID         uuid.UUID
	OriginTitle       string
	OriginCoords      domain.Coordinates
	DestinationTitle  string
	DestinationCoords domain.Coordinates
	DepartureTime     string // ISO 8601 / RFC3339
	SeatsOffered      int
	Currency          domain.Currency
	PricePerSeat      domain.Money
	Stops             []CreateTripStopInput
}

// TripStopDTO proyección de aplicación de una parada intermedia planificada
type TripStopDTO struct {
	ID                   uuid.UUID
	TripID               uuid.UUID
	StopOrder            int
	LocationTitle        string
	Coords               domain.Coordinates
	EstimatedArrivalTime string
	CreatedAt            string
}

// TripDTO proyección de aplicación de un viaje publicado o gestionado
type TripDTO struct {
	ID                   uuid.UUID
	DriverID             uuid.UUID
	VehicleID            uuid.UUID
	OriginTitle          string
	OriginCoords         domain.Coordinates
	DestinationTitle     string
	DestinationCoords    domain.Coordinates
	RoutePath            domain.LineString
	RoutePreviewPolyline string
	DepartureTime        string
	EstimatedArrivalTime string
	TotalDistanceKm      float64
	TotalDurationMinutes int
	TotalSeatsOffered    int
	AvailableSeats       int
	Currency             domain.Currency
	PricePerSeat         domain.Money
	CapPricePerSeat      domain.Money
	EstimatedFuelCost    domain.Money
	EstimatedTollCost    domain.Money
	Status               domain.TripStatus
	Stops                []TripStopDTO
	CreatedAt            string
	UpdatedAt            string
}

// SearchTripsQuery parámetros puros de aplicación con los criterios de búsqueda de viajes
type SearchTripsQuery struct {
	OriginCoords    domain.Coordinates
	DestCoords      domain.Coordinates
	DepartureDate   string // YYYY-MM-DD
	RequiredSeats   int
	ToleranceMeters float64 // Default 10000m (10km)
	Limit           int
	Offset          int
}

// TripSearchResult proyección de aplicación para cada viaje coincidente en la búsqueda
type TripSearchResult struct {
	ID                   uuid.UUID
	DriverID             uuid.UUID
	DriverFullName       string
	DriverAvatarURL      string
	VehicleBrand         string
	VehicleModel         string
	VehiclePlate         string
	OriginTitle          string
	DestinationTitle     string
	DepartureTime        string
	EstimatedArrivalTime string
	TotalDistanceKm      float64
	TotalDurationMinutes int
	AvailableSeats       int
	Currency             domain.Currency
	PricePerSeat         domain.Money
	CapPricePerSeat      domain.Money
	PickupWalkDistanceM  float64
	DropoffWalkDistanceM float64
	RoutePreviewPolyline string
	Status               domain.TripStatus
}

// TripDetailDTO proyección de aplicación con el detalle completo del viaje, itinerario y desglose de Cap Pricing
type TripDetailDTO struct {
	Trip       *TripDTO
	Driver     *UserDTO
	Vehicle    *VehicleDTO
	CostMatrix domain.CostMatrix
}

// CreateBookingInput parámetros puros de aplicación para solicitar la reserva de asientos en un viaje
type CreateBookingInput struct {
	TripID         uuid.UUID
	PassengerID    uuid.UUID
	SeatsRequested int
	PickupStopID   *uuid.UUID
	DropoffStopID  *uuid.UUID
}

// ConfirmBookingPaymentInput parámetros puros de aplicación para acreditar la captura de pago de una reserva
type ConfirmBookingPaymentInput struct {
	BookingID         uuid.UUID
	RequestingUserID  uuid.UUID // ID del usuario autenticado (requerido para clientes)
	PaymentGatewayRef string
	IsWebhook         bool // True cuando proviene de un webhook firmado por la pasarela
}

// CancelBookingInput parámetros puros de aplicación para cancelar una reserva activa
type CancelBookingInput struct {
	BookingID        uuid.UUID
	RequestingUserID uuid.UUID
	Reason           string
}

// RefundDTO proyección de aplicación de una transacción de reembolso contable
type RefundDTO struct {
	ID                       uuid.UUID
	EscrowTransactionID      uuid.UUID
	BookingID                uuid.UUID
	PassengerRefundAmount    domain.Money
	DriverCompensationAmount domain.Money
	RefundType               domain.RefundType
	GatewayRefundRef         string
	ProcessedAt              string
	CreatedAt                string
}

// BookingTripSummary resumen contextual del viaje asociado a la reserva
type BookingTripSummary struct {
	TripID           uuid.UUID
	OriginTitle      string
	DestinationTitle string
	DepartureTime    string
	DriverID         uuid.UUID
	DriverFullName   string
	VehicleBrand     string
	VehicleModel     string
	VehiclePlate     string
}

// BookingDTO proyección enriquecida de la reserva en la capa de aplicación
type BookingDTO struct {
	ID                 uuid.UUID
	TripID             uuid.UUID
	PassengerID        uuid.UUID
	SeatsBooked        int
	Currency           domain.Currency
	UnitPrice          domain.Money
	TotalPrice         domain.Money
	Status             domain.BookingStatus
	PickupStopID       *uuid.UUID
	DropoffStopID      *uuid.UUID
	PaymentIntentID    *string
	ExpiresAt          string
	ConfirmedAt        *string
	CancelledAt        *string
	CancellationReason *string
	CreatedAt          string
	PaymentIntent      *PaymentIntentResult
	TripSummary        *BookingTripSummary
}

// ContactLinkDTO representa el Deeplink de WhatsApp generado y el teléfono del conductor
type ContactLinkDTO struct {
	WhatsAppURL string
	DriverPhone string
}

// CreateReviewInput parámetros puros de aplicación para registrar una nueva reseña
type CreateReviewInput struct {
	BookingID  uuid.UUID
	ReviewerID uuid.UUID
	Rating     int
	Comment    string
}

// ReviewDTO proyección enriquecida de una reseña en la capa de aplicación
type ReviewDTO struct {
	ID                uuid.UUID
	TripID            uuid.UUID
	BookingID         uuid.UUID
	ReviewerID        uuid.UUID
	ReviewerFirstName string
	ReviewerLastName  string
	ReviewerAvatarURL string
	RevieweeID        uuid.UUID
	Rating            int
	Comment           string
	CreatedAt         string
}

// UserReputationDTO proyección del perfil reputacional de un usuario en la capa de aplicación
type UserReputationDTO struct {
	UserID      uuid.UUID
	RatingAvg   float64
	RatingCount int
	Reviews     []*ReviewDTO
}

// OpenDisputeInput parámetros puros de aplicación para la apertura formal de una disputa sobre un escrow
type OpenDisputeInput struct {
	BookingID    uuid.UUID
	ReporterID   uuid.UUID
	Reason       domain.DisputeReason
	Description  string
	EvidenceURLs []string
}

// ResolveDisputeInput parámetros puros de aplicación para la resolución administrativa de una disputa
type ResolveDisputeInput struct {
	DisputeID  uuid.UUID
	AdminID    uuid.UUID
	Resolution domain.DisputeStatus // RESOLVED_PASSENGER_REFUND, RESOLVED_DRIVER_PAYOUT, REJECTED
	AdminNotes string
}

// DisputeDTO proyección de una disputa en la capa de aplicación
type DisputeDTO struct {
	ID                  uuid.UUID
	EscrowTransactionID uuid.UUID
	BookingID           uuid.UUID
	TripID              uuid.UUID
	ReporterID          uuid.UUID
	DefendantID         uuid.UUID
	Reason              domain.DisputeReason
	Description         string
	EvidenceURLs        []string
	Status              domain.DisputeStatus
	AdminNotes          *string
	ResolvedBy          *uuid.UUID
	ResolvedAt          *string
	CreatedAt           string
	UpdatedAt           string
}
