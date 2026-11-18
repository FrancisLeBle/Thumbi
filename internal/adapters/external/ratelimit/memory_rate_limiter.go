package ratelimit

import (
	"context"
	"sync"
	"time"

	"github.com/thumbi/backend/internal/core/ports"
)

// MemoryRateLimiter implementa ports.RateLimiter utilizando un mapa concurrente en memoria
// protegido por mutex y limpieza periódica por TTL. Elimina la necesidad de Redis para
// instancias únicas o despliegues MVP con costo de infraestructura cero.
type MemoryRateLimiter struct {
	mu      sync.Mutex
	entries map[string]*rateLimitEntry
	nowFn   func() time.Time
}

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

// NewMemoryRateLimiter crea un limitador de tasa en memoria seguro para concurrencia.
func NewMemoryRateLimiter() *MemoryRateLimiter {
	return &MemoryRateLimiter{
		entries: make(map[string]*rateLimitEntry),
		nowFn:   func() time.Time { return time.Now().UTC() },
	}
}

// Allow evalúa y actualiza atómicamente el contador de solicitudes para la clave dada dentro de la ventana temporal.
func (m *MemoryRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	if limit <= 0 {
		return false, 0, window, nil
	}
	if window <= 0 {
		window = time.Minute
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFn()
	entry, exists := m.entries[key]

	if !exists || !now.Before(entry.expiresAt) {
		m.entries[key] = &rateLimitEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
		remaining := limit - 1
		if remaining < 0 {
			remaining = 0
		}
		return true, remaining, window, nil
	}

	entry.count++
	resetDur := entry.expiresAt.Sub(now)
	if resetDur <= 0 {
		resetDur = time.Second
	}

	if entry.count > limit {
		return false, 0, resetDur, nil
	}

	return true, limit - entry.count, resetDur, nil
}

// PurgeExpired elimina todas las entradas vencidas para mantener acotado el uso de memoria RAM.
func (m *MemoryRateLimiter) PurgeExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := m.nowFn()
	purged := 0
	for k, entry := range m.entries {
		if !now.Before(entry.expiresAt) {
			delete(m.entries, k)
			purged++
		}
	}
	return purged
}

// StartCleanup ejecuta la depuración periódica de ventanas expiradas hasta que ctx se cancele.
func (m *MemoryRateLimiter) StartCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.PurgeExpired()
		}
	}
}

var _ ports.RateLimiter = (*MemoryRateLimiter)(nil)
