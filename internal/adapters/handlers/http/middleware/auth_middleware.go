package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
)

const (
	CtxUserClaimsKey    = "user_claims"
	CtxUserIDKey        = "user_id"
	CtxUserRoleKey      = "user_role"
	CtxUserKYCStatusKey = "kyc_status"

	// Alias para compatibilidad de contexto
	UserIDKey        = CtxUserIDKey
	UserRoleKey      = CtxUserRoleKey
	UserKYCStatusKey = CtxUserKYCStatusKey
	UserClaimsKey    = CtxUserClaimsKey
)

// TokenValidator define el contrato mínimo requerido por el middleware de autenticación (SOLID · ISP).
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (*domain.UserClaims, error)
}

// AuthMiddleware intercepta las solicitudes HTTP, extrae y valida el token de sesión opaco Bearer (TTL 20 min)
func AuthMiddleware(validator TokenValidator) gin.HandlerFunc {
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
		claims, err := validator.ValidateToken(c.Request.Context(), tokenString)
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

		// Almacenar información de identidad y estado vigente (resuelto desde sesión/DB) en el contexto
		c.Set(CtxUserClaimsKey, claims)
		c.Set(CtxUserIDKey, claims.UserID)
		c.Set(CtxUserRoleKey, claims.Role)
		c.Set(CtxUserKYCStatusKey, claims.KYCStatus)
		// Establecer alias para compatibilidad
		c.Set(UserIDKey, claims.UserID)
		c.Set(UserRoleKey, claims.Role)
		c.Set(UserKYCStatusKey, claims.KYCStatus)

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

func extractUUIDFromContextValue(val interface{}) (uuid.UUID, bool) {
	switch v := val.(type) {
	case uuid.UUID:
		if v != uuid.Nil {
			return v, true
		}
	case string:
		if id, err := domain.ParseUUID(v); err == nil && id != uuid.Nil {
			return id, true
		}
	}
	return uuid.Nil, false
}

// GetUserID helper para obtener el ID de usuario del contexto
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	if val, exists := c.Get(CtxUserIDKey); exists {
		if id, ok := extractUUIDFromContextValue(val); ok {
			return id, true
		}
	}
	if val, exists := c.Get(UserIDKey); exists {
		if id, ok := extractUUIDFromContextValue(val); ok {
			return id, true
		}
	}
	if claims, ok := GetUserClaims(c); ok && claims != nil && claims.UserID != uuid.Nil {
		return claims.UserID, true
	}
	return uuid.Nil, false
}

// GetUserRole helper para obtener el rol del usuario autenticado del contexto Gin
func GetUserRole(c *gin.Context) (domain.UserRole, bool) {
	if val, exists := c.Get(CtxUserRoleKey); exists {
		switch r := val.(type) {
		case domain.UserRole:
			if r.IsValid() {
				return r, true
			}
		case string:
			role := domain.UserRole(strings.TrimSpace(r))
			if role.IsValid() {
				return role, true
			}
		}
	}
	if val, exists := c.Get(UserRoleKey); exists {
		switch r := val.(type) {
		case domain.UserRole:
			if r.IsValid() {
				return r, true
			}
		case string:
			role := domain.UserRole(strings.TrimSpace(r))
			if role.IsValid() {
				return role, true
			}
		}
	}
	if claims, ok := GetUserClaims(c); ok && claims != nil && claims.Role.IsValid() {
		return claims.Role, true
	}
	return "", false
}

// GetUserKYCStatus helper para obtener el estado KYC vigente del usuario autenticado del contexto Gin
func GetUserKYCStatus(c *gin.Context) (domain.KYCStatus, bool) {
	if val, exists := c.Get(CtxUserKYCStatusKey); exists {
		switch s := val.(type) {
		case domain.KYCStatus:
			if s.IsValid() {
				return s, true
			}
		case string:
			status := domain.KYCStatus(strings.TrimSpace(s))
			if status.IsValid() {
				return status, true
			}
		}
	}
	if val, exists := c.Get(UserKYCStatusKey); exists {
		switch s := val.(type) {
		case domain.KYCStatus:
			if s.IsValid() {
				return s, true
			}
		case string:
			status := domain.KYCStatus(strings.TrimSpace(s))
			if status.IsValid() {
				return status, true
			}
		}
	}
	if claims, ok := GetUserClaims(c); ok && claims != nil && claims.KYCStatus.IsValid() {
		return claims.KYCStatus, true
	}
	return "", false
}

// RequireRole crea un middleware que valida que el usuario posea uno de los roles autorizados
func RequireRole(allowedRoles ...domain.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, exists := GetUserID(c)
		if !exists || userID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "usuario no autenticado",
				"code":  "UNAUTHORIZED",
			})
			return
		}

		userRole, roleExists := GetUserRole(c)
		if !roleExists || !userRole.IsValid() {
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
