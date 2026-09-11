package domain

import (
	"time"
)

// UserRole define los roles disponibles en la plataforma Thumbi
type UserRole string

const (
	RolePassenger UserRole = "PASSENGER"
	RoleDriver    UserRole = "DRIVER"
)

// AuthProvider define los proveedores sociales soportados
type AuthProvider string

const (
	ProviderGoogle AuthProvider = "GOOGLE"
	ProviderApple  AuthProvider = "APPLE"
)

// KYCStatus define el estado general de verificación de identidad del usuario
type KYCStatus string

const (
	KYCStatusNotStarted         KYCStatus = "NOT_STARTED"
	KYCStatusPending            KYCStatus = "PENDING_VERIFICATION"
	KYCStatusApproved           KYCStatus = "APPROVED"
	KYCStatusRejected           KYCStatus = "REJECTED"
	KYCStatusPendingManualReview KYCStatus = "PENDING_MANUAL_REVIEW"
)

// User representa la entidad de usuario unificada con rol base de Pasajero (RF-01, RF-02, RF-03)
type User struct {
	ID           string       `json:"id"`
	Email        string       `json:"email"`
	FirstName    string       `json:"first_name"`
	LastName     string       `json:"last_name"`
	Phone        string       `json:"phone,omitempty"`
	AvatarURL    string       `json:"avatar_url,omitempty"`
	Provider     AuthProvider `json:"provider"`
	ProviderID   string       `json:"provider_id"`
	Role         UserRole     `json:"role"`
	KYCStatus    KYCStatus    `json:"kyc_status"`
	IsDriverActive bool       `json:"is_driver_active"`
	RatingAvg    float64      `json:"rating_avg"`
	RatingCount  int          `json:"rating_count"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// NewUser crea una nueva instancia de User garantizando los valores por defecto del dominio
func NewUser(id, email, firstName, lastName, avatarURL string, provider AuthProvider, providerID string) *User {
	now := time.Now().UTC()
	return &User{
		ID:             id,
		Email:          email,
		FirstName:      firstName,
		LastName:       lastName,
		AvatarURL:      avatarURL,
		Provider:       provider,
		ProviderID:     providerID,
		Role:           RolePassenger, // RF-02: Perfil unificado base de Pasajero por defecto
		KYCStatus:      KYCStatusNotStarted,
		IsDriverActive: false,
		RatingAvg:      5.00,
		RatingCount:    0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// FullName retorna el nombre completo formateado
func (u *User) FullName() string {
	if u.LastName == "" {
		return u.FirstName
	}
	return u.FirstName + " " + u.LastName
}

// CanBookRides valida si el usuario tiene habilitadas las funciones de búsqueda y reserva (RF-13)
// Retiene 100% de capacidad como pasajero aún si su verificación de conductor fue rechazada o está pendiente
func (u *User) CanBookRides() bool {
	return true
}

// CanPublishRides valida si el usuario puede crear y publicar viajes como conductor
func (u *User) CanPublishRides() bool {
	return u.Role == RoleDriver && u.KYCStatus == KYCStatusApproved && u.IsDriverActive
}

// PromoteToDriver promueve el usuario al rol de conductor si el KYC está aprobado
func (u *User) PromoteToDriver() error {
	if u.KYCStatus != KYCStatusApproved {
		return ErrDriverKYCRequired
	}
	u.Role = RoleDriver
	u.IsDriverActive = true
	u.UpdatedAt = time.Now().UTC()
	return nil
}

// UpdateReputation actualiza atómicamente el promedio y conteo de calificaciones del usuario
func (u *User) UpdateReputation(avg float64, count int) {
	if count < 0 {
		count = 0
	}
	if avg < 1.00 {
		avg = 1.00
	} else if avg > 5.00 {
		avg = 5.00
	}
	u.RatingAvg = avg
	u.RatingCount = count
	u.UpdatedAt = time.Now().UTC()
}
