package oauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// Constantes oficiales de endpoints JWKS y emisores (Issuers)
const (
	GoogleJWKSURL      = "https://www.googleapis.com/oauth2/v3/certs"
	GoogleIssuerSecure = "https://accounts.google.com"
	GoogleIssuerPlain  = "accounts.google.com"

	AppleJWKSURL = "https://appleid.apple.com/auth/keys"
	AppleIssuer  = "https://appleid.apple.com"
)

var (
	ErrInvalidSocialToken    = errors.New("token social inválido o expirado")
	ErrUnsupportedPayload    = errors.New("formato de token social no soportado")
	ErrMockTokensForbidden   = errors.New("tokens simulados (mock/sintéticos) estrictamente prohibidos en producción")
	ErrUnverifiedEmail       = errors.New("el correo electrónico no ha sido verificado por el proveedor social")
	ErrAudienceMismatch      = errors.New("el destinatario (aud) del token no coincide con el client_id configurado")
	ErrIssuerMismatch        = errors.New("el emisor (iss) del token no es reconocido ni confiable")
	ErrMissingKeyID          = errors.New("el header del token no incluye el identificador de clave pública (kid)")
	ErrUnsupportedAlgorithm  = errors.New("algoritmo criptográfico de token no soportado, se requiere RS256")
)

// flexibleBool permite parsear valores booleanos que los proveedores envían tanto como booleanos como strings ("true"/"false")
type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var boolVal bool
	if err := json.Unmarshal(data, &boolVal); err == nil {
		*b = flexibleBool(boolVal)
		return nil
	}
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		*b = flexibleBool(strings.EqualFold(strVal, "true") || strVal == "1")
		return nil
	}
	return nil
}

// GoogleClaims define los claims esperados en un Google ID Token conforme a OpenID Connect
type GoogleClaims struct {
	Email         string       `json:"email"`
	EmailVerified flexibleBool `json:"email_verified"`
	Name          string       `json:"name"`
	GivenName     string       `json:"given_name"`
	FamilyName    string       `json:"family_name"`
	Picture       string       `json:"picture"`
	jwt.RegisteredClaims
}

// AppleClaims define los claims esperados en un Apple ID Token
type AppleClaims struct {
	Email         string       `json:"email"`
	EmailVerified flexibleBool `json:"email_verified"`
	jwt.RegisteredClaims
}

// JWK representa una clave pública RSA individual según RFC 7517
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSResponse representa la colección de claves públicas retornada por los endpoints de Google y Apple
type JWKSResponse struct {
	Keys []JWK `json:"keys"`
}

// JWKSManager gestiona la obtención, parseo y almacenamiento en caché seguro y concurrente
// de las claves públicas (JWKS) emitidas por Google y Apple.
type JWKSManager struct {
	jwksURL    string
	httpClient *http.Client
	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	expiresAt  time.Time
	cacheTTL   time.Duration
}

// NewJWKSManager inicializa un nuevo gestor de claves públicas con caché de 1 hora
func NewJWKSManager(jwksURL string) *JWKSManager {
	return &JWKSManager{
		jwksURL: jwksURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		keys:     make(map[string]*rsa.PublicKey),
		cacheTTL: 1 * time.Hour,
	}
}

// GetKey obtiene la clave pública RSA asociada al 'kid' desde la caché en memoria o consultando el endpoint remoto
func (m *JWKSManager) GetKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	// 1. Acceso en lectura concurrente para evitar contención de bloqueo
	m.mu.RLock()
	if time.Now().Before(m.expiresAt) {
		if key, exists := m.keys[kid]; exists {
			m.mu.RUnlock()
			return key, nil
		}
	}
	m.mu.RUnlock()

	// 2. Clave no presente o caché expirada: bloqueo exclusivo de escritura
	m.mu.Lock()
	defer m.mu.Unlock()

	// Comprobación doble tras adquirir el bloqueo de escritura
	if time.Now().Before(m.expiresAt) {
		if key, exists := m.keys[kid]; exists {
			return key, nil
		}
	}

	// 3. Consultar endpoint remoto oficial
	if err := m.refreshKeys(ctx); err != nil {
		// Fallback de contingencia: si la red falla temporalmente, usar clave previa si existe
		if key, exists := m.keys[kid]; exists {
			return key, nil
		}
		return nil, fmt.Errorf("fallo al refrescar JWKS desde %s: %w", m.jwksURL, err)
	}

	key, exists := m.keys[kid]
	if !exists {
		return nil, fmt.Errorf("clave pública con kid '%s' no encontrada en el JWKS de %s", kid, m.jwksURL)
	}

	return key, nil
}

