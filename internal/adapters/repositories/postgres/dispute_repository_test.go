package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestDisputeRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("inserta disputa exitosamente con evidencias", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:                  "disp-1",
			EscrowTransactionID: "escrow-101",
			BookingID:           "booking-202",
			TripID:              "trip-303",
			ReporterID:          "passenger-1",
			DefendantID:         "driver-1",
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
				args[0] == "disp-1" &&
				args[1] == "escrow-101" &&
				args[2] == "booking-202" &&
				args[3] == "trip-303" &&
				args[4] == "passenger-1" &&
				args[5] == "driver-1" &&
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
			ID:                  "disp-dup",
			EscrowTransactionID: "escrow-101",
			BookingID:           "booking-202",
			TripID:              "trip-303",
			ReporterID:          "passenger-1",
			DefendantID:         "driver-1",
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
			ID:                  "disp-err",
			EscrowTransactionID: "escrow-999",
			BookingID:           "b-1",
			TripID:              "t-1",
			ReporterID:          "p-1",
			DefendantID:         "d-1",
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

	t.Run("recupera disputa por ID exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		adminNotes := "En revisión por mediación"
		adminID := "admin-1"

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") && strings.Contains(sql, "WHERE id = $1")
		}), []any{"disp-101"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "disp-101"
				*dest[1].(*string) = "escrow-101"
				*dest[2].(*string) = "booking-101"
				*dest[3].(*string) = "trip-101"
				*dest[4].(*string) = "passenger-1"
				*dest[5].(*string) = "driver-1"
				*dest[6].(*string) = "NO_SHOW"
				*dest[7].(*string) = "No se presentó"
				*dest[8].(*[]string) = []string{"https://evidence.example.com/1.jpg"}
				*dest[9].(*string) = "IN_REVIEW"
				*dest[10].(**string) = &adminNotes
				*dest[11].(**string) = &adminID
				*dest[12].(**time.Time) = nil
				*dest[13].(*time.Time) = now
				*dest[14].(*time.Time) = now
				return nil
			},
		})

		disp, err := repo.FindByID(ctx, "disp-101")
		require.NoError(t, err)
		assert.NotNil(t, disp)
		assert.Equal(t, "disp-101", disp.ID)
		assert.Equal(t, "escrow-101", disp.EscrowTransactionID)
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

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"non-existent"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		disp, err := repo.FindByID(ctx, "non-existent")
		assert.ErrorIs(t, err, domain.ErrDisputeNotFound)
		assert.Nil(t, disp)
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_FindByEscrowID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("recupera disputa por EscrowTransactionID exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") &&
				strings.Contains(sql, "WHERE escrow_transaction_id = $1")
		}), []any{"escrow-555"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*string) = "disp-555"
				*dest[1].(*string) = "escrow-555"
				*dest[2].(*string) = "booking-555"
				*dest[3].(*string) = "trip-555"
				*dest[4].(*string) = "passenger-1"
				*dest[5].(*string) = "driver-1"
				*dest[6].(*string) = "RECKLESS_DRIVING"
				*dest[7].(*string) = "Conducción peligrosa"
				*dest[8].(*[]string) = []string{}
				*dest[9].(*string) = "OPENED"
				*dest[10].(**string) = nil
				*dest[11].(**string) = nil
				*dest[12].(**time.Time) = nil
				*dest[13].(*time.Time) = now
				*dest[14].(*time.Time) = now
				return nil
			},
		})

		disp, err := repo.FindByEscrowID(ctx, "escrow-555")
		require.NoError(t, err)
		assert.NotNil(t, disp)
		assert.Equal(t, "disp-555", disp.ID)
		assert.Equal(t, domain.DisputeReasonRecklessDriving, disp.Reason)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeNotFound si no hay disputa para el escrow", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, []any{"escrow-999"}).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		disp, err := repo.FindByEscrowID(ctx, "escrow-999")
		assert.ErrorIs(t, err, domain.ErrDisputeNotFound)
		assert.Nil(t, disp)
		mockDB.AssertExpectations(t)
	})
}

func TestDisputeRepository_Update(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("actualiza estado y resolución de la disputa exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		notes := "Resolución a favor del pasajero"
		adminID := "admin-master"

		dispute := &domain.Dispute{
			ID:         "disp-1",
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
			"disp-1",
		}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.Update(ctx, dispute)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("retorna ErrDisputeNotFound si no se actualizó ninguna fila", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		dispute := &domain.Dispute{
			ID:        "non-existent",
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

	t.Run("retorna listado paginado de disputas", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewDisputeRepository(mockDB)

		notes := "Aguardando descargo"
		mockRows := NewMockRows([][]any{
			{"disp-1", "escrow-1", "b-1", "t-1", "p-1", "d-1", "NO_SHOW", "No vino", []string{}, "OPENED", nil, nil, nil, now, now},
			{"disp-2", "escrow-2", "b-2", "t-2", "p-2", "d-2", "ROUTE_DEVIATION", "Desvío", []string{"http://img.png"}, "IN_REVIEW", &notes, nil, nil, now, now},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM disputes") &&
				strings.Contains(sql, "ORDER BY created_at DESC") &&
				strings.Contains(sql, "LIMIT $1 OFFSET $2")
		}), []any{10, 0}).Return(mockRows, nil)

		disputes, err := repo.List(ctx, 10, 0)
		require.NoError(t, err)
		assert.Len(t, disputes, 2)
		assert.Equal(t, "disp-1", disputes[0].ID)
		assert.Equal(t, domain.DisputeReasonNoShow, disputes[0].Reason)
		assert.Equal(t, "disp-2", disputes[1].ID)
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
