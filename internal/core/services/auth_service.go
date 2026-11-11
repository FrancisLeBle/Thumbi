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
	userRepo       ports.UserRepository
	sessionRepo    ports.SessionRepository
	oauthProviders map[domain.AuthProvider]ports.OAuthProvider
	tokenProvider  TokenProvider
	identityRepo   ports.IdentityRepository
}

// NewAuthService construye una nueva instancia inyectando las dependencias necesarias.
// Acepta opcionalmente un IdentityRepository para almacenar vínculos en la tabla dedicada de identidades.
func NewAuthService(
	userRepo ports.UserRepository,
	sessionRepo ports.SessionRepository,
	providers []ports.OAuthProvider,
	tokenProvider TokenProvider,
	identityRepos ...ports.IdentityRepository,
) ports.AuthService {
	providerMap := make(map[domain.AuthProvider]ports.OAuthProvider)
	for _, p := range providers {
		providerMap[p.GetProviderName()] = p
	}

	var identityRepo ports.IdentityRepository
	if len(identityRepos) > 0 && identityRepos[0] != nil {
		identityRepo = identityRepos[0]
	} else if ir, ok := userRepo.(ports.IdentityRepository); ok {
		identityRepo = ir
	}

	return &authService{
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
		oauthProviders: providerMap,
		tokenProvider:  tokenProvider,
		identityRepo:   identityRepo,
	}
}

// LoginWithSocial maneja la autenticación social (RF-01, RF-02, RF-03, RF-04)
// con vinculación segura de cuentas (Account Linking) basada en identidades explícitas
// y validación estricta de email_verified=true para prevenir secuestro de cuentas (Account Takeover).
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

	if socialProfile == nil || socialProfile.ProviderID == "" {
		return nil, fmt.Errorf("%w: perfil social o identificador de proveedor ausente", domain.ErrInvalidCredentials)
	}

	// 2. Buscar si la identidad federada ya fue registrada previamente en la tabla de identidades
	var user *domain.User
	var identity *domain.UserIdentity
	isNewUser := false

	if s.identityRepo != nil {
		identity, err = s.identityRepo.GetByProviderAndID(ctx, socialProfile.Provider, socialProfile.ProviderID)
		if err != nil && !errors.Is(err, domain.ErrIdentityNotFound) && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, fmt.Errorf("error al consultar identidad federada: %w", err)
		}
	}

	if identity != nil {
		// La identidad ya está vinculada a un usuario específico
		user, err = s.userRepo.GetByID(ctx, identity.UserID)
		if err != nil {
			return nil, fmt.Errorf("error al obtener usuario asociado a la identidad: %w", err)
		}
	} else {
		// Verificación de compatibilidad con usuarios registrados previamente por proveedor
		user, err = s.userRepo.GetByProvider(ctx, socialProfile.Provider, socialProfile.ProviderID)
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, fmt.Errorf("error al consultar usuario por proveedor: %w", err)
		}

		if user != nil && s.identityRepo != nil {
			// Registrar la identidad en la tabla separada para trazabilidad y migraciones transparentes
			linkedIdent := domain.NewUserIdentity(
				uuid.New().String(),
				user.ID,
				socialProfile.Provider,
				socialProfile.ProviderID,
				socialProfile.Email,
				socialProfile.EmailVerified,
			)
			_ = s.identityRepo.Create(ctx, linkedIdent)
		}
	}

	// 3. Si no existe identidad para este (provider, provider_id):
	// Evaluación segura de vinculación a cuenta existente por correo o creación de nuevo usuario.
	if user == nil {
		if socialProfile.Email == "" {
			return nil, fmt.Errorf("%w: el proveedor social no proporcionó una dirección de correo", domain.ErrInvalidInput)
		}

		// Consultar si ya existe una cuenta con este correo electrónico
		existingUser, err := s.userRepo.GetByEmail(ctx, socialProfile.Email)
		if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
			return nil, fmt.Errorf("error al verificar existencia previa por correo: %w", err)
		}

		if existingUser != nil {
			// -------------------------------------------------------------------------
			// CONTROL DE SEGURIDAD CRÍTICO: Prevenir Account Takeover
			// -------------------------------------------------------------------------
			// Si una cuenta con este correo ya existe, PROHIBIR terminantemente la vinculación
			// a menos que el proveedor externo certifique explícitamente que el correo está verificado
			// (email_verified == true). Esto evita que un atacante cree una cuenta con correo ajeno
			// en un proveedor que no valida correos para secuestrar la cuenta de la víctima.
			if !socialProfile.EmailVerified {
				return nil, fmt.Errorf("%w: el correo '%s' no ha sido verificado por el proveedor %s, vinculación bloqueada por seguridad", domain.ErrUnverifiedEmail, socialProfile.Email, socialProfile.Provider)
			}

			// Correo certificado: vincular de forma segura la nueva identidad a la cuenta existente
			user = existingUser
			if s.identityRepo != nil {
				newIdentity := domain.NewUserIdentity(
					uuid.New().String(),
					existingUser.ID,
					socialProfile.Provider,
					socialProfile.ProviderID,
					socialProfile.Email,
					socialProfile.EmailVerified,
				)
				if err := s.identityRepo.Create(ctx, newIdentity); err != nil {
					return nil, fmt.Errorf("error al registrar vinculación de identidad federada: %w", err)
				}
			}
		} else {
			// 4. Usuario completamente nuevo: registrar usuario y su identidad federada
			// Se exige igualmente que el correo esté verificado para registrar cuentas federadas legítimas
			if !socialProfile.EmailVerified {
				return nil, fmt.Errorf("%w: no se permite registrar un nuevo usuario con correo no verificado", domain.ErrUnverifiedEmail)
			}

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

			// Guardar la identidad externa en la tabla dedicada
			if s.identityRepo != nil {
				newIdentity := domain.NewUserIdentity(
					uuid.New().String(),
					userID,
					socialProfile.Provider,
					socialProfile.ProviderID,
					socialProfile.Email,
					socialProfile.EmailVerified,
				)
				if err := s.identityRepo.Create(ctx, newIdentity); err != nil {
					return nil, fmt.Errorf("error al registrar identidad federada del nuevo usuario: %w", err)
				}
			}
		}
	} else {
		// Actualizar avatar si cambió
		if socialProfile.AvatarURL != "" && user.AvatarURL != socialProfile.AvatarURL {
			user.AvatarURL = socialProfile.AvatarURL
			user.UpdatedAt = time.Now().UTC()
			_ = s.userRepo.Update(ctx, user)
		}
	}

	// 5. RF-04: Generar JWT con TTL de exactamente 20 minutos
	accessToken, err := s.tokenProvider.GenerateToken(user)
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	// 6. Guardar sesión activa con expiración de 20 minutos
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
