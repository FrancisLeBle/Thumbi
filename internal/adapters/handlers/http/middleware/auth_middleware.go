package middleware

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

const (
	CtxUserClaimsKey = "user_claims"
	CtxUserIDKey     = "user_id"
	CtxUserRoleKey   = "user_role"
)

// AuthMiddleware intercepta las solicitudes HTTP, extrae y valida el Bearer JWT (TTL 20 min)
func AuthMiddleware(authService ports.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "encabezado de autorización ausente",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "formato de token inválido. Se espera 'Bearer <token>'",
				"code":  "INVALID_TOKEN_FORMAT",
			})
			return
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "token vacío",
				"code":  "EMPTY_TOKEN",
			})
			return
		}

		// Validar token y expiración de 20 minutos (RF-04, RF-06)
		claims, err := authService.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			status := http.StatusUnauthorized
			errorCode := "UNAUTHORIZED"
			errMsg := "token no autorizado o inválido"

			if errors.Is(err, domain.ErrSessionExpired) {
				errorCode = "SESSION_EXPIRED"
				errMsg = "la sesión ha expirado tras 20 minutos de inactividad o límite de tiempo"
			}

			c.AbortWithStatusJSON(status, gin.H{
				"error": errMsg,
				"code":  errorCode,
			})
			return
		}

		// Almacenar información de identidad en el contexto de la solicitud
		c.Set(CtxUserClaimsKey, claims)
		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxUserRoleKey, claims.Role)

		c.Next()
	}
}

// GetUserClaims helper para recuperar los claims del contexto Gin
func GetUserClaims(c *gin.Context) (*domain.UserClaims, bool) {
	val, exists := c.Get(CtxUserClaimsKey)
	if !exists {
		return nil, false
	}
	claims, ok := val.(*domain.UserClaims)
	return claims, ok
}

// GetUserID helper para obtener el ID de usuario del contexto
func GetUserID(c *gin.Context) (string, bool) {
	val, exists := c.Get(CtxUserIDKey)
	if !exists {
		return "", false
	}
	id, ok := val.(string)
	return id, ok
}
