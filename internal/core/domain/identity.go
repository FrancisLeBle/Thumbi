package domain

import (
	"time"
)

// UserIdentity representa la vinculación explícita, auditable y segura entre una cuenta de usuario interna
// y una identidad de un proveedor federado externo (Google, Apple, etc.).
// Esto desacopla la autenticación federada del correo electrónico simple, mitigando ataques de account takeover.
type UserIdentity struct {
	ID             string       `json:"id"`
	UserID         string       `json:"user_id"`
	Provider       AuthProvider `json:"provider"`
	ProviderUserID string       `json:"provider_user_id"`
	Email          string       `json:"email"`
	EmailVerified  bool         `json:"email_verified"`
	CreatedAt      time.Time    `json:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at"`
}

// NewUserIdentity crea una nueva instancia de UserIdentity garantizando timestamps en UTC
func NewUserIdentity(id, userID string, provider AuthProvider, providerUserID, email string, emailVerified bool) *UserIdentity {
	now := time.Now().UTC()
	return &UserIdentity{
		ID:             id,
		UserID:         userID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          email,
		EmailVerified:  emailVerified,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
