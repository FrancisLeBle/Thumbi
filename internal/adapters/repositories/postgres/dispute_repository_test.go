package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestDisputeRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dispID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	dispDupID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	dispErrID := uuid.MustParse("01933e3d-0000-7000-8000-000000000003")
	escrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	escrowErrID := uuid.MustParse("01933e3d-0000-7000-8000-000000000999")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000202")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000303")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000401")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000501")

	t.Run("inserta disputa exitosamente con evidencias", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:                  dispID,
			EscrowTransactionID: escrowID,
			BookingID:           bookingID,
			TripID:              tripID,
			ReporterID:          passengerID,
			DefendantID:         driverID,
			Reason:              domain.DisputeReasonNoShow,
			Description:         "El conductor nunca llegó al punto de encuentro acordado.",
			EvidenceURLs:        []string{"https://s3.amazonaws.com/evidence/chat.png"},
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO disputes") &&
				strings.Contains(sql, "escrow_transaction_id") &&
				strings.Contains(sql, "evidence_urls")
		}), mock.MatchedBy(func(args []any) bool {
			return len(args) == 15 &&
				args[0] == dispID &&
				args[1] == escrowID &&
				args[2] == bookingID &&
				args[3] == tripID &&
				args[4] == passengerID &&
				args[5] == driverID &&
				args[6] == "NO_SHOW" &&
				args[7] == "El conductor nunca llegó al punto de encuentro acordado." &&
				len(args[8].([]string)) == 1 &&
				args[9] == "OPENED"
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		err := repo.Save(ctx, dispute)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeAlreadyExists si ya existe disputa sobre el escrow", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:                  dispDupID,
			EscrowTransactionID: escrowID,
			BookingID:           bookingID,
			TripID:              tripID,
			ReporterID:          passengerID,
			DefendantID:         driverID,
			Reason:              domain.DisputeReasonRouteDeviation,
			Description:         "Desvío injustificado",
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		pgErr := &pgconn.PgError{Code: "23505"} // unique_violation
		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, pgErr)

		err := repo.Save(ctx, dispute)
		assert.ErrorIs(t, err, domain.ErrDisputeAlreadyExists)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna error ante fallo genérico de base de datos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:                  dispErrID,
			EscrowTransactionID: escrowErrID,
			BookingID:           bookingID,
			TripID:              tripID,
			ReporterID:          passengerID,
			DefendantID:         driverID,
			Reason:              domain.DisputeReasonOther,
			Description:         "Error",
			Status:              domain.DisputeStatusOpened,
			CreatedAt:           now,
			UpdatedAt:           now,
		}

		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.CommandTag{}, errors.New("connection reset"))

		err := repo.Save(ctx, dispute)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error al insertar disputa")
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dispID := uuid.MustParse("01933e3d-0000-7000-8000-000000000101")
	escrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000102")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000103")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000104")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000105")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000106")
	adminID := uuid.MustParse("01933e3d-0000-7000-8000-000000000900")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	t.Run("recupera disputa por ID exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		adminNotes := "En revisión por mediación"

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") && strings.Contains(sql, "WHERE id = $1")
		}), []any{dispID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = dispID
				*dest[1].(*uuid.UUID) = escrowID
				*dest[2].(*uuid.UUID) = bookingID
				*dest[3].(*uuid.UUID) = tripID
				*dest[4].(*uuid.UUID) = passengerID
				*dest[5].(*uuid.UUID) = driverID
				*dest[6].(*string) = "NO_SHOW"
				*dest[7].(*string) = "No se presentó"
				*dest[8].(*[]string) = []string{"https://evidence.example.com/1.jpg"}
				*dest[9].(*string) = "IN_REVIEW"
				*dest[10].(**string) = &adminNotes
				*dest[11].(**uuid.UUID) = &adminID
				*dest[12].(**time.Time) = nil
				*dest[13].(*time.Time) = now
				*dest[14].(*time.Time) = now
				return nil
			},
		})

		disp, err := repo.FindByID(ctx, dispID)
		require.NoError(t, err)
		assert.NotNil(t, disp)
		assert.Equal(t, dispID, disp.ID)
		assert.Equal(t, escrowID, disp.EscrowTransactionID)
		assert.Equal(t, domain.DisputeReasonNoShow, disp.Reason)
		assert.Equal(t, domain.DisputeStatusInReview, disp.Status)
		assert.Equal(t, &adminNotes, disp.AdminNotes)
		assert.Equal(t, &adminID, disp.ResolvedBy)
		assert.Len(t, disp.EvidenceURLs, 1)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeNotFound si la disputa no existe", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{missingID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		disp, err := repo.FindByID(ctx, missingID)
		assert.ErrorIs(t, err, domain.ErrDisputeNotFound)
		assert.Nil(t, disp)
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_FindByEscrowID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dispID := uuid.MustParse("01933e3d-0000-7000-8000-000000000555")
	escrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000556")
	bookingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000557")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000558")
	passengerID := uuid.MustParse("01933e3d-0000-7000-8000-000000000559")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000560")
	missingEscrowID := uuid.MustParse("01933e3d-0000-7000-8000-000000000999")

	t.Run("recupera disputa por EscrowTransactionID exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") &&
				strings.Contains(sql, "WHERE escrow_transaction_id = $1")
		}), []any{escrowID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = dispID
				*dest[1].(*uuid.UUID) = escrowID
				*dest[2].(*uuid.UUID) = bookingID
				*dest[3].(*uuid.UUID) = tripID
				*dest[4].(*uuid.UUID) = passengerID
				*dest[5].(*uuid.UUID) = driverID
				*dest[6].(*string) = "RECKLESS_DRIVING"
				*dest[7].(*string) = "Conducción peligrosa"
				*dest[8].(*[]string) = []string{}
				*dest[9].(*string) = "OPENED"
				*dest[10].(**string) = nil
				*dest[11].(**uuid.UUID) = nil
				*dest[12].(**time.Time) = nil
				*dest[13].(*time.Time) = now
				*dest[14].(*time.Time) = now
				return nil
			},
		})

		disp, err := repo.FindByEscrowID(ctx, escrowID)
		require.NoError(t, err)
		assert.NotNil(t, disp)
		assert.Equal(t, dispID, disp.ID)
		assert.Equal(t, domain.DisputeReasonRecklessDriving, disp.Reason)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeNotFound si no hay disputa para el escrow", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{missingEscrowID}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		disp, err := repo.FindByEscrowID(ctx, missingEscrowID)
		assert.ErrorIs(t, err, domain.ErrDisputeNotFound)
		assert.Nil(t, disp)
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_Update(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	dispID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")
	adminID := uuid.MustParse("01933e3d-0000-7000-8000-000000000900")

	t.Run("actualiza estado y resolución de la disputa exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		notes := "Resolución a favor del pasajero"

		dispute := &domain.Dispute{
			ID:         dispID,
			Status:     domain.DisputeStatusResolvedPassengerRefund,
			AdminNotes: &notes,
			ResolvedBy: &adminID,
			ResolvedAt: &now,
			UpdatedAt:  now,
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE disputes") &&
				strings.Contains(sql, "SET status = $1") &&
				strings.Contains(sql, "WHERE id = $6")
		}), []any{
			"RESOLVED_PASSENGER_REFUND",
			&notes,
			&adminID,
			&now,
			now,
			dispID,
		}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.Update(ctx, dispute)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeNotFound si no se actualizó ninguna fila", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:        missingID,
			Status:    domain.DisputeStatusRejected,
			UpdatedAt: now,
		}

		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		err := repo.Update(ctx, dispute)
		assert.ErrorIs(t, err, domain.ErrDisputeNotFound)
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_List(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	disp1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	disp2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	esc1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000011")
	esc2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000012")
	b1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000021")
	b2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000022")
	t1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000031")
	t2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000032")
	p1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000041")
	p2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000042")
	d1ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000051")
	d2ID := uuid.MustParse("01933e3d-0000-7000-8000-000000000052")

	t.Run("retorna listado paginado de disputas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		notes := "Aguardando descargo"
		mockRows := NewMockRows([][]any{
			{disp1ID, esc1ID, b1ID, t1ID, p1ID, d1ID, "NO_SHOW", "No vino", []string{}, "OPENED", nil, nil, nil, now, now},
			{disp2ID, esc2ID, b2ID, t2ID, p2ID, d2ID, "ROUTE_DEVIATION", "Desvío", []string{"http://img.png"}, "IN_REVIEW", &notes, nil, nil, now, now},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") &&
				strings.Contains(sql, "ORDER BY created_at DESC") &&
				strings.Contains(sql, "LIMIT $1 OFFSET $2")
		}), []any{10, 0}).Return(mockRows, nil)

		disputes, err := repo.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, disputes, 2)
		assert.Equal(t, disp1ID, disputes[0].ID)
		assert.Equal(t, domain.DisputeReasonNoShow, disputes[0].Reason)
		assert.Equal(t, disp2ID, disputes[1].ID)
		assert.Equal(t, domain.DisputeReasonRouteDeviation, disputes[1].Reason)
		assert.Equal(t, &notes, disputes[1].AdminNotes)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna lista vacía si no hay registros", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.Anything, []any{10, 0}).Return(mockRows, nil)

		disputes, err := repo.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.Empty(t, disputes)
		mockDB.AssertExpectations(t)
	})
}
