package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/backend/internal/core/domain"
)

const (
	// RequestIDHeader es la cabecera HTTP estándar para propagar o devolver el ID de rastreo
	RequestIDHeader = "X-Request-ID"
	// CtxRequestIDKey es la clave utilizada para almacenar el Request ID en el contexto de Gin
	CtxRequestIDKey = "request_id"
)

// RequestIDMiddleware extrae el ID de solicitud de los encabezados HTTP o genera uno nuevo basado en UUID v7.
// Además de inyectarlo en el contexto de Gin, lo refleja en la cabecera X-Request-ID de la respuesta.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
		if reqID == "" {
			reqID = domain.NewUUIDv7().String()
		}

		c.Set(CtxRequestIDKey, reqID)
		c.Writer.Header().Set(RequestIDHeader, reqID)
		c.Next()
	}
}

// GetRequestID recupera el ID de solicitud del contexto Gin, generando un fallback seguro si no existe.
func GetRequestID(c *gin.Context) string {
	if c == nil {
		return domain.NewUUIDv7().String()
	}
	if val, exists := c.Get(CtxRequestIDKey); exists {
		if id, ok := val.(string); ok && id != "" {
			return id
		}
	}
	// Fallback si no fue procesado por el middleware
	reqID := strings.TrimSpace(c.GetHeader(RequestIDHeader))
	if reqID == "" {
		reqID = domain.NewUUIDv7().String()
		c.Set(CtxRequestIDKey, reqID)
		c.Writer.Header().Set(RequestIDHeader, reqID)
	}
	return reqID
}
