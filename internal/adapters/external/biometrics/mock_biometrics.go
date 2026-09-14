package biometrics

import (
	"context"
	"strings"
)

// MockBiometricsProvider simula el motor de biometría y OCR para pruebas de vida y validación de DNI
type MockBiometricsProvider struct {
	DefaultLivenessScore  float64
	DefaultFaceMatchScore float64
	ForceFailure          bool
	FailureReason         string
}

// NewMockBiometricsProvider inicializa el simulador biométrico con parámetros por defecto
func NewMockBiometricsProvider() *MockBiometricsProvider {
	return &MockBiometricsProvider{
		DefaultLivenessScore:  0.94, // Supera el umbral de 0.85
		DefaultFaceMatchScore: 0.90, // Coincidencia de rostro alta
		ForceFailure:          false,
	}
}

func (p *MockBiometricsProvider) VerifyIdentity(
	ctx context.Context,
	docType, frontBase64, backBase64, selfie3DBase64 string,
) (*ports.BiometricEvaluationResult, error) {
	// 1. Simulación por configuración estática
	if p.ForceFailure {
		reason := p.FailureReason
		if reason == "" {
			reason = "Prueba de vida biométrica fallida: movimiento facial artificial o estático"
		}
		return &ports.BiometricEvaluationResult{
			IsValidDocument: false,
			LivenessScore:   0.42,
			FaceMatchScore:  0.35,
			RejectionReason: reason,
		}, nil
	}

	// 2. Simulación dinámica basada en strings de entrada para testing declarativo
	combined := strings.ToLower(frontBase64 + backBase64 + selfie3DBase64)

	// Simular fallo de legibilidad de documento
	if strings.Contains(combined, "fail_ocr") || strings.Contains(combined, "bad_doc") {
		return &ports.BiometricEvaluationResult{
			IsValidDocument: false,
			LivenessScore:   p.DefaultLivenessScore,
			FaceMatchScore:  0.0,
			RejectionReason: "DNI ilegible o con reflejos que impiden el OCR de la identidad",
		}, nil
	}

	// Simular fallo de prueba de vida (liveness < 0.85)
	if strings.Contains(combined, "fail_liveness") || strings.Contains(combined, "spoof") {
		return &ports.BiometricEvaluationResult{
			IsValidDocument: true,
			LivenessScore:   0.55, // Por debajo del umbral de 0.85
			FaceMatchScore:  0.88,
			ExtractedDNI:    "30111222",
			ExtractedName:   "Usuario Prueba",
			RejectionReason: "No se superó la prueba de vida (score 0.55 < 0.85)",
		}, nil
	}

	// Simular fallo de coincidencia facial
	if strings.Contains(combined, "fail_face_match") || strings.Contains(combined, "diff_person") {
		return &ports.BiometricEvaluationResult{
			IsValidDocument: true,
			LivenessScore:   0.92,
			FaceMatchScore:  0.41, // Muy bajo
			ExtractedDNI:    "30111222",
			ExtractedName:   "Usuario Prueba",
			RejectionReason: "El rostro de la selfie 3D no coincide con la fotografía del DNI",
		}, nil
	}

	// Caso exitoso por defecto
	return &ports.BiometricEvaluationResult{
		IsValidDocument: true,
		LivenessScore:   p.DefaultLivenessScore,
		FaceMatchScore:  p.DefaultFaceMatchScore,
		ExtractedDNI:    "38920144",
		ExtractedName:   "Juan Carlos Rodriguez",
	}, nil
}

// SetForcedFailure permite alternar dinámicamente el comportamiento del mock
func (p *MockBiometricsProvider) SetForcedFailure(failed bool, reason string) {
	p.ForceFailure = failed
	p.FailureReason = reason
}

var _ ports.BiometricsProvider = (*MockBiometricsProvider)(nil)
