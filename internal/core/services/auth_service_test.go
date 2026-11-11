package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
)

func TestAuthService_LoginWithSocial_NewUser(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	googleProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider},
		tokenProvider,
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
	tokenProvider.On("GenerateToken", mock.Anything).Return("signed-jwt-token", nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return s.Token == "signed-jwt-token"
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
	assert.Equal(t, "signed-jwt-token", res.AccessToken)

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
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{appleProvider},
		tokenProvider,
		identityRepo,
	)

	// Usuario existente registrado previamente con Google
	existingUser := domain.NewUser(
		"user-uuid-1",
		"target.user@example.com",
		"Target",
		"User",
		"",
		domain.ProviderGoogle,
		"google-orig-id",
	)

	// Perfil recibido de Apple con el mismo correo y email_verified = true
	socialProfile := &ports.SocialUserProfile{
		ProviderID:    "apple-sub-999",
		Email:         "target.user@example.com",
		FirstName:     "Target",
		LastName:      "User",
		Provider:      domain.ProviderApple,
		EmailVerified: true,
	}

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
	tokenProvider.On("GenerateToken", existingUser).Return("apple-session-token", nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return s.UserID == existingUser.ID && s.Token == "apple-session-token"
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
	assert.Equal(t, "apple-session-token", res.AccessToken)

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
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{evilProvider},
		tokenProvider,
		identityRepo,
	)

	// Usuario víctima existente en el sistema
	existingVictim := domain.NewUser(
		"victim-uuid",
		"victim@company.com",
		"Victim",
		"Doe",
		"",
		domain.ProviderGoogle,
		"legit-sub-123",
	)

	// Atacante intenta autenticarse alegando el correo de la víctima, pero email_verified = FALSE
	attackerProfile := &ports.SocialUserProfile{
		ProviderID:    "attacker-sub-666",
		Email:         "victim@company.com",
		FirstName:     "Attacker",
		LastName:      "Malicious",
		Provider:      domain.ProviderGoogle,
		EmailVerified: false, // ¡NO VERIFICADO!
	}

	evilProvider.On("ValidateToken", ctx, "attacker-token").Return(attackerProfile, nil)
	identityRepo.On("GetByProviderAndID", ctx, domain.ProviderGoogle, "attacker-sub-666").Return(nil, domain.ErrIdentityNotFound)
	userRepo.On("GetByProvider", ctx, domain.ProviderGoogle, "attacker-sub-666").Return(nil, domain.ErrUserNotFound)
	userRepo.On("GetByEmail", ctx, "victim@company.com").Return(existingVictim, nil)

	// Ejecución: debe ser tajantemente bloqueado para prevenir Account Takeover
	res, err := service.LoginWithSocial(ctx, ports.SocialAuthInput{
		Provider:  domain.ProviderGoogle,
		IDToken:   "attacker-token",
		UserAgent: "Mozilla/5.0",
		ClientIP:  "203.0.113.5",
	})

	assert.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrUnverifiedEmail, "Debe rechazar la vinculación por correo no verificado")
	assert.Nil(t, res)

	// Verificar que jamás se guardó sesión ni se vinculó identidad
	identityRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
	sessionRepo.AssertNotCalled(t, "Save", mock.Anything, mock.Anything, mock.Anything)
}

func TestAuthService_LoginWithSocial_ExistingIdentity(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	identityRepo := new(MockIdentityRepository)
	googleProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider},
		tokenProvider,
		identityRepo,
	)

	existingUser := domain.NewUser(
		"user-100",
		"existing@example.com",
		"Existing",
		"User",
		"",
		domain.ProviderGoogle,
		"google-id-100",
	)

	existingIdentity := domain.NewUserIdentity(
		"ident-1",
		"user-100",
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
	userRepo.On("GetByID", ctx, "user-100").Return(existingUser, nil)
	tokenProvider.On("GenerateToken", existingUser).Return("token-jwt-100", nil)
	sessionRepo.On("Save", ctx, mock.MatchedBy(func(s *domain.Session) bool {
		return s.UserID == "user-100" && s.Token == "token-jwt-100"
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
	assert.Equal(t, "user-100", res.User.ID)
	// No debe consultar por email porque ya existe identidad registrada directamente
	userRepo.AssertNotCalled(t, "GetByEmail", mock.Anything, mock.Anything)
}

func TestAuthService_RefreshSession_Expired(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	sessionRepo := new(MockSessionRepository)
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		nil,
		tokenProvider,
	)

	// Simular token expirado
	tokenProvider.On("ValidateToken", "expired-token").Return(nil, domain.ErrSessionExpired)

	res, err := service.RefreshSession(ctx, "expired-token", "Agent", "127.0.0.1")

	assert.ErrorIs(t, err, domain.ErrSessionExpired, "RF-06: Sesión vencida debe fallar y requerir re-autenticación")
	assert.Nil(t, res)
}
