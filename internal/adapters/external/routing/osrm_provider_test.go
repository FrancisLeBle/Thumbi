package routing_test

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/external/routing"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
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
		assert.Equal(t, domain.MoneyFromCents(300000, domain.CurrencyARS), details.EstimatedTollCost)
		assert.Greater(t, details.EstimatedFuelCost.Amount, int64(0))
		assert.Equal(t, domain.CurrencyARS, details.EstimatedFuelCost.Currency)
		assert.Len(t, details.RoutePath.Coordinates, 3)
	})

	t.Run("conmuta a fallback de polilínea si OSRM no responde o da 500 delegando distancia a PostGIS", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		provider := routing.NewOSRMProvider(server.URL, server.Client())
		details, err := provider.CalculateRoute(ctx, req)

		require.NoError(t, err)
		assert.NotNil(t, details)
		assert.Zero(t, details.DistanceKm)
		assert.Zero(t, details.DurationMinutes)
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

	t.Run("soporta servidor OSRM sobre HTTPS/TLS", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		assert.InDelta(t, 300.45, details.DistanceKm, 0.1)
	})
}

func TestNewSecureHTTPClient_EnforcesTLS12(t *testing.T) {
	client := routing.NewSecureHTTPClient(5 * time.Second)
	require.NotNil(t, client)
	assert.Equal(t, 5*time.Second, client.Timeout)

	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.TLSClientConfig)
	assert.Equal(t, uint16(tls.VersionTLS12), transport.TLSClientConfig.MinVersion)
}

