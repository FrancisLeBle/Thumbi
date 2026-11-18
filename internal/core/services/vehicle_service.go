package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// VehicleService implementa los casos de uso para el alta y tramitación asíncrona de vehículos.
type VehicleService struct {
	userRepo    ports.UserRepository
	vehicleRepo ports.VehicleRepository
	notifier    ports.Notifier
}

// NewVehicleService construye una nueva instancia inyectando repositorios y notificador
func NewVehicleService(
	userRepo ports.UserRepository,
	vehicleRepo ports.VehicleRepository,
	notifier ports.Notifier,
) *VehicleService {
	return &VehicleService{
		userRepo:    userRepo,
		vehicleRepo: vehicleRepo,
		notifier:    notifier,
	}
}

// RegisterVehicle inicia el trámite de registro de vehículo y licencias (RF-10, RF-11)
func (s *VehicleService) RegisterVehicle(ctx context.Context, input ports.VehicleRegistrationInput) (*ports.VehicleDTO, error) {
	// 1. Validar que el usuario exista y tenga la verificación de identidad (DNI + Selfie) aprobada
	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, domain.ErrUserNotFound
	}

	if user.KYCStatus != domain.KYCStatusApproved {
		return nil, domain.ErrDriverKYCRequired
	}

	// 2. Validar datos mínimos obligatorios del automóvil
	if input.Brand == "" || input.Model == "" || input.PlateNumber == "" || input.SeatCapacity <= 0 {
		return nil, domain.ErrInvalidVehicleData
	}

	// 3. Validar si la patente ya se encuentra registrada
	existingVeh, err := s.vehicleRepo.GetByPlateNumber(ctx, input.PlateNumber)
	if err == nil && existingVeh != nil {
		if existingVeh.Status == domain.VehicleStatusApproved {
			return nil, domain.ErrVehicleAlreadyVerified
		}
	}

	// 4. Crear registro en estado PENDING_VERIFICATION (RF-11)
	vehID := domain.NewUUIDv7()
	vehicle := domain.NewVehicle(
		vehID,
		input.UserID,
		input.Brand,
		input.Model,
		input.Year,
		input.PlateNumber,
		input.Color,
		input.SeatCapacity,
		"https://storage.thumbi.app/docs/license_"+vehID.String()+".jpg",
		"https://storage.thumbi.app/docs/cedula_"+vehID.String()+".jpg",
	)

	if err := s.vehicleRepo.Create(ctx, vehicle); err != nil {
		return nil, fmt.Errorf("error al guardar trámite de vehículo: %w", err)
	}

	// 5. Notificar recepción del trámite asíncrono
	if s.notifier != nil {
		_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
			UserID:  user.ID,
			Email:   user.Email,
			Title:   "Trámite de Vehículo en Proceso",
			Message: fmt.Sprintf("Hemos recibido la documentación de tu %s %s (%s). Te notificaremos cuando concluya la revisión.", vehicle.Brand, vehicle.Model, vehicle.PlateNumber),
			Type:    "VEHICLE_PENDING_VERIFICATION",
			Metadata: map[string]string{
				"vehicle_id": vehicle.ID.String(),
			},
		})
	}

	return toVehicleDTO(vehicle), nil
}

// GetVehicleStatus consulta el estado del vehículo asociado al usuario
func (s *VehicleService) GetVehicleStatus(ctx context.Context, userID uuid.UUID) (*ports.VehicleDTO, error) {
	vehicle, err := s.vehicleRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if vehicle == nil {
		return nil, domain.ErrVehicleNotFound
	}
	return toVehicleDTO(vehicle), nil
}

// ProcessAsyncResolution procesa el resultado del trámite (RF-12, RF-13)
func (s *VehicleService) ProcessAsyncResolution(ctx context.Context, vehicleID uuid.UUID, approved bool, reason string) error {
	// 1. Obtener el vehículo
	vehicle, err := s.vehicleRepo.GetByID(ctx, vehicleID)
	if err != nil {
		return err
	}
	if vehicle == nil {
		return domain.ErrVehicleNotFound
	}

	// 2. Obtener el usuario asociado
	user, err := s.userRepo.GetByID(ctx, vehicle.UserID)
	if err != nil {
		return domain.ErrUserNotFound
	}

	if approved {
		// RF-12: Actualizar estado a APPROVED
		vehicle.Approve()

		// Activar rol de Conductor
		if err := user.PromoteToDriver(); err != nil {
			return err
		}

		if err := s.vehicleRepo.Update(ctx, vehicle); err != nil {
			return err
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return err
		}

		// Notificar al usuario (RF-12)
		if s.notifier != nil {
			_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
				UserID:  user.ID,
				Email:   user.Email,
				Title:   "¡Vehículo y Conductor Aprobados!",
				Message: fmt.Sprintf("Tu %s %s (%s) fue aprobado con éxito. Ya puedes publicar tus primeros viajes compartidos.", vehicle.Brand, vehicle.Model, vehicle.PlateNumber),
				Type:    "VEHICLE_APPROVED",
				Metadata: map[string]string{
					"vehicle_id": vehicle.ID.String(),
				},
			})
		}
	} else {
		// RF-12: Actualizar estado a REJECTED con motivo
		vehicle.Reject(reason)

		// RF-13: MIENTRAS la documentación sea rechazada, el sistema DEBE mantener habilitadas
		// las funciones de navegación, búsqueda y reserva en rol de Pasajero (100% operativo como pasajero)
		user.Role = domain.RolePassenger
		user.IsDriverActive = false
		user.UpdatedAt = time.Now().UTC()

		if err := s.vehicleRepo.Update(ctx, vehicle); err != nil {
			return err
		}
		if err := s.userRepo.Update(ctx, user); err != nil {
			return err
		}

		// Notificar al usuario (RF-12)
		if s.notifier != nil {
			_ = s.notifier.SendResolutionNotification(ctx, ports.NotificationPayload{
				UserID:  user.ID,
				Email:   user.Email,
				Title:   "Documentación de Vehículo Rechazada",
				Message: fmt.Sprintf("La documentación de tu vehículo no pudo ser validada. Motivo: %s. Aún puedes buscar y reservar viajes como pasajero sin restricciones.", reason),
				Type:    "VEHICLE_REJECTED",
				Metadata: map[string]string{
					"vehicle_id":       vehicle.ID.String(),
					"rejection_reason": reason,
				},
			})
		}
	}

	return nil
}

func toVehicleDTO(v *domain.Vehicle) *ports.VehicleDTO {
	if v == nil {
		return nil
	}
	return &ports.VehicleDTO{
		ID:                 v.ID,
		UserID:             v.UserID,
		Brand:              v.Brand,
		Model:              v.Model,
		Year:               v.Year,
		PlateNumber:        v.PlateNumber,
		Color:              v.Color,
		SeatCapacity:       v.SeatCapacity,
		DriverLicenseURL:   v.DriverLicenseURL,
		VehicleCedulaURL:   v.VehicleCedulaURL,
		InsurancePolicyURL: v.InsurancePolicyURL,
		Status:             v.Status,
		RejectionReason:    v.RejectionReason,
		VerifiedAt:         formatOptionalTimeRFC3339(v.VerifiedAt),
		CreatedAt:          formatTimeRFC3339(v.CreatedAt),
		UpdatedAt:          formatTimeRFC3339(v.UpdatedAt),
	}
}
