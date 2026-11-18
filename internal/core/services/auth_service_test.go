package services_test

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func assertValidOpaqueToken(t *testing.T, token string) {
	t.Helper()
	assert.NotEmpty(t, token)
	assert.False(t, strings.Contains(token, "."), "El token opaco no debe tener estructura JWT con segmentos separados por puntos")
	raw, err := base64.RawURLEncoding.DecodeString(token)
	require.NoError(t, err, "El token opaco debe estar codificado en base64url sin padding")
	assert.Len(t, raw, 32, "El token opaco debe aportar exactamente 256 bits (32 bytes) de entropía")
}

func TestAuthService_LoginWithSocial_NewUser(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	googleProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider},
		identityRepo,
	)

	socialProfile := &ports.SocialUserProfile{
		ProviderID:    "goog-123",
		Email:         "maria.perez@example.com",
		FirstName:     "Maria",
		LastName:      "Perez",
		Provider:      domain.ProviderGoogle,
		EmailVerified: true,
	}

	var savedToken string
	googleProvider.On("ValidateToken", ctx, "valid-id-token").Return(socialProfile, nil)
	identityRepo.On("GetByProviderAndID", ctx, domain.ProviderGoogle, "goog-123").Return(nil, domain.ErrIdentityNotFound)
	userRepo.On("GetByProvider", ctx, domain.ProviderGoogle, "goog-123").Return(nil, domain.ErrUserNotFound)
	userRepo.On("GetByEmail", ctx, "maria.perez@example.com").Return(nil, domain.ErrUserNotFound)
	userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == "maria.perez@example.com" && u.Role == domain.RolePassenger
	})).Return(nil)
	identityRepo.On("Create", ctx, mock.MatchedBy(func(i *domain.UserIdentity) bool {
		return i.Provider == domain.ProviderGoogle && i.ProviderUserID == "goog-123" && i.EmailVerified == true
	})).Return(nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		savedToken = s.Token
		return s.Token != ""
	}), domain.SessionTTL).Return(nil)

	// Ejecutar login
	res, err := service.LoginWithSocial(ctx, ports.SocialAuthInput{
		Provider:  domain.ProviderGoogle,
		IDToken:   "valid-id-token",
		UserAgent: "iOS/ThumbiApp",
		ClientIP:  "190.18.20.1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.IsNewUser, "Debe marcarse como nuevo usuario")
	assert.Equal(t, domain.RolePassenger, res.User.Role, "RF-02: Rol base debe ser Pasajero")
	assert.Equal(t, int64(1200), res.ExpiresIn, "RF-04: Expiración debe ser exactamente 20 minutos (1200 seg)")
	assert.Equal(t, savedToken, res.AccessToken)
	assertValidOpaqueToken(t, res.AccessToken)

	userRepo.AssertExpectations(t)
	sessionRepo.AssertExpectations(t)
	identityRepo.AssertExpectations(t)
	googleProvider.AssertExpectations(t)
}

func TestAuthService_LoginWithSocial_AccountLinking_VerifiedEmail(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	appleProvider := &MockOAuthProvider{ProviderName: domain.ProviderApple}

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{appleProvider},
		identityRepo,
	)

	existingUserID := uuid.MustParse("01960000-0000-7000-8000-000000000001")
	existingUser := domain.NewUser(
		existingUserID,
		"target.user@example.com",
		"Target",
		"User",
		"",
		domain.ProviderGoogle,
		"google-orig-id",
	)

	socialProfile := &ports.SocialUserProfile{
		ProviderID:    "apple-sub-999",
		Email:         "target.user@example.com",
		FirstName:     "Target",
		LastName:      "User",
		Provider:      domain.ProviderApple,
		EmailVerified: true,
	}

	var savedToken string
	appleProvider.On("ValidateToken", ctx, "valid-apple-token").Return(socialProfile, nil)
	identityRepo.On("GetByProviderAndID", ctx, domain.ProviderApple, "apple-sub-999").Return(nil, domain.ErrIdentityNotFound)
	userRepo.On("GetByProvider", ctx, domain.ProviderApple, "apple-sub-999").Return(nil, domain.ErrUserNotFound)
	userRepo.On("GetByEmail", ctx, "target.user@example.com").Return(existingUser, nil)
	identityRepo.On("Create", ctx, mock.MatchedBy(func(i *domain.UserIdentity) bool {
		return i.UserID == existingUser.ID &&
			i.Provider == domain.ProviderApple &&
			i.ProviderUserID == "apple-sub-999" &&
			i.EmailVerified == true
	})).Return(nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		savedToken = s.Token
		return s.UserID == existingUser.ID && s.Token != ""
	}), domain.SessionTTL).Return(nil)

	res, err := service.LoginWithSocial(ctx, ports.SocialAuthInput{
		Provider:  domain.ProviderApple,
		IDToken:   "valid-apple-token",
		UserAgent: "iOS/ThumbiApp",
		ClientIP:  "190.18.20.1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.IsNewUser, "No debe marcarse como nuevo usuario, sino cuenta vinculada")
	assert.Equal(t, existingUser.ID, res.User.ID)
	assert.Equal(t, savedToken, res.AccessToken)
	assertValidOpaqueToken(t, res.AccessToken)

	userRepo.AssertExpectations(t)
	identityRepo.AssertExpectations(t)
	sessionRepo.AssertExpectations(t)
}

func TestAuthService_LoginWithSocial_AccountLinking_UnverifiedEmail_Blocked(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	evilProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{evilProvider},
		identityRepo,
	)

	victimID := uuid.MustParse("01960000-0000-7000-8000-000000000002")
	existingVictim := domain.NewUser(
		victimID,
		"victim@company.com",
		"Victim",
		"Doe",
		"",
		domain.ProviderGoogle,
		"legit-sub-123",
	)

	attackerProfile := &ports.SocialUserProfile{
		ProviderID:    "attacker-sub-666",
		Email:         "victim@company.com",
		FirstName:     "Attacker",
		LastName:      "Malicious",
		Provider:      domain.ProviderGoogle,
		EmailVerified: false,
	}

	evilProvider.On("ValidateToken", ctx, "attacker-token").Return(attackerProfile, nil)
	identityRepo.On("GetByProviderAndID", ctx, domain.ProviderGoogle, "attacker-sub-666").Return(nil, domain.ErrIdentityNotFound)
	userRepo.On("GetByProvider", ctx, domain.ProviderGoogle, "attacker-sub-666").Return(nil, domain.ErrUserNotFound)
	userRepo.On("GetByEmail", ctx, "victim@company.com").Return(existingVictim, nil)

	res, err := service.LoginWithSocial(ctx, ports.SocialAuthInput{
		Provider:  domain.ProviderGoogle,
		IDToken:   "attacker-token",
		UserAgent: "Mozilla/5.0",
		ClientIP:  "203.0.113.5",
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnverifiedEmail, "Debe rechazar la vinculación por correo no verificado")
	assert.Nil(t, res)

	identityRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	sessionRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthService_LoginWithSocial_ExistingIdentity(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	googleProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider},
		identityRepo,
	)

	existingUserID := uuid.MustParse("01960000-0000-7000-8000-000000000100")
	existingIdentityID := uuid.MustParse("01960000-0000-7000-8000-000000000101")

	existingUser := domain.NewUser(
		existingUserID,
		"existing@example.com",
		"Existing",
		"User",
		"",
		domain.ProviderGoogle,
		"google-id-100",
	)

	existingIdentity := domain.NewUserIdentity(
		existingIdentityID,
		existingUserID,
		domain.ProviderGoogle,
		"google-id-100",
		"existing@example.com",
		true,
	)

	socialProfile := &ports.SocialUserProfile{
		ProviderID:    "google-id-100",
		Email:         "existing@example.com",
		FirstName:     "Existing",
		LastName:      "User",
		Provider:      domain.ProviderGoogle,
		EmailVerified: true,
	}

	googleProvider.On("ValidateToken", ctx, "token-100").Return(socialProfile, nil)
	identityRepo.On("GetByProviderAndID", ctx, domain.ProviderGoogle, "google-id-100").Return(existingIdentity, nil)
	userRepo.On("GetByID", ctx, existingUserID).Return(existingUser, nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return s.UserID == existingUserID && s.Token != ""
	}), domain.SessionTTL).Return(nil)

	res, err := service.LoginWithSocial(ctx, ports.SocialAuthInput{
		Provider:  domain.ProviderGoogle,
		IDToken:   "token-100",
		UserAgent: "Chrome/120.0",
		ClientIP:  "190.18.20.1",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.False(t, res.IsNewUser)
	assert.Equal(t, existingUserID, res.User.ID)
	assertValidOpaqueToken(t, res.AccessToken)
	userRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
}

func TestAuthService_RefreshSession(t *testing.T) {
	ctx := context.Background()

	t.Run("rechaza sesión expirada o inexistente", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		sessionRepo.On("GetByToken", ctx, "expired-token").Return(nil, domain.ErrSessionExpired)

		res, err := service.RefreshSession(ctx, "expired-token", "Agent", "127.0.0.1")
		assert.ErrorIs(t, err, domain.ErrSessionExpired, "RF-06: Sesión vencida debe fallar y requerir re-autenticación")
		assert.Nil(t, res)
	})

	t.Run("rota el token opaco e invalida la sesión anterior", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		userID := uuid.MustParse("01960000-0000-7000-8000-000000000201")
		oldSessionID := uuid.MustParse("01960000-0000-7000-8000-000000000202")
		oldSession := domain.NewSession(oldSessionID, userID, "opaque-old-token", "Agent", "127.0.0.1")
		user := domain.NewUser(userID, "refresh@example.com", "Ana", "Gomez", "", domain.ProviderGoogle, "goog-201")

		sessionRepo.On("GetByToken", ctx, "opaque-old-token").Return(oldSession, nil)
		userRepo.On("GetByID", ctx, userID).Return(user, nil)
		sessionRepo.On("Delete", ctx, oldSessionID).Return(nil)
		sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
			return s.UserID == userID && s.ID != oldSessionID && s.Token != "opaque-old-token"
		}), domain.SessionTTL).Return(nil)

		res, err := service.RefreshSession(ctx, "opaque-old-token", "NewAgent", "10.0.0.2")
		require.NoError(t, err)
		require.NotNil(t, res)
		assert.NotEqual(t, "opaque-old-token", res.AccessToken)
		assertValidOpaqueToken(t, res.AccessToken)
		assert.Equal(t, int64(1200), res.ExpiresIn)

		sessionRepo.AssertExpectations(t)
		userRepo.AssertExpectations(t)
	})
}

