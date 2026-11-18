package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

type escrowRepository struct {
	db DBExecutor
}

// NewEscrowRepository crea una nueva instancia del repositorio PostgreSQL para custodia (Escrow) y reembolsos
func NewEscrowRepository(db DBExecutor) ports.EscrowRepository {
	return &escrowRepository{db: db}
}

// CreateEscrow persiste el registro inicial de retención de fondos en custodia
func (r *escrowRepository) CreateEscrow(ctx context.Context, escrow *domain.EscrowTransaction) error {
	if !escrow.Status.IsValid() {
		return fmt.Errorf("%w: estado de escrow %s", domain.ErrInvalidStatus, escrow.Status)
	}
	escrowCur := escrow.Currency
	if strings.TrimSpace(string(escrowCur)) == "" {
		escrowCur = escrow.Amount.EffectiveCurrency()
	}
	parsedCur, err := domain.ParseCurrency(string(escrowCur))
	if err != nil {
		return err
	}

	query := `
		INSERT INTO escrow_transactions (
			id, booking_id, trip_id, payer_id, payee_id,
			amount, base_amount, platform_fee, currency, payment_gateway_ref, status,
			held_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11,
			$12, $13, $14
		)
	`

	_, err = executor(ctx, r.db).Exec(
		ctx,
		query,
		escrow.ID,
		escrow.BookingID,
		escrow.TripID,
		escrow.PayerID,
		escrow.PayeeID,
		moneyToPGNumeric(escrow.Amount),
		moneyToPGNumeric(escrow.BaseAmount),
		moneyToPGNumeric(escrow.PlatformFee),
		parsedCur.String(),
		escrow.PaymentGatewayRef,
		escrow.Status.String(),
		escrow.HeldAt,
		escrow.CreatedAt,
		escrow.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error persistiendo transacción en custodia para reserva %s: %w", escrow.BookingID, err)
	}

	return nil
}

// FindByBookingID recupera la transacción de custodia vinculada a una reserva
func (r *escrowRepository) FindByBookingID(ctx context.Context, bookingID uuid.UUID) (*domain.EscrowTransaction, error) {
	if bookingID == uuid.Nil {
		return nil, domain.ErrEscrowNotFound
	}
	query := `
		SELECT 
			id, booking_id, trip_id, payer_id, payee_id,
			amount, base_amount, platform_fee, currency, payment_gateway_ref, status,
			held_at, released_at, refunded_at, created_at, updated_at
		FROM escrow_transactions
		WHERE booking_id = $1
	`

	var (
		e              domain.EscrowTransaction
		amountNum      pgtype.Numeric
		baseAmountNum  pgtype.Numeric
		platformFeeNum pgtype.Numeric
		currencyStr    string
		statusStr      string
	)

	err := executor(ctx, r.db).QueryRow(ctx, query, bookingID).Scan(
		&e.ID,
		&e.BookingID,
		&e.TripID,
		&e.PayerID,
		&e.PayeeID,
		&amountNum,
		&baseAmountNum,
		&platformFeeNum,
		&currencyStr,
		&e.PaymentGatewayRef,
		&statusStr,
		&e.HeldAt,
		&e.ReleasedAt,
		&e.RefundedAt,
		&e.CreatedAt,
		&e.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrEscrowNotFound
		}
		return nil, fmt.Errorf("error consultando escrow para reserva %s: %w", bookingID, err)
	}

	cur, err := domain.ParseCurrency(currencyStr)
	if err != nil {
		return nil, fmt.Errorf("error parseando currency de escrow: %w", err)
	}
	e.Currency = cur

	if e.Amount, err = pgNumericToMoney(amountNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo monto de escrow: %w", err)
	}
	if e.BaseAmount, err = pgNumericToMoney(baseAmountNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo base_amount de escrow: %w", err)
	}
	if e.PlatformFee, err = pgNumericToMoney(platformFeeNum, cur); err != nil {
		return nil, fmt.Errorf("error convirtiendo platform_fee de escrow: %w", err)
	}

	e.Status = domain.EscrowStatus(statusStr)
	if !e.Status.IsValid() {
		return nil, fmt.Errorf("%w: estado de escrow inválido %s", domain.ErrInvalidStatus, statusStr)
	}
	return &e, nil
}

// UpdateEscrowStatus actualiza el estado de la custodia y las marcas temporales de liberación o reembolso
func (r *escrowRepository) UpdateEscrowStatus(ctx context.Context, escrowID uuid.UUID, newStatus domain.EscrowStatus) error {
	if escrowID == uuid.Nil {
		return domain.ErrEscrowNotFound
	}
	if !newStatus.IsValid() {
		return fmt.Errorf("%w: estado de escrow %s", domain.ErrInvalidStatus, newStatus)
	}
	query := `
		UPDATE escrow_transactions
		SET status = $1,
		    released_at = CASE WHEN $1 = 'RELEASED' THEN NOW() ELSE released_at END,
		    refunded_at = CASE WHEN $1 IN ('REFUNDED_FULL', 'REFUNDED_PARTIAL') THEN NOW() ELSE refunded_at END,
		    updated_at = NOW()
		WHERE id = $2
	`

	cmd, err := executor(ctx, r.db).Exec(ctx, query, newStatus.String(), escrowID)
	if err != nil {
		return fmt.Errorf("error actualizando estado de custodia %s: %w", escrowID, err)
	}

	if cmd.RowsAffected() == 0 {
		return domain.ErrEscrowNotFound
	}

	return nil
}

// RecordRefund registra de forma inmutable la trazabilidad contable del reembolso
func (r *escrowRepository) RecordRefund(ctx context.Context, refund *domain.RefundTransaction) error {
	if !refund.RefundType.IsValid() {
		return fmt.Errorf("%w: tipo de reembolso %s", domain.ErrInvalidStatus, refund.RefundType)
	}
	query := `
		INSERT INTO refund_transactions (
			id, escrow_transaction_id, booking_id,
			passenger_refund_amount, driver_compensation_amount, platform_retained_amount,
			refund_type, gateway_refund_ref,
			processed_at, created_at
		) VALUES (
			$1, $2, $3,
			$4, $5, $6,
			$7, $8,
			$9, $10
		)
	`

	_, err := executor(ctx, r.db).Exec(
		ctx,
		query,
		refund.ID,
		refund.EscrowTransactionID,
		refund.BookingID,
		moneyToPGNumeric(refund.PassengerRefundAmount),
		moneyToPGNumeric(refund.DriverCompensationAmount),
		moneyToPGNumeric(refund.PlatformRetainedAmount),
		refund.RefundType.String(),
		refund.GatewayRefundRef,
		refund.ProcessedAt,
		refund.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("error registrando reembolso para reserva %s: %w", refund.BookingID, err)
	}

	return nil
}
