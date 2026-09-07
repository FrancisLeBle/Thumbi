package routing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/external/routing"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

func TestOSRMProvider_CalculateRoute(t *testing.T) {
	ctx := context.Background()

	orig, _ := domain.NewCoordinates(-34.6037, -58.3816) // Buenos Aires
	dest, _ := domain.NewCoordinates(-32.9468, -60.6393) // Rosario

	req := ports.RouteCalculationRequest{
		Origin:      orig,
		Destination: dest,
	}

	t.Run("calcula exitosamente a través de respuesta simulada de OSRM", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Contains(t, r.URL.Path, "/route/v1/driving/")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"code": "Ok",
				"routes": [
					{
						"distance": 300450.0,
						"duration": 14400.0,
						"geometry": {
							"type": "LineString",
							"coordinates": [
								[-58.3816, -34.6037],
								[-59.5000, -33.8000],
								[-60.6393, -32.9468]
							]
						}
					}
				]
			}`))
		}))
		defer server.Close()

		provider := routing.NewOSRMProvider(server.URL, server.Client())
		details, err := provider.CalculateRoute(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, details)
		assert.InDelta(t, 300.45, details.DistanceKm, 0.1)
		assert.Equal(t, 240, details.DurationMinutes)
		assert.Equal(t, 2, details.TollsCount) // ~300km / 130km = 2
		assert.Equal(t, 3000.0, details.EstimatedTollCost)
		assert.Greater(t, details.EstimatedFuelCost, 0.0)
		assert.Len(t, details.RoutePath.Coordinates, 3)
	})

	t.Run("conmuta a fallback determinístico si OSRM no responde o da 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := routing.NewOSRMProvider(server.URL, server.Client())
		details, err := provider.CalculateRoute(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, details)
		assert.Greater(t, details.DistanceKm, 250.0)
		assert.Greater(t, details.DurationMinutes, 120)
		assert.Greater(t, len(details.RoutePath.Coordinates), 2)
	})

	t.Run("rechaza solicitud con coordenadas inválidas", func(t *testing.T) {
		provider := routing.NewOSRMProvider("http://localhost:9999", nil)
		_, err := provider.CalculateRoute(ctx, ports.RouteCalculationRequest{
			Origin:      domain.Coordinates{Latitude: 100.0, Longitude: 0.0},
			Destination: dest,
		})
		assert.ErrorIs(t, err, domain.ErrInvalidRouteCoordinates)
	})
}
