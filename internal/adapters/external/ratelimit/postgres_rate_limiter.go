package ratelimit

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thumbi/backend/internal/core/ports"
)

// DBExecutor define la interfaz mínima de PostgreSQL/pgx requerida para rate limiting atómico.
type DBExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRateLimiter implementa ports.RateLimiter utilizando una tabla UNLOGGED en PostgreSQL
// con operaciones atómicas INSERT ... ON CONFLICT DO UPDATE y limpieza concurrente mediante
// FOR UPDATE SKIP LOCKED. Permite compartir contadores entre múltiples réplicas sin Redis.
// Ante fallos transitorios de red, degrada de forma segura al MemoryRateLimiter local.
type PostgresRateLimiter struct {
	db       DBExecutor
	fallback *MemoryRateLimiter
}

// NewPostgresRateLimiter construye un limitador distribuido respaldado por PostgreSQL con fallback en memoria.
func NewPostgresRateLimiter(db DBExecutor) *PostgresRateLimiter {
	return &PostgresRateLimiter{
		db:       db,
		fallback: NewMemoryRateLimiter(),
	}
}

// Allow incrementa atómicamente el contador en la tabla UNLOGGED `rate_limits` de PostgreSQL.
func (p *PostgresRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	if limit <= 0 {
		return false, 0, window, nil
	}
	if window <= 0 {
		window = time.Minute
	}
	if p.db == nil {
		return p.fallback.Allow(ctx, key, limit, window)
	}

	windowSeconds := int(math.Ceil(window.Seconds()))
	if windowSeconds <= 0 {
		windowSeconds = 1
	}

	query := `
		INSERT INTO rate_limits (key, count, expires_at)
		VALUES ($1, 1, NOW() + ($2 * INTERVAL '1 second'))
		ON CONFLICT (key) DO UPDATE SET
			count = CASE
				WHEN rate_limits.expires_at <= NOW() THEN 1
				ELSE rate_limits.count + 1
			END,
			expires_at = CASE
				WHEN rate_limits.expires_at <= NOW() THEN EXCLUDED.expires_at
				ELSE rate_limits.expires_at
			END
		RETURNING count, GREATEST(1, CEIL(EXTRACT(EPOCH FROM (expires_at - NOW()))))::int
	`

	var count int
	var resetSec int
	if err := p.db.QueryRow(ctx, query, key, windowSeconds).Scan(&count, &resetSec); err != nil {
		// Resiliencia: fallback transparente a memoria local si la BD no está disponible o aún no migró
		return p.fallback.Allow(ctx, key, limit, window)
	}

	resetDur := time.Duration(resetSec) * time.Second
	if count > limit {
		return false, 0, resetDur, nil
	}

	return true, limit - count, resetDur, nil
}

// PurgeExpired depura en lotes acotados los registros vencidos utilizando FOR UPDATE SKIP LOCKED
// para evitar contención entre múltiples réplicas ejecutando limpieza en paralelo.
func (p *PostgresRateLimiter) PurgeExpired(ctx context.Context) (int64, error) {
	if p.db == nil {
		return int64(p.fallback.PurgeExpired()), nil
	}

	query := `
		DELETE FROM rate_limits
		WHERE key IN (
			SELECT key
			FROM rate_limits
			WHERE expires_at <= NOW()
			ORDER BY expires_at ASC
			LIMIT 1000
			FOR UPDATE SKIP LOCKED
		)
	`
	tag, err := p.db.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("error depurando rate_limits expirados en PostgreSQL: %w", err)
	}
	return tag.RowsAffected(), nil
}

// StartCleanup ejecuta la limpieza periódica tanto en PostgreSQL como en el fallback local en memoria.
func (p *PostgresRateLimiter) StartCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.fallback.PurgeExpired()
			if p.db != nil {
				purgeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				if _, err := p.PurgeExpired(purgeCtx); err != nil {
					log.Printf("[WARN] Error depurando rate_limits en PostgreSQL: %v", err)
				}
				cancel()
			}
		}
	}
}

var _ ports.RateLimiter = (*PostgresRateLimiter)(nil)
