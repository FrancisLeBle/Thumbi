package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

var (
	ErrTokenExpired       = errors.New("token expirado")
	ErrInvalidSignature   = errors.New("firma de token inválida")
	ErrMalformedToken     = errors.New("token malformado")
	ErrInvalidClaims      = errors.New("claims del token inválidos")
)

// TokenManager maneja la generación y validación de tokens JWT
type TokenManager struct {
	secretKey     []byte
	issuer        string
	tokenDuration time.Duration
}

// CustomClaims envuelve los claims de dominio junto con RegisteredClaims
type CustomClaims struct {
	UserID    string            `json:"user_id"`
	Email     string            `json:"email"`
	Role      domain.UserRole   `json:"role"`
	KYCStatus domain.KYCStatus  `json:"kyc_status"`
	jwt.RegisteredClaims
}

// NewTokenManager inicializa el generador de tokens
func NewTokenManager(secretKey string, issuer string) *TokenManager {
	return &TokenManager{
		secretKey:     []byte(secretKey),
		issuer:        issuer,
		tokenDuration: domain.SessionTTL, // 20 minutos estricto según RF-04
	}
}

// GenerateToken crea un nuevo JWT firmado con los datos del usuario y 20 min de TTL
func (m *TokenManager) GenerateToken(user *domain.User) (string, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(m.tokenDuration)

	claims := CustomClaims{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		KYCStatus: user.KYCStatus,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   user.ID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secretKey)
	if err != nil {
		return "", err
	}

	return signedToken, nil
}

// ValidateToken decodifica y verifica la validez del token JWT
func (m *TokenManager) ValidateToken(tokenString string) (*domain.UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidSignature
		}
		return m.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrSessionExpired
		}
		return nil, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	return &domain.UserClaims{
		UserID:    claims.UserID,
		Email:     claims.Email,
		Role:      claims.Role,
		KYCStatus: claims.KYCStatus,
		IssuedAt:  claims.IssuedAt.Unix(),
		ExpiresAt: claims.ExpiresAt.Unix(),
	}, nil
}
