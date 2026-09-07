package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type AuthHandler struct {
	authService ports.AuthService
	userRepo    ports.UserRepository
}

// NewAuthHandler construye el controlador de autenticación HTTP
func NewAuthHandler(authService ports.AuthService, userRepo ports.UserRepository) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		userRepo:    userRepo,
	}
}

type SocialLoginRequest struct {
	Provider domain.AuthProvider `json:"provider" binding:"required"`
	IDToken  string              `json:"id_token" binding:"required"`
}

type RefreshTokenRequest struct {
	Token string `json:"token"`
}

// SocialLogin maneja la autenticación social con Google y Apple (RF-01, RF-02, RF-03, RF-04)
// POST /v1/auth/social-login
func (h *AuthHandler) SocialLogin(c *gin.Context) {
	var req SocialLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "parámetros inválidos en la solicitud",
			"details": err.Error(),
		})
		return
	}

	result, err := h.authService.LoginWithSocial(c.Request.Context(), ports.SocialAuthInput{
		Provider:  req.Provider,
		IDToken:   req.IDToken,
		UserAgent: c.Request.UserAgent(),
		ClientIP:  c.ClientIP(),
	})
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, domain.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	statusCode := http.StatusOK
	if result.IsNewUser {
		statusCode = http.StatusCreated
	}

	c.JSON(statusCode, gin.H{
		"message":      "autenticación exitosa",
		"access_token": result.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   result.ExpiresIn, // 1200 segundos = 20 minutos
		"is_new_user":  result.IsNewUser,
		"user": gin.H{
			"id":               result.User.ID,
			"email":            result.User.Email,
			"first_name":       result.User.FirstName,
			"last_name":        result.User.LastName,
			"avatar_url":       result.User.AvatarURL,
			"role":             result.User.Role, // PASSENGER por defecto (RF-02)
			"kyc_status":       result.User.KYCStatus,
			"is_driver_active": result.User.IsDriverActive,
		},
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "debe proporcionar el token actual a renovar",
		})
		return
	}

	result, err := h.authService.RefreshSession(c.Request.Context(), currentToken, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, domain.ErrSessionExpired) {
			c.JSON(status, gin.H{
				"error": "la sesión ha expirado. Por favor inicie sesión nuevamente.",
				"code":  "SESSION_EXPIRED",
			})
			return
		}
		c.JSON(status, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "sesión extendida exitosamente",
		"access_token": result.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   result.ExpiresIn,
		"user":         result.User,
	})
}

// Me obtiene el perfil del usuario autenticado y su estado KYC (RF-13)
// GET /v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	userID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no autenticado"})
		return
	}

	user, err := h.userRepo.GetByID(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuario no encontrado"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
		"capabilities": gin.H{
			"can_book_rides":    user.CanBookRides(),    // Siempre true (RF-13)
			"can_publish_rides": user.CanPublishRides(), // Solo con rol DRIVER y KYC aprobado
		},
	})
}

// Logout destruye la sesión activa
// POST /v1/auth/logout
func (h *AuthHandler) Logout(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) == 2 {
		token := strings.TrimSpace(parts[1])
		claims, err := h.authService.ValidateToken(c.Request.Context(), token)
		if err == nil && claims != nil {
			_ = h.authService.InvalidateSession(c.Request.Context(), claims.UserID)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "sesión cerrada exitosamente",
	})
}
