package postgres_test

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestPricingConfigRepository(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	adminID := uuid.MustParse("01960000-0000-7000-8000-000000000900")

	t.Run("GetActive retorna la configuración activa almacenada en BD", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewPricingConfigRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM platform_pricing_config") &&
				strings.Contains(sql, "WHERE is_active = TRUE")
		}), []any(nil)).Return(&MockRow{
			scanFn: func(dest ...any) error {
				*dest[0].(*uuid.UUID) = domain.DefaultPlatformPricingConfigID
				*dest[1].(*string) = "ARS"
				*dest[2].(*int64) = 1200
				*dest[3].(*pgtype.Numeric) = pgtype.Numeric{Int: big.NewInt(5000), Exp: -2, Valid: true}
				*dest[4].(*int64) = 10000
				*dest[5].(*int) = 24
				*dest[6].(*int64) = 10000
				*dest[7].(*int64) = 4000
				*dest[8].(*int64) = 4000
				*dest[9].(*int64) = 2000
				*dest[10].(*bool) = true
				*dest[11].(**uuid.UUID) = &adminID
				*dest[12].(*time.Time) = now
				*dest[13].(*time.Time) = now
				return nil
			},
		})

		cfg, err := repo.GetActive(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1200), cfg.PassengerServiceFeeBps)
		assert.Equal(t, domain.MoneyFromCents(5000, domain.CurrencyARS), cfg.PassengerFixedFee)
		assert.Equal(t, int64(2000), cfg.LateCancelPlatformRetentionBps)
		assert.Equal(t, &adminID, cfg.UpdatedBy)
	})

	t.Run("GetActive retorna configuración por defecto si la tabla aún está vacía", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewPricingConfigRepository(mockDB)

		mockDB.On("QueryRow", ctx, mock.Anything, mock.Anything).Return(&MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		})

		cfg, err := repo.GetActive(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1500), cfg.PassengerServiceFeeBps)
		assert.Equal(t, 24, cfg.LateCancelWindowHours)
	})

	t.Run("UpdateActive actualiza la configuración activa existente sin hacer INSERT redundante", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewPricingConfigRepository(mockDB)

		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 1800
		cfg.UpdatedBy = &adminID

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE platform_pricing_config") &&
				strings.Contains(sql, "WHERE is_active = TRUE")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		err := repo.UpdateActive(ctx, &cfg)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})

	t.Run("UpdateActive rechaza configuración inválida antes de tocar la BD", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewPricingConfigRepository(mockDB)

		cfg := domain.DefaultPlatformPricingConfig()
		cfg.PassengerServiceFeeBps = 9999 // > MaxPassengerServiceFeeBps

		err := repo.UpdateActive(ctx, &cfg)
		assert.ErrorIs(t, err, domain.ErrInvalidPricingConfig)
		mockDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	})
}
