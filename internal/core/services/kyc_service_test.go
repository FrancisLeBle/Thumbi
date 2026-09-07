package services_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
)

func TestKYCService_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	kycRepo := new(MockKYCRepository)
	bioProvider := new(MockBiometricsProvider)
	notifier := new(MockNotifier)

	service := services.NewKYCService(userRepo, kycRepo, bioProvider, notifier)

	user := domain.NewUser("usr-123", "carlos@example.com", "Carlos", "Gomez", "", domain.ProviderGoogle, "g-1")
	userRepo.On("GetByID", ctx, "usr-123").Return(user, nil)
	kycRepo.On("GetLatestByUserID", ctx, "usr-123").Return(nil, domain.ErrUserNotFound)
	kycRepo.On("Create", ctx, mock.Anything).Return(nil)

	// Simular biometría aprobada
	bioResult := &ports.BiometricEvaluationResult{
		IsValidDocument: true,
		LivenessScore:   0.92, // > 0.85
		FaceMatchScore:  0.88,
		ExtractedDNI:    "35999888",
		ExtractedName:   "Carlos Gomez",
	}
	bioProvider.On("VerifyIdentity", ctx, "DNI", "front", "back", "selfie").Return(bioResult, nil)
	kycRepo.On("Update", ctx, mock.MatchedBy(func(k *domain.KYCVerification) bool {
		return k.Status == domain.KYCStatusApproved
	})).Return(nil)
	userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.KYCStatus == domain.KYCStatusApproved
	})).Return(nil)
	notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)

	res, err := service.SubmitVerification(ctx, ports.KYCSubmissionInput{
		UserID:           "usr-123",
		DocumentType:     "DNI",
		DocumentNumber:   "35999888",
		FrontImageBase64: "front",
		BackImageBase64:  "back",
		Selfie3DBase64:   "selfie",
	})

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, domain.KYCStatusApproved, res.Status)
	assert.Equal(t, domain.KYCStatusApproved, user.KYCStatus)
}

func TestKYCService_Max3Retries_DerivesToManualReview(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	kycRepo := new(MockKYCRepository)
	bioProvider := new(MockBiometricsProvider)
	notifier := new(MockNotifier)

	service := services.NewKYCService(userRepo, kycRepo, bioProvider, notifier)

	user := domain.NewUser("usr-123", "carlos@example.com", "Carlos", "Gomez", "", domain.ProviderGoogle, "g-1")
	existingKYC := domain.NewKYCVerification("kyc-1", "usr-123", "DNI", "35999888", "f.jpg", "b.jpg", "s.mp4")
	existingKYC.RetryCount = 2 // Ya tiene 2 fallos anteriores

	userRepo.On("GetByID", ctx, "usr-123").Return(user, nil)
	kycRepo.On("GetLatestByUserID", ctx, "usr-123").Return(existingKYC, nil)

	// Simular 3er fallo de prueba de vida
	bioResult := &ports.BiometricEvaluationResult{
		IsValidDocument: true,
		LivenessScore:   0.45, // Fallo de liveness
		FaceMatchScore:  0.40,
		RejectionReason: "Movimiento facial no detectado",
	}
	bioProvider.On("VerifyIdentity", ctx, "DNI", "front", "back", "selfie").Return(bioResult, nil)
	kycRepo.On("Update", ctx, mock.MatchedBy(func(k *domain.KYCVerification) bool {
		return k.Status == domain.KYCStatusPendingManualReview && k.RetryCount == 3
	})).Return(nil)
	userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.KYCStatus == domain.KYCStatusPendingManualReview
	})).Return(nil)

	res, err := service.SubmitVerification(ctx, ports.KYCSubmissionInput{
		UserID:           "usr-123",
		DocumentType:     "DNI",
		DocumentNumber:   "35999888",
		FrontImageBase64: "front",
		BackImageBase64:  "back",
		Selfie3DBase64:   "selfie",
	})

	// RF-09: Al superar 3 reintentos fallidos, marcar en PENDING_MANUAL_REVIEW
	assert.ErrorIs(t, err, domain.ErrMaxRetriesExceeded)
	assert.Equal(t, domain.KYCStatusPendingManualReview, res.Status)
	assert.Equal(t, 3, res.RetryCount)
	assert.Equal(t, domain.KYCStatusPendingManualReview, user.KYCStatus)
}
