package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
	"github.com/thumbi/auth-kyc-service/internal/core/services"
)

func TestDisputeService_OpenDispute(t *testing.T) {
	ctx := context.Background()

	trip := &domain.Trip{
		ID:       "trip-d-1",
		DriverID: "driver-1",
		Status:   domain.TripStatusCompleted,
	}

	booking := &domain.Booking{
		ID:          "booking-d-1",
		TripID:      "trip-d-1",
		PassengerID: "passenger-1",
		Status:      domain.BookingStatusCompleted,
	}

	escrowHeld, _ := domain.NewEscrowTransaction(
		"escrow-d-1",
		"booking-d-1",
		"trip-d-1",
		"passenger-1",
		"driver-1",
		5000.0,
		"ARS",
		"pi_test_d1",
	)

	t.Run("falla si faltan campos obligatorios", func(t *testing.T) {
		svc := services.NewDisputeService(nil, nil, nil, nil, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   "",
			ReporterID:  "pass-1",
			Description: "",
		})
		assert.ErrorIs(t, err, domain.ErrInvalidDisputeData)
		assert.Nil(t, res)
	})

	t.Run("falla si la reserva no existe", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockBookingRepo.On("FindByID", ctx, "unknown-b").Return(nil, domain.ErrBookingNotFound)

		svc := services.NewDisputeService(nil, nil, mockBookingRepo, nil, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   "unknown-b",
			ReporterID:  "pass-1",
			Description: "Problema en el viaje",
		})
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
		assert.Nil(t, res)
	})

	t.Run("falla si el denunciante no es parte de la reserva", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-d-1").Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-d-1").Return(trip, nil)

		svc := services.NewDisputeService(nil, nil, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   "booking-d-1",
			ReporterID:  "intruder-user",
			Description: "Reclamación no autorizada",
		})
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si el escrow ya está liquidado (RF-17)", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-d-1").Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-d-1").Return(trip, nil)

		settledEscrow := *escrowHeld
		settledEscrow.Status = domain.EscrowStatusReleased
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-d-1").Return(&settledEscrow, nil)

		svc := services.NewDisputeService(nil, mockEscrowRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   "booking-d-1",
			ReporterID:  "passenger-1",
			Description: "Intento de disputa extemporánea",
		})
		assert.ErrorIs(t, err, domain.ErrEscrowAlreadySettled)
		assert.Nil(t, res)
	})

	t.Run("falla si ya existe una disputa para este escrow (RF-18)", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockEscrowRepo := new(MockEscrowRepository)
		mockDisputeRepo := new(MockDisputeRepository)

		mockBookingRepo.On("FindByID", ctx, "booking-d-1").Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-d-1").Return(trip, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-d-1").Return(escrowHeld, nil)

		existing := &domain.Dispute{
			ID:                  "disp-existing",
			EscrowTransactionID: escrowHeld.ID,
			Status:              domain.DisputeStatusOpened,
		}
		mockDisputeRepo.On("FindByEscrowID", ctx, escrowHeld.ID).Return(existing, nil)

		svc := services.NewDisputeService(mockDisputeRepo, mockEscrowRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   "booking-d-1",
			ReporterID:  "passenger-1",
			Description: "Reintento de disputa",
		})
		assert.ErrorIs(t, err, domain.ErrDisputeAlreadyExists)
		assert.Nil(t, res)
	})

	t.Run("abre exitosamente disputa y congela el Escrow en DISPUTED (RF-14 y RF-15)", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockEscrowRepo := new(MockEscrowRepository)
		mockDisputeRepo := new(MockDisputeRepository)

		escrowCopy := *escrowHeld
		mockBookingRepo.On("FindByID", ctx, "booking-d-1").Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, "trip-d-1").Return(trip, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-d-1").Return(&escrowCopy, nil)
		mockDisputeRepo.On("FindByEscrowID", ctx, escrowHeld.ID).Return(nil, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowHeld.ID, domain.EscrowStatusDisputed).Return(nil)
		mockDisputeRepo.On("Save", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		svc := services.NewDisputeService(mockDisputeRepo, mockEscrowRepo, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:    "booking-d-1",
			ReporterID:   "passenger-1",
			Reason:       domain.DisputeReasonRouteDeviation,
			Description:  "El conductor cambió la ruta y demoró 3 horas adicionales.",
			EvidenceURLs: []string{"https://thumbi-evidence.s3.amazonaws.com/route_log.pdf"},
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.DisputeStatusOpened, res.Status)
		assert.Equal(t, "passenger-1", res.ReporterID)
		assert.Equal(t, "driver-1", res.DefendantID)
		assert.Equal(t, domain.EscrowStatusDisputed, escrowCopy.Status)

		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
	})
}

