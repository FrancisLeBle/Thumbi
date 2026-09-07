package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
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
	query := `
		INSERT INTO escrow_transactions (
			id, booking_id, trip_id, payer_id, payee_id,
			amount, currency, payment_gateway_ref, status,
			held_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12
		)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		escrow.ID,
		escrow.BookingID,
		escrow.TripID,
		escrow.PayerID,
		escrow.PayeeID,
		escrow.Amount,
		escrow.Currency,
		escrow.PaymentGatewayRef,
		string(escrow.Status),
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
func (r *escrowRepository) FindByBookingID(ctx context.Context, bookingID string) (*domain.EscrowTransaction, error) {
	query := `
		SELECT 
			id, booking_id, trip_id, payer_id, payee_id,
			amount, currency, payment_gateway_ref, status,
			held_at, released_at, refunded_at, created_at, updated_at
		FROM escrow_transactions
		WHERE booking_id = $1
	`

	var e domain.EscrowTransaction
	var statusStr string

	err := r.db.QueryRow(ctx, query, bookingID).Scan(
		&e.ID,
		&e.BookingID,
		&e.TripID,
		&e.PayerID,
		&e.PayeeID,
		&e.Amount,
		&e.Currency,
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

	e.Status = domain.EscrowStatus(statusStr)
	return &e, nil
}

// UpdateEscrowStatus actualiza el estado de la custodia y las marcas temporales de liberación o reembolso
func (r *escrowRepository) UpdateEscrowStatus(ctx context.Context, escrowID string, newStatus domain.EscrowStatus) error {
	query := `
		UPDATE escrow_transactions
		SET status = $1,
		    released_at = CASE WHEN $1 = 'RELEASED' THEN NOW() ELSE released_at END,
		    refunded_at = CASE WHEN $1 IN ('REFUNDED_FULL', 'REFUNDED_PARTIAL') THEN NOW() ELSE refunded_at END,
		    updated_at = NOW()
		WHERE id = $2
	`

	cmd, err := r.db.Exec(ctx, query, string(newStatus), escrowID)
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
	query := `
		INSERT INTO refund_transactions (
			id, escrow_transaction_id, booking_id,
			passenger_refund_amount, driver_compensation_amount,
			refund_type, gateway_refund_ref,
			processed_at, created_at
		) VALUES (
			$1, $2, $3,
			$4, $5,
			$6, $7,
			$8, $9
		)
	`

	_, err := r.db.Exec(
		ctx,
		query,
		refund.ID,
		refund.EscrowTransactionID,
		refund.BookingID,
		refund.PassengerRefundAmount,
		refund.DriverCompensationAmount,
		string(refund.RefundType),
		refund.GatewayRefundRef,
		refund.ProcessedAt,
		refund.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("error registrando reembolso para reserva %s: %w", refund.BookingID, err)
	}

	return nil
}