// refreshKeys realiza la petición HTTP al endpoint de JWKS y actualiza la caché
func (m *JWKSManager) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("error construyendo petición HTTP para JWKS: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Thumbi-OAuth-Service/1.0")

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error en petición HTTP a JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("respuesta HTTP inesperada (%d) desde endpoint JWKS %s", resp.StatusCode, m.jwksURL)
	}

	// Protección contra ataques de denegación de servicio (DoS) por memoria: limitar a 512KB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return fmt.Errorf("error leyendo respuesta JWKS: %w", err)
	}

	var jwks JWKSResponse
	if err := json.Unmarshal(body, &jwks); err != nil {
		return fmt.Errorf("error deserializando JSON de JWKS: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey)
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" {
			continue
		}
		pubKey, err := parseRSAPublicKey(jwk.N, jwk.E)
		if err != nil {
			continue
		}
		newKeys[jwk.Kid] = pubKey
	}

	if len(newKeys) == 0 {
		return errors.New("no se encontraron claves públicas RSA válidas en el JWKS recibido")
	}

	m.keys = newKeys
	m.expiresAt = time.Now().Add(m.cacheTTL)
	return nil
}

// parseRSAPublicKey reconstruye un *rsa.PublicKey a partir de su módulo (n) y exponente (e) en base64url
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nStr)
	if err != nil {
		return nil, fmt.Errorf("módulo (n) inválido en JWK: %w", err)
	}

	eBytes, err := decodeBase64URL(eStr)
	if err != nil {
		return nil, fmt.Errorf("exponente (e) inválido en JWK: %w", err)
	}

	var eInt uint64
	for _, b := range eBytes {
		eInt = (eInt << 8) | uint64(b)
	}
	if eInt == 0 || eInt > 1<<31-1 {
		return nil, fmt.Errorf("exponente RSA fuera de rango: %d", eInt)
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: int(eInt),
	}, nil
}

// decodeBase64URL decodifica cadenas base64URL con o sin padding de forma segura
func decodeBase64URL(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// isDevEnvironment verifica si el entorno actual autoriza el uso de tokens mock o sintéticos.
// CONTROL DE SEGURIDAD CRÍTICO:
// Los tokens simulados solo se permiten explícitamente en desarrollo local (ENV=local o ENV=development)
// o durante la ejecución de pruebas unitarias (`go test`).
// En entornos de producción, staging o cualquier entorno no declarado explícitamente, se rechazan.
func isDevEnvironment(configuredEnv string) bool {
	env := strings.ToLower(strings.TrimSpace(configuredEnv))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	}
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	}

	switch env {
	case "development", "local":
		return true
	default:
		// Detecta si la ejecución proviene del runner de pruebas de Go (go test)
		return isRunningTests()
	}
}

// isRunningTests determina si el binario se ejecuta bajo el framework de pruebas de Go
func isRunningTests() bool {
	return flag.Lookup("test.v") != nil || strings.HasSuffix(os.Args[0], ".test")
}

// GoogleOAuthProvider maneja la validación criptográfica estricta de tokens de Google Sign-In
type GoogleOAuthProvider struct {
	clientID    string
	environment string
	jwksManager *JWKSManager
}

// NewGoogleOAuthProvider crea un proveedor con la configuración por defecto y JWKS oficial de Google
func NewGoogleOAuthProvider(clientID string) ports.OAuthProvider {
	return NewGoogleOAuthProviderWithEnv(clientID, os.Getenv("ENV"))
}

// NewGoogleOAuthProviderWithEnv permite inyectar explícitamente el entorno de ejecución
func NewGoogleOAuthProviderWithEnv(clientID, environment string) ports.OAuthProvider {
	return &GoogleOAuthProvider{
		clientID:    clientID,
		environment: environment,
		jwksManager: NewJWKSManager(GoogleJWKSURL),
	}
}

func (p *GoogleOAuthProvider) GetProviderName() domain.AuthProvider {
	return domain.ProviderGoogle
}

