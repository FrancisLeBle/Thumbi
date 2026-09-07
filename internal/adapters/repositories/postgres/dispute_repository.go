package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// disputeRepository implementa ports.DisputeRepository interactuando con PostgreSQL
type disputeRepository struct {
	db DBExecutor
}

// NewDisputeRepository crea una nueva instancia del repositorio PostgreSQL para disputas
func NewDisputeRepository(db DBExecutor) ports.DisputeRepository {
	return &disputeRepository{db: db}
}

// Save registra una nueva reclamación formal en la tabla disputes
func (r *disputeRepository) Save(ctx context.Context, dispute *domain.Dispute) error {
	query := `
		INSERT INTO disputes (
			id, escrow_transaction_id, booking_id, trip_id,
			reporter_id, defendant_id, reason, description,
			evidence_urls, status, admin_notes, resolved_by,
			resolved_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7, $8,
			$9, $10, $11, $12,
			$13, $14, $15
		)
	`

	evidence := dispute.EvidenceURLs
	if evidence == nil {
		evidence = []string{}
	}

	_, err := r.db.Exec(
		ctx,
		query,
		dispute.ID,
		dispute.EscrowTransactionID,
		dispute.BookingID,
		dispute.TripID,
		dispute.ReporterID,
		dispute.DefendantID,
		string(dispute.Reason),
		dispute.Description,
		evidence,
		string(dispute.Status),
		dispute.AdminNotes,
		dispute.ResolvedBy,
		dispute.ResolvedAt,
		dispute.CreatedAt,
		dispute.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDisputeAlreadyExists
		}
		return fmt.Errorf("error al insertar disputa: %w", err)
	}

	return nil
}

// FindByID recupera una disputa por su identificador único
func (r *disputeRepository) FindByID(ctx context.Context, id string) (*domain.Dispute, error) {
	query := `
		SELECT 
			id, escrow_transaction_id, booking_id, trip_id,
			reporter_id, defendant_id, reason, description,
			evidence_urls, status, admin_notes, resolved_by,
			resolved_at, created_at, updated_at
		FROM disputes
		WHERE id = $1
	`

	return r.scanDispute(r.db.QueryRow(ctx, query, id))
}

// FindByEscrowID recupera la disputa asociada a una transacción de fondos en custodia específica
func (r *disputeRepository) FindByEscrowID(ctx context.Context, escrowID string) (*domain.Dispute, error) {
	query := `
		SELECT 
			id, escrow_transaction_id, booking_id, trip_id,
			reporter_id, defendant_id, reason, description,
			evidence_urls, status, admin_notes, resolved_by,
			resolved_at, created_at, updated_at
		FROM disputes
		WHERE escrow_transaction_id = $1
	`

	return r.scanDispute(r.db.QueryRow(ctx, query, escrowID))
}

// Update persiste los cambios de estado, notas administrativas y resolución de la disputa
func (r *disputeRepository) Update(ctx context.Context, dispute *domain.Dispute) error {
	query := `
		UPDATE disputes
		SET status = $1,
		    admin_notes = $2,
		    resolved_by = $3,
		    resolved_at = $4,
		    updated_at = $5
		WHERE id = $6
	`

	tag, err := r.db.Exec(
		ctx,
		query,
		string(dispute.Status),
		dispute.AdminNotes,
		dispute.ResolvedBy,
		dispute.ResolvedAt,
		dispute.UpdatedAt,
		dispute.ID,
	)
	if err != nil {
		return fmt.Errorf("error al actualizar disputa con id %s: %w", dispute.ID, err)
	}

	if tag.RowsAffected() == 0 {
		return domain.ErrDisputeNotFound
	}

	return nil
}

// List consulta el catálogo de disputas de forma paginada para la mesa de mediación o soporte
func (r *disputeRepository) List(ctx context.Context, limit, offset int) ([]*domain.Dispute, error) {
	query := `
		SELECT 
			id, escrow_transaction_id, booking_id, trip_id,
			reporter_id, defendant_id, reason, description,
			evidence_urls, status, admin_notes, resolved_by,
			resolved_at, created_at, updated_at
		FROM disputes
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error listando disputas: %w", err)
	}
	defer rows.Close()

	disputes := make([]*domain.Dispute, 0)
	for rows.Next() {
		var d domain.Dispute
		var reasonStr, statusStr string

		if err := rows.Scan(
			&d.ID,
			&d.EscrowTransactionID,
			&d.BookingID,
			&d.TripID,
			&d.ReporterID,
			&d.DefendantID,
			&reasonStr,
			&d.Description,
			&d.EvidenceURLs,
			&statusStr,
			&d.AdminNotes,
			&d.ResolvedBy,
			&d.ResolvedAt,
			&d.CreatedAt,
			&d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando fila de disputa: %w", err)
		}

		d.Reason = domain.DisputeReason(reasonStr)
		d.Status = domain.DisputeStatus(statusStr)
		if d.EvidenceURLs == nil {
			d.EvidenceURLs = []string{}
		}

		disputes = append(disputes, &d)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando filas de disputas: %w", err)
	}

	return disputes, nil
}

// scanDispute es una función auxiliar para el mapeo de una fila única a domain.Dispute
func (r *disputeRepository) scanDispute(row pgx.Row) (*domain.Dispute, error) {
	var d domain.Dispute
	var reasonStr, statusStr string

	err := row.Scan(
		&d.ID,
		&d.EscrowTransactionID,
		&d.BookingID,
		&d.TripID,
		&d.ReporterID,
		&d.DefendantID,
		&reasonStr,
		&d.Description,
		&d.EvidenceURLs,
		&statusStr,
		&d.AdminNotes,
		&d.ResolvedBy,
		&d.ResolvedAt,
		&d.CreatedAt,
		&d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDisputeNotFound
		}
		return nil, fmt.Errorf("error escaneando datos de la disputa: %w", err)
	}

	d.Reason = domain.DisputeReason(reasonStr)
	d.Status = domain.DisputeStatus(statusStr)
	if d.EvidenceURLs == nil {
		d.EvidenceURLs = []string{}
	}

	return &d, nil
}
