package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// RateLimitByIP aplica limitación de tasa basada en la dirección IP del cliente (para endpoints públicos).
func RateLimitByIP(limiter ports.RateLimiter, limit int, window time.Duration, routeKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:ip:%s:%s", routeKey, clientIP)

		allowed, remaining, resetDur, err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			// Fail-open si hay un fallo imprevisto para no degradar el servicio a clientes legítimos
			c.Next()
			return
		}

		resetSec := int(resetDur.Seconds())
		if resetSec <= 0 {
			resetSec = 1
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.Itoa(resetSec))

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(resetSec))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":               "demasiadas solicitudes desde esta dirección IP; por favor intenta nuevamente en unos momentos",
				"code":                "RATE_LIMIT_EXCEEDED",
				"retry_after_seconds": resetSec,
			})
			return
		}

		c.Next()
	}
}

// RateLimitByUser aplica limitación de tasa basada en el ID del usuario autenticado (con fallback a IP).
func RateLimitByUser(limiter ports.RateLimiter, limit int, window time.Duration, routeKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limiter == nil {
			c.Next()
			return
		}

		var key string
		userID, exists := GetUserID(c)
		if exists && userID != "" {
			key = fmt.Sprintf("ratelimit:user:%s:%s", routeKey, userID)
		} else {
			key = fmt.Sprintf("ratelimit:ip:%s:%s", routeKey, c.ClientIP())
		}

		allowed, remaining, resetDur, err := limiter.Allow(c.Request.Context(), key, limit, window)
		if err != nil {
			c.Next()
			return
		}

		resetSec := int(resetDur.Seconds())
		if resetSec <= 0 {
			resetSec = 1
		}

		c.Header("X-RateLimit-Limit", strconv.Itoa(limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.Itoa(resetSec))

		if !allowed {
			c.Header("Retry-After", strconv.Itoa(resetSec))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":               "has excedido el límite de solicitudes permitidas para esta operación; intenta nuevamente en unos momentos",
				"code":                "RATE_LIMIT_EXCEEDED",
				"retry_after_seconds": resetSec,
			})
			return
		}

		c.Next()
	}
}
