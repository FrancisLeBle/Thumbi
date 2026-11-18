package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestWebhook_VehicleResolution_Security(t *testing.T) {
	env := setupTestApp()

	// 1. Crear usuario y aprobar KYC
	loginPayload, _ := json.Marshal(map[string]string{
		"provider": "GOOGLE",
		"id_token": "mock-google-webhookuser",
	})
	reqLogin := httptest.NewRequest(http.MethodPost, "/v1/auth/social-login", bytes.NewReader(loginPayload))
	wLogin := httptest.NewRecorder()
	env.router.ServeHTTP(wLogin, reqLogin)
	assert.Equal(t, http.StatusCreated, wLogin.Code)

	var loginRes map[string]interface{}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginRes)
	token := loginRes["access_token"].(string)
	userMap := loginRes["user"].(map[string]interface{})
	userID := userMap["id"].(string)

	// Aprobar KYC
	env.biometrics.SetForcedFailure(false, "")
	kycPayload, _ := json.Marshal(map[string]string{
		"document_type":      "DNI",
		"document_number":    "34123456",
		"front_image_base64": "front_data",
		"back_image_base64":  "back_data",
		"selfie_3d_base64":   "selfie_data",
	})
	reqKYC := httptest.NewRequest(http.MethodPost, "/v1/kyc/submit", bytes.NewReader(kycPayload))
	reqKYC.Header.Set("Authorization", "Bearer "+token)
	wKYC := httptest.NewRecorder()
	env.router.ServeHTTP(wKYC, reqKYC)
	assert.Equal(t, http.StatusOK, wKYC.Code)

	// 2. Registrar vehículo -> PENDING_VERIFICATION
	vehPayload, _ := json.Marshal(map[string]interface{}{
		"brand":                 "Toyota",
		"model":                 "Corolla",
		"year":                  2022,
		"plate_number":          "AF999ZZ",
		"color":                 "Blanco",
		"seat_capacity":         4,
		"driver_license_base64": "license_b64",
		"vehicle_cedula_base64": "cedula_b64",
	})
	reqVeh := httptest.NewRequest(http.MethodPost, "/v1/vehicles/register", bytes.NewReader(vehPayload))
	reqVeh.Header.Set("Authorization", "Bearer "+token)
	wVeh := httptest.NewRecorder()
	env.router.ServeHTTP(wVeh, reqVeh)
	assert.Equal(t, http.StatusCreated, wVeh.Code)

	var vehRes map[string]interface{}
	_ = json.Unmarshal(wVeh.Body.Bytes(), &vehRes)
	vehData := vehRes["vehicle"].(map[string]interface{})
	vehID := vehData["id"].(string)

	secret := testWebhookSecret

	approvalPayload, _ := json.Marshal(map[string]interface{}{
		"approved": true,
		"reason":   "Todos los documentos verificados con éxito",
	})

	// Caso 1: Vulnerabilidad previa - Solicitud anónima SIN autenticación
	t.Run("Anonymous request without headers should be rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(approvalPayload))
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var errRes map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &errRes)
		assert.Equal(t, "UNAUTHORIZED_WEBHOOK", errRes["code"])
	})

	// Caso 2: Solicitud con firma HMAC manipulada o inválida
	t.Run("Forged or invalid HMAC signature should be rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(approvalPayload))
		req.Header.Set(middleware.HeaderWebhookSignature, "sha256=invalid_forged_hmac_signature_0123456789abcdef")
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var errRes map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &errRes)
		assert.Equal(t, "INVALID_WEBHOOK_SIGNATURE", errRes["code"])
	})

	// Caso 3: Solicitud con secreto compartido incorrecto
	t.Run("Incorrect shared secret should be rejected with 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(approvalPayload))
		req.Header.Set(middleware.HeaderWebhookSecret, "wrong-shared-secret")
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		var errRes map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &errRes)
		assert.Equal(t, "INVALID_WEBHOOK_SECRET", errRes["code"])
	})

	// Caso 4: Solicitud legítima con secreto compartido en X-Webhook-Secret
	t.Run("Valid shared secret header should be accepted with 200", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(approvalPayload))
		req.Header.Set(middleware.HeaderWebhookSecret, secret)
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	// Caso 5: Solicitud legítima con firma HMAC-SHA256 (prefijo sha256=)
	t.Run("Valid HMAC-SHA256 signature should approve vehicle and promote driver", func(t *testing.T) {
		sig := middleware.ComputeWebhookSignature(approvalPayload, secret)

		req := httptest.NewRequest(http.MethodPost, "/v1/vehicles/"+vehID+"/resolution", bytes.NewReader(approvalPayload))
		req.Header.Set(middleware.HeaderWebhookSignature, "sha256="+sig)
		w := httptest.NewRecorder()
		env.router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Verificar que el vehículo fue aprobado
		vehicle, err := env.vehicleRepo.GetByID(context.Background(), uuid.MustParse(vehID))
		assert.NoError(t, err)
		assert.Equal(t, domain.VehicleStatusApproved, vehicle.Status)

		// Verificar que el usuario fue promovido a conductor
		user, err := env.userRepo.GetByID(context.Background(), uuid.MustParse(userID))
		assert.NoError(t, err)
		assert.Equal(t, domain.RoleDriver, user.Role)
		assert.True(t, user.IsDriverActive)
		assert.True(t, user.CanPublishRides())
	})
}
