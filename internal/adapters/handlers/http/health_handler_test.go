package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	adapterHttp "github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http"
)

type mockHealthDB struct {
	pingErr         error
	postgisVersion  string
	postgisQueryErr error
}

func (m *mockHealthDB) Ping(ctx context.Context) error {
	return m.pingErr
}

func (m *mockHealthDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return &mockHealthRow{
		version: m.postgisVersion,
		err:     m.postgisQueryErr,
	}
}

type mockHealthRow struct {
	version string
	err     error
}

func (r *mockHealthRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) > 0 {
		if ptr, ok := dest[0].(*string); ok {
			*ptr = r.version
		}
	}
	return nil
}

func setupHealthRouter(h *adapterHttp.HealthHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", h.Health)
	r.GET("/ready", h.Ready)
	return r
}

func TestHealthHandler_Health_UP(t *testing.T) {
	dbMock := &mockHealthDB{
		pingErr: nil,
	}

	handler := adapterHttp.NewHealthHandler(dbMock)
	handler.SetStatsFunc(func() map[string]any {
		return map[string]any{
			"total_conns":    int32(10),
			"idle_conns":     int32(8),
			"acquired_conns": int32(2),
			"max_conns":      int32(20),
		}
	})

	router := setupHealthRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var res map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &res)
	assert.NoError(t, err)
	assert.Equal(t, "UP", res["status"])
	assert.Equal(t, "thumbi-api", res["service"])

	checks := res["checks"].(map[string]any)
	dbCheck := checks["database"].(map[string]any)
	assert.Equal(t, "UP", dbCheck["status"])

	metrics := res["metrics"].(map[string]any)
	assert.NotNil(t, metrics["goroutines"])
	assert.NotNil(t, metrics["memory_alloc_mb"])
	assert.NotNil(t, metrics["db_pool"])
}

func TestHealthHandler_Health_DB_Down(t *testing.T) {
	dbMock := &mockHealthDB{
		pingErr: errors.New("connection refused to postgres:5432"),
	}

	handler := adapterHttp.NewHealthHandler(dbMock)
	router := setupHealthRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var res map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &res)
	assert.NoError(t, err)
	assert.Equal(t, "DOWN", res["status"])

	checks := res["checks"].(map[string]any)
	dbCheck := checks["database"].(map[string]any)
	assert.Equal(t, "DOWN", dbCheck["status"])
	assert.Contains(t, dbCheck["error"], "connection refused")
}

func TestHealthHandler_Ready_UP(t *testing.T) {
	dbMock := &mockHealthDB{
		pingErr:        nil,
		postgisVersion: "3.4.2",
	}

	handler := adapterHttp.NewHealthHandler(dbMock)
	router := setupHealthRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var res map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &res)
	assert.NoError(t, err)
	assert.Equal(t, "UP", res["status"])
	assert.Equal(t, true, res["ready"])

	checks := res["checks"].(map[string]any)
	dbCheck := checks["database"].(map[string]any)
	assert.Equal(t, "UP", dbCheck["status"])

	gisCheck := checks["postgis"].(map[string]any)
	assert.Equal(t, "UP", gisCheck["status"])
	assert.Equal(t, "3.4.2", gisCheck["version"])
}

func TestHealthHandler_Ready_PostGIS_Missing(t *testing.T) {
	dbMock := &mockHealthDB{
		pingErr:         nil,
		postgisQueryErr: errors.New("extension \"postgis\" does not exist"),
	}

	handler := adapterHttp.NewHealthHandler(dbMock)
	router := setupHealthRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var res map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &res)
	assert.NoError(t, err)
	assert.Equal(t, "DOWN", res["status"])
	assert.Equal(t, false, res["ready"])

	checks := res["checks"].(map[string]any)
	gisCheck := checks["postgis"].(map[string]any)
	assert.Equal(t, "DOWN", gisCheck["status"])
	assert.Contains(t, gisCheck["error"], "PostGIS extension missing")
}

func TestHealthHandler_Ready_NilDB(t *testing.T) {
	handler := adapterHttp.NewHealthHandler(nil)
	router := setupHealthRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
