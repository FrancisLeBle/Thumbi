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

type vehicleRepository struct {
	db DBExecutor
}

// NewVehicleRepository crea una nueva instancia del repositorio PostgreSQL para vehículos
func NewVehicleRepository(db DBExecutor) ports.VehicleRepository {
	return &vehicleRepository{db: db}
}

func (r *vehicleRepository) Create(ctx context.Context, vehicle *domain.Vehicle) error {
	query := `
		INSERT INTO vehicles (
			id, user_id, brand, model, year, plate_number, color, seat_capacity,
			driver_license_url, vehicle_cedula_url, insurance_policy_url,
			status, rejection_reason, verified_at, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	var (
		rejectionReason sql.NullString
		insuranceURL    sql.NullString
		verifiedAt      sql.NullTime
	)

	if vehicle.RejectionReason != "" {
		rejectionReason = sql.NullString{String: vehicle.RejectionReason, Valid: true}
	}
	if vehicle.InsurancePolicyURL != "" {
		insuranceURL = sql.NullString{String: vehicle.InsurancePolicyURL, Valid: true}
	}
	if vehicle.VerifiedAt != nil {
		verifiedAt = sql.NullTime{Time: *vehicle.VerifiedAt, Valid: true}
	}

	_, err := r.db.Exec(ctx, query,
		vehicle.ID,
		vehicle.UserID,
		vehicle.Brand,
		vehicle.Model,
		vehicle.Year,
		vehicle.PlateNumber,
		vehicle.Color,
		vehicle.SeatCapacity,
		vehicle.DriverLicenseURL,
		vehicle.VehicleCedulaURL,
		insuranceURL,
		string(vehicle.Status),
		rejectionReason,
		verifiedAt,
		vehicle.CreatedAt,
		vehicle.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error inserting vehicle: %w", err)
	}
	return nil
}

func (r *vehicleRepository) GetByID(ctx context.Context, id string) (*domain.Vehicle, error) {
	query := `
		SELECT id, user_id, brand, model, year, plate_number, color, seat_capacity,
		       driver_license_url, vehicle_cedula_url, insurance_policy_url,
		       status, rejection_reason, verified_at, created_at, updated_at
		FROM vehicles
		WHERE id = $1
	`
	return r.scanVehicle(r.db.QueryRow(ctx, query, id))
}

func (r *vehicleRepository) GetByUserID(ctx context.Context, userID string) (*domain.Vehicle, error) {
	query := `
		SELECT id, user_id, brand, model, year, plate_number, color, seat_capacity,
		       driver_license_url, vehicle_cedula_url, insurance_policy_url,
		       status, rejection_reason, verified_at, created_at, updated_at
		FROM vehicles
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`
	return r.scanVehicle(r.db.QueryRow(ctx, query, userID))
}

func (r *vehicleRepository) GetByPlateNumber(ctx context.Context, plateNumber string) (*domain.Vehicle, error) {
	query := `
		SELECT id, user_id, brand, model, year, plate_number, color, seat_capacity,
		       driver_license_url, vehicle_cedula_url, insurance_policy_url,
		       status, rejection_reason, verified_at, created_at, updated_at
		FROM vehicles
		WHERE UPPER(plate_number) = UPPER($1)
		LIMIT 1
	`
	return r.scanVehicle(r.db.QueryRow(ctx, query, plateNumber))
}

func (r *vehicleRepository) Update(ctx context.Context, vehicle *domain.Vehicle) error {
	query := `
		UPDATE vehicles
		SET status = $2,
		    rejection_reason = $3,
		    verified_at = $4,
		    updated_at = $5
		WHERE id = $1
	`
	var (
		rejectionReason sql.NullString
		verifiedAt      sql.NullTime
	)

	if vehicle.RejectionReason != "" {
		rejectionReason = sql.NullString{String: vehicle.RejectionReason, Valid: true}
	}
	if vehicle.VerifiedAt != nil {
		verifiedAt = sql.NullTime{Time: *vehicle.VerifiedAt, Valid: true}
	}

	tag, err := r.db.Exec(ctx, query,
		vehicle.ID,
		string(vehicle.Status),
		rejectionReason,
		verifiedAt,
		vehicle.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("error updating vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrVehicleNotFound
	}
	return nil
}

func (r *vehicleRepository) ListPendingVerification(ctx context.Context, limit, offset int) ([]*domain.Vehicle, error) {
	query := `
		SELECT id, user_id, brand, model, year, plate_number, color, seat_capacity,
		       driver_license_url, vehicle_cedula_url, insurance_policy_url,
		       status, rejection_reason, verified_at, created_at, updated_at
		FROM vehicles
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.Query(ctx, query, string(domain.VehicleStatusPendingVerification), limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error querying pending vehicles: %w", err)
	}
	defer rows.Close()

	var list []*domain.Vehicle
	for rows.Next() {
		veh, err := r.scanVehicle(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, veh)
	}
	return list, nil
}

func (r *vehicleRepository) scanVehicle(row pgx.Row) (*domain.Vehicle, error) {
	var (
		v               domain.Vehicle
		statusStr       string
		rejectionReason sql.NullString
		insuranceURL    sql.NullString
		verifiedAt      sql.NullTime
	)

	err := row.Scan(
		&v.ID,
		&v.UserID,
		&v.Brand,
		&v.Model,
		&v.Year,
		&v.PlateNumber,
		&v.Color,
		&v.SeatCapacity,
		&v.DriverLicenseURL,
		&v.VehicleCedulaURL,
		&insuranceURL,
		&statusStr,
		&rejectionReason,
		&verifiedAt,
		&v.CreatedAt,
		&v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrVehicleNotFound
		}
		return nil, fmt.Errorf("error scanning vehicle row: %w", err)
	}

	v.Status = domain.VehicleStatus(statusStr)
	if rejectionReason.Valid {
		v.RejectionReason = rejectionReason.String
	}
	if insuranceURL.Valid {
		v.InsurancePolicyURL = insuranceURL.String
	}
	if verifiedAt.Valid {
		t := verifiedAt.Time
		v.VerifiedAt = &t
	}

	return &v, nil
}
