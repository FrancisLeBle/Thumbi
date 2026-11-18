package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/external/ratelimit"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
)

// MockRateLimiter es una implementación en memoria de ports.RateLimiter para pruebas unitarias de middlewares
type MockRateLimiter struct {
	mu      sync.Mutex
	counts  map[string]int
	expires map[string]time.Time
}

func NewMockRateLimiter() *MockRateLimiter {
	return &MockRateLimiter{
		counts:  make(map[string]int),
		expires: make(map[string]time.Time),
	}
}

func (m *MockRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	exp, exists := m.expires[key]
	if !exists || now.After(exp) {
		m.counts[key] = 1
		m.expires[key] = now.Add(window)
		remaining := limit - 1
		if remaining < 0 {
			remaining = 0
		}
		return true, remaining, window, nil
	}

	m.counts[key]++
	remaining := limit - m.counts[key]
	if remaining < 0 {
		remaining = 0
	}
	ttl := m.expires[key].Sub(now)
	if ttl <= 0 {
		ttl = time.Second
	}

	if m.counts[key] > limit {
		return false, 0, ttl, nil
	}

	return true, remaining, ttl, nil
}

func TestRateLimitByIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("permite solicitudes dentro del límite y añade cabeceras estándar", func(t *testing.T) {
		limiter := NewMockRateLimiter()
		r := gin.New()
		r.POST("/test", middleware.RateLimitByIP(limiter, 3, time.Minute, "test_route"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		// 1ª solicitud
		req1, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusOK, w1.Code)
		assert.Equal(t, "3", w1.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "2", w1.Header().Get("X-RateLimit-Remaining"))

		// 2ª solicitud
		req2, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Equal(t, "1", w2.Header().Get("X-RateLimit-Remaining"))

		// 3ª solicitud
		req3, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusOK, w3.Code)
		assert.Equal(t, "0", w3.Header().Get("X-RateLimit-Remaining"))

		// 4ª solicitud (supera límite)
		req4, _ := http.NewRequest(http.MethodPost, "/test", nil)
		w4 := httptest.NewRecorder()
		r.ServeHTTP(w4, req4)
		assert.Equal(t, http.StatusTooManyRequests, w4.Code)
		assert.NotEmpty(t, w4.Header().Get("Retry-After"))
		assert.Equal(t, "0", w4.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("si limiter es nil, permite la solicitud sin bloquear", func(t *testing.T) {
		r := gin.New()
		r.POST("/nil-limiter", middleware.RateLimitByIP(nil, 3, time.Minute, "nil_test"), func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		})

		req, _ := http.NewRequest(http.MethodPost, "/nil-limiter", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestRateLimitByUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("aplica límite por ID de usuario autenticado", func(t *testing.T) {
		limiter := NewMockRateLimiter()
		r := gin.New()
		r.Use(func(c *gin.Context) {
			authHeader := c.GetHeader("X-User-ID")
			if authHeader != "" {
				if uid, err := uuid.Parse(authHeader); err == nil {
					c.Set(middleware.CtxUserIDKey, uid)
				}
				c.Set(middleware.CtxUserRoleKey, domain.UserRolePassenger)
			}
			c.Next()
		})
		r.POST("/bookings", middleware.RateLimitByUser(limiter, 2, time.Minute, "booking"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{"booking": "created"})
		})

		aliceID := uuid.MustParse("01960000-0000-7000-8000-000000000001").String()
		bobID := uuid.MustParse("01960000-0000-7000-8000-000000000002").String()

		// Usuario 1 realiza 2 peticiones (límite: 2)
		for i := 0; i < 2; i++ {
			req, _ := http.NewRequest(http.MethodPost, "/bookings", nil)
			req.Header.Set("X-User-ID", aliceID)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusCreated, w.Code, "intento %d de user-alice", i+1)
		}

		// Usuario 1 realiza la 3ª petición -> 429
		req3, _ := http.NewRequest(http.MethodPost, "/bookings", nil)
		req3.Header.Set("X-User-ID", aliceID)
		w3 := httptest.NewRecorder()
		r.ServeHTTP(w3, req3)
		assert.Equal(t, http.StatusTooManyRequests, w3.Code)

		// Usuario 2 realiza una petición -> Debe permitirse ya que el límite es por usuario
		reqBob, _ := http.NewRequest(http.MethodPost, "/bookings", nil)
		reqBob.Header.Set("X-User-ID", bobID)
		wBob := httptest.NewRecorder()
		r.ServeHTTP(wBob, reqBob)
		assert.Equal(t, http.StatusCreated, wBob.Code)
	})
}

func TestInMemoryRateLimiter_Integration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	limiter := ratelimit.NewMemoryRateLimiter()
	mw := middleware.RateLimitByIP(limiter, 2, time.Minute, "in_memory")
	require.NotNil(t, mw)

	r := gin.New()
	r.GET("/ping", mw, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 2; i++ {
		req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	req3, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusTooManyRequests, w3.Code)
	assert.NotEmpty(t, strconv.Itoa(w3.Code))
}
