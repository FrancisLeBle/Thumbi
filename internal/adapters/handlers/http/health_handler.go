package http

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPingChecker define las operaciones requeridas para validar la salud de PostgreSQL y PostGIS.
type DBPingChecker interface {
	Ping(ctx context.Context) error
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// HealthHandler administra los endpoints de observabilidad, salud y readiness de Thumbi Core.
type HealthHandler struct {
	db        DBPingChecker
	statsFunc func() map[string]any
	startTime time.Time
	version   string
}

// NewHealthHandler inicializa el handler de observabilidad.
func NewHealthHandler(db DBPingChecker) *HealthHandler {
	hh := &HealthHandler{
		db:        db,
		startTime: time.Now(),
		version:   "1.0.0",
	}

	// Si se recibe un *pgxpool.Pool, extraemos métricas del connection pool
	if pool, ok := db.(*pgxpool.Pool); ok && pool != nil {
		hh.statsFunc = func() map[string]any {
			stat := pool.Stat()
			return map[string]any{
				"total_conns":        stat.TotalConns(),
				"idle_conns":         stat.IdleConns(),
				"acquired_conns":     stat.AcquiredConns(),
				"max_conns":          stat.MaxConns(),
				"constructing_conns": stat.ConstructingConns(),
			}
		}
	}

	return hh
}

// SetStatsFunc permite inyectar una función de métricas del pool personalizada (útil para pruebas).
func (h *HealthHandler) SetStatsFunc(fn func() map[string]any) {
	h.statsFunc = fn
}

// Health evalúa la vitalidad básica del servicio (Liveness Probe) y latencia hacia la base de datos.
// GET /health
func (h *HealthHandler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	checks := make(map[string]any)
	isHealthy := true

	if h.db != nil {
		start := time.Now()
		if err := h.db.Ping(ctx); err != nil {
			isHealthy = false
			checks["database"] = map[string]any{
				"status": "DOWN",
				"error":  err.Error(),
			}
		} else {
			checks["database"] = map[string]any{
				"status":     "UP",
				"latency_ms": float64(time.Since(start).Microseconds()) / 1000.0,
			}
		}
	} else {
		checks["database"] = map[string]any{
			"status": "DISABLED",
			"info":   "No database connection configured",
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := map[string]any{
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc_mb": fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/1024/1024),
		"uptime_seconds":  int(time.Since(h.startTime).Seconds()),
	}

	if h.statsFunc != nil {
		metrics["db_pool"] = h.statsFunc()
	}

	statusCode := http.StatusOK
	statusStr := "UP"
	if !isHealthy {
		statusCode = http.StatusServiceUnavailable
		statusStr = "DOWN"
	}

	c.JSON(statusCode, gin.H{
		"status":    statusStr,
		"service":   "thumbi-api",
		"version":   h.version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).Truncate(time.Second).String(),
		"checks":    checks,
		"metrics":   metrics,
	})
}

// Ready evalúa la preparación operativa del servicio (Readiness Probe), validando conectividad activa
// con PostgreSQL y la disponibilidad funcional de la extensión geoespacial PostGIS.
// GET /ready
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 4*time.Second)
	defer cancel()

	checks := make(map[string]any)
	isReady := true

	if h.db == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status":    "DOWN",
			"ready":     false,
			"service":   "thumbi-api",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"error":     "Database pool not initialized",
		})
		return
	}

	// 1. Verificación activa de PostgreSQL (Ping)
	startDB := time.Now()
	if err := h.db.Ping(ctx); err != nil {
		isReady = false
		checks["database"] = map[string]any{
			"status": "DOWN",
			"error":  err.Error(),
		}
	} else {
		checks["database"] = map[string]any{
			"status":     "UP",
			"latency_ms": float64(time.Since(startDB).Microseconds()) / 1000.0,
		}
	}

	// 2. Verificación de la extensión PostGIS (consulta activa de versión)
	startGIS := time.Now()
	var postgisVersion string
	err := h.db.QueryRow(ctx, "SELECT COALESCE(extversion, 'unknown') FROM pg_extension WHERE extname = 'postgis'").Scan(&postgisVersion)
	if err != nil {
		// Intento alternativo mediante función nativa PostGIS_Version()
		err = h.db.QueryRow(ctx, "SELECT PostGIS_Version()").Scan(&postgisVersion)
	}

	if err != nil {
		isReady = false
		checks["postgis"] = map[string]any{
			"status": "DOWN",
			"error":  fmt.Sprintf("PostGIS extension missing or unavailable: %v", err),
		}
	} else {
		checks["postgis"] = map[string]any{
			"status":     "UP",
			"version":    postgisVersion,
			"latency_ms": float64(time.Since(startGIS).Microseconds()) / 1000.0,
		}
	}

	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	metrics := map[string]any{
		"goroutines":      runtime.NumGoroutine(),
		"memory_alloc_mb": fmt.Sprintf("%.2f MB", float64(memStats.Alloc)/1024/1024),
		"uptime_seconds":  int(time.Since(h.startTime).Seconds()),
	}

	if h.statsFunc != nil {
		metrics["db_pool"] = h.statsFunc()
	}

	statusCode := http.StatusOK
	statusStr := "UP"
	if !isReady {
		statusCode = http.StatusServiceUnavailable
		statusStr = "DOWN"
	}

	c.JSON(statusCode, gin.H{
		"status":    statusStr,
		"ready":     isReady,
		"service":   "thumbi-api",
		"version":   h.version,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).Truncate(time.Second).String(),
		"checks":    checks,
		"metrics":   metrics,
	})
}
