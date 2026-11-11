package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type identityRepository struct {
	db DBExecutor
}

// NewIdentityRepository crea una nueva instancia del repositorio PostgreSQL para identidades federadas
func NewIdentityRepository(db DBExecutor) ports.IdentityRepository {
	return &identityRepository{db: db}
}

func (r *identityRepository) Create(ctx context.Context, identity *domain.UserIdentity) error {
	query := `
		INSERT INTO user_identities (
			id, user_id, provider, provider_user_id, email, email_verified,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.db.Exec(ctx, query,
		identity.ID,
		identity.UserID,
		string(identity.Provider),
		identity.ProviderUserID,
		identity.Email,
		identity.EmailVerified,
		identity.CreatedAt,
		identity.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error inserting user identity: %w", err)
	}
	return nil
}

func (r *identityRepository) GetByID(ctx context.Context, id string) (*domain.UserIdentity, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, email, email_verified,
		       created_at, updated_at
		FROM user_identities
		WHERE id = $1
	`
	return r.scanIdentity(r.db.QueryRow(ctx, query, id))
}

func (r *identityRepository) GetByProviderAndID(ctx context.Context, provider domain.AuthProvider, providerUserID string) (*domain.UserIdentity, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, email, email_verified,
		       created_at, updated_at
		FROM user_identities
		WHERE provider = $1 AND provider_user_id = $2
	`
	return r.scanIdentity(r.db.QueryRow(ctx, query, string(provider), providerUserID))
}

func (r *identityRepository) ListByUserID(ctx context.Context, userID string) ([]*domain.UserIdentity, error) {
	query := `
		SELECT id, user_id, provider, provider_user_id, email, email_verified,
		       created_at, updated_at
		FROM user_identities
		WHERE user_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("error querying user identities: %w", err)
	}
	defer rows.Close()

	var identities []*domain.UserIdentity
	for rows.Next() {
		var (
			ident       domain.UserIdentity
			providerStr string
		)
		if err := rows.Scan(
			&ident.ID,
			&ident.UserID,
			&providerStr,
			&ident.ProviderUserID,
			&ident.Email,
			&ident.EmailVerified,
			&ident.CreatedAt,
			&ident.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error scanning user identity row: %w", err)
		}
		ident.Provider = domain.AuthProvider(providerStr)
		identities = append(identities, &ident)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating user identities rows: %w", err)
	}

	return identities, nil
}

func (r *identityRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM user_identities WHERE id = $1`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("error deleting user identity: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrIdentityNotFound
	}
	return nil
}

func (r *identityRepository) scanIdentity(row pgx.Row) (*domain.UserIdentity, error) {
	var (
		ident       domain.UserIdentity
		providerStr string
	)

	err := row.Scan(
		&ident.ID,
		&ident.UserID,
		&providerStr,
		&ident.ProviderUserID,
		&ident.Email,
		&ident.EmailVerified,
		&ident.CreatedAt,
		&ident.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrIdentityNotFound
		}
		return nil, fmt.Errorf("error scanning user identity: %w", err)
	}

	ident.Provider = domain.AuthProvider(providerStr)
	return &ident, nil
}