// ValidateToken valida criptográficamente el Google ID Token contra los certificados JWKS de Google.
// Ejecuta las siguientes verificaciones de seguridad:
// 1. Verificación de entorno para tokens mock (bloqueados en producción).
// 2. Firma criptográfica RSA RS256 con claves públicas oficiales de Google.
// 3. Emisor (iss): Debe ser accounts.google.com o https://accounts.google.com.
// 4. Audiencia (aud): Debe coincidir con el client_id de la aplicación.
// 5. Expiración (exp): El token no debe haber vencido.
// 6. Correo verificado (email_verified): Debe ser estrictamente true para prevenir suplantación.
func (p *GoogleOAuthProvider) ValidateToken(ctx context.Context, idToken string) (*ports.SocialUserProfile, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return nil, ErrInvalidSocialToken
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 1: Aislamiento estricto de tokens de prueba (Mock)
	// -------------------------------------------------------------------------
	if strings.HasPrefix(idToken, "mock-google-") || strings.HasPrefix(idToken, "goog-") {
		if !isDevEnvironment(p.environment) {
			return nil, fmt.Errorf("%w: tokens de prueba 'mock-google-'/'goog-' prohibidos fuera de entorno de desarrollo", ErrMockTokensForbidden)
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(idToken, "mock-google-"), "goog-")
		return &ports.SocialUserProfile{
			ProviderID:    "google-sub-" + suffix,
			Email:         fmt.Sprintf("user.%s@gmail.com", suffix),
			FirstName:     "Usuario",
			LastName:      "Google " + suffix,
			AvatarURL:     "https://lh3.googleusercontent.com/a/default-avatar",
			Provider:      domain.ProviderGoogle,
			EmailVerified: true,
		}, nil
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 2: Tokens sintéticos solo permitidos en entorno dev/test
	// -------------------------------------------------------------------------
	if isDevEnvironment(p.environment) {
		if profile, err := parseSyntheticJWT(idToken, domain.ProviderGoogle); err == nil && profile != nil {
			return profile, nil
		}
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 3: Verificación Criptográfica de Firma con JWKS de Google
	// -------------------------------------------------------------------------
	claims := &GoogleClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		// Validar que el algoritmo de firma sea estrictamente RSA RS256 (previene ataques de sustitución 'none' o 'HS256')
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: algoritmo inesperado '%v', se espera RS256", ErrUnsupportedAlgorithm, t.Header["alg"])
		}

		// Extraer el Key ID (kid) del header del token
		kidVal, ok := t.Header["kid"]
		if !ok {
			return nil, ErrMissingKeyID
		}
		kid, ok := kidVal.(string)
		if !ok || kid == "" {
			return nil, ErrMissingKeyID
		}

		// Obtener la clave pública verificada desde el JWKS oficial de Google
		return p.jwksManager.GetKey(ctx, kid)
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: fallo en verificación de firma con Google: %v", ErrInvalidSocialToken, err)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 4: Verificación del Emisor (Issuer 'iss')
	// -------------------------------------------------------------------------
	if claims.Issuer != GoogleIssuerSecure && claims.Issuer != GoogleIssuerPlain {
		return nil, fmt.Errorf("%w: emisor inválido '%s', esperado '%s' o '%s'", ErrIssuerMismatch, claims.Issuer, GoogleIssuerSecure, GoogleIssuerPlain)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 5: Verificación de Audiencia (Audience 'aud')
	// -------------------------------------------------------------------------
	if strings.TrimSpace(p.clientID) == "" {
		if !isDevEnvironment(p.environment) {
			return nil, fmt.Errorf("%w: GOOGLE_CLIENT_ID no configurado en entorno no local", ErrAudienceMismatch)
		}
	} else {
		var audMatched bool
		for _, aud := range claims.Audience {
			if aud == p.clientID {
				audMatched = true
				break
			}
		}
		if !audMatched {
			return nil, fmt.Errorf("%w: aud '%v' no coincide con client_id configurado '%s'", ErrAudienceMismatch, claims.Audience, p.clientID)
		}
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 6: Verificación de Expiración (Expiry 'exp')
	// -------------------------------------------------------------------------
	if claims.ExpiresAt == nil || time.Now().UTC().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("%w: el Google ID token ha expirado", ErrInvalidSocialToken)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 7: Identificador de Usuario (Subject 'sub')
	// -------------------------------------------------------------------------
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("%w: subject ('sub') ausente en Google ID token", ErrInvalidSocialToken)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 8: Correo Electrónico y Validación de email_verified
	// -------------------------------------------------------------------------
	if strings.TrimSpace(claims.Email) == "" {
		return nil, fmt.Errorf("%w: email ausente en Google ID token", ErrInvalidSocialToken)
	}
	if !bool(claims.EmailVerified) {
		return nil, fmt.Errorf("%w: la cuenta de Google tiene el correo '%s' no verificado", ErrUnverifiedEmail, claims.Email)
	}

	firstName := claims.GivenName
	lastName := claims.FamilyName
	if firstName == "" && claims.Name != "" {
		nameParts := strings.SplitN(claims.Name, " ", 2)
		firstName = nameParts[0]
		if len(nameParts) > 1 {
			lastName = nameParts[1]
		}
	}

	return &ports.SocialUserProfile{
		ProviderID:    claims.Subject,
		Email:         claims.Email,
		FirstName:     firstName,
		LastName:      lastName,
		AvatarURL:     claims.Picture,
		Provider:      domain.ProviderGoogle,
		EmailVerified: bool(claims.EmailVerified),
	}, nil
}

// AppleOAuthProvider maneja la validación criptográfica estricta de tokens de Apple Sign-In
type AppleOAuthProvider struct {
	clientID    string
	environment string
	jwksManager *JWKSManager
}

// NewAppleOAuthProvider crea un proveedor con la configuración por defecto y JWKS oficial de Apple
func NewAppleOAuthProvider(clientID string) ports.OAuthProvider {
	return NewAppleOAuthProviderWithEnv(clientID, os.Getenv("ENV"))
}

// NewAppleOAuthProviderWithEnv permite inyectar explícitamente el entorno de ejecución
func NewAppleOAuthProviderWithEnv(clientID, environment string) ports.OAuthProvider {
	return &AppleOAuthProvider{
		clientID:    clientID,
		environment: environment,
		jwksManager: NewJWKSManager(AppleJWKSURL),
	}
}

func (p *AppleOAuthProvider) GetProviderName() domain.AuthProvider {
	return domain.ProviderApple
}

// ValidateToken valida criptográficamente el Apple ID Token contra los certificados JWKS de Apple.
// Ejecuta las siguientes comprobaciones de seguridad:
// 1. Verificación de entorno para tokens mock (bloqueados en producción).
// 2. Firma criptográfica RSA RS256 con claves públicas oficiales de Apple.
// 3. Emisor (iss): Debe ser estrictamente https://appleid.apple.com.
// 4. Audiencia (aud): Debe coincidir con el Bundle ID o Services ID configurado.
// 5. Expiración (exp): El token no debe haber vencido.
// 6. Correo verificado (email_verified): Si contiene email, debe ser verificado por Apple.
func (p *AppleOAuthProvider) ValidateToken(ctx context.Context, idToken string) (*ports.SocialUserProfile, error) {
	idToken = strings.TrimSpace(idToken)
	if idToken == "" {
		return nil, ErrInvalidSocialToken
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 1: Aislamiento estricto de tokens de prueba (Mock)
	// -------------------------------------------------------------------------
	if strings.HasPrefix(idToken, "mock-apple-") || strings.HasPrefix(idToken, "appl-") {
		if !isDevEnvironment(p.environment) {
			return nil, fmt.Errorf("%w: tokens de prueba 'mock-apple-'/'appl-' prohibidos fuera de entorno de desarrollo", ErrMockTokensForbidden)
		}
		suffix := strings.TrimPrefix(strings.TrimPrefix(idToken, "mock-apple-"), "appl-")
		return &ports.SocialUserProfile{
			ProviderID:    "apple-sub-" + suffix,
			Email:         fmt.Sprintf("user.%s@privaterelay.appleid.com", suffix),
			FirstName:     "Apple",
			LastName:      "User " + suffix,
			AvatarURL:     "",
			Provider:      domain.ProviderApple,
			EmailVerified: true,
		}, nil
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 2: Tokens sintéticos solo permitidos en entorno dev/test
	// -------------------------------------------------------------------------
	if isDevEnvironment(p.environment) {
		if profile, err := parseSyntheticJWT(idToken, domain.ProviderApple); err == nil && profile != nil {
			return profile, nil
		}
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 3: Verificación Criptográfica de Firma con JWKS de Apple
	// -------------------------------------------------------------------------
	claims := &AppleClaims{}
	token, err := jwt.ParseWithClaims(idToken, claims, func(t *jwt.Token) (interface{}, error) {
		// Validar que el algoritmo de firma sea estrictamente RSA RS256
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("%w: algoritmo inesperado '%v', se espera RS256", ErrUnsupportedAlgorithm, t.Header["alg"])
		}

		// Extraer el Key ID (kid) del header del token
		kidVal, ok := t.Header["kid"]
		if !ok {
			return nil, ErrMissingKeyID
		}
		kid, ok := kidVal.(string)
		if !ok || kid == "" {
			return nil, ErrMissingKeyID
		}

		// Obtener la clave pública verificada desde el JWKS oficial de Apple
		return p.jwksManager.GetKey(ctx, kid)
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("%w: fallo en verificación de firma con Apple: %v", ErrInvalidSocialToken, err)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 4: Verificación del Emisor (Issuer 'iss')
	// -------------------------------------------------------------------------
	if claims.Issuer != AppleIssuer {
		return nil, fmt.Errorf("%w: emisor inválido '%s', esperado '%s'", ErrIssuerMismatch, claims.Issuer, AppleIssuer)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 5: Verificación de Audiencia (Audience 'aud')
	// -------------------------------------------------------------------------
	if strings.TrimSpace(p.clientID) == "" {
		if !isDevEnvironment(p.environment) {
			return nil, fmt.Errorf("%w: APPLE_CLIENT_ID no configurado en entorno no local", ErrAudienceMismatch)
		}
	} else {
		var audMatched bool
		for _, aud := range claims.Audience {
			if aud == p.clientID {
				audMatched = true
				break
			}
		}
		if !audMatched {
			return nil, fmt.Errorf("%w: aud '%v' no coincide con el client_id de Apple '%s'", ErrAudienceMismatch, claims.Audience, p.clientID)
		}
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 6: Verificación de Expiración (Expiry 'exp')
	// -------------------------------------------------------------------------
	if claims.ExpiresAt == nil || time.Now().UTC().After(claims.ExpiresAt.Time) {
		return nil, fmt.Errorf("%w: el Apple ID token ha expirado", ErrInvalidSocialToken)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 7: Identificador de Usuario (Subject 'sub')
	// -------------------------------------------------------------------------
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("%w: subject ('sub') ausente en Apple ID token", ErrInvalidSocialToken)
	}

	// -------------------------------------------------------------------------
	// CONTROL DE SEGURIDAD 8: Correo Electrónico y Validación de email_verified
	// -------------------------------------------------------------------------
	if strings.TrimSpace(claims.Email) == "" {
		return nil, fmt.Errorf("%w: email ausente en Apple ID token", ErrInvalidSocialToken)
	}
	if !bool(claims.EmailVerified) {
		return nil, fmt.Errorf("%w: el correo '%s' asociado a Apple no está verificado", ErrUnverifiedEmail, claims.Email)
	}

	return &ports.SocialUserProfile{
		ProviderID:    claims.Subject,
		Email:         claims.Email,
		FirstName:     "Apple",
		LastName:      "User",
		AvatarURL:     "",
		Provider:      domain.ProviderApple,
		EmailVerified: bool(claims.EmailVerified),
	}, nil
}

// parseSyntheticJWT decodifica la parte media (payload) de un JWT para pruebas locales de desarrollo y tests unitarios.
// SEGURIDAD: Esta función NUNCA es alcanzable en producción, ya que tanto GoogleOAuthProvider como AppleOAuthProvider
// verifican previamente isDevEnvironment() y prohíben su ejecución fuera de desarrollo.
func parseSyntheticJWT(idToken string, provider domain.AuthProvider) (*ports.SocialUserProfile, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return nil, ErrUnsupportedPayload
	}

	payloadSegment := parts[1]
	if l := len(payloadSegment) % 4; l > 0 {
		payloadSegment += strings.Repeat("=", 4-l)
	}

	data, err := base64.URLEncoding.DecodeString(payloadSegment)
	if err != nil {
		data, err = base64.StdEncoding.DecodeString(payloadSegment)
		if err != nil {
			return nil, err
		}
	}

	var claims struct {
		Sub           string       `json:"sub"`
		Email         string       `json:"email"`
		EmailVerified flexibleBool `json:"email_verified"`
		Name          string       `json:"name"`
		GivenName     string       `json:"given_name"`
		FamilyName    string       `json:"family_name"`
		Picture       string       `json:"picture"`
	}

	if err := json.Unmarshal(data, &claims); err != nil {
		return nil, err
	}

	if claims.Sub == "" || claims.Email == "" {
		return nil, ErrInvalidSocialToken
	}

	// Por defecto en sintético se considera verificado a menos que venga explícitamente "email_verified": false
	emailVerified := true
	if strings.Contains(string(data), "email_verified") {
		emailVerified = bool(claims.EmailVerified)
	}

	firstName := claims.GivenName
	lastName := claims.FamilyName
	if firstName == "" && claims.Name != "" {
		nameParts := strings.SplitN(claims.Name, " ", 2)
		firstName = nameParts[0]
		if len(nameParts) > 1 {
			lastName = nameParts[1]
		}
	}

	return &ports.SocialUserProfile{
		ProviderID:    claims.Sub,
		Email:         claims.Email,
		FirstName:     firstName,
		LastName:      lastName,
		AvatarURL:     claims.Picture,
		Provider:      provider,
		EmailVerified: emailVerified,
	}, nil
}
