package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thumbi/backend/internal/adapters/external/biometrics"
	"github.com/thumbi/backend/internal/adapters/external/notifier"
	"github.com/thumbi/backend/internal/adapters/external/oauth"
	adapterHttp "github.com/thumbi/backend/internal/adapters/handlers/http"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/adapters/repositories/memory"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

// In-memory mock DB for testing the HTTP layer end-to-end
type testEnv struct {
	userRepo       *inMemoryUserRepo
	kycRepo        *inMemoryKYCRepo
	vehicleRepo    *inMemoryVehicleRepo
	sessionRepo    *memory.SessionRepository
	authService    adapterHttp.AuthService
	kycService     adapterHttp.KYCService
	vehicleService adapterHttp.VehicleService
	biometrics     *biometrics.MockBiometricsProvider
	notifier       *notifier.PushNotifier
	router         http.Handler
}

type inMemoryUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func newInMemoryUserRepo() *inMemoryUserRepo {
	return &inMemoryUserRepo{users: make(map[uuid.UUID]*domain.User)}
}

func (r *inMemoryUserRepo) Create(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *inMemoryUserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}
func (r *inMemoryUserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (r *inMemoryUserRepo) GetByProvider(ctx context.Context, p domain.AuthProvider, pID string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Provider == p && u.ProviderID == pID {
			return u, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (r *inMemoryUserRepo) Update(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}

type inMemoryIdentityRepo struct {
	identities map[uuid.UUID]*domain.UserIdentity
}

func newInMemoryIdentityRepo() *inMemoryIdentityRepo {
	return &inMemoryIdentityRepo{identities: make(map[uuid.UUID]*domain.UserIdentity)}
}

func (r *inMemoryIdentityRepo) Create(ctx context.Context, ident *domain.UserIdentity) error {
	r.identities[ident.ID] = ident
	return nil
}

func (r *inMemoryIdentityRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.UserIdentity, error) {
	ident, ok := r.identities[id]
	if !ok {
		return nil, domain.ErrIdentityNotFound
	}
	return ident, nil
}

func (r *inMemoryIdentityRepo) GetByProviderAndID(ctx context.Context, provider domain.AuthProvider, providerUserID string) (*domain.UserIdentity, error) {
	for _, ident := range r.identities {
		if ident.Provider == provider && ident.ProviderUserID == providerUserID {
			return ident, nil
		}
	}
	return nil, domain.ErrIdentityNotFound
}

func (r *inMemoryIdentityRepo) ListByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.UserIdentity, error) {
	var res []*domain.UserIdentity
	for _, ident := range r.identities {
		if ident.UserID == userID {
			res = append(res, ident)
		}
	}
	return res, nil
}

func (r *inMemoryIdentityRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := r.identities[id]; !ok {
		return domain.ErrIdentityNotFound
	}
	delete(r.identities, id)
	return nil
}

type inMemoryKYCRepo struct {
	kycs map[uuid.UUID]*domain.KYCVerification
}

func newInMemoryKYCRepo() *inMemoryKYCRepo {
	return &inMemoryKYCRepo{kycs: make(map[uuid.UUID]*domain.KYCVerification)}
}

func (r *inMemoryKYCRepo) Create(ctx context.Context, k *domain.KYCVerification) error {
	r.kycs[k.ID] = k
	return nil
}
func (r *inMemoryKYCRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.KYCVerification, error) {
	k, ok := r.kycs[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return k, nil
}
func (r *inMemoryKYCRepo) GetLatestByUserID(ctx context.Context, userID uuid.UUID) (*domain.KYCVerification, error) {
	for _, k := range r.kycs {
		if k.UserID == userID {
			return k, nil
		}
	}
	return nil, domain.ErrUserNotFound
}
func (r *inMemoryKYCRepo) Update(ctx context.Context, k *domain.KYCVerification) error {
	r.kycs[k.ID] = k
	return nil
}

type inMemoryVehicleRepo struct {
	vehicles map[uuid.UUID]*domain.Vehicle
}

func newInMemoryVehicleRepo() *inMemoryVehicleRepo {
	return &inMemoryVehicleRepo{vehicles: make(map[uuid.UUID]*domain.Vehicle)}
}

func (r *inMemoryVehicleRepo) Create(ctx context.Context, v *domain.Vehicle) error {
	r.vehicles[v.ID] = v
	return nil
}
func (r *inMemoryVehicleRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Vehicle, error) {
	v, ok := r.vehicles[id]
	if !ok {
		return nil, domain.ErrVehicleNotFound
	}
	return v, nil
}
func (r *inMemoryVehicleRepo) GetByUserID(ctx context.Context, userID uuid.UUID) (*domain.Vehicle, error) {
	for _, v := range r.vehicles {
		if v.UserID == userID {
			return v, nil
		}
	}
	return nil, domain.ErrVehicleNotFound
}
func (r *inMemoryVehicleRepo) GetByPlateNumber(ctx context.Context, plate string) (*domain.Vehicle, error) {
	for _, v := range r.vehicles {
		if v.PlateNumber == plate {
			return v, nil
		}
	}
	return nil, domain.ErrVehicleNotFound
}
func (r *inMemoryVehicleRepo) Update(ctx context.Context, v *domain.Vehicle) error {
	r.vehicles[v.ID] = v
	return nil
}

func setupTestApp() *testEnv {
	userRepo := newInMemoryUserRepo()
	kycRepo := newInMemoryKYCRepo()
	vehicleRepo := newInMemoryVehicleRepo()
	sessionRepo := memory.NewSessionRepository(0)
	identityRepo := newInMemoryIdentityRepo()

	googleProvider := oauth.NewGoogleOAuthProvider("google-client-id")
	appleProvider := oauth.NewAppleOAuthProvider("apple-client-id")

	authService := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider, appleProvider},
		identityRepo,
	)

	bioProvider := biometrics.NewMockBiometricsProvider()
	notifierService := notifier.NewPushNotifier()

	kycService := services.NewKYCService(userRepo, kycRepo, bioProvider, notifierService)
	vehicleService := services.NewVehicleService(userRepo, vehicleRepo, notifierService)

	authHandler := adapterHttp.NewAuthHandler(authService)
	kycHandler := adapterHttp.NewKYCHandler(kycService)
	vehicleHandler := adapterHttp.NewVehicleHandler(vehicleService)
	authMiddleware := middleware.AuthMiddleware(authService)

	cfg := baseTestRouterConfig()
	cfg.AuthHandler = authHandler
	cfg.KYCHandler = kycHandler
	cfg.VehicleHandler = vehicleHandler
	cfg.AuthMiddleware = authMiddleware

	router := mustSetupTestRouter(cfg)

	return &testEnv{
		userRepo:       userRepo,
		kycRepo:        kycRepo,
		vehicleRepo:    vehicleRepo,
		sessionRepo:    sessionRepo,
		authService:    authService,
		kycService:     kycService,
		vehicleService: vehicleService,
		biometrics:     bioProvider,
		notifier:       notifierService,
		router:         router,
	}
}

func TestHTTP_SocialLogin_And_TTL20Minutes(t *testing.T) {
	env := setupTestApp()

	// 1. Social Login con Google (RF-01, RF-02, RF-03, RF-04)
	body, _ := json.Marshal(map[string]string{
		"provider": "GOOGLE",
		"id_token": "mock-google-testuser",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/social-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var res map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &res)

	token, ok := res["access_token"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, token)

	expiresIn, ok := res["expires_in"].(float64)
	assert.True(t, ok)
	assert.Equal(t, float64(1200), expiresIn, "RF-04: TTL de 20 minutos (1200s)")

	userMap := res["user"].(map[string]interface{})
	assert.Equal(t, "PASSENGER", userMap["role"], "RF-02: Perfil unificado base de Pasajero")

	// 2. Acceso a ruta protegida con token opaco de sesión
	reqMe := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	wMe := httptest.NewRecorder()
	env.router.ServeHTTP(wMe, reqMe)

	assert.Equal(t, http.StatusOK, wMe.Code)
	var meRes adapterHttp.AuthMeResponse
	_ = json.Unmarshal(wMe.Body.Bytes(), &meRes)
	if assert.NotNil(t, meRes.User) {
		assert.Equal(t, domain.RolePassenger, meRes.User.Role)
		assert.True(t, meRes.Capabilities.CanBookRides)
		assert.False(t, meRes.Capabilities.CanPublishRides)
	}

	// 3. Logout destruye la sesión inmediatamente -> /v1/auth/me debe responder 401
	reqLogout := httptest.NewRequest(http.MethodPost, "/v1/auth/logout", nil)
	reqLogout.Header.Set("Authorization", "Bearer "+token)
	wLogout := httptest.NewRecorder()
	env.router.ServeHTTP(wLogout, reqLogout)
	assert.Equal(t, http.StatusOK, wLogout.Code)

	reqMeAfterLogout := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	reqMeAfterLogout.Header.Set("Authorization", "Bearer "+token)
	wMeAfterLogout := httptest.NewRecorder()
	env.router.ServeHTTP(wMeAfterLogout, reqMeAfterLogout)
	assert.Equal(t, http.StatusUnauthorized, wMeAfterLogout.Code)
}

func TestHTTP_KYC_Max3Retries_Flow(t *testing.T) {
	env := setupTestApp()

	// Autenticar usuario
	body, _ := json.Marshal(map[string]string{
		"provider": "GOOGLE",
		"id_token": "mock-google-carlos",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/social-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var res map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	token := res["access_token"].(string)

	// Enviar 1er intento fallido
	env.biometrics.SetForcedFailure(true, "Liveness no superado")

	kycPayload, _ := json.Marshal(map[string]string{
		"document_type":      "DNI",
		"document_number":    "34123456",
		"front_image_base64": "front_data",
		"back_image_base64":  "back_data",
		"selfie_3d_base64":   "selfie_data",
	})

	// Intento 1
	reqK1 := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", bytes.NewReader(kycPayload))
	reqK1.Header.Set("Authorization", "Bearer "+token)
	wK1 := httptest.NewRecorder()
	env.router.ServeHTTP(wK1, reqK1)
	assert.Equal(t, http.StatusUnprocessableEntity, wK1.Code)

	// Intento 2
	reqK2 := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", bytes.NewReader(kycPayload))
	reqK2.Header.Set("Authorization", "Bearer "+token)
	wK2 := httptest.NewRecorder()
	env.router.ServeHTTP(wK2, reqK2)
	assert.Equal(t, http.StatusUnprocessableEntity, wK2.Code)

	// Intento 3 (RF-09: supera reintentos -> PENDING_MANUAL_REVIEW)
	reqK3 := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", bytes.NewReader(kycPayload))
	reqK3.Header.Set("Authorization", "Bearer "+token)
	wK3 := httptest.NewRecorder()
	env.router.ServeHTTP(wK3, reqK3)

	assert.Equal(t, http.StatusUnprocessableEntity, wK3.Code)
	var kycRes map[string]interface{}
	_ = json.Unmarshal(wK3.Body.Bytes(), &kycRes)
	assert.Equal(t, "PENDING_MANUAL_REVIEW", kycRes["code"])
	assert.Equal(t, false, kycRes["can_retry"])
}

func TestHTTP_VehicleRegistration_And_Rejection_RetainsPassenger(t *testing.T) {
	env := setupTestApp()

	// 1. Crear usuario
	body, _ := json.Marshal(map[string]string{
		"provider": "APPLE",
		"id_token": "mock-apple-marta",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/social-login", bytes.NewReader(body))
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	var res map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	token := res["access_token"].(string)
	userMap := res["user"].(map[string]interface{})
	userID := userMap["id"].(string)

	// 2. Intentar registrar vehículo SIN KYC aprobado -> 403 Forbidden
	vehPayload, _ := json.Marshal(map[string]interface{}{
		"brand":                 "Volkswagen",
		"model":                 "Gol",
		"year":                  2020,
		"plate_number":          "AF111GG",
		"color":                 "Rojo",
		"seat_capacity":         4,
		"driver_license_base64": "license_b64",
		"vehicle_cedula_base64": "cedula_b64",
	})

	reqV1 := httptest.NewRequest(http.MethodPost, "/v1/vehicles/register", bytes.NewReader(vehPayload))
	reqV1.Header.Set("Authorization", "Bearer "+token)
	wV1 := httptest.NewRecorder()
	env.router.ServeHTTP(wV1, reqV1)
	assert.Equal(t, http.StatusForbidden, wV1.Code)

	// 3. Aprobar KYC exitosamente
	env.biometrics.SetForcedFailure(false, "")
	kycPayload, _ := json.Marshal(map[string]string{
		"document_type":      "DNI",
		"document_number":    "34123456",
		"front_image_base64": "front_data",
		"back_image_base64":  "back_data",
		"selfie_3d_base64":   "selfie_data",
	})
	reqK := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", bytes.NewReader(kycPayload))
	reqK.Header.Set("Authorization", "Bearer "+token)
	wK := httptest.NewRecorder()
	env.router.ServeHTTP(wK, reqK)
	assert.Equal(t, http.StatusOK, wK.Code)

	// 4. Registrar vehículo con KYC aprobado -> 201 Created (PENDING_VERIFICATION)
	reqV2 := httptest.NewRequest(http.MethodPost, "/v1/vehicles/register", bytes.NewReader(vehPayload))
	reqV2.Header.Set("Authorization", "Bearer "+token)
	wV2 := httptest.NewRecorder()
	env.router.ServeHTTP(wV2, reqV2)
	assert.Equal(t, http.StatusCreated, wV2.Code)

	var vehRes map[string]interface{}
	_ = json.Unmarshal(wV2.Body.Bytes(), &vehRes)
	assert.Equal(t, "PENDING_VERIFICATION", vehRes["status"])
	vehData := vehRes["vehicle"].(map[string]interface{})
	vehID := vehData["id"].(string)

	// 5. Simular dictamen asíncrono de rechazo (RF-12, RF-13)
	resolPayload, _ := json.Marshal(map[string]interface{}{
		"approved": false,
		"reason":   "Cédula no coincide con el titular",
	})

	// 5.1 Intento de llamada anónima sin autenticación -> 401 Unauthorized
	reqAnon := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(resolPayload))
	wAnon := httptest.NewRecorder()
	env.router.ServeHTTP(wAnon, reqAnon)
	assert.Equal(t, http.StatusUnauthorized, wAnon.Code, "Llamada anónima al webhook debe ser rechazada")

	// 5.2 Llamada con firma HMAC válida del proveedor de revisión -> 200 OK
	reqRes := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(resolPayload))
	sig := middleware.ComputeWebhookSignature(resolPayload, testWebhookSecret)
	reqRes.Header.Set(middleware.HeaderWebhookSignature, "sha256="+sig)
	wRes := httptest.NewRecorder()
	env.router.ServeHTTP(wRes, reqRes)
	assert.Equal(t, http.StatusOK, wRes.Code)

	// 6. Validar que el usuario retiene 100% capacidad como pasajero (RF-13)
	user, err := env.userRepo.GetByID(context.Background(), uuid.MustParse(userID))
	assert.NoError(t, err)
	assert.Equal(t, domain.RolePassenger, user.Role)
	assert.False(t, user.IsDriverActive)
	assert.True(t, user.CanBookRides(), "RF-13: Retiene capacidad para buscar y reservar viajes")
	assert.False(t, user.CanPublishRides(), "Bloqueada la creación de viajes como conductor")
}
