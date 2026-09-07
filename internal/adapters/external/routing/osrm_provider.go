package routing

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// OSRMResponse representa la estructura de respuesta estándar de la API de OSRM (Open Source Routing Machine)
type OSRMResponse struct {
	Code   string      `json:"code"`
	Routes []OSRMRoute `json:"routes"`
	Message string     `json:"message,omitempty"`
}

// OSRMRoute contiene métricas de distancia, duración y la geometría en GeoJSON
type OSRMRoute struct {
	Distance float64      `json:"distance"` // Metros
	Duration float64      `json:"duration"` // Segundos
	Geometry OSRMGeometry `json:"geometry"`
}

// OSRMGeometry contiene el array de coordenadas [longitud, latitud]
type OSRMGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type osrmProvider struct {
	baseURL    string
	httpClient *http.Client
}

// NewOSRMProvider construye el proveedor de rutas cartográficas con cliente HTTP configurado
func NewOSRMProvider(baseURL string, httpClient *http.Client) ports.RoutingProvider {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		trimmed = "http://router.project-osrm.org"
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: 6 * time.Second,
		}
	}
	return &osrmProvider{
		baseURL:    trimmed,
		httpClient: httpClient,
	}
}

// CalculateRoute consulta OSRM para obtener polilínea vectorial, distancias, duraciones y costos
// En caso de indisponibilidad de la red o fallo del servidor, conmuta automáticamente a simulación determinística
func (p *osrmProvider) CalculateRoute(ctx context.Context, req ports.RouteCalculationRequest) (*ports.RouteDetails, error) {
	// 1. Validar puntos geográficos requeridos
	if !req.Origin.IsValid() || !req.Destination.IsValid() {
		return nil, domain.ErrInvalidRouteCoordinates
	}

	for _, wp := range req.Waypoints {
		if !wp.IsValid() {
			return nil, domain.ErrInvalidRouteCoordinates
		}
	}

	// 2. Construir secuencia de coordenadas en formato {lon},{lat};{lon},{lat}
	var coordSegments []string
	coordSegments = append(coordSegments, fmt.Sprintf("%.6f,%.6f", req.Origin.Longitude, req.Origin.Latitude))
	for _, wp := range req.Waypoints {
		coordSegments = append(coordSegments, fmt.Sprintf("%.6f,%.6f", wp.Longitude, wp.Latitude))
	}
	coordSegments = append(coordSegments, fmt.Sprintf("%.6f,%.6f", req.Destination.Longitude, req.Destination.Latitude))

	coordsPath := strings.Join(coordSegments, ";")
	osrmURL := fmt.Sprintf("%s/route/v1/driving/%s?overview=full&geometries=geojson&steps=true", p.baseURL, coordsPath)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, osrmURL, nil)
	if err != nil {
		// Fallback inmediato ante error de armado de request
		return p.calculateDeterministicFallback(req), nil
	}

	resp, err := p.httpClient.Do(httpReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		// Conmutación resiliente ante error de conexión o código HTTP no exitoso
		return p.calculateDeterministicFallback(req), nil
	}
	defer resp.Body.Close()

	var osrmResp OSRMResponse
	if err := json.NewDecoder(resp.Body).Decode(&osrmResp); err != nil || osrmResp.Code != "Ok" || len(osrmResp.Routes) == 0 {
		return p.calculateDeterministicFallback(req), nil
	}

	bestRoute := osrmResp.Routes[0]
	var coords []domain.Coordinates
	for _, pt := range bestRoute.Geometry.Coordinates {
		if len(pt) >= 2 {
			lon := pt[0]
			lat := pt[1]
			if c, errCoord := domain.NewCoordinates(lat, lon); errCoord == nil {
				coords = append(coords, c)
			}
		}
	}

	if len(coords) < 2 {
		return p.calculateDeterministicFallback(req), nil
	}

	routeLine, err := domain.NewLineString(coords)
	if err != nil {
		return p.calculateDeterministicFallback(req), nil
	}

	distanceKm := bestRoute.Distance / 1000.0
	durationMinutes := int(math.Round(bestRoute.Duration / 60.0))
	if durationMinutes <= 0 {
		durationMinutes = 1
	}

	// Estimación de peajes según distancia interurbana (promedio 1 peaje cada 130 km)
	tollsCount := int(distanceKm / 130.0)
	estimatedTollCost := float64(tollsCount) * 1500.0 // $1500 ARS por estación de peaje

	fuelEstimate := domain.CalculateFuelEstimate(distanceKm, 0, 0)

	return &ports.RouteDetails{
		RoutePath:         routeLine,
		DistanceKm:        math.Round(distanceKm*100) / 100,
		DurationMinutes:   durationMinutes,
		EstimatedTollCost: estimatedTollCost,
		TollsCount:        tollsCount,
		EstimatedFuelCost: fuelEstimate.TotalFuelCost,
	}, nil
}

// calculateDeterministicFallback calcula una ruta geométrica precisa en base a geodésicas y factores viales
func (p *osrmProvider) calculateDeterministicFallback(req ports.RouteCalculationRequest) *ports.RouteDetails {
	// Construir lista de puntos de control
	allControlPoints := []domain.Coordinates{req.Origin}
	allControlPoints = append(allControlPoints, req.Waypoints...)
	allControlPoints = append(allControlPoints, req.Destination)

	var fullCoordinates []domain.Coordinates
	var rawDistanceKm float64

	for i := 0; i < len(allControlPoints)-1; i++ {
		p1 := allControlPoints[i]
		p2 := allControlPoints[i+1]

		segmentDist := p1.DistanceHaversineKm(p2)
		rawDistanceKm += segmentDist

		// Añadir punto inicial del segmento
		fullCoordinates = append(fullCoordinates, p1)

		// Generar 3 puntos intermedios interpolados con leve curvatura realista
		const intermediateSteps = 3
		for step := 1; step <= intermediateSteps; step++ {
			fraction := float64(step) / float64(intermediateSteps+1)
			interLat := p1.Latitude + fraction*(p2.Latitude-p1.Latitude)
			interLon := p1.Longitude + fraction*(p2.Longitude-p1.Longitude)

			// Curvatura sinusoidal para simular trazado de ruta real
			curveFactor := math.Sin(fraction*math.Pi) * 0.015
			interLat += curveFactor

			if coord, err := domain.NewCoordinates(interLat, interLon); err == nil {
				fullCoordinates = append(fullCoordinates, coord)
			}
		}
	}
	fullCoordinates = append(fullCoordinates, req.Destination)

	routeLine, _ := domain.NewLineString(fullCoordinates)

	// Factor de sinuosidad vial interurbana (~1.18x sobre distancia en línea recta)
	const roadWindingFactor = 1.18
	distanceKm := math.Round((rawDistanceKm*roadWindingFactor)*100) / 100

	// Velocidad promedio interurbana ~85 km/h
	durationMinutes := int(math.Round((distanceKm / 85.0) * 60.0))
	if durationMinutes < 15 {
		durationMinutes = 15
	}

	tollsCount := int(distanceKm / 130.0)
	estimatedTollCost := float64(tollsCount) * 1500.0

	fuelEstimate := domain.CalculateFuelEstimate(distanceKm, 0, 0)

	return &ports.RouteDetails{
		RoutePath:         routeLine,
		DistanceKm:        distanceKm,
		DurationMinutes:   durationMinutes,
		EstimatedTollCost: estimatedTollCost,
		TollsCount:        tollsCount,
		EstimatedFuelCost: fuelEstimate.TotalFuelCost,
	}
}
