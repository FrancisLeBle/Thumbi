package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

const opaqueTokenBytes = 32 // 256 bits de entropía criptográfica para tokens opacos de sesión

func generateOpaqueToken() (string, error) {
	buf := make([]byte, opaqueTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("error generando entropía para token de sesión: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// AuthService implementa los casos de uso de autenticación usando tokens opacos aleatorios respaldados por el almacén de sesiones.
type AuthService struct {
	userRepo       ports.UserRepository
	sessionRepo    ports.SessionRepository
	oauthProviders map[domain.AuthProvider]ports.OAuthProvider
	identityRepo   ports.IdentityRepository
}

// NewAuthService construye una nueva instancia inyectando las dependencias necesarias.
// Acepta opcionalmente un IdentityRepository para almacenar vínculos en la tabla dedicada de identidades.
func NewAuthService(
	userRepo ports.UserRepository,
	sessionRepo ports.SessionRepository,
	providers []ports.OAuthProvider,
	identityRepos ...ports.IdentityRepository,
) *AuthService {
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

	return &AuthService{
		userRepo:       userRepo,
		sessionRepo:    sessionRepo,
		oauthProviders: providerMap,
		identityRepo:   identityRepo,
	}
}

// LoginWithSocial maneja la autenticación social (RF-01, RF-02, RF-03, RF-04)
// con vinculación segura de cuentas (Account Linking) basada en identidades explícitas
// y validación estricta de email_verified=true para prevenir secuestro de cuentas (Account Takeover).
func (s *AuthService) LoginWithSocial(ctx context.Context, input ports.SocialAuthInput) (*ports.AuthResult, error) {
	if !input.Provider.IsValid() {
		return nil, fmt.Errorf("%w: proveedor de autenticación no soportado: %s", domain.ErrInvalidStatus, input.Provider)
	}
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
				domain.NewUUIDv7(),
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
					domain.NewUUIDv7(),
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
			userID := domain.NewUUIDv7()
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
					domain.NewUUIDv7(),
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

	// 5. RF-04: Generar token opaco aleatorio (256 bits) asociado a una sesión con TTL de exactamente 20 minutos
	accessToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("error al generar token de acceso: %w", err)
	}

	// 6. Guardar sesión activa con expiración de 20 minutos (el repositorio almacena su hash SHA-256)
	sessionID := domain.NewUUIDv7()
	session := domain.NewSession(sessionID, user.ID, accessToken, input.UserAgent, input.ClientIP)
	if err := s.sessionRepo.Save(ctx, session, domain.SessionTTL); err != nil {
		return nil, fmt.Errorf("error al registrar sesión activa: %w", err)
	}

	return &ports.AuthResult{
		User:        toUserDTO(user),
		SessionID:   session.ID,
		AccessToken: accessToken,
		ExpiresIn:   int64(domain.SessionTTL.Seconds()), // 1200 segundos (20 min)
		IsNewUser:   isNewUser,
	}, nil
}

// RefreshSession rota el token opaco y extiende la sesión si la actual sigue activa (RF-05)
func (s *AuthService) RefreshSession(ctx context.Context, currentToken string, userAgent, clientIP string) (*ports.AuthResult, error) {
	trimmedToken := strings.TrimSpace(currentToken)
	if trimmedToken == "" {
		return nil, domain.ErrInvalidToken
	}

	// 1. Verificar que la sesión exista en el repositorio y no haya expirado ni sido cerrada
	session, err := s.sessionRepo.GetByToken(ctx, trimmedToken)
	if err != nil {
		if errors.Is(err, domain.ErrSessionExpired) {
			return nil, domain.ErrSessionExpired
		}
		return nil, fmt.Errorf("error al consultar sesión activa: %w", err)
	}
	if session == nil || session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}
	if session.UserID == uuid.Nil {
		return nil, domain.ErrInvalidToken
	}

	// 2. Obtener el usuario actualizado desde base de datos
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error al consultar usuario de la sesión: %w", err)
	}

	// 3. Invalidar inmediatamente la sesión anterior (rotación de token)
	if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
		return nil, fmt.Errorf("error al invalidar sesión anterior: %w", err)
	}

	// 4. Emitir nuevo token opaco aleatorio con 20 minutos de TTL
	newToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("error al emitir nuevo token: %w", err)
	}

	newSessionID := domain.NewUUIDv7()
	newSession := domain.NewSession(newSessionID, user.ID, newToken, userAgent, clientIP)
	if err := s.sessionRepo.Save(ctx, newSession, domain.SessionTTL); err != nil {
		return nil, fmt.Errorf("error al almacenar nueva sesión: %w", err)
	}

	return &ports.AuthResult{
		User:        toUserDTO(user),
		SessionID:   newSession.ID,
		AccessToken: newToken,
		ExpiresIn:   int64(domain.SessionTTL.Seconds()),
		IsNewUser:   false,
	}, nil
}

