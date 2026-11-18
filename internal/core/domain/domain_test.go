package domain_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestUser_DefaultPassengerRole(t *testing.T) {
	user := domain.NewUser(domain.NewUUIDv7(), "juan@example.com", "Juan", "Flor", "", domain.ProviderGoogle, "goog-123")

	assert.Equal(t, domain.RolePassenger, user.Role, "RF-02: Debe crearse como PASSENGER por defecto")
	assert.Equal(t, domain.KYCStatusNotStarted, user.KYCStatus)
	assert.True(t, user.CanBookRides(), "RF-13: Pasajero siempre puede reservar")
	assert.False(t, user.CanPublishRides(), "No debe publicar viajes sin KYC aprobado y rol Driver")
}

func TestSession_TTL20Minutes(t *testing.T) {
	session := domain.NewSession(domain.NewUUIDv7(), domain.NewUUIDv7(), "jwt-token", "Mozilla/5.0", "127.0.0.1")

	assert.Equal(t, domain.SessionTTL, 20*time.Minute, "RF-04: TTL debe ser exactamente 20 minutos")
	assert.False(t, session.IsExpired())
	assert.WithinDuration(t, time.Now().UTC().Add(20*time.Minute), session.ExpiresAt, 2*time.Second)
}

func TestKYC_Max3Retries(t *testing.T) {
	kyc := domain.NewKYCVerification(domain.NewUUIDv7(), domain.NewUUIDv7(), "DNI", "12345678", "f.jpg", "b.jpg", "s.jpg")

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
	veh := domain.NewVehicle(domain.NewUUIDv7(), domain.NewUUIDv7(), "Toyota", "Corolla", 2022, "AB123CD", "Blanco", 4, "lic.jpg", "ced.jpg")

	assert.Equal(t, domain.VehicleStatusPendingVerification, veh.Status, "RF-11: Debe iniciar en PENDING_VERIFICATION")
	assert.False(t, veh.IsReadyForPublishing())

	veh.Approve()
	assert.Equal(t, domain.VehicleStatusApproved, veh.Status)
	assert.True(t, veh.IsReadyForPublishing())
	assert.NotNil(t, veh.VerifiedAt)
}

func TestUUIDv7_GenerationAndParsing(t *testing.T) {
	id1 := domain.NewUUIDv7()
	id2 := domain.NewUUIDv7()

	assert.NotEqual(t, uuid.Nil, id1)
	assert.NotEqual(t, uuid.Nil, id2)
	assert.NotEqual(t, id1, id2)
	assert.Equal(t, uuid.Version(7), id1.Version())
	assert.Equal(t, uuid.RFC4122, id1.Variant())

	parsed, err := domain.ParseUUID(id1.String())
	require.NoError(t, err)
	assert.Equal(t, id1, parsed)

	_, errNil := domain.ParseUUID(uuid.Nil.String())
	assert.ErrorIs(t, errNil, domain.ErrInvalidInput)

	_, errBad := domain.ParseUUID("not-a-uuid")
	assert.ErrorIs(t, errBad, domain.ErrInvalidInput)
}

func TestTypedEnums_IsValid(t *testing.T) {
	assert.True(t, domain.UserRolePassenger.IsValid())
	assert.True(t, domain.UserRoleDriver.IsValid())
	assert.True(t, domain.UserRoleAdmin.IsValid())
	assert.False(t, domain.UserRole("SUPERUSER").IsValid())

	assert.True(t, domain.ProviderGoogle.IsValid())
	assert.True(t, domain.ProviderApple.IsValid())
	assert.False(t, domain.AuthProvider("GITHUB").IsValid())

	assert.True(t, domain.KYCStatusApproved.IsValid())
	assert.False(t, domain.KYCStatus("UNKNOWN").IsValid())

	assert.True(t, domain.VehicleStatusApproved.IsValid())
	assert.False(t, domain.VehicleStatus("NONE").IsValid())

	assert.True(t, domain.TripStatusPublished.IsValid())
	assert.False(t, domain.TripStatus("ARCHIVED").IsValid())

	assert.True(t, domain.BookingStatusConfirmed.IsValid())
	assert.False(t, domain.BookingStatus("PAID").IsValid())

	assert.True(t, domain.EscrowStatusHeld.IsValid())
	assert.False(t, domain.EscrowStatus("LOCKED").IsValid())

	assert.True(t, domain.RefundTypeAdminDispute.IsValid())
	assert.False(t, domain.RefundType("OTHER").IsValid())

	assert.True(t, domain.DisputeStatusOpened.IsValid())
	assert.False(t, domain.DisputeStatusOpened.IsValidResolution())
	assert.True(t, domain.DisputeStatusResolvedPassengerRefund.IsValidResolution())
	assert.True(t, domain.DisputeStatusRejected.IsValidResolution())
	assert.False(t, domain.DisputeStatus("CLOSED").IsValid())

	assert.True(t, domain.DisputeReasonRouteDeviation.IsValid())
	assert.False(t, domain.DisputeReason("INVALID_REASON").IsValid())
}