func TestAuthService_InvalidateSessionByToken(t *testing.T) {
	ctx := context.Background()

	t.Run("elimina la sesión activa inmediatamente por token opaco", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		userID := uuid.MustParse("01960000-0000-7000-8000-000000000251")
		sessionID := uuid.MustParse("01960000-0000-7000-8000-000000000252")
		activeSession := domain.NewSession(sessionID, userID, "opaque-logout-token", "Agent", "127.0.0.1")

		sessionRepo.On("GetByToken", ctx, "opaque-logout-token").Return(activeSession, nil)
		sessionRepo.On("Delete", ctx, sessionID).Return(nil)

		err := service.InvalidateSessionByToken(ctx, "opaque-logout-token")
		assert.NoError(t, err)
		sessionRepo.AssertExpectations(t)
		userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("es idempotente si el token ya expiró o fue invalidado", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		sessionRepo.On("GetByToken", ctx, "unknown-token").Return(nil, domain.ErrSessionExpired)

		err := service.InvalidateSessionByToken(ctx, "unknown-token")
		assert.NoError(t, err)
		sessionRepo.AssertNotCalled(t, "Delete", mock.Anything, mock.Anything)
	})
}

func TestAuthService_ValidateToken_ResolvesLiveRoleAndKYCFromDB(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)

	service := services.NewAuthService(userRepo, sessionRepo, nil)

	userID := uuid.MustParse("01960000-0000-7000-8000-000000000301")
	sessionID := uuid.MustParse("01960000-0000-7000-8000-000000000302")
	activeToken := "opaque-session-token-active"
	activeSession := domain.NewSession(sessionID, userID, activeToken, "Agent", "127.0.0.1")

	// Primera lectura: usuario aún es PASSENGER con KYC NOT_STARTED
	initialUser := domain.NewUser(userID, "live@example.com", "Live", "User", "", domain.ProviderGoogle, "goog-live")
	// Segunda lectura con EL MISMO token opaco: usuario promovido a DRIVER y KYC APPROVED en DB
	promotedUser := domain.NewUser(userID, "live@example.com", "Live", "User", "", domain.ProviderGoogle, "goog-live")
	promotedUser.Role = domain.UserRoleDriver
	promotedUser.KYCStatus = domain.KYCStatusApproved

	sessionRepo.On("GetByToken", ctx, activeToken).Return(activeSession, nil).Twice()
	userRepo.On("GetByID", ctx, userID).Return(initialUser, nil).Once()
	userRepo.On("GetByID", ctx, userID).Return(promotedUser, nil).Once()

	claims1, err := service.ValidateToken(ctx, activeToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims1.UserID)
	assert.Equal(t, sessionID, claims1.SessionID)
	assert.Equal(t, domain.UserRolePassenger, claims1.Role)
	assert.Equal(t, domain.KYCStatusNotStarted, claims1.KYCStatus)

	// Mismo token tras cambio de rol y KYC en base de datos -> refleja cambio inmediato
	claims2, err := service.ValidateToken(ctx, activeToken)
	assert.NoError(t, err)
	assert.Equal(t, userID, claims2.UserID)
	assert.Equal(t, sessionID, claims2.SessionID)
	assert.Equal(t, domain.UserRoleDriver, claims2.Role)
	assert.Equal(t, domain.KYCStatusApproved, claims2.KYCStatus)

	sessionRepo.AssertExpectations(t)
	userRepo.AssertExpectations(t)
}

