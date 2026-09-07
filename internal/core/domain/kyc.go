package domain

import (
	"time"
)

// MaxKYCRetries establece el límite estricto de reintentos automatizados permitidos (RF-08)
const MaxKYCRetries = 3

// LivenessScoreThreshold umbral mínimo de confianza para la selfie 3D
const LivenessScoreThreshold = 0.85

// KYCVerification representa el proceso de validación biométrica y documental de identidad (RF-07, RF-08, RF-09)
type KYCVerification struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	DocumentType   string    `json:"document_type"` // DNI, Pasaporte, Cédula
	DocumentNumber string    `json:"document_number"`
	FrontImageURL  string    `json:"front_image_url"`
	BackImageURL   string    `json:"back_image_url"`
	Selfie3DURL    string    `json:"selfie_3d_url"`
	LivenessScore  float64   `json:"liveness_score"`
	RetryCount     int       `json:"retry_count"`
	Status         KYCStatus `json:"status"`
	RejectionReason string   `json:"rejection_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// NewKYCVerification inicia una nueva solicitud de verificación
func NewKYCVerification(id, userID, docType, docNumber, frontURL, backURL, selfieURL string) *KYCVerification {
	now := time.Now().UTC()
	return &KYCVerification{
		ID:             id,
		UserID:         userID,
		DocumentType:   docType,
		DocumentNumber: docNumber,
		FrontImageURL:  frontURL,
		BackImageURL:   backURL,
		Selfie3DURL:    selfieURL,
		LivenessScore:  0.0,
		RetryCount:     0,
		Status:         KYCStatusPending,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Approve marca la verificación como exitosa
func (k *KYCVerification) Approve(score float64) {
	k.LivenessScore = score
	k.Status = KYCStatusApproved
	k.RejectionReason = ""
	k.UpdatedAt = time.Now().UTC()
}

// RecordFailure incrementa el contador de reintentos y determina si pasa a PENDING_MANUAL_REVIEW (RF-08, RF-09)
func (k *KYCVerification) RecordFailure(reason string, score float64) error {
	k.LivenessScore = score
	k.RetryCount++
	k.RejectionReason = reason
	k.UpdatedAt = time.Now().UTC()

	// RF-09: Al superar los 3 reintentos fallidos, marcar en PENDING_MANUAL_REVIEW
	if k.RetryCount >= MaxKYCRetries {
		k.Status = KYCStatusPendingManualReview
		return ErrMaxRetriesExceeded
	}

	// RF-08: Permitir reintentos manteniéndolo como REJECTED transitorio o PENDING
	k.Status = KYCStatusRejected
	return nil
}

// CanRetry verifica si el usuario aún puede realizar un intento automatizado
func (k *KYCVerification) CanRetry() bool {
	return k.RetryCount < MaxKYCRetries && k.Status != KYCStatusPendingManualReview && k.Status != KYCStatusApproved
}
