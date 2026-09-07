package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestDispute_ValidCreationAndLifecycle(t *testing.T) {
	now := time.Now().UTC()
	params := domain.DisputeParams{
		ID:                  "disp-1",
		EscrowTransactionID: "escrow-100",
		BookingID:           "book-100",
		TripID:              "trip-100",
		ReporterID:          "passenger-1",
		DefendantID:         "driver-1",
		Reason:              domain.DisputeReasonRouteDeviation,
		Description:         "El conductor tomó una ruta distinta y cobró peajes adicionales en efectivo.",
		EvidenceURLs:        []string{"https://s3.amazonaws.com/thumbi/ev1.jpg"},
		EscrowStatus:        domain.EscrowStatusHeld,
	}

	dispute, err := domain.NewDispute(params)
	assert.NoError(t, err)
	assert.NotNil(t, dispute)
	assert.Equal(t, domain.DisputeStatusOpened, dispute.Status)
	assert.False(t, dispute.IsClosed())

	// Transición a IN_REVIEW por administrador
	err = dispute.StartReview("admin-99", now)
	assert.NoError(t, err)
	assert.Equal(t, domain.DisputeStatusInReview, dispute.Status)
	assert.Equal(t, "admin-99", *dispute.ResolvedBy)

	// Resolución a favor del pasajero
	err = dispute.ResolvePassengerRefund("admin-99", "Evidencias válidas, se procede al reembolso del 100%.", now)
	assert.NoError(t, err)
	assert.Equal(t, domain.DisputeStatusResolvedPassengerRefund, dispute.Status)
	assert.True(t, dispute.IsClosed())
	assert.NotNil(t, dispute.ResolvedAt)

	// Intento de reabrir o modificar disputa cerrada
	err = dispute.ResolveDriverPayout("admin-99", "Reintentando payout", now)
	assert.ErrorIs(t, err, domain.ErrDisputeAlreadyResolved)
}

func TestDispute_RejectSelfDispute(t *testing.T) {
	params := domain.DisputeParams{
		ID:                  "disp-2",
		EscrowTransactionID: "escrow-200",
		BookingID:           "book-200",
		TripID:              "trip-200",
		ReporterID:          "user-1",
		DefendantID:         "user-1", // Mismo usuario
		Reason:              domain.DisputeReasonNoShow,
		Description:         "Auto disputa",
		EscrowStatus:        domain.EscrowStatusHeld,
	}

	dispute, err := domain.NewDispute(params)
	assert.ErrorIs(t, err, domain.ErrDisputeSelfNotAllowed)
	assert.Nil(t, dispute)
}

func TestDispute_CannotDisputeSettledEscrow(t *testing.T) {
	params := domain.DisputeParams{
		ID:                  "disp-3",
		EscrowTransactionID: "escrow-300",
		BookingID:           "book-300",
		TripID:              "trip-300",
		ReporterID:          "passenger-1",
		DefendantID:         "driver-1",
		Reason:              domain.DisputeReasonVehicleMismatch,
		Description:         "Vehículo no correspondía con el auto registrado.",
		EscrowStatus:        domain.EscrowStatusReleased, // Ya liquidado
	}

	dispute, err := domain.NewDispute(params)
	assert.ErrorIs(t, err, domain.ErrEscrowAlreadySettled, "RF-17: No se puede abrir disputa sobre fondos ya liquidados")
	assert.Nil(t, dispute)
}

func TestDispute_DriverPayoutResolutionAllowsEscrowRelease(t *testing.T) {
	now := time.Now().UTC()
	escrow, err := domain.NewEscrowTransaction("escrow-400", "book-400", "trip-400", "pass-1", "driver-1", 45.00, "USD", "pi_test_400")
	assert.NoError(t, err)

	// Se abre disputa sobre el escrow
	err = escrow.OpenDispute(now)
	assert.NoError(t, err)
	assert.Equal(t, domain.EscrowStatusDisputed, escrow.Status)

	// Disputa resuelta a favor del conductor
	disputeParams := domain.DisputeParams{
		ID:                  "disp-400",
		EscrowTransactionID: escrow.ID,
		BookingID:           escrow.BookingID,
		TripID:              escrow.TripID,
		ReporterID:          escrow.PayerID,
		DefendantID:         escrow.PayeeID,
		Reason:              domain.DisputeReasonPassengerMisconduct,
		Description:         "Pasajero no se presentó al punto acordado.",
		EscrowStatus:        escrow.Status,
	}
	dispute, err := domain.NewDispute(disputeParams)
	assert.NoError(t, err)

	err = dispute.ResolveDriverPayout("admin-1", "Se verificó la espera del conductor.", now)
	assert.NoError(t, err)

	// El escrow en estado DISPUTED ahora puede ser liberado al conductor
	err = escrow.Release(now)
	assert.NoError(t, err)
	assert.Equal(t, domain.EscrowStatusReleased, escrow.Status)
}
