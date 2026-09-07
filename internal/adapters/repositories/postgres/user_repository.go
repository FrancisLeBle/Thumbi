package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type userRepository struct {
	db DBExecutor
}

// NewUserRepository crea una nueva instancia del repositorio PostgreSQL para usuarios
func NewUserRepository(db DBExecutor) ports.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	query := `
		INSERT INTO users (
			id, email, first_name, last_name, avatar_url,
			provider, provider_id, role, kyc_status, is_driver_active,
			created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		string(user.Provider),
		user.ProviderID,
		string(user.Role),
		string(user.KYCStatus),
		user.IsDriverActive,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error inserting user: %w", err)
	}
	return nil
}

func (r *userRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url,
		       provider, provider_id, role, kyc_status, is_driver_active,
		       created_at, updated_at
		FROM users
		WHERE id = $1
	`
	return r.scanUser(r.db.QueryRow(ctx, query, id))
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url,
		       provider, provider_id, role, kyc_status, is_driver_active,
		       created_at, updated_at
		FROM users
		WHERE email = $1
	`
	return r.scanUser(r.db.QueryRow(ctx, query, email))
}

func (r *userRepository) GetByProvider(ctx context.Context, provider domain.AuthProvider, providerID string) (*domain.User, error) {
	query := `
		SELECT id, email, first_name, last_name, avatar_url,
		       provider, provider_id, role, kyc_status, is_driver_active,
		       created_at, updated_at
		FROM users
		WHERE provider = $1 AND provider_id = $2
	`
	return r.scanUser(r.db.QueryRow(ctx, query, string(provider), providerID))
}

func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users
		SET email = $2,
		    first_name = $3,
		    last_name = $4,
		    avatar_url = $5,
		    role = $6,
		    kyc_status = $7,
		    is_driver_active = $8,
		    updated_at = $9
		WHERE id = $1
	`
	tag, err := r.db.Exec(ctx, query,
		user.ID,
		user.Email,
		user.FirstName,
		user.LastName,
		user.AvatarURL,
		string(user.Role),
		string(user.KYCStatus),
		user.IsDriverActive,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error updating user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) scanUser(row pgx.Row) (*domain.User, error) {
	var (
		u              domain.User
		providerStr    string
		roleStr        string
		kycStatusStr   string
	)

	err := row.Scan(
		&u.ID,
		&u.Email,
		&u.FirstName,
		&u.LastName,
		&u.AvatarURL,
		&providerStr,
		&u.ProviderID,
		&roleStr,
		&kycStatusStr,
		&u.IsDriverActive,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, fmt.Errorf("error scanning user row: %w", err)
	}

	u.Provider = domain.AuthProvider(providerStr)
	u.Role = domain.UserRole(roleStr)
	u.KYCStatus = domain.KYCStatus(kycStatusStr)

	return &u, nil
}
