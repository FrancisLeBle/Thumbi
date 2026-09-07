package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// kycService implementa ports.KYCService
type kycService struct {
	userRepo           ports.UserRepository
	kycRepo            ports.KYCRepository
	biometricsProvider ports.BiometricsProvider
	notifier           ports.Notifier
}

// NewKYCService construye una nueva instancia del servicio de KYC
func NewKYCService(
	userRepo ports.UserRepository,
	kycRepo ports.KYCRepository,
	biometricsProvider ports.BiometricsProvider,
	notifier ports.Notifier,
) ports.KYCService {
	return &kycService{
		userRepo:           userRepo,
		kycRepo:            kycRepo,
		biometricsProvider: biometricsProvider,
		notifier:           notifier,
	}
}

// SubmitVerification procesa la validación biométrica y documental de identidad (RF-07, RF-08, RF-09)
func (s *kycService) SubmitVerification(ctx context.Context, input ports.KYCSubmissionInput) (*domain.KYCVerification, error) {
	// 1. Verificar que el usuario exista
	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	if user.KYCStatus == domain.KYCStatusApproved {
		return nil, domain.ErrKYCAlreadyApproved
	}

	if user.KYCStatus == domain.KYCStatusPendingManualReview {
		return nil, domain.ErrKYCInManualReview
	}

	// 2. Obtener o inicializar la entidad de verificación para control de reintentos
	kyc, err := s.kycRepo.GetLatestByUserID(ctx, input.UserID)
	if err != nil && !errors.Is(err, domain.ErrUserNotFound) {
		// Error de base de datos no esperado
		return nil, err
	}

	if kyc == nil {
		kycID := uuid.New().String()
		kyc = domain.NewKYCVerification(
			kycID,
			input.UserID,
			input.DocumentType,
			input.DocumentNumber,
			"https://storage.thumbi.app/kyc/front_"+kycID+".jpg",
			"https://storage.thumbi.app/kyc/back_"+kycID+".jpg",
			"https://storage.thumbi.app/kyc/selfie_"+kycID+".mp4",
		)
		if err := s.kycRepo.Create(ctx, kyc); err != nil {
			return nil, fmt.Errorf("error al inicializar registro KYC: %w", err)
		}
	} else {
		// Verificar si ya agotó los reintentos
		if !kyc.CanRetry() {
			if kyc.Status == domain.KYCStatusPendingManualReview {
				return kyc, domain.ErrKYCInManualReview
			}
			return kyc, domain.ErrMaxRetriesExceeded
		}
	}

	// 3. Evaluar con el motor biométrico (Selfie 3D + OCR DNI)
	bioResult, err := s.biometricsProvider.VerifyIdentity(
		ctx,
		input.DocumentType,
		input.FrontImageBase64,
		input.BackImageBase64,
		input.Selfie3DBase64,
	)
	if err != nil {
		return nil, fmt.Errorf("error en servicio biométrico: %w", err)
	}

	// 4. Evaluar resultado: liveness >= 0.85 y documento válido
	passed := bioResult.IsValidDocument && bioResult.LivenessScore >= domain.LivenessScoreThreshold && bioResult.FaceMatchScore >= 0.75

	if passed {
		// Aprobación exitosa
		kyc.Approve(bioResult.LivenessScore)
		user.KYCStatus = domain.KYCStatusApproved
		user.UpdatedAt = time.Now().UTC()

		if err := s.kycRepo.Update(ctx, kyc); err != nil {
			return nil, err
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return nil, err
		}

		// Notificar al usuario
		if s.notifier != nil {
			_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
				UserID:  user.ID,
				Email:   user.Email,
				Title:   "Identidad Verificada",
				Message: "Tu validación biométrica y DNI fueron aprobados con éxito.",
				Type:    "KYC_APPROVED",
			})
		}

		return kyc, nil
	}

	// 5. Fallo: Registrar fallo e incrementar contador de reintentos (RF-08, RF-09)
	reason := bioResult.RejectionReason
	if reason == "" {
		if !bioResult.IsValidDocument {
			reason = "No fue posible leer con claridad el DNI. Asegúrate de evitar reflejos."
		} else if bioResult.LivenessScore < domain.LivenessScoreThreshold {
			reason = "Prueba de vida insuficiente. Sigue las instrucciones del movimiento facial."
		} else {
			reason = "El rostro de la selfie no coincide suficientemente con la foto del DNI."
		}
	}

	failErr := kyc.RecordFailure(reason, bioResult.LivenessScore)

	// RF-09: Si superó los 3 intentos, marcar usuario en PENDING_MANUAL_REVIEW
	if kyc.Status == domain.KYCStatusPendingManualReview {
		user.KYCStatus = domain.KYCStatusPendingManualReview
	} else {
		user.KYCStatus = domain.KYCStatusRejected
	}
	user.UpdatedAt = time.Now().UTC()

	_ = s.kycRepo.Update(ctx, kyc)
	_ = s.userRepo.Update(ctx, user)

	if failErr != nil {
		return kyc, failErr
	}

	return kyc, domain.ErrLivenessCheckFailed
}

// GetStatus retorna el estado de la verificación KYC de un usuario
func (s *kycService) GetStatus(ctx context.Context, userID string) (*domain.KYCVerification, error) {
	kyc, err := s.kycRepo.GetLatestByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if kyc == nil {
		return nil, domain.ErrUserNotFound
	}
	return kyc, nil
}

// ReviewManually permite a un operador de mesa de control aprobar o rechazar manualmente una solicitud bloqueada
func (s *kycService) ReviewManually(ctx context.Context, kycID string, approved bool, reason string) error {
	kyc, err := s.kycRepo.GetByID(ctx, kycID)
	if err != nil {
		return err
	}

	user, err := s.userRepo.GetByID(ctx, kyc.UserID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	if approved {
		kyc.Approve(1.0)
		user.KYCStatus = domain.KYCStatusApproved
	} else {
		kyc.Status = domain.KYCStatusRejected
		kyc.RejectionReason = reason
		kyc.UpdatedAt = time.Now().UTC()
		user.KYCStatus = domain.KYCStatusRejected
	}
	user.UpdatedAt = time.Now().UTC()

	if err := s.kycRepo.Update(ctx, kyc); err != nil {
		return err
	}
	if err := s.userRepo.Update(ctx, user); err != nil {
		return err
	}

	if s.notifier != nil {
		title := "Resolución de Verificación Manual"
		msg := "Tu trámite ha sido aprobado."
		if !approved {
			msg = fmt.Sprintf("Tu trámite fue rechazado: %s", reason)
		}
		_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
			UserID:  user.ID,
			Email:   user.Email,
			Title:   title,
			Message: msg,
			Type:    "KYC_MANUAL_REVIEW_RESOLVED",
		})
	}

	return nil
}
