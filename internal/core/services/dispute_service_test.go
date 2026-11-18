package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
	"github.com/thumbi/backend/internal/core/services"
)

func TestDisputeService_OpenDispute(t *testing.T) {
	ctx := context.Background()

	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000701")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000702")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000703")
	intruderID := uuid.MustParse("01960000-0000-7000-8000-000000000704")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000705")
	unknownBookingID := uuid.MustParse("01960000-0000-7000-8000-000000000799")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000706")
	existingDisputeID := uuid.MustParse("01960000-0000-7000-8000-000000000707")

	trip := &domain.Trip{
		ID:       tripID,
		DriverID: driverID,
		Status:   domain.TripStatusCompleted,
	}

	booking := &domain.Booking{
		ID:          bookingID,
		TripID:      tripID,
		PassengerID: passengerID,
		Status:      domain.BookingStatusCompleted,
	}

	escrowHeld, _ := domain.NewEscrowTransaction(
		escrowID,
		bookingID,
		tripID,
		passengerID,
		driverID,
		domain.MoneyFromCents(500000, domain.CurrencyARS),
		"pi_test_d1",
	)

	t.Run("falla si faltan campos obligatorios", func(t *testing.T) {
		svc := services.NewDisputeService(nil, nil, nil, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   uuid.Nil,
			ReporterID:  passengerID,
			Description: "",
		})
		assert.ErrorIs(t, err, domain.ErrInvalidDisputeData)
		assert.Nil(t, res)
	})

	t.Run("falla si la reserva no existe", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockBookingRepo.On("FindByID", ctx, unknownBookingID).Return(nil, domain.ErrBookingNotFound)

		svc := services.NewDisputeService(nil, mockBookingRepo, nil, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   unknownBookingID,
			ReporterID:  passengerID,
			Reason:      domain.DisputeReasonOther,
			Description: "Problema en el viaje",
		})
		assert.ErrorIs(t, err, domain.ErrBookingNotFound)
		assert.Nil(t, res)
	})

	t.Run("falla si el denunciante no es parte de la reserva", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(trip, nil)

		svc := services.NewDisputeService(nil, mockBookingRepo, mockTripRepo, nil)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   bookingID,
			ReporterID:  intruderID,
			Reason:      domain.DisputeReasonOther,
			Description: "Reclamación no autorizada",
		})
		assert.ErrorIs(t, err, domain.ErrUnauthorized)
		assert.Nil(t, res)
	})

	t.Run("falla si el escrow ya está liquidado (RF-17)", func(t *testing.T) {
		mockBookingRepo := new(MockBookingRepository)
		mockTripRepo := new(MockTripRepository)
		mockEscrowRepo := new(MockEscrowRepository)

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(trip, nil)

		settledEscrow := *escrowHeld
		settledEscrow.Status = domain.EscrowStatusReleased
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(&settledEscrow, nil)

		settlement := services.NewSettlementService(mockEscrowRepo, nil)
		svc := services.NewDisputeService(nil, mockBookingRepo, mockTripRepo, settlement)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   bookingID,
			ReporterID:  passengerID,
			Reason:      domain.DisputeReasonOther,
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

		mockBookingRepo.On("FindByID", ctx, bookingID).Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(trip, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(escrowHeld, nil)

		existing := &domain.Dispute{
			ID:                  existingDisputeID,
			EscrowTransactionID: escrowHeld.ID,
			Status:              domain.DisputeStatusOpened,
		}
		mockDisputeRepo.On("FindByEscrowID", ctx, escrowHeld.ID).Return(existing, nil)

		settlement := services.NewSettlementService(mockEscrowRepo, nil)
		svc := services.NewDisputeService(mockDisputeRepo, mockBookingRepo, mockTripRepo, settlement)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:   bookingID,
			ReporterID:  passengerID,
			Reason:      domain.DisputeReasonOther,
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
		mockBookingRepo.On("FindByID", ctx, bookingID).Return(booking, nil)
		mockTripRepo.On("FindByID", ctx, tripID).Return(trip, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(&escrowCopy, nil)
		mockDisputeRepo.On("FindByEscrowID", ctx, escrowHeld.ID).Return(nil, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowHeld.ID, domain.EscrowStatusDisputed).Return(nil)
		mockDisputeRepo.On("Save", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		settlement := services.NewSettlementService(mockEscrowRepo, nil)
		svc := services.NewDisputeService(mockDisputeRepo, mockBookingRepo, mockTripRepo, settlement)
		res, err := svc.OpenDispute(ctx, ports.OpenDisputeInput{
			BookingID:    bookingID,
			ReporterID:   passengerID,
			Reason:       domain.DisputeReasonRouteDeviation,
			Description:  "El conductor cambió la ruta y demoró 3 horas adicionales.",
			EvidenceURLs: []string{"https://thumbi-evidence.s3.amazonaws.com/route_log.pdf"},
		})

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, domain.DisputeStatusOpened, res.Status)
		assert.Equal(t, passengerID, res.ReporterID)
		assert.Equal(t, driverID, res.DefendantID)
		assert.Equal(t, domain.EscrowStatusDisputed, escrowCopy.Status)

		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
	})
}

func TestDisputeService_ResolveDispute(t *testing.T) {
	ctx := context.Background()

	disputeID := uuid.MustParse("01960000-0000-7000-8000-000000000801")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000802")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000803")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000804")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000805")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000806")
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000807")

	baseDispute := &domain.Dispute{
		ID:                  disputeID,
		EscrowTransactionID: escrowID,
		BookingID:           bookingID,
		TripID:              tripID,
		ReporterID:          passengerID,
		DefendantID:         driverID,
		Reason:              domain.DisputeReasonNoShow,
		Description:         "El conductor no llegó al punto de encuentro",
		Status:              domain.DisputeStatusOpened,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	escrowDisputed, _ := domain.NewEscrowTransaction(
		escrowID,
		bookingID,
		tripID,
		passengerID,
		driverID,
		domain.MoneyFromCents(450000, domain.CurrencyARS),
		"pi_gateway_123",
	)
	_ = escrowDisputed.OpenDispute(time.Now().UTC())

	t.Run("falla si la disputa ya está cerrada", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		closedDispute := *baseDispute
		closedDispute.Status = domain.DisputeStatusResolvedPassengerRefund

		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(&closedDispute, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  disputeID,
			AdminID:    adminID,
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
		expectedRefundAmount := domain.MoneyFromCents(450000, domain.CurrencyARS)

		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowID, domain.EscrowStatusRefundedFull).Return(nil)
		mockPaymentGateway.On("ProcessRefund", ctx, "pi_gateway_123", expectedRefundAmount, mock.Anything).Return(&ports.RefundResult{
			GatewayRefundRef: "ref_gw_999",
			Amount:           expectedRefundAmount,
			Status:           "succeeded",
		}, nil)
		mockEscrowRepo.On("RecordRefund", ctx, mock.AnythingOfType("*domain.RefundTransaction")).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		settlement := services.NewSettlementService(mockEscrowRepo, mockPaymentGateway)
		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, settlement)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  disputeID,
			AdminID:    adminID,
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

		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowID, domain.EscrowStatusReleased).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		settlement := services.NewSettlementService(mockEscrowRepo, nil)
		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, settlement)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  disputeID,
			AdminID:    adminID,
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

		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(&disputeCopy, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(&escrowCopy, nil)

		mockEscrowRepo.On("UpdateEscrowStatus", ctx, escrowID, domain.EscrowStatusHeld).Return(nil)
		mockDisputeRepo.On("Update", ctx, mock.AnythingOfType("*domain.Dispute")).Return(nil)

		settlement := services.NewSettlementService(mockEscrowRepo, nil)
		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, settlement)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  disputeID,
			AdminID:    adminID,
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

	disputeID := uuid.MustParse("01960000-0000-7000-8000-000000000901")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000902")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000903")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000904")
	reporterID := uuid.MustParse("01960000-0000-7000-8000-000000000905")
	defendantID := uuid.MustParse("01960000-0000-7000-8000-000000000906")

	d1 := &domain.Dispute{
		ID:                  disputeID,
		EscrowTransactionID: escrowID,
		BookingID:           bookingID,
		TripID:              tripID,
		ReporterID:          reporterID,
		DefendantID:         defendantID,
		Reason:              domain.DisputeReasonVehicleMismatch,
		Description:         "Vehículo distinto",
		Status:              domain.DisputeStatusOpened,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	t.Run("GetDisputeByID existente", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(d1, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil)
		res, err := svc.GetDisputeByID(ctx, disputeID)

		require.NoError(t, err)
		assert.Equal(t, disputeID, res.ID)
		assert.Equal(t, domain.DisputeReasonVehicleMismatch, res.Reason)
	})

	t.Run("ListDisputes paginado", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockDisputeRepo.On("List", ctx, 10, 0).Return([]*domain.Dispute{d1}, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil)
		res, err := svc.ListDisputes(ctx, 10, 0)

		require.NoError(t, err)
		assert.Len(t, res, 1)
		assert.Equal(t, disputeID, res[0].ID)
		mockDisputeRepo.AssertExpectations(t)
	})

	t.Run("ListDisputes aplica límites por defecto si limit u offset están fuera de rango", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockDisputeRepo.On("List", ctx, 20, 0).Return([]*domain.Dispute{}, nil)

		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil)
		res, err := svc.ListDisputes(ctx, -5, -1)

		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Empty(t, res)
		mockDisputeRepo.AssertExpectations(t)
	})
}

