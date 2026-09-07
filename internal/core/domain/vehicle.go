package domain

import (
	"time"
)

// VehicleStatus define el estado de tramitación asíncrona del vehículo (RF-11, RF-12)
type VehicleStatus string

const (
	VehicleStatusPendingVerification VehicleStatus = "PENDING_VERIFICATION"
	VehicleStatusApproved            VehicleStatus = "APPROVED"
	VehicleStatusRejected            VehicleStatus = "REJECTED"
)

// Vehicle representa el automóvil y la documentación de conducción presentada (RF-10, RF-11, RF-12, RF-13)
type Vehicle struct {
	ID                 string        `json:"id"`
	UserID             string        `json:"user_id"`
	Brand              string        `json:"brand"`               // Ej: Toyota
	Model              string        `json:"model"`               // Ej: Corolla
	Year               int           `json:"year"`                // Ej: 2022
	PlateNumber        string        `json:"plate_number"`        // Ej: AB123CD
	Color              string        `json:"color"`               // Ej: Blanco
	SeatCapacity       int           `json:"seat_capacity"`       // Capacidad de pasajeros
	DriverLicenseURL   string        `json:"driver_license_url"`  // Foto de Licencia de Conducir
	VehicleCedulaURL   string        `json:"vehicle_cedula_url"`  // Cédula de Identificación del Vehículo
	InsurancePolicyURL string        `json:"insurance_policy_url,omitempty"`
	Status             VehicleStatus `json:"status"`
	RejectionReason    string        `json:"rejection_reason,omitempty"`
	VerifiedAt         *time.Time    `json:"verified_at,omitempty"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`
}

// NewVehicle crea una nueva solicitud de registro de vehículo en estado PENDING_VERIFICATION (RF-11)
func NewVehicle(id, userID, brand, model string, year int, plate, color string, seats int, licenseURL, cedulaURL string) *Vehicle {
	now := time.Now().UTC()
	return &Vehicle{
		ID:               id,
		UserID:           userID,
		Brand:            brand,
		Model:            model,
		Year:             year,
		PlateNumber:      plate,
		Color:            color,
		SeatCapacity:     seats,
		DriverLicenseURL: licenseURL,
		VehicleCedulaURL: cedulaURL,
		Status:           VehicleStatusPendingVerification, // RF-11: Se registra en estado PENDING_VERIFICATION
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// Approve marca la verificación asíncrona del vehículo como aprobada (RF-12)
func (v *Vehicle) Approve() {
	now := time.Now().UTC()
	v.Status = VehicleStatusApproved
	v.RejectionReason = ""
	v.VerifiedAt = &now
	v.UpdatedAt = now
}

// Reject marca la verificación asíncrona del vehículo como rechazada con su motivo (RF-12)
func (v *Vehicle) Reject(reason string) {
	now := time.Now().UTC()
	v.Status = VehicleStatusRejected
	v.RejectionReason = reason
	v.UpdatedAt = now
}

// IsReadyForPublishing confirma si el vehículo cuenta con la aprobación necesaria para publicar viajes
func (v *Vehicle) IsReadyForPublishing() bool {
	return v.Status == VehicleStatusApproved
}