// InvalidateSession destruye la sesión activa para cerrar sesión o por expulsión (RF-06)
func (s *AuthService) InvalidateSession(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

// InvalidateSessionByToken destruye de inmediato la sesión asociada a un token opaco durante el logout (RF-06).
func (s *AuthService) InvalidateSessionByToken(ctx context.Context, token string) error {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil
	}

	session, err := s.sessionRepo.GetByToken(ctx, trimmed)
	if err != nil {
		if errors.Is(err, domain.ErrSessionExpired) {
			return nil
		}
		return fmt.Errorf("error al localizar sesión por token: %w", err)
	}
	if session == nil {
		return nil
	}

	return s.sessionRepo.Delete(ctx, session.ID)
}

// ValidateToken valida el token opaco contra el almacén de sesiones y resuelve Role/KYCStatus frescos desde base de datos.
func (s *AuthService) ValidateToken(ctx context.Context, token string) (*domain.UserClaims, error) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return nil, domain.ErrInvalidToken
	}

	// 1. Consulta única de la sesión activa en el almacén de sesiones (indexada por hash SHA-256)
	session, err := s.sessionRepo.GetByToken(ctx, trimmed)
	if err != nil {
		if errors.Is(err, domain.ErrSessionExpired) {
			return nil, domain.ErrSessionExpired
		}
		return nil, fmt.Errorf("error al consultar sesión activa: %w", err)
	}
	if session == nil || session.IsExpired() {
		return nil, domain.ErrSessionExpired
	}
	if session.UserID == uuid.Nil {
		return nil, domain.ErrInvalidToken
	}

	// 2. Resolver Role y KYCStatus vigentes desde la base de datos en cada solicitud
	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrUnauthorized
		}
		return nil, fmt.Errorf("error al consultar estado vigente del usuario: %w", err)
	}
	if !user.Role.IsValid() || !user.KYCStatus.IsValid() {
		return nil, domain.ErrInvalidStatus
	}

	return &domain.UserClaims{
		UserID:    user.ID,
		SessionID: session.ID,
		Email:     user.Email,
		Role:      user.Role,
		KYCStatus: user.KYCStatus,
		IssuedAt:  session.CreatedAt.Unix(),
		ExpiresAt: session.ExpiresAt.Unix(),
	}, nil
}

// GetCurrentUser recupera el perfil vigente del usuario autenticado como DTO a través de la capa de servicio
func (s *AuthService) GetCurrentUser(ctx context.Context, userID uuid.UUID) (*ports.UserDTO, error) {
	if userID == uuid.Nil {
		return nil, domain.ErrUnauthorized
	}

	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	return toUserDTO(user), nil
}

func formatTimeRFC3339(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func formatOptionalTimeRFC3339(t *time.Time) *string {
	if t == nil || t.IsZero() {
		return nil
	}
	formatted := t.UTC().Format(time.RFC3339)
	return &formatted
}

func toUserDTO(u *domain.User) *ports.UserDTO {
	if u == nil {
		return nil
	}
	return &ports.UserDTO{
		ID:              u.ID,
		Email:           u.Email,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		Phone:           u.Phone,
		AvatarURL:       u.AvatarURL,
		Provider:        u.Provider,
		Role:            u.Role,
		KYCStatus:       u.KYCStatus,
		IsDriverActive:  u.IsDriverActive,
		RatingAvg:       u.RatingAvg,
		RatingCount:     u.RatingCount,
		CanBookRides:    u.CanBookRides(),
		CanPublishRides: u.CanPublishRides(),
		CreatedAt:       formatTimeRFC3339(u.CreatedAt),
		UpdatedAt:       formatTimeRFC3339(u.UpdatedAt),
	}
}
