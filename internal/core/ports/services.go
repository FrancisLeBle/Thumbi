package ports

import (
	"context"

	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// AuthResult encapsula el resultado exitoso de autenticación
type AuthResult struct {
	User        *domain.User    `json:"user"`
	Session     *domain.Session `json:"session"`
	AccessToken string          `json:"access_token"`
	ExpiresIn   int64           `json:"expires_in"` // Segundos hasta la expiración (1200s = 20m)
	IsNewUser   bool            `json:"is_new_user"`
}

// SocialAuthInput DTO para recibir el token social desde el cliente
type SocialAuthInput struct {
	Provider    domain.AuthProvider `json:"provider"`
	IDToken     string              `json:"id_token"`
	UserAgent   string              `json:"user_agent"`
	ClientIP    string              `json:"client_ip"`
}

// KYCSubmissionInput DTO para la carga de documentos y selfie 3D
type KYCSubmissionInput struct {
	UserID         string `json:"user_id"`
	DocumentType   string `json:"document_type"`
	DocumentNumber string `json:"document_number"`
	FrontImageBase64 string `json:"front_image_base64"`
	BackImageBase64  string `json:"back_image_base64"`
	Selfie3DBase64   string `json:"selfie_3d_base64"`
}

// VehicleRegistrationInput DTO para el registro de vehículo
type VehicleRegistrationInput struct {
	UserID             string `json:"user_id"`
	Brand              string `json:"brand"`
	Model              string `json:"model"`
	Year               int    `json:"year"`
	PlateNumber        string `json:"plate_number"`
	Color              string `json:"color"`
	SeatCapacity       int    `json:"seat_capacity"`
	DriverLicenseBase64 string `json:"driver_license_base64"`
	VehicleCedulaBase64 string `json:"vehicle_cedula_base64"`
	InsuranceBase64    string `json:"insurance_base64,omitempty"`
}

// AuthService define los casos de uso para autenticación social y ciclo de vida de sesiones
type AuthService interface {
	LoginWithSocial(ctx context.Context, input SocialAuthInput) (*AuthResult, error)
	RefreshSession(ctx context.Context, currentToken string, userAgent, clientIP string) (*AuthResult, error)
	InvalidateSession(ctx context.Context, sessionID string) error
	ValidateToken(ctx context.Context, token string) (*domain.UserClaims, error)
}

// KYCService define los casos de uso para verificación de identidad y prueba de vida
type KYCService interface {
	SubmitVerification(ctx context.Context, input KYCSubmissionInput) (*domain.KYCVerification, error)
	GetStatus(ctx context.Context, userID string) (*domain.KYCVerification, error)
	ReviewManually(ctx context.Context, kycID string, approved bool, reason string) error
}

// VehicleService define los casos de uso para el alta y tramitación asíncrona de vehículos
type VehicleService interface {
	RegisterVehicle(ctx context.Context, input VehicleRegistrationInput) (*domain.Vehicle, error)
	GetVehicleStatus(ctx context.Context, userID string) (*domain.Vehicle, error)
	ProcessAsyncResolution(ctx context.Context, vehicleID string, approved bool, reason string) error
}

// StopInput DTO para ingresar una parada intermedia al crear un viaje
type StopInput struct {
	StopOrder            int                `json:"stop_order"`
	LocationTitle        string             `json:"location_title"`
	Coords               domain.Coordinates `json:"coords"`
	EstimatedArrivalTime string             `json:"estimated_arrival_time"`
}

// CreateTripInput DTO para la publicación de un nuevo viaje por un conductor verificado
type CreateTripInput struct {
	DriverID          string             `json:"driver_id"`
	VehicleID         string             `json:"vehicle_id"`
	OriginTitle       string             `json:"origin_title"`
	OriginCoords      domain.Coordinates `json:"origin_coords"`
	DestinationTitle  string             `json:"destination_title"`
	DestinationCoords domain.Coordinates `json:"destination_coords"`
	DepartureTime     string             `json:"departure_time"` // ISO 8601 / RFC3339
	SeatsOffered      int                `json:"seats_offered"`
	PricePerSeat      float64            `json:"price_per_seat"`
	Stops             []StopInput        `json:"stops,omitempty"`
}

// SearchTripsQuery DTO con los parámetros de búsqueda enviados por el pasajero
type SearchTripsQuery struct {
	OriginCoords    domain.Coordinates `json:"origin_coords"`
	DestCoords      domain.Coordinates `json:"dest_coords"`
	DepartureDate   string             `json:"departure_date"` // YYYY-MM-DD
	RequiredSeats   int                `json:"required_seats"`
	ToleranceMeters float64            `json:"tolerance_meters"` // Default 10000m (10km)
	Limit           int                `json:"limit"`
	Offset          int                `json:"offset"`
}

// TripSearchResult DTO de respuesta para cada viaje coincidente en la búsqueda
type TripSearchResult struct {
	ID                   string            `json:"id"`
	DriverID             string            `json:"driver_id"`
	DriverFullName       string            `json:"driver_full_name"`
	DriverAvatarURL      string            `json:"driver_avatar_url"`
	VehicleBrand         string            `json:"vehicle_brand"`
	VehicleModel         string            `json:"vehicle_model"`
	VehiclePlate         string            `json:"vehicle_plate"`
	OriginTitle          string            `json:"origin_title"`
	DestinationTitle     string            `json:"destination_title"`
	DepartureTime        string            `json:"departure_time"`
	EstimatedArrivalTime string            `json:"estimated_arrival_time"`
	TotalDistanceKm      float64           `json:"total_distance_km"`
	TotalDurationMinutes int               `json:"total_duration_minutes"`
	AvailableSeats       int               `json:"available_seats"`
	PricePerSeat         float64           `json:"price_per_seat"`
	CapPricePerSeat      float64           `json:"cap_price_per_seat"`
	PickupWalkDistanceM  float64           `json:"pickup_walk_distance_m"`
	DropoffWalkDistanceM float64           `json:"dropoff_walk_distance_m"`
	Status               domain.TripStatus `json:"status"`
}

// TripDetailDTO DTO con el detalle completo del viaje, itinerario y desglose de Cap Pricing
type TripDetailDTO struct {
	Trip       *domain.Trip      `json:"trip"`
	Driver     *domain.User      `json:"driver"`
	Vehicle    *domain.Vehicle   `json:"vehicle"`
	CostMatrix domain.CostMatrix `json:"cost_matrix"`
}

// TripService define los casos de uso para publicación, cálculo de Cap Pricing, búsqueda y cancelación de viajes
type TripService interface {
	CreateTrip(ctx context.Context, input CreateTripInput) (*domain.Trip, error)
	SearchTrips(ctx context.Context, query SearchTripsQuery) ([]TripSearchResult, error)
	GetTripByID(ctx context.Context, tripID string) (*TripDetailDTO, error)
	CancelTrip(ctx context.Context, tripID, driverID string) error
	CompleteTrip(ctx context.Context, tripID, driverID string) (*domain.Trip, error)
}

// CreateBookingInput DTO para solicitar la reserva de asientos en un viaje
type CreateBookingInput struct {
	TripID          string  `json:"trip_id"`
	PassengerID     string  `json:"passenger_id"`
	SeatsRequested  int     `json:"seats_requested"`
	PickupStopID    *string `json:"pickup_stop_id,omitempty"`
	DropoffStopID   *string `json:"dropoff_stop_id,omitempty"`
}

// ConfirmBookingPaymentInput DTO para acreditar la captura de pago de una reserva
type ConfirmBookingPaymentInput struct {
	BookingID         string `json:"booking_id"`
	PaymentGatewayRef string `json:"payment_gateway_ref"`
}

// CancelBookingInput DTO para cancelar una reserva activa
type CancelBookingInput struct {
	BookingID        string `json:"booking_id"`
	RequestingUserID string `json:"requesting_user_id"`
	Reason           string `json:"reason"`
}

// BookingTripSummary resumen contextual del viaje asociado a la reserva
type BookingTripSummary struct {
	TripID           string    `json:"trip_id"`
	OriginTitle      string    `json:"origin_title"`
	DestinationTitle string    `json:"destination_title"`
	DepartureTime    string    `json:"departure_time"`
	DriverID         string    `json:"driver_id"`
	DriverFullName   string    `json:"driver_full_name"`
	VehicleBrand     string    `json:"vehicle_brand"`
	VehicleModel     string    `json:"vehicle_model"`
	VehiclePlate     string    `json:"vehicle_plate"`
}

// BookingDTO proyección enriquecida de la reserva para el cliente y APIs
type BookingDTO struct {
	ID                 string               `json:"id"`
	TripID             string               `json:"trip_id"`
	PassengerID        string               `json:"passenger_id"`
	SeatsBooked        int                  `json:"seats_booked"`
	UnitPrice          float64              `json:"unit_price"`
	TotalPrice         float64              `json:"total_price"`
	Status             domain.BookingStatus `json:"status"`
	PickupStopID       *string              `json:"pickup_stop_id,omitempty"`
	DropoffStopID      *string              `json:"dropoff_stop_id,omitempty"`
	ExpiresAt          string               `json:"expires_at"`
	ConfirmedAt        *string              `json:"confirmed_at,omitempty"`
	CancelledAt        *string              `json:"cancelled_at,omitempty"`
	CancellationReason *string              `json:"cancellation_reason,omitempty"`
	CreatedAt          string               `json:"created_at"`
	PaymentIntent      *PaymentIntentResult `json:"payment_intent,omitempty"`
	TripSummary        *BookingTripSummary  `json:"trip_summary,omitempty"`
}

// ContactLinkDTO representa el Deeplink de WhatsApp generado y el teléfono del conductor
type ContactLinkDTO struct {
	WhatsAppURL string `json:"whatsapp_url"`
	DriverPhone string `json:"driver_phone"`
}

// BookingService define los casos de uso para gestión de reservas, pagos en Escrow y cancelaciones
type BookingService interface {
	CreateBooking(ctx context.Context, input CreateBookingInput) (*BookingDTO, error)
	ConfirmBookingPayment(ctx context.Context, input ConfirmBookingPaymentInput) (*BookingDTO, error)
	CancelBooking(ctx context.Context, input CancelBookingInput) (*domain.RefundTransaction, error)
	GetBookingByID(ctx context.Context, bookingID, requestingUserID string) (*BookingDTO, error)
	GetContactLink(ctx context.Context, bookingID, requestingUserID string) (*ContactLinkDTO, error)
	ListPassengerBookings(ctx context.Context, passengerID string, limit, offset int) ([]*BookingDTO, error)
	ListTripBookings(ctx context.Context, tripID, driverID string) ([]*BookingDTO, error)
	ProcessExpiredBookings(ctx context.Context) (int, error)
}

// CreateReviewInput DTO para registrar una nueva reseña
type CreateReviewInput struct {
	BookingID  string `json:"booking_id"`
	ReviewerID string `json:"reviewer_id"`
	Rating     int    `json:"rating"`
	Comment    string `json:"comment"`
}

// ReviewDTO proyección enriquecida de una reseña
type ReviewDTO struct {
	ID                string `json:"id"`
	TripID            string `json:"trip_id"`
	BookingID         string `json:"booking_id"`
	ReviewerID        string `json:"reviewer_id"`
	ReviewerFirstName string `json:"reviewer_first_name,omitempty"`
	ReviewerLastName  string `json:"reviewer_last_name,omitempty"`
	ReviewerAvatarURL string `json:"reviewer_avatar_url,omitempty"`
	RevieweeID        string `json:"reviewee_id"`
	Rating            int    `json:"rating"`
	Comment           string `json:"comment"`
	CreatedAt         string `json:"created_at"`
}

// UserReputationDTO proyección del perfil reputacional de un usuario
type UserReputationDTO struct {
	UserID      string       `json:"user_id"`
	RatingAvg   float64      `json:"rating_avg"`
	RatingCount int          `json:"rating_count"`
	Reviews     []*ReviewDTO `json:"reviews"`
}

// ReputationService define los casos de uso para la emisión de calificaciones y consulta reputacional
type ReputationService interface {
	CreateReview(ctx context.Context, input CreateReviewInput) (*ReviewDTO, error)
	GetUserReviews(ctx context.Context, userID string, limit, offset int) (*UserReputationDTO, error)
}

// OpenDisputeInput DTO para la apertura formal de una disputa sobre un escrow
type OpenDisputeInput struct {
	BookingID    string               `json:"booking_id"`
	ReporterID   string               `json:"reporter_id"`
	Reason       domain.DisputeReason `json:"reason"`
	Description  string               `json:"description"`
	EvidenceURLs []string             `json:"evidence_urls,omitempty"`
}

// ResolveDisputeInput DTO para la resolución administrativa de una disputa
type ResolveDisputeInput struct {
	DisputeID  string               `json:"dispute_id"`
	AdminID    string               `json:"admin_id"`
	Resolution domain.DisputeStatus `json:"resolution"` // RESOLVED_PASSENGER_REFUND, RESOLVED_DRIVER_PAYOUT, REJECTED
	AdminNotes string               `json:"admin_notes"`
}

// DisputeDTO proyección de una disputa para clientes y mesa de control
type DisputeDTO struct {
	ID                  string               `json:"id"`
	EscrowTransactionID string               `json:"escrow_transaction_id"`
	BookingID           string               `json:"booking_id"`
	TripID              string               `json:"trip_id"`
	ReporterID          string               `json:"reporter_id"`
	DefendantID         string               `json:"defendant_id"`
	Reason              domain.DisputeReason `json:"reason"`
	Description         string               `json:"description"`
	EvidenceURLs        []string             `json:"evidence_urls"`
	Status              domain.DisputeStatus `json:"status"`
	AdminNotes          *string              `json:"admin_notes,omitempty"`
	ResolvedBy          *string              `json:"resolved_by,omitempty"`
	ResolvedAt          *string              `json:"resolved_at,omitempty"`
	CreatedAt           string               `json:"created_at"`
	UpdatedAt           string               `json:"updated_at"`
}

// DisputeService define los casos de uso para la gestión y arbitraje de reclamos en custodia
type DisputeService interface {
	OpenDispute(ctx context.Context, input OpenDisputeInput) (*DisputeDTO, error)
	ResolveDispute(ctx context.Context, input ResolveDisputeInput) (*DisputeDTO, error)
	GetDisputeByID(ctx context.Context, disputeID string) (*DisputeDTO, error)
	ListDisputes(ctx context.Context, limit, offset int) ([]*DisputeDTO, error)
}

// CreateTripStopInput representa los datos para agregar una parada
type CreateTripStopInput struct {
	TripID        string  `json:"trip_id"`
	LocationName  string  `json:"location_name"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	StopOrder     int     `json:"stop_order"`
	EstimatedTime string  `json:"estimated_time"`
}
