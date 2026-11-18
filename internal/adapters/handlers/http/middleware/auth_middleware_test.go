package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/adapters/repositories/memory"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/services"
)

type statefulUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (r *statefulUserRepo) Create(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *statefulUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	copyUser := *u
	return &copyUser, nil
}
func (r *statefulUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			copyUser := *u
			return &copyUser, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (r *statefulUserRepo) GetByProvider(ctx context.Context, p domain.AuthProvider, pID string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Provider == p && u.ProviderID == pID {
			copyUser := *u
			return &copyUser, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (r *statefulUserRepo) Update(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}

func TestAuthMiddleware_ReflectsLiveRoleAndKYCChangesImmediately(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := domain.NewUUIDv7()
	sessionID := domain.NewUUIDv7()
	userRepo := &statefulUserRepo{
		users: map[uuid.UUID]*domain.User{
			userID: domain.NewUser(userID, "user@example.com", "Ana", "Gomez", "", domain.ProviderGoogle, "goog-1"),
		},
	}
	sessionRepo := memory.NewSessionRepository(0)
	authService := services.NewAuthService(userRepo, sessionRepo, nil)

	// Emitir token opaco aleatorio y registrar sesión activa
	token := "opaque-middleware-test-token-32bytes"
	err := sessionRepo.Save(context.Background(), domain.NewSession(sessionID, userID, token, "TestAgent", "127.0.0.1"), 20*time.Minute)
	require.NoError(t, err)

	r := gin.New()
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/profile", func(c *gin.Context) {
			uid, hasID := middleware.GetUserID(c)
			role, hasRole := middleware.GetUserRole(c)
			kycStatus, hasKYC := middleware.GetUserKYCStatus(c)
			claims, hasClaims := middleware.GetUserClaims(c)
			assert.True(t, hasID)
			assert.True(t, hasRole)
			assert.True(t, hasKYC)
			assert.True(t, hasClaims)
			c.JSON(http.StatusOK, gin.H{
				"user_id":    uid.String(),
				"role":       role,
				"kyc_status": kycStatus,
				"session_id": claims.SessionID.String(),
			})
		})
		protected.GET("/admin", middleware.RequireRole(domain.RoleAdmin), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"ok": true})
		})
	}

	// 1. Estado inicial: PASSENGER / NOT_STARTED -> /admin retorna 403 Forbidden
	req1 := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusForbidden, w1.Code)

	// 2. Actualizar rol a ADMIN y KYC a APPROVED en DB manteniendo EL MISMO token JWT
	userRepo.users[userID].Role = domain.RoleAdmin
	userRepo.users[userID].KYCStatus = domain.KYCStatusApproved

	req2 := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code, "El cambio de rol en DB debe surtir efecto inmediato con el mismo token")

	reqProfile := httptest.NewRequest(http.MethodGet, "/profile", nil)
	reqProfile.Header.Set("Authorization", "Bearer "+token)
	wProfile := httptest.NewRecorder()
	r.ServeHTTP(wProfile, reqProfile)
	assert.Equal(t, http.StatusOK, wProfile.Code)
	assert.Contains(t, wProfile.Body.String(), `"kyc_status":"APPROVED"`)
	assert.Contains(t, wProfile.Body.String(), `"role":"ADMIN"`)

	// 3. Revocar rol en DB a PASSENGER -> /admin vuelve a 403 Forbidden de inmediato
	userRepo.users[userID].Role = domain.RolePassenger
	req3 := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusForbidden, w3.Code, "La degradación de rol en DB debe bloquear acceso inmediatamente")

	// 4. Invalidar sesión (logout) -> cualquier solicitud posterior con el mismo token opaco falla de inmediato con 401
	err = authService.InvalidateSessionByToken(context.Background(), token)
	require.NoError(t, err)
	req4 := httptest.NewRequest(http.MethodGet, "/profile", nil)
	req4.Header.Set("Authorization", "Bearer "+token)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusUnauthorized, w4.Code, "El logout debe invalidar el token opaco de inmediato")

	// Asegurar que *services.AuthService satisface middleware.TokenValidator
	var _ middleware.TokenValidator = authService
}