func TestDisputeService_ResolveDispute(t *testing.T) {
	ctx := context.Background()

	baseDispute := &domain.Dispute{
		ID:                  "disp-res-1",
		EscrowTransactionID: "escrow-res-1",
		BookingID:           "booking-res-1",
		TripID:              "trip-res-1",
		ReporterID:          "passenger-1",
		DefendantID:         "driver-1",
		Reason:              domain.DisputeReasonNoShow,
		Description:         "El conductor no llegó al punto de encuentro",
		Status:              domain.DisputeStatusOpened,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	escrowDisputed, _ := domain.NewEscrowTransaction(
		"escrow-res-1",
		"booking-res-1",
		"trip-res-1",
		"passenger-1",
		"driver-1",
		4500.0,
		"ARS",
		"pi_gateway_123",
	)
	_ = escrowDisputed.OpenDispute(time.Now().UTC())

	t.Run("falla si la disputa ya está cerrada", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		closedDispute := *baseDispute
		closedDispute.Status = domain.DisputeStatusResolvedPassengerRefund

		mockDisputeRepo.On("FindByID", ctx, "disp-res-1").Return(&closedDispute, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil, nil)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  "disp-res-1",
			AdminID:    "admin-root",
			Resolution: domain.DisputeStatusResolvedDriverPayout,
		})

		assert.ErrorIs(t, err, domain.ErrDisputeAlreadyResolved)
		assert.Nil(t, res)
	})

	t.Run("dictamina reembolso al pasajero (RESOLVED_PASSENGER_REFUND)", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockEscrowRepo := new(MockEscrowRepository)
		mockPaymentGateway := new(MockPaymentGateway)

		disputeCopy := *baseDispute
		escrowCopy := *escrowDisputed

		mockDisputeRepo.On("FindByID", ctx, "disp-res-1").Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-res-1").Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, "escrow-res-1", domain.EscrowStatusRefundedFull).Return(nil)
		mockPaymentGateway.On("ProcessRefund", ctx, "pi_gateway_123", 4500.0, mock.Anything).Return(&ports.RefundResult{
			GatewayRefundRef: "ref_gw_999",
			Amount:           4500.0,
			Status:           "succeeded",
		}, nil)
		mockEscrowRepo.On("RecordRefund", ctx, mock.AnythingOfType("*domain.RefundTransaction")).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		svc := services.NewDisputeService(mockDisputeRepo, mockEscrowRepo, nil, nil, mockPaymentGateway)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  "disp-res-1",
			AdminID:    "admin-root",
			Resolution: domain.DisputeStatusResolvedPassengerRefund,
			AdminNotes: "No se constató presencia del conductor. Reembolso total aprobado.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.DisputeStatusResolvedPassengerRefund, res.Status)
		assert.Equal(t, domain.EscrowStatusRefundedFull, escrowCopy.Status)
		assert.NotNil(t, res.ResolvedAt)

		mockPaymentGateway.AssertExpectations(t)
		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
	})

	t.Run("dictamina liquidación al conductor (RESOLVED_DRIVER_PAYOUT)", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		disputeCopy := *baseDispute
		escrowCopy := *escrowDisputed

		mockDisputeRepo.On("FindByID", ctx, "disp-res-1").Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-res-1").Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, "escrow-res-1", domain.EscrowStatusReleased).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		svc := services.NewDisputeService(mockDisputeRepo, mockEscrowRepo, nil, nil, nil)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  "disp-res-1",
			AdminID:    "admin-root",
			Resolution: domain.DisputeStatusResolvedDriverPayout,
			AdminNotes: "Conductor esperó los 15 minutos de cortesía y presentó registro GPS.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.DisputeStatusResolvedDriverPayout, res.Status)
		assert.Equal(t, domain.EscrowStatusReleased, escrowCopy.Status)

		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
	})

	t.Run("desestima la disputa (REJECTED) y restaura HELD", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		disputeCopy := *baseDispute
		escrowCopy := *escrowDisputed

		mockDisputeRepo.On("FindByID", ctx, "disp-res-1").Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, "booking-res-1").Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, "escrow-res-1", domain.EscrowStatusHeld).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		svc := services.NewDisputeService(mockDisputeRepo, mockEscrowRepo, nil, nil, nil)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  "disp-res-1",
			AdminID:    "admin-root",
			Resolution: domain.DisputeStatusRejected,
			AdminNotes: "Reclamo infundado sin elementos probatorios.",
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.DisputeStatusRejected, res.Status)
		assert.Equal(t, domain.EscrowStatusHeld, escrowCopy.Status)

		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
	})
}

func TestDisputeService_GetAndList(t *testing.T) {
	ctx := context.Background()

	d1 := &domain.Dispute{
		ID:                  "disp-1",
		EscrowTransactionID: "escrow-1",
		BookingID:           "b-1",
		TripID:              "t-1",
		ReporterID:          "p-1",
		DefendantID:         "d-1",
		Reason:              domain.DisputeReasonVehicleMismatch,
		Description:         "Vehículo distinto",
		Status:              domain.DisputeStatusOpened,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	t.Run("GetDisputeByID existente", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockDisputeRepo.On("FindByID", ctx, "disp-1").Return(d1, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil, nil)
		res, err := svc.GetDisputeByID(ctx, "disp-1")

		require.NoError(t, err)
		assert.Equal(t, "disp-1", res.ID)
		assert.Equal(t, domain.DisputeReasonVehicleMismatch, res.Reason)
	})

	t.Run("ListDisputes paginado", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockDisputeRepo.On("List", ctx, 10, 0).Return([]*domain.Dispute{d1}, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil, nil)
		res, err := svc.ListDisputes(ctx, 10, 0)

		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, "disp-1", res[0].ID)
	})
}
