package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestUser_DefaultPassengerRole(t *testing.T) {
	user := domain.NewUser("usr-1", "juan@example.com", "Juan", "Flor", "", domain.ProviderGoogle, "goog-123")

	assert.Equal(t, domain.RolePassenger, user.Role, "RF-02: Debe crearse como PASSENGER por defecto")
	assert.Equal(t, domain.KYCStatusNotStarted, user.KYCStatus)
	assert.True(t, user.CanBookRides(), "RF-13: Pasajero siempre puede reservar")
	assert.False(t, user.CanPublishRides(), "No debe publicar viajes sin KYC aprobado y rol Driver")
}

func TestSession_TTL20Minutes(t *testing.T) {
	session := domain.NewSession("sess-1", "usr-1", "jwt-token", "Mozilla/5.0", "127.0.0.1")

	assert.Equal(t, domain.SessionTTL, 20*time.Minute, "RF-04: TTL debe ser exactamente 20 minutos")
	assert.False(t, session.IsExpired())
	assert.WithinDuration(t, time.Now().UTC().Add(20*time.Minute), session.ExpiresAt, 2*time.Second)
}

func TestKYC_Max3Retries(t *testing.T) {
	kyc := domain.NewKYCVerification("kyc-1", "usr-1", "DNI", "12345678", "f.jpg", "b.jpg", "s.jpg")

	// Intento 1 fallido
	err := kyc.RecordFailure("Baja calidad de imagen", 0.60)
	assert.NoError(t, err)
	assert.Equal(t, 1, kyc.RetryCount)
	assert.Equal(t, domain.KYCStatusRejected, kyc.Status)
	assert.True(t, kyc.CanRetry())

	// Intento 2 fallido
	err = kyc.RecordFailure("Rostro no coincide", 0.50)
	assert.NoError(t, err)
	assert.Equal(t, 2, kyc.RetryCount)
	assert.True(t, kyc.CanRetry())

	// Intento 3 fallido -> RF-09: Exceder reintentos deriva a PENDING_MANUAL_REVIEW
	err = kyc.RecordFailure("DNI no coincide con selfie", 0.40)
	assert.ErrorIs(t, err, domain.ErrMaxRetriesExceeded)
	assert.Equal(t, 3, kyc.RetryCount)
	assert.Equal(t, domain.KYCStatusPendingManualReview, kyc.Status)
	assert.False(t, kyc.CanRetry())
}

func TestVehicle_AsyncTransitions(t *testing.T) {
	veh := domain.NewVehicle("veh-1", "usr-1", "Toyota", "Corolla", 2022, "AB123CD", "Blanco", 4, "lic.jpg", "ced.jpg")

	assert.Equal(t, domain.VehicleStatusPendingVerification, veh.Status, "RF-11: Debe iniciar en PENDING_VERIFICATION")
	assert.False(t, veh.IsReadyForPublishing())

	veh.Approve()
	assert.Equal(t, domain.VehicleStatusApproved, veh.Status)
	assert.True(t, veh.IsReadyForPublishing())
	assert.NotNil(t, veh.VerifiedAt)
}
