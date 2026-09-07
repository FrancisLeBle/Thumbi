package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/biometrics"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/notifier"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/oauth"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
	"github.com/thumbi/auth-kyc-service/pkg/jwt"
)

// In-memory mock DB for testing the HTTP layer end-to-end
type testEnv struct {
	userRepo       *inMemoryUserRepo
	kycRepo        *inMemoryKYCRepo
	vehicleRepo    *inMemoryVehicleRepo
	sessionRepo    *postgres.MemorySessionRepository
	tokenManager   *jwt.TokenManager
	authService    ports.AuthService
	kycService     ports.KYCService
	vehicleService ports.VehicleService
	biometrics     *biometrics.MockBiometricsProvider
	notifier       *notifier.PushNotifier
	router         http.Handler
}

type inMemoryUserRepo struct {
	users map[string]*domain.User
}

func newInMemoryUserRepo() *inMemoryUserRepo {
	return &inMemoryUserRepo{users: make(map[string]*domain.User)}
}

func (r *inMemoryUserRepo) Create(ctx context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *inMemoryUserRepo) GetByID(ctx context.Context, id string) (*domain.User, error) {
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

type inMemoryKYCRepo struct {
	kycs map[string]*domain.KYCVerification
}

func newInMemoryKYCRepo() *inMemoryKYCRepo {
	return &inMemoryKYCRepo{kycs: make(map[string]*domain.KYCVerification)}
}

func (r *inMemoryKYCRepo) Create(ctx context.Context, k *domain.KYCVerification) error {
	r.kycs[k.ID] = k
	return nil
}
func (r *inMemoryKYCRepo) GetByID(ctx context.Context, id string) (*domain.KYCVerification, error) {
	k, ok := r.kycs[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return k, nil
}
func (r *inMemoryKYCRepo) GetLatestByUserID(ctx context.Context, userID string) (*domain.KYCVerification, error) {
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
	vehicles map[string]*domain.Vehicle
}

func newInMemoryVehicleRepo() *inMemoryVehicleRepo {
	return &inMemoryVehicleRepo{vehicles: make(map[string]*domain.Vehicle)}
}

func (r *inMemoryVehicleRepo) Create(ctx context.Context, v *domain.Vehicle) error {
	r.vehicles[v.ID] = v
	return nil
}
func (r *inMemoryVehicleRepo) GetByID(ctx context.Context, id string) (*domain.Vehicle, error) {
	v, ok := r.vehicles[id]
	if !ok {
		return nil, domain.ErrVehicleNotFound
	}
	return v, nil
}
func (r *inMemoryVehicleRepo) GetByUserID(ctx context.Context, userID string) (*domain.Vehicle, error) {
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
func (r *inMemoryVehicleRepo) ListPendingVerification(ctx context.Context, limit, offset int) ([]*domain.Vehicle, error) {
	var list []*domain.Vehicle
	for _, v := range r.vehicles {
		if v.Status == domain.VehicleStatusPendingVerification {
			list = append(list, v)
		}
	}
	return list, nil
}

func setupTestApp() *testEnv {
	userRepo := newInMemoryUserRepo()
	kycRepo := newInMemoryKYCRepo()
	vehicleRepo := newInMemoryVehicleRepo()
	sessionRepo := postgres.NewMemorySessionRepository(0)

	tokenManager := jwt.NewTokenManager("super-secret-key-for-test-purposes-only", "thumbi-test")
	googleProvider := oauth.NewGoogleOAuthProvider("google-client-id")
	appleProvider := oauth.NewAppleOAuthProvider("apple-client-id")

	authService := services.NewAuthService(
		userRepo,
		sessionRepo,
		[]ports.OAuthProvider{googleProvider, appleProvider},
		tokenManager,
	)

	bioProvider := biometrics.NewMockBiometricsProvider()
	notifierService := notifier.NewPushNotifier()

	kycService := services.NewKYCService(userRepo, kycRepo, bioProvider, notifierService)
	vehicleService := services.NewVehicleService(userRepo, vehicleRepo, notifierService)

	authHandler := adapterHttp.NewAuthHandler(authService, userRepo)
	kycHandler := adapterHttp.NewKYCHandler(kycService)
	vehicleHandler := adapterHttp.NewVehicleHandler(vehicleService)
	authMiddleware := middleware.AuthMiddleware(authService)

	router := adapterHttp.SetupRouter(adapterHttp.RouterConfig{
		AuthHandler:    authHandler,
		KYCHandler:     kycHandler,
		VehicleHandler: vehicleHandler,
		AuthMiddleware: authMiddleware,
	})

	return &testEnv{
		userRepo:       userRepo,
		kycRepo:        kycRepo,
		vehicleRepo:    vehicleRepo,
		sessionRepo:    sessionRepo,
		tokenManager:   tokenManager,
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

	// 2. Acceso a ruta protegida con JWT
	reqMe := httptest.NewRequest(http.MethodGet, "/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+token)
	wMe := httptest.NewRecorder()
	env.router.ServeHTTP(wMe, reqMe)

	assert.Equal(t, http.StatusOK, wMe.Code)
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
	reqRes := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(resolPayload))
	wRes := httptest.NewRecorder()
	env.router.ServeHTTP(wRes, reqRes)
	assert.Equal(t, http.StatusOK, wRes.Code)

	// 6. Validar que el usuario retiene 100% capacidad como pasajero (RF-13)
	user, err := env.userRepo.GetByID(context.Background(), userID)
	assert.NoError(t, err)
	assert.Equal(t, domain.RolePassenger, user.Role)
	assert.False(t, user.IsDriverActive)
	assert.True(t, user.CanBookRides(), "RF-13: Retiene capacidad para buscar y reservar viajes")
	assert.False(t, user.CanPublishRides(), "Bloqueada la creación de viajes como conductor")
}
