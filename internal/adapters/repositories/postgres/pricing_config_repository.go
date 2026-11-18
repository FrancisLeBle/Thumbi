package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

type pricingConfigRepository struct {
	db DBExecutor
}

// NewPricingConfigRepository crea la implementación PostgreSQL de PricingConfigRepository.
func NewPricingConfigRepository(db DBExecutor) ports.PricingConfigRepository {
	return &pricingConfigRepository{db: db}
}

// GetActive obtiene la configuración activa de tarifas y políticas de cancelación de la plataforma.
// Si no existe fila activa en BD, retorna la configuración estándar por defecto del dominio.
func (r *pricingConfigRepository) GetActive(ctx context.Context) (*domain.PlatformPricingConfig, error) {
	query := `
		SELECT
			id, currency,
			passenger_service_fee_bps, passenger_fixed_fee,
			early_cancel_fee_retention_bps,
			late_cancel_window_hours, late_cancel_fee_retention_bps,
			late_cancel_passenger_refund_bps, late_cancel_driver_comp_bps, late_cancel_platform_retention_bps,
			is_active, updated_by, created_at, updated_at
		FROM platform_pricing_config
		WHERE is_active = TRUE
		LIMIT 1
	`

	var (
		cfg         domain.PlatformPricingConfig
		currencyStr string
		fixedFeeNum pgtype.Numeric
	)

	err := executor(ctx, r.db).QueryRow(ctx, query).Scan(
		&cfg.ID,
		&currencyStr,
		&cfg.PassengerServiceFeeBps,
		&fixedFeeNum,
		&cfg.EarlyCancelFeeRetentionBps,
		&cfg.LateCancelWindowHours,
		&cfg.LateCancelFeeRetentionBps,
		&cfg.LateCancelPassengerRefundBps,
		&cfg.LateCancelDriverCompBps,
		&cfg.LateCancelPlatformRetentionBps,
		&cfg.IsActive,
		&cfg.UpdatedBy,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			def := domain.DefaultPlatformPricingConfig()
			return &def, nil
		}
		return nil, fmt.Errorf("error consultando configuración de precios activa: %w", err)
	}

	cur, err := domain.ParseCurrency(currencyStr)
	if err != nil {
		return nil, fmt.Errorf("error parseando currency en platform_pricing_config: %w", err)
	}
	cfg.Currency = cur

	if cfg.PassengerFixedFee, err = pgNumericToMoney(fixedFeeNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo passenger_fixed_fee: %w", err)
	}

	return &cfg, nil
}

// UpdateActive actualiza (o inicializa si estuviese vacía) la configuración activa de monetización.
func (r *pricingConfigRepository) UpdateActive(ctx context.Context, cfg *domain.PlatformPricingConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	nowUTC := time.Now().UTC()
	if cfg.ID == uuid.Nil {
		cfg.ID = domain.DefaultPlatformPricingConfigID
	}
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = nowUTC
	}
	cfg.UpdatedAt = nowUTC
	cfg.IsActive = true

	updateQuery := `
		UPDATE platform_pricing_config
		SET currency = $1,
		    passenger_service_fee_bps = $2,
		    passenger_fixed_fee = $3,
		    early_cancel_fee_retention_bps = $4,
		    late_cancel_window_hours = $5,
		    late_cancel_fee_retention_bps = $6,
		    late_cancel_passenger_refund_bps = $7,
		    late_cancel_driver_comp_bps = $8,
		    late_cancel_platform_retention_bps = $9,
		    updated_by = $10,
		    updated_at = $11
		WHERE is_active = TRUE
	`

	cmd, err := executor(ctx, r.db).Exec(
		ctx,
		updateQuery,
		cfg.Currency.String(),
		cfg.PassengerServiceFeeBps,
		moneyToPGNumeric(cfg.PassengerFixedFee),
		cfg.EarlyCancelFeeRetentionBps,
		cfg.LateCancelWindowHours,
		cfg.LateCancelFeeRetentionBps,
		cfg.LateCancelPassengerRefundBps,
		cfg.LateCancelDriverCompBps,
		cfg.LateCancelPlatformRetentionBps,
		cfg.UpdatedBy,
		cfg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error actualizando platform_pricing_config: %w", err)
	}

	if cmd.RowsAffected() > 0 {
		return nil
	}

	insertQuery := `
		INSERT INTO platform_pricing_config (
			id, currency,
			passenger_service_fee_bps, passenger_fixed_fee,
			early_cancel_fee_retention_bps,
			late_cancel_window_hours, late_cancel_fee_retention_bps,
			late_cancel_passenger_refund_bps, late_cancel_driver_comp_bps, late_cancel_platform_retention_bps,
			is_active, updated_by, created_at, updated_at
		) VALUES (
			$1, $2,
			$3, $4,
			$5,
			$6, $7,
			$8, $9, $10,
			TRUE, $11, $12, $13
		)
	`

	_, err = executor(ctx, r.db).Exec(
		ctx,
		insertQuery,
		cfg.ID,
		cfg.Currency.String(),
		cfg.PassengerServiceFeeBps,
		moneyToPGNumeric(cfg.PassengerFixedFee),
		cfg.EarlyCancelFeeRetentionBps,
		cfg.LateCancelWindowHours,
		cfg.LateCancelFeeRetentionBps,
		cfg.LateCancelPassengerRefundBps,
		cfg.LateCancelDriverCompBps,
		cfg.LateCancelPlatformRetentionBps,
		cfg.UpdatedBy,
		cfg.CreatedAt,
		cfg.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error insertando configuración activa en platform_pricing_config: %w", err)
	}

	return nil
}