func TestDisputeService_ResolveDispute_ValidationAndTransaction(t *testing.T) {
	ctx := context.Background()

	disputeID := uuid.MustParse("01960000-0000-7000-8000-000000000a01")
	escrowID := uuid.MustParse("01960000-0000-7000-8000-000000000a02")
	bookingID := uuid.MustParse("01960000-0000-7000-8000-000000000a03")
	tripID := uuid.MustParse("01960000-0000-7000-8000-000000000a04")
	passengerID := uuid.MustParse("01960000-0000-7000-8000-000000000a05")
	driverID := uuid.MustParse("01960000-0000-7000-8000-000000000a06")
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000a07")

	t.Run("rechaza estados arbitrarios o literales no válidos como resolución", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, nil)

		for _, invalid := range []domain.DisputeStatus{
			domain.DisputeStatus("RESOLVED"),
			domain.DisputeStatus("INVALID_ACTION"),
			domain.DisputeStatusOpened,
			domain.DisputeStatusInReview,
			domain.DisputeStatus(""),
		} {
			res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
				DisputeID:  disputeID,
				AdminID:    adminID,
				Resolution: invalid,
				AdminNotes: "Nota administrativa",
			})
			assert.ErrorIs(t, err, domain.ErrInvalidDisputeData)
			assert.Nil(t, res)
		}
		mockDisputeRepo.AssertNotCalled(t, "FindByID", mock.Anything, mock.Anything)
	})

	t.Run("ejecuta reembolso, registro contable y actualización de disputa dentro de TransactionManager", func(t *testing.T) {
		mockDisputeRepo := new(MockDisputeRepository)
		mockEscrowRepo := new(MockEscrowRepository)
		mockPaymentGateway := new(MockPaymentGateway)
		mockTx := new(MockTransactionManager)

		dispute := &domain.Dispute{
			ID:                  disputeID,
			EscrowTransactionID: escrowID,
			BookingID:           bookingID,
			TripID:              tripID,
			ReporterID:          passengerID,
			DefendantID:         driverID,
			Reason:              domain.DisputeReasonNoShow,
			Description:         "Conductor ausente",
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           time.Now().UTC(),
			UpdatedAt:           time.Now().UTC(),
		}
		refundAmount := domain.MoneyFromCents(320000, domain.CurrencyARS)
		escrow, err := domain.NewEscrowTransaction(escrowID, bookingID, tripID, passengerID, driverID, refundAmount, "pi_tx_1")
		require.NoError(t, err)
		require.NoError(t, escrow.OpenDispute(time.Now().UTC()))

		mockDisputeRepo.On("FindByID", ctx, disputeID).Return(dispute, nil)
		mockEscrowRepo.On("FindByBookingID", ctx, bookingID).Return(escrow, nil)
		mockPaymentGateway.On("ProcessRefund", ctx, "pi_tx_1", refundAmount, mock.Anything).Return(&ports.RefundResult{
			GatewayRefundRef: "re_tx_1",
			Amount:           refundAmount,
			Status:           "succeeded",
		}, nil)

		mockTx.On("WithinTransaction", ctx).Return(nil)
		mockEscrowRepo.On("UpdateEscrowStatus", mock.Anything, escrowID, domain.EscrowStatusRefundedFull).Return(nil)
		mockEscrowRepo.On("RecordRefund", mock.Anything, mock.MatchedBy(func(r *domain.RefundTransaction) bool {
			return r.RefundType == domain.RefundTypeAdminDispute && r.PassengerRefundAmount == refundAmount && r.GatewayRefundRef == "re_tx_1"
		})).Return(nil)
		mockDisputeRepo.On("Update", mock.Anything, mock.MatchedBy(func(d *domain.Dispute) bool {
			return d.ID == disputeID && d.Status == domain.DisputeStatusResolvedPassengerRefund
		})).Return(nil)

		settlement := services.NewSettlementService(mockEscrowRepo, mockPaymentGateway, mockTx)
		svc := services.NewDisputeService(mockDisputeRepo, nil, nil, settlement, mockTx)
		res, err := svc.ResolveDispute(ctx, ports.ResolveDisputeInput{
			DisputeID:  disputeID,
			AdminID:    adminID,
			Resolution: domain.DisputeStatusResolvedPassengerRefund,
			AdminNotes: "Reembolso verificado",
		})

		require.NoError(t, err)
		assert.Equal(t, domain.DisputeStatusResolvedPassengerRefund, res.Status)
		mockTx.AssertExpectations(t)
		mockEscrowRepo.AssertExpectations(t)
		mockDisputeRepo.AssertExpectations(t)
		mockPaymentGateway.AssertExpectations(t)
	})
}
