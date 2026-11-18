package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	// Encabezados estándar de webhook admitidos
	HeaderWebhookSignature = "X-Webhook-Signature"
	HeaderSignatureSHA256  = "X-Signature-SHA256"
	HeaderWebhookSecret    = "X-Webhook-Secret"
)

// WebhookAuthOptions opciones avanzadas de configuración para WebhookAuthMiddleware
type WebhookAuthOptions struct {
	Secret        string
	AllowedIPs    []string
	AllowLoopback bool
}

// WebhookAuthMiddleware protege los endpoints de callbacks y webhooks de revisión (ej. aprobación vehicular).
// Valida:
// 1. Firma HMAC-SHA256 del cuerpo de la solicitud en 'X-Webhook-Signature' o 'X-Signature-SHA256' (con o sin prefijo 'sha256=').
// 2. Token secreto compartido en 'X-Webhook-Secret' o 'Authorization: Bearer <secret>'.
// 3. Restricción opcional por lista blanca de IPs internas / redes confiables.
//
// Evita ataques de temporización utilizando comparaciones en tiempo constante (subtle.ConstantTimeCompare).
// Falla inmediatamente al arrancar si el secreto está vacío (sin secretos de respaldo hardcodeados).
func WebhookAuthMiddleware(secret string, allowedIPs ...string) gin.HandlerFunc {
	if strings.TrimSpace(secret) == "" {
		panic("error de seguridad crítico: WebhookAuthMiddleware requiere un WEBHOOK_SECRET no vacío")
	}

	return func(c *gin.Context) {
		// 1. Validación opcional por IP si se configuran redes o IPs permitidas
		if len(allowedIPs) > 0 {
			clientIP := c.ClientIP()
			ipAllowed := false
			parsedClientIP := net.ParseIP(clientIP)

			for _, allowed := range allowedIPs {
				// Chequeo si es CIDR
				if strings.Contains(allowed, "/") {
					_, ipNet, err := net.ParseCIDR(allowed)
					if err == nil && parsedClientIP != nil && ipNet.Contains(parsedClientIP) {
						ipAllowed = true
						break
					}
				} else if clientIP == allowed {
					ipAllowed = true
					break
				}
			}

			if !ipAllowed {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "acceso denegado: dirección IP de origen no autorizada para el webhook",
					"code":  "FORBIDDEN_IP",
				})
				return
			}
		}

		// 2. Comprobar firma HMAC-SHA256 si está presente
		sigHeader := c.GetHeader(HeaderWebhookSignature)
		if sigHeader == "" {
			sigHeader = c.GetHeader(HeaderSignatureSHA256)
		}

		if sigHeader != "" {
			// Leer el cuerpo crudo de la petición para calcular el HMAC
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
					"error": "error al leer el cuerpo de la solicitud para validar firma",
					"code":  "INVALID_BODY",
				})
				return
			}

			// Restaurar el Request Body para que los siguientes handlers puedan bindear JSON
			c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Quitar prefijo opcional 'sha256=' o 'sha256:'
			cleanSig := strings.TrimSpace(sigHeader)
			lowerSig := strings.ToLower(cleanSig)
			if strings.HasPrefix(lowerSig, "sha256=") {
				cleanSig = cleanSig[7:]
			} else if strings.HasPrefix(lowerSig, "sha256:") {
				cleanSig = cleanSig[7:]
			}

			// Calcular HMAC-SHA256 esperado sobre el payload crudo
			expectedSigHex := ComputeWebhookSignature(bodyBytes, secret)

			// Comparación en tiempo constante para mitigar ataques de temporización
			if subtle.ConstantTimeCompare([]byte(strings.ToLower(cleanSig)), []byte(strings.ToLower(expectedSigHex))) == 1 {
				c.Set("webhook_authenticated", true)
				c.Set("webhook_auth_type", "hmac_sha256")
				c.Next()
				return
			}

			// Firma HMAC no coincide
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "firma HMAC del webhook inválida",
				"code":  "INVALID_WEBHOOK_SIGNATURE",
			})
			return
		}

		// 3. Comprobar secreto compartido en encabezado 'X-Webhook-Secret'
		secretHeader := c.GetHeader(HeaderWebhookSecret)
		if secretHeader != "" {
			if subtle.ConstantTimeCompare([]byte(secretHeader), []byte(secret)) == 1 {
				c.Set("webhook_authenticated", true)
				c.Set("webhook_auth_type", "shared_secret_header")
				c.Next()
				return
			}

			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "secreto de webhook proporcionado en encabezado es inválido",
				"code":  "INVALID_WEBHOOK_SECRET",
			})
			return
		}

		// 4. Comprobar secreto en 'Authorization: Bearer <secret>' (para proveedores que usan autenticación Bearer)
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
				token := strings.TrimSpace(parts[1])
				if subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1 {
					c.Set("webhook_authenticated", true)
					c.Set("webhook_auth_type", "bearer_secret")
					c.Next()
					return
				}
			}
		}

		// 5. Si no se presentó ni firma HMAC ni secreto compartido válido, rechazar la llamada anónima
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error": "autenticación requerida para el webhook: proporcione 'X-Webhook-Signature', 'X-Webhook-Secret' o 'Authorization: Bearer <secret>'",
			"code":  "UNAUTHORIZED_WEBHOOK",
		})
	}
}

// ComputeWebhookSignature genera la firma hexadecimal HMAC-SHA256 de un payload dado y secreto
func ComputeWebhookSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
