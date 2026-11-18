package ratelimit

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryRateLimiter_AllowAndExpire(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)

	limiter := NewMemoryRateLimiter()
	limiter.nowFn = func() time.Time { return now }

	// Primeras 3 peticiones permitidas
	for i := 1; i <= 3; i++ {
		allowed, remaining, reset, err := limiter.Allow(ctx, "ip:1.2.3.4", 3, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, 3-i, remaining)
		assert.Equal(t, time.Minute, reset)
	}

	// 4ta petición bloqueada
	allowed, remaining, _, err := limiter.Allow(ctx, "ip:1.2.3.4", 3, time.Minute)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, 0, remaining)

	// Avanzar reloj más allá de la ventana y verificar reinicio + limpieza
	now = now.Add(61 * time.Second)
	assert.Equal(t, 1, limiter.PurgeExpired())

	allowed, remaining, _, err = limiter.Allow(ctx, "ip:1.2.3.4", 3, time.Minute)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 2, remaining)
}

func TestMemoryRateLimiter_ConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	limiter := NewMemoryRateLimiter()

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, _, _, err := limiter.Allow(ctx, "user:concurrent", 5, time.Minute)
			require.NoError(t, err)
			if allowed {
				mu.Lock()
				allowedCount++
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, 5, allowedCount)
}

type stubRow struct {
	scanFn func(dest ...any) error
}

func (r *stubRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

type stubDB struct {
	lastQuery string
	row       pgx.Row
	execTag   pgconn.CommandTag
	execErr   error
}

func (s *stubDB) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	s.lastQuery = sql
	return s.execTag, s.execErr
}

func (s *stubDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	s.lastQuery = sql
	return s.row
}

func TestPostgresRateLimiter_AtomicUpsertAndSkipLockedPurge(t *testing.T) {
	ctx := context.Background()

	t.Run("ejecuta upsert atómico en tabla rate_limits", func(t *testing.T) {
		db := &stubDB{
			row: &stubRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*int) = 2
					*dest[1].(*int) = 45
					return nil
				},
			},
		}
		limiter := NewPostgresRateLimiter(db)

		allowed, remaining, reset, err := limiter.Allow(ctx, "ratelimit:ip:login:10.0.0.1", 5, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, 3, remaining)
		assert.Equal(t, 45*time.Second, reset)
		assert.Contains(t, db.lastQuery, "INSERT INTO rate_limits")
		assert.Contains(t, db.lastQuery, "ON CONFLICT (key) DO UPDATE")
	})

	t.Run("fallback transparente a memoria si PostgreSQL falla", func(t *testing.T) {
		db := &stubDB{
			row: &stubRow{
				scanFn: func(dest ...any) error {
					return errors.New("connection reset")
				},
			},
		}
		limiter := NewPostgresRateLimiter(db)

		allowed, remaining, _, err := limiter.Allow(ctx, "fallback-key", 2, time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, 1, remaining)
	})

	t.Run("PurgeExpired usa FOR UPDATE SKIP LOCKED", func(t *testing.T) {
		db := &stubDB{
			execTag: pgconn.NewCommandTag("DELETE 4"),
		}
		limiter := NewPostgresRateLimiter(db)

		purged, err := limiter.PurgeExpired(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(4), purged)
		assert.True(t, strings.Contains(db.lastQuery, "FOR UPDATE SKIP LOCKED"))
		assert.True(t, strings.Contains(db.lastQuery, "DELETE FROM rate_limits"))
	})
}