func TestAuthService_ValidateToken_RejectsInvalidAndMissingUser(t *testing.T) {
	ctx := context.Background()

	t.Run("rechaza token vacío con ErrInvalidToken", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		claims, err := service.ValidateToken(ctx, "   ")
		assert.ErrorIs(t, err, domain.ErrInvalidToken)
		assert.Nil(t, claims)
		sessionRepo.AssertNotCalled(t, "GetByToken", mock.Anything, mock.Anything)
	})

	t.Run("rechaza cuando el usuario fue eliminado de base de datos", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		userID := uuid.MustParse("01960000-0000-7000-8000-000000000501")
		sessionID := uuid.MustParse("01960000-0000-7000-8000-000000000502")

		sessionRepo.On("GetByToken", ctx, "orphan-token").Return(
			domain.NewSession(sessionID, userID, "orphan-token", "Agent", "127.0.0.1"),
			nil,
		)
		userRepo.On("GetByID", ctx, userID).Return(nil, domain.ErrUserNotFound)

		claims, err := service.ValidateToken(ctx, "orphan-token")
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, claims)
	})
}

func TestAuthService_GetCurrentUser(t *testing.T) {
	ctx := context.Background()

	t.Run("retorna usuario vigente desde el repositorio", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		userID := uuid.MustParse("01960000-0000-7000-8000-000000000601")
		expected := domain.NewUser(userID, "me@example.com", "Juan", "Perez", "", domain.ProviderGoogle, "goog-me")
		userRepo.On("GetByID", ctx, userID).Return(expected, nil)

		user, err := service.GetCurrentUser(ctx, userID)
		assert.NoError(t, err)
		require.NotNil(t, user)
		assert.Equal(t, expected.ID, user.ID)
		assert.Equal(t, expected.Email, user.Email)
		assert.Equal(t, expected.FirstName, user.FirstName)
		assert.Equal(t, expected.LastName, user.LastName)
		assert.Equal(t, expected.Provider, user.Provider)
		assert.Equal(t, expected.Role, user.Role)
		assert.Equal(t, expected.KYCStatus, user.KYCStatus)
		assert.Equal(t, expected.IsDriverActive, user.IsDriverActive)
		assert.True(t, user.CanBookRides)
		assert.False(t, user.CanPublishRides)
		userRepo.AssertExpectations(t)
	})

	t.Run("rechaza uuid.Nil con ErrUnauthorized", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		user, err := service.GetCurrentUser(ctx, uuid.Nil)
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, user)
		userRepo.AssertNotCalled(t, "GetByID", mock.Anything, mock.Anything)
	})

	t.Run("propaga ErrUserNotFound si el usuario no existe", func(t *testing.T) {
		userRepo := new(MockUserRepository)
		sessionRepo := new(MockSessionRepository)
		service := services.NewAuthService(userRepo, sessionRepo, nil)

		missingID := uuid.MustParse("01960000-0000-7000-8000-000000000699")
		userRepo.On("GetByID", ctx, missingID).Return(nil, domain.ErrUserNotFound)

		user, err := service.GetCurrentUser(ctx, missingID)
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
		assert.Nil(t, user)
		userRepo.AssertExpectations(t)
	})
}
