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

	// Alias para compatibilidad de contexto
	UserIDKey     = CtxUserIDKey
	UserRoleKey   = CtxUserRoleKey
	UserClaimsKey = CtxUserClaimsKey
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
		// Establecer alias para compatibilidad
		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, claims.Role)

		c.Next()
	}
}

// GetUserClaims helper para recuperar los claims del contexto Gin
func GetUserClaims(c *gin.Context) (*domain.UserClaims, bool) {
	val, exists := c.Get(CtxUserClaimsKey)
	if !exists {
		val, exists = c.Get(UserClaimsKey)
		if !exists {
			return nil, false
		}
	}
	claims, ok := val.(*domain.UserClaims)
	return claims, ok
}

// GetUserID helper para obtener el ID de usuario del contexto
func GetUserID(c *gin.Context) (string, bool) {
	if val, exists := c.Get(CtxUserIDKey); exists {
		if id, ok := val.(string); ok && id != "" {
			return id, true
		}
	}
	if val, exists := c.Get(UserIDKey); exists {
		if id, ok := val.(string); ok && id != "" {
			return id, true
		}
	}
	if claims, ok := GetUserClaims(c); ok && claims != nil && claims.UserID != "" {
		return claims.UserID, true
	}
	return "", false
}

// GetUserRole helper para obtener el rol del usuario autenticado del contexto Gin
func GetUserRole(c *gin.Context) (domain.UserRole, bool) {
	if val, exists := c.Get(CtxUserRoleKey); exists {
		switch r := val.(type) {
		case domain.UserRole:
			if r != "" {
				return r, true
			}
		case string:
			if r != "" {
				return domain.UserRole(r), true
			}
		}
	}
	if val, exists := c.Get(UserRoleKey); exists {
		switch r := val.(type) {
		case domain.UserRole:
			if r != "" {
				return r, true
			}
		case string:
			if r != "" {
				return domain.UserRole(r), true
			}
		}
	}
	if claims, ok := GetUserClaims(c); ok && claims != nil && claims.Role != "" {
		return claims.Role, true
	}
	return "", false
}

// RequireRole crea un middleware que valida que el usuario posea uno de los roles autorizados
func RequireRole(allowedRoles ...domain.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetUserID(c)
		if !exists || strings.TrimSpace(userID) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "usuario no autenticado",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		userRole, roleExists := GetUserRole(c)
		if !roleExists || strings.TrimSpace(string(userRole)) == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "acceso denegado: rol de usuario no definido",
				"code":  "FORBIDDEN",
			})
			return
		}

		for _, allowed := range allowedRoles {
			if userRole == allowed {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "acceso denegado: permisos insuficientes para esta operación",
			"code":  "FORBIDDEN",
		})
	}
}
