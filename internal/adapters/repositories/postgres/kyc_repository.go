package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type kycRepository struct {
	db DBExecutor
}

// NewKYCRepository crea una nueva instancia del repositorio PostgreSQL para verificaciones KYC
func NewKYCRepository(db DBExecutor) ports.KYCRepository {
	return &kycRepository{db: db}
}

func (r *kycRepository) Create(ctx context.Context, kyc *domain.KYCVerification) error {
	query := `
		INSERT INTO kyc_verifications (
			id, user_id, document_type, document_number,
			front_image_url, back_image_url, selfie_3d_url,
			liveness_score, retry_count, status, rejection_reason,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	var rejectionReason sql.NullString
	if kyc.RejectionReason != "" {
		rejectionReason = sql.NullString{String: kyc.RejectionReason, Valid: true}
	}

	_, err := r.db.Exec(ctx, query,
		kyc.ID,
		kyc.UserID,
		kyc.DocumentType,
		kyc.DocumentNumber,
		kyc.FrontImageURL,
		kyc.BackImageURL,
		kyc.Selfie3DURL,
		kyc.LivenessScore,
		kyc.RetryCount,
		string(kyc.Status),
		rejectionReason,
		kyc.CreatedAt,
		kyc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error inserting kyc verification: %w", err)
	}
	return nil
}

func (r *kycRepository) GetByID(ctx context.Context, id string) (*domain.KYCVerification, error) {
	query := `
		SELECT id, user_id, document_type, document_number,
		       front_image_url, back_image_url, selfie_3d_url,
		       liveness_score, retry_count, status, rejection_reason,
		       created_at, updated_at
		FROM kyc_verifications
		WHERE id = $1
	`
	return r.scanKYC(r.db.QueryRow(ctx, query, id))
}

func (r *kycRepository) GetLatestByUserID(ctx context.Context, userID string) (*domain.KYCVerification, error) {
	query := `
		SELECT id, user_id, document_type, document_number,
		       front_image_url, back_image_url, selfie_3d_url,
		       liveness_score, retry_count, status, rejection_reason,
		       created_at, updated_at
		FROM kyc_verifications
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanKYC(r.db.QueryRow(ctx, query, userID))
}

func (r *kycRepository) Update(ctx context.Context, kyc *domain.KYCVerification) error {
	query := `
		UPDATE kyc_verifications
		SET liveness_score = $2,
		    retry_count = $3,
		    status = $4,
		    rejection_reason = $5,
		    updated_at = $6
		WHERE id = $1
	`
	var rejectionReason sql.NullString
	if kyc.RejectionReason != "" {
		rejectionReason = sql.NullString{String: kyc.RejectionReason, Valid: true}
	}

	tag, err := r.db.Exec(ctx, query,
		kyc.ID,
		kyc.LivenessScore,
		kyc.RetryCount,
		string(kyc.Status),
		rejectionReason,
		kyc.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error updating kyc verification: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *kycRepository) scanKYC(row pgx.Row) (*domain.KYCVerification, error) {
	var (
		k               domain.KYCVerification
		statusStr       string
		rejectionReason sql.NullString
	)

	err := row.Scan(
		&k.ID,
		&k.UserID,
		&k.DocumentType,
		&k.DocumentNumber,
		&k.FrontImageURL,
		&k.BackImageURL,
		&k.Selfie3DURL,
		&k.LivenessScore,
		&k.RetryCount,
		&statusStr,
		&rejectionReason,
		&k.CreatedAt,
		&k.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error scanning kyc row: %w", err)
	}

	k.Status = domain.KYCStatus(statusStr)
	if rejectionReason.Valid {
		k.RejectionReason = rejectionReason.String
	}

	return &k, nil
}
