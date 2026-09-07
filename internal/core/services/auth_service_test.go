package services_test

import (
	"context"
	"testing"
	"time"

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
	googleProvider := &MockOAuthProvider{ProviderName: domain.ProviderGoogle}
	tokenProvider := new(MockTokenProvider)

	service := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider},
		tokenProvider,
	)

	socialProfile := &ports.SocialUserProfile{
		ProviderID: "goog-123",
		Email:      "maria.perez@example.com",
		FirstName:  "Maria",
		LastName:   "Perez",
		Provider:   domain.ProviderGoogle,
	}

	googleProvider.On("ValidateToken", ctx, "valid-id-token").Return(socialProfile, nil)
	userRepo.On("GetByProvider", ctx, domain.ProviderGoogle, "goog-123").Return(nil, domain.ErrUserNotFound)
	userRepo.On("GetByEmail", ctx, "maria.perez@example.com").Return(nil, domain.ErrUserNotFound)
	userRepo.On("Create", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Email == "maria.perez@example.com" && u.Role == domain.RolePassenger
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
	googleProvider.AssertExpectations(t)
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
