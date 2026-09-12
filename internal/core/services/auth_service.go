package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// TokenProvider interfaz para el generador de tokens
type TokenProvider interface {
	GenerateToken(user *domain.User) (string, error)
	ValidateToken(tokenString string) (*domain.UserClaims, error)
}

// authService implementa ports.AuthService
type authService struct {
	userRepo      ports.UserRepository
	sessionRepo   ports.SessionRepository
	oauthProviders map[domain.AuthProvider]ports.OAuthProvider
	tokenProvider TokenProvider
}

// NewAuthService construye una nueva instancia inyectando las dependencias necesarias
func NewAuthService(
	userRepo ports.UserRepository,
	sessionRepo ports.SessionRepository,
	providers []ports.OAuthProvider,
	tokenProvider TokenProvider,
) ports.AuthService {
	providerMap := make(map[domain.AuthProvider]ports.OAuthProvider)
	for _, p := range providers {
		providerMap[p.GetProviderName()] = p
	}

	return &authService{
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
		oauthProviders: providerMap,
		tokenProvider:  tokenProvider,
	}
}

// LoginWithSocial maneja la autenticación social (RF-01, RF-02, RF-03, RF-04)
func (s *authService) LoginWithSocial(ctx context.Context, input ports.SocialAuthInput) (*ports.AuthResult, error) {
	provider, ok := s.oauthProviders[input.Provider]
	if !ok {
		return nil, fmt.Errorf("proveedor de autenticación no soportado: %s", input.Provider)
	}

	// 1. Validar token social con el proveedor externo
	socialProfile, err := provider.ValidateToken(ctx, input.IDToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalidCredentials, err)
	}

	// 2. Buscar si el usuario ya existe en base de datos
	isNewUser := false
	user, err := s.userRepo.GetByProvider(ctx, socialProfile.Provider, socialProfile.ProviderID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			// Intentar buscar por correo electrónico
			user, err = s.userRepo.GetByEmail(ctx, socialProfile.Email)
			if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// 3. RF-02 / RF-03: Si el usuario es nuevo, crearlo síncronamente con rol PASSENGER por defecto
	if user == nil {
		isNewUser = true
		userID := uuid.New().String()
		user = domain.NewUser(
			userID,
			socialProfile.Email,
			socialProfile.FirstName,
			socialProfile.LastName,
			socialProfile.AvatarURL,
			socialProfile.Provider,
			socialProfile.ProviderID,
		)

		if err := s.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("error al registrar nuevo usuario: %w", err)
		}
	} else {
		// Actualizar datos si cambiaron
		if socialProfile.AvatarURL != "" && user.AvatarURL != socialProfile.AvatarURL {
			user.AvatarURL = socialProfile.AvatarURL
			user.UpdatedAt = time.Now().UTC()
			_ = s.userRepo.Update(ctx, user)
		}
	}

	// 4. RF-04: Generar JWT con TTL de exactamente 20 minutos
	accessToken, err := s.tokenProvider.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	// 5. Guardar sesión activa con expiración de 20 minutos
	sessionID := uuid.New().String()
	session := domain.NewSession(sessionID, user.ID, accessToken, input.UserAgent, input.ClientIP)
	if err := s.sessionRepo.Save(ctx, session, domain.SessionTTL); err != nil {
		return nil, fmt.Errorf("error al registrar sesión activa: %w", err)
	}

	return &ports.AuthResult{
		User:        user,
		Session:     session,
		AccessToken: accessToken,
		ExpiresIn:   int64(domain.SessionTTL.Seconds()), // 1200 segundos (20 min)
		IsNewUser:   isNewUser,
	}, nil
}

// RefreshSession extiende o emite una nueva sesión si la actual es válida (RF-05)
func (s *authService) RefreshSession(ctx context.Context, currentToken string, userAgent, clientIP string) (*ports.AuthResult, error) {
	// 1. Validar token actual
	claims, err := s.tokenProvider.ValidateToken(currentToken)
	if err != nil {
		return nil, err
	}

	// 2. Verificar que la sesión aún exista en el repositorio (no invalidada previamente)
	session, err := s.sessionRepo.GetByToken(ctx, currentToken)
	if err != nil || session == nil || session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	// 3. Obtener el usuario actualizado
	user, err := s.userRepo.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	// 4. Invalidar la sesión anterior
	_ = s.sessionRepo.Delete(ctx, session.ID)

	// 5. Emitir nuevo token con 20 minutos de TTL
	newToken, err := s.tokenProvider.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("error al emitir nuevo token: %w", err)
	}

	newSessionID := uuid.New().String()
	newSession := domain.NewSession(newSessionID, user.ID, newToken, userAgent, clientIP)
	if err := s.sessionRepo.Save(ctx, newSession, domain.SessionTTL); err != nil {
		return nil, fmt.Errorf("error al almacenar nueva sesión: %w", err)
	}

	return &ports.AuthResult{
		User:        user,
		Session:     newSession,
		AccessToken: newToken,
		ExpiresIn:   int64(domain.SessionTTL.Seconds()),
		IsNewUser:   false,
	}, nil
}

// InvalidateSession destruye la sesión activa para cerrar sesión o por expulsión (RF-06)
func (s *authService) InvalidateSession(ctx context.Context, sessionID string) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

// ValidateToken decodifica y verifica los claims del token
func (s *authService) ValidateToken(ctx context.Context, token string) (*domain.UserClaims, error) {
	claims, err := s.tokenProvider.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// Validar que la sesión no haya sido destruida
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil || session == nil || session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}

	return claims, nil
}
