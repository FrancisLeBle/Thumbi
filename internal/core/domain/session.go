package domain

import (
	"time"
)

// SessionTTL define el tiempo de vida estricto del access_token (RF-04: exactamente 20 minutos)
const SessionTTL = 20 * time.Minute

// RefreshThreshold define la ventana de tiempo para notificar al usuario antes de la expiración (RF-05)
const RefreshThreshold = 5 * time.Minute

// Session representa la entidad de sesión activa de un usuario en el sistema
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	UserAgent string    `json:"user_agent,omitempty"`
	ClientIP  string    `json:"client_ip,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NewSession instancia una nueva sesión con exactamente 20 minutos de TTL (RF-04)
func NewSession(id, userID, token, userAgent, clientIP string) *Session {
	now := time.Now().UTC()
	return &Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		UserAgent: userAgent,
		ClientIP:  clientIP,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionTTL),
	}
}

// IsExpired valida si la sesión ha superado su TTL de 20 minutos (RF-06)
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// NeedsRefreshWarning determina si la sesión está próxima a expirar y se debe alertar al cliente (RF-05)
func (s *Session) NeedsRefreshWarning() bool {
	remaining := time.Until(s.ExpiresAt)
	return remaining > 0 && remaining <= RefreshThreshold
}

// UserClaims estructura los datos de identidad que viajan de forma segura dentro del JWT
type UserClaims struct {
	UserID    string   `json:"user_id"`
	Email     string   `json:"email"`
	Role      UserRole `json:"role"`
	KYCStatus KYCStatus `json:"kyc_status"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

// NewUserClaims genera los claims respetando el TTL de 20 minutos
func NewUserClaims(user *User) UserClaims {
	now := time.Now().UTC()
	return UserClaims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		KYCStatus: user.KYCStatus,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(SessionTTL).Unix(),
	}
}
