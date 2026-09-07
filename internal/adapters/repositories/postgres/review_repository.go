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

// reviewRepository implementa ports.ReviewRepository interactuando con PostgreSQL
type reviewRepository struct {
	db DBExecutor
}

// NewReviewRepository inicializa una nueva instancia del repositorio PostgreSQL para reseñas
func NewReviewRepository(db DBExecutor) ports.ReviewRepository {
	return &reviewRepository{db: db}
}

// Save inserta una nueva reseña en la tabla reviews.
// Nota: El disparador de base de datos 'trg_reviews_update_reputation' se encarga de
// actualizar de manera atómica rating_avg y rating_count en la tabla users.
func (r *reviewRepository) Save(ctx context.Context, review *domain.Review) error {
	query := `
		INSERT INTO reviews (
			id, trip_id, booking_id, reviewer_id, reviewee_id,
			rating, comment, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
	`

	var comment *string
	if review.Comment != "" {
		comment = &review.Comment
	}

	_, err := r.db.Exec(
		ctx,
		query,
		review.ID,
		review.TripID,
		review.BookingID,
		review.ReviewerID,
		review.RevieweeID,
		review.Rating,
		comment,
		review.CreatedAt,
		review.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrDuplicateReview
		}
		return fmt.Errorf("error al insertar reseña: %w", err)
	}

	return nil
}

// FindByID recupera una calificación por su identificador único
func (r *reviewRepository) FindByID(ctx context.Context, id string) (*domain.Review, error) {
	query := `
		SELECT 
			id, trip_id, booking_id, reviewer_id, reviewee_id,
			rating, comment, created_at, updated_at
		FROM reviews
		WHERE id = $1
	`

	var rev domain.Review
	var comment *string

	err := r.db.QueryRow(ctx, query, id).Scan(
		&rev.ID,
		&rev.TripID,
		&rev.BookingID,
		&rev.ReviewerID,
		&rev.RevieweeID,
		&rev.Rating,
		&comment,
		&rev.CreatedAt,
		&rev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReviewNotFound
		}
		return nil, fmt.Errorf("error consultando reseña con id %s: %w", id, err)
	}

	if comment != nil {
		rev.Comment = *comment
	}

	return &rev, nil
}

// FindByBookingAndReviewer consulta una reseña específica por reserva y usuario emisor
func (r *reviewRepository) FindByBookingAndReviewer(ctx context.Context, bookingID, reviewerID string) (*domain.Review, error) {
	query := `
		SELECT 
			id, trip_id, booking_id, reviewer_id, reviewee_id,
			rating, comment, created_at, updated_at
		FROM reviews
		WHERE booking_id = $1 AND reviewer_id = $2
	`

	var rev domain.Review
	var comment *string

	err := r.db.QueryRow(ctx, query, bookingID, reviewerID).Scan(
		&rev.ID,
		&rev.TripID,
		&rev.BookingID,
		&rev.ReviewerID,
		&rev.RevieweeID,
		&rev.Rating,
		&comment,
		&rev.CreatedAt,
		&rev.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrReviewNotFound
		}
		return nil, fmt.Errorf("error consultando reseña por reserva %s y evaluador %s: %w", bookingID, reviewerID, err)
	}

	if comment != nil {
		rev.Comment = *comment
	}

	return &rev, nil
}

// ListByUserID obtiene el listado paginado de reseñas recibidas por un usuario (reviewee_id = $1), ordenadas por created_at DESC
func (r *reviewRepository) ListByUserID(ctx context.Context, userID string, limit, offset int) ([]*domain.Review, error) {
	query := `
		SELECT 
			id, trip_id, booking_id, reviewer_id, reviewee_id,
			rating, comment, created_at, updated_at
		FROM reviews
		WHERE reviewee_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("error consultando lista de reseñas para usuario %s: %w", userID, err)
	}
	defer rows.Close()

	reviews := make([]*domain.Review, 0)
	for rows.Next() {
		var rev domain.Review
		var comment *string

		if err := rows.Scan(
			&rev.ID,
			&rev.TripID,
			&rev.BookingID,
			&rev.ReviewerID,
			&rev.RevieweeID,
			&rev.Rating,
			&comment,
			&rev.CreatedAt,
			&rev.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("error escaneando fila de reseña: %w", err)
		}

		if comment != nil {
			rev.Comment = *comment
		}

		reviews = append(reviews, &rev)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterando filas de reseñas: %w", err)
	}

	return reviews, nil
}

// GetAverageRating ejecuta una consulta agregada directa (AVG(rating), COUNT(*)) como fallback o verificación independiente
func (r *reviewRepository) GetAverageRating(ctx context.Context, userID string) (float64, int, error) {
	query := `
		SELECT 
			COALESCE(ROUND(AVG(rating)::numeric, 2), 5.00),
			COUNT(*)
		FROM reviews
		WHERE reviewee_id = $1
	`

	var avg float64
	var count int

	err := r.db.QueryRow(ctx, query, userID).Scan(&avg, &count)
	if err != nil {
		return 5.00, 0, fmt.Errorf("error obteniendo promedio de calificaciones para usuario %s: %w", userID, err)
	}

	if count == 0 {
		avg = 5.00
	}

	return avg, count, nil
}
