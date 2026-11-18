package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// AuthService define el contrato mínimo de autenticación requerido por AuthHandler (SOLID · ISP).
type AuthService interface {
	LoginWithSocial(ctx context.Context, input ports.SocialAuthInput) (*ports.AuthResult, error)
	RefreshSession(ctx context.Context, currentToken string, userAgent, clientIP string) (*ports.AuthResult, error)
	InvalidateSessionByToken(ctx context.Context, token string) error
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (*ports.UserDTO, error)
}

// AuthHandler gestiona los endpoints HTTP de autenticación y perfil vigente,
// dependiendo exclusivamente de su interfaz mínima AuthService (SOLID · DIP/ISP).
type AuthHandler struct {
	authService AuthService
}

// NewAuthHandler construye el controlador de autenticación HTTP
func NewAuthHandler(authService AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authService,
	}
}

type SocialLoginRequest struct {
	Provider domain.AuthProvider `json:"provider" binding:"required"`
	IDToken  string              `json:"id_token" binding:"required"`
}

type RefreshTokenRequest struct {
	Token string `json:"token"`
}

type AuthUserResponse struct {
	ID             uuid.UUID           `json:"id"`
	Email          string              `json:"email"`
	FirstName      string              `json:"first_name"`
	LastName       string              `json:"last_name"`
	Phone          string              `json:"phone,omitempty"`
	AvatarURL      string              `json:"avatar_url,omitempty"`
	Provider       domain.AuthProvider `json:"provider,omitempty"`
	Role           domain.UserRole     `json:"role"`
	KYCStatus      domain.KYCStatus    `json:"kyc_status"`
	IsDriverActive bool                `json:"is_driver_active"`
	RatingAvg      float64             `json:"rating_avg"`
	RatingCount    int                 `json:"rating_count"`
	CreatedAt      string              `json:"created_at,omitempty"`
	UpdatedAt      string              `json:"updated_at,omitempty"`
}

type SocialLoginResponse struct {
	Message     string            `json:"message"`
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int64             `json:"expires_in"`
	IsNewUser   bool              `json:"is_new_user"`
	User        *AuthUserResponse `json:"user"`
}

type RefreshSessionResponse struct {
	Message     string            `json:"message"`
	AccessToken string            `json:"access_token"`
	TokenType   string            `json:"token_type"`
	ExpiresIn   int64             `json:"expires_in"`
	User        *AuthUserResponse `json:"user"`
}

type AuthCapabilitiesResponse struct {
	CanBookRides    bool `json:"can_book_rides"`
	CanPublishRides bool `json:"can_publish_rides"`
}

type AuthMeResponse struct {
	User         *AuthUserResponse        `json:"user"`
	Capabilities AuthCapabilitiesResponse `json:"capabilities"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}

func toAuthUserResponse(user *ports.UserDTO) *AuthUserResponse {
	if user == nil {
		return nil
	}
	return &AuthUserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FirstName:      user.FirstName,
		LastName:       user.LastName,
		Phone:          user.Phone,
		AvatarURL:      user.AvatarURL,
		Provider:       user.Provider,
		Role:           user.Role,
		KYCStatus:      user.KYCStatus,
		IsDriverActive: user.IsDriverActive,
		RatingAvg:      user.RatingAvg,
		RatingCount:    user.RatingCount,
		CreatedAt:      user.CreatedAt,
		UpdatedAt:      user.UpdatedAt,
	}
}

// SocialLogin maneja la autenticación social con Google y Apple (RF-01, RF-02, RF-03, RF-04)
// POST /v1/auth/social-login
func (h *AuthHandler) SocialLogin(c *gin.Context) {
	var req SocialLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "parámetros inválidos en la solicitud", "INVALID_REQUEST_PARAMETERS")
		return
	}

	if !req.Provider.IsValid() {
		RespondBadRequest(c, "proveedor de autenticación inválido", "INVALID_PROVIDER")
		return
	}

	result, err := h.authService.LoginWithSocial(c.Request.Context(), ports.SocialAuthInput{
		Provider:  req.Provider,
		IDToken:   req.IDToken,
		UserAgent: c.Request.UserAgent(),
		ClientIP:  c.ClientIP(),
	})
	if err != nil {
		respondError(c, err)
		return
	}

	statusCode := http.StatusOK
	if result.IsNewUser {
		statusCode = http.StatusCreated
	}

	c.JSON(statusCode, SocialLoginResponse{
		Message:     "autenticación exitosa",
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn, // 1200 segundos = 20 minutos
		IsNewUser:   result.IsNewUser,
		User:        toAuthUserResponse(result.User),
	})
}

// Refresh maneja la renovación o re-autenticación de sesión (RF-05)
// POST /v1/auth/refresh
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req RefreshTokenRequest
	_ = c.ShouldBindJSON(&req)

	currentToken := req.Token
	if currentToken == "" {
		// Intentar obtener del header Authorization
		authHeader := c.GetHeader("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			currentToken = strings.TrimSpace(parts[1])
		}
	}

	if currentToken == "" {
		RespondBadRequest(c, "debe proporcionar el token actual a renovar", "MISSING_TOKEN")
		return
	}

	result, err := h.authService.RefreshSession(c.Request.Context(), currentToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, RefreshSessionResponse{
		Message:     "sesión extendida exitosamente",
		AccessToken: result.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   result.ExpiresIn,
		User:        toAuthUserResponse(result.User),
	})
}

// Me obtiene el perfil del usuario autenticado y su estado KYC a través de AuthService (RF-13)
// GET /v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "no autenticado", "UNAUTHORIZED")
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, AuthMeResponse{
		User: toAuthUserResponse(user),
		Capabilities: AuthCapabilitiesResponse{
			CanBookRides:    user.CanBookRides,    // Siempre true (RF-13)
			CanPublishRides: user.CanPublishRides, // Solo con rol DRIVER y KYC aprobado
		},
	})
}

// Logout destruye la sesión activa de forma inmediata por token opaco
// POST /v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") && h.authService != nil {
		token := strings.TrimSpace(parts[1])
		_ = h.authService.InvalidateSessionByToken(c.Request.Context(), token)
	}

	c.JSON(http.StatusOK, LogoutResponse{
		Message: "sesión cerrada exitosamente",
	})
}
