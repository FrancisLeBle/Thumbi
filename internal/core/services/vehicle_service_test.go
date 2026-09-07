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

func TestVehicleService_RegisterVehicle_RequiresKYC(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	vehicleRepo := new(MockVehicleRepository)
	notifier := new(MockNotifier)

	service := services.NewVehicleService(userRepo, vehicleRepo, notifier)

	// Usuario sin KYC aprobado
	user := domain.NewUser("usr-1", "user@example.com", "Laura", "Diaz", "", domain.ProviderGoogle, "g-2")
	user.KYCStatus = domain.KYCStatusPending // No aprobado

	userRepo.On("GetByID", ctx, "usr-1").Return(user, nil)

	_, err := service.RegisterVehicle(ctx, ports.VehicleRegistrationInput{
		UserID:       "usr-1",
		Brand:        "Toyota",
		Model:        "Yaris",
		Year:         2021,
		PlateNumber:  "AD123EF",
		SeatCapacity: 4,
	})

	assert.ErrorIs(t, err, domain.ErrDriverKYCRequired, "Debe exigir KYC aprobado antes de registrar vehículo")
}

func TestVehicleService_ProcessAsyncResolution_Approved(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	vehicleRepo := new(MockVehicleRepository)
	notifier := new(MockNotifier)

	service := services.NewVehicleService(userRepo, vehicleRepo, notifier)

	user := domain.NewUser("usr-1", "user@example.com", "Laura", "Diaz", "", domain.ProviderGoogle, "g-2")
	user.KYCStatus = domain.KYCStatusApproved

	vehicle := domain.NewVehicle("veh-100", "usr-1", "Toyota", "Yaris", 2021, "AD123EF", "Gris", 4, "lic.jpg", "ced.jpg")

	vehicleRepo.On("GetByID", ctx, "veh-100").Return(vehicle, nil)
	userRepo.On("GetByID", ctx, "usr-1").Return(user, nil)
	vehicleRepo.On("Update", ctx, mock.MatchedBy(func(v *domain.Vehicle) bool {
		return v.Status == domain.VehicleStatusApproved
	})).Return(nil)
	userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Role == domain.RoleDriver && u.IsDriverActive
	})).Return(nil)
	notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)

	err := service.ProcessAsyncResolution(ctx, "veh-100", true, "")

	assert.NoError(t, err)
	assert.Equal(t, domain.VehicleStatusApproved, vehicle.Status, "RF-12: Estado actualizado a APPROVED")
	assert.Equal(t, domain.RoleDriver, user.Role, "Usuario promovido a DRIVER")
	assert.True(t, user.IsDriverActive)
}

func TestVehicleService_ProcessAsyncResolution_Rejected_RetainsPassenger(t *testing.T) {
	ctx := context.Background()
	userRepo := new(MockUserRepository)
	vehicleRepo := new(MockVehicleRepository)
	notifier := new(MockNotifier)

	service := services.NewVehicleService(userRepo, vehicleRepo, notifier)

	user := domain.NewUser("usr-1", "user@example.com", "Laura", "Diaz", "", domain.ProviderGoogle, "g-2")
	user.KYCStatus = domain.KYCStatusApproved

	vehicle := domain.NewVehicle("veh-100", "usr-1", "Toyota", "Yaris", 2021, "AD123EF", "Gris", 4, "lic.jpg", "ced.jpg")

	vehicleRepo.On("GetByID", ctx, "veh-100").Return(vehicle, nil)
	userRepo.On("GetByID", ctx, "usr-1").Return(user, nil)
	vehicleRepo.On("Update", ctx, mock.MatchedBy(func(v *domain.Vehicle) bool {
		return v.Status == domain.VehicleStatusRejected && v.RejectionReason == "Licencia vencida"
	})).Return(nil)
	userRepo.On("Update", ctx, mock.MatchedBy(func(u *domain.User) bool {
		return u.Role == domain.RolePassenger && !u.IsDriverActive
	})).Return(nil)
	notifier.On("SendResolutionNotification", ctx, mock.Anything).Return(nil)

	err := service.ProcessAsyncResolution(ctx, "veh-100", false, "Licencia vencida")

	assert.NoError(t, err)
	assert.Equal(t, domain.VehicleStatusRejected, vehicle.Status)
	assert.Equal(t, domain.RolePassenger, user.Role, "RF-13: Retiene 100% de capacidad como Pasajero")
	assert.False(t, user.IsDriverActive)
	assert.True(t, user.CanBookRides(), "RF-13: Puede continuar buscando y reservando viajes")
}
