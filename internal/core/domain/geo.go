package domain

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Coordinates representa una coordenada geográfica en latitud y longitud (WGS 84, SRID 4326)
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// NewCoordinates crea y valida coordenadas geográficas en grados decimales
func NewCoordinates(lat, lon float64) (Coordinates, error) {
	if lat < -90.0 || lat > 90.0 || lon < -180.0 || lon > 180.0 {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}
	return Coordinates{
		Latitude:  lat,
		Longitude: lon,
	}, nil
}

// IsValid verifica si las coordenadas se encuentran dentro del rango geodésico admisible
func (c Coordinates) IsValid() bool {
	return c.Latitude >= -90.0 && c.Latitude <= 90.0 && c.Longitude >= -180.0 && c.Longitude <= 180.0
}

// ToWKTPoint formatea las coordenadas como WKT (Well-Known Text): POINT(lon lat)
// Nota PostGIS/WKT: el formato estándar utiliza primero X (Longitud) y luego Y (Latitud)
func (c Coordinates) ToWKTPoint() string {
	return fmt.Sprintf("POINT(%.6f %.6f)", c.Longitude, c.Latitude)
}

// DistanceHaversineKm calcula la distancia del gran círculo entre dos coordenadas en kilómetros
func (c Coordinates) DistanceHaversineKm(other Coordinates) float64 {
	const earthRadiusKm = 6371.0

	dLat := (other.Latitude - c.Latitude) * (math.Pi / 180.0)
	dLon := (other.Longitude - c.Longitude) * (math.Pi / 180.0)

	lat1 := c.Latitude * (math.Pi / 180.0)
	lat2 := other.Latitude * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(lat1)*math.Cos(lat2)
	cVal := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * cVal
}

// Point representa un punto geográfico con nombre o dirección descriptiva
type Point struct {
	Title  string      `json:"title"`
	Coords Coordinates `json:"coords"`
}

// NewPoint crea un Point validado
func NewPoint(title string, coords Coordinates) (Point, error) {
	if strings.TrimSpace(title) == "" || !coords.IsValid() {
		return Point{}, ErrInvalidRouteCoordinates
	}
	return Point{
		Title:  strings.TrimSpace(title),
		Coords: coords,
	}, nil
}

// ParseWKTPoint descompone un WKT POINT(lon lat) a una estructura Coordinates
func ParseWKTPoint(wkt string) (Coordinates, error) {
	wkt = strings.TrimSpace(wkt)
	upper := strings.ToUpper(wkt)
	if !strings.HasPrefix(upper, "POINT") {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}

	start := strings.Index(wkt, "(")
	end := strings.LastIndex(wkt, ")")
	if start == -1 || end == -1 || start >= end {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}

	fields := strings.Fields(strings.TrimSpace(wkt[start+1 : end]))
	if len(fields) < 2 {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}

	lon, err1 := strconv.ParseFloat(fields[0], 64)
	lat, err2 := strconv.ParseFloat(fields[1], 64)
	if err1 != nil || err2 != nil {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}

	return NewCoordinates(lat, lon)
}

// LineString representa una secuencia ordenada de coordenadas que forman el trazado de una ruta
type LineString struct {
	Coordinates []Coordinates `json:"coordinates"`
}

// NewLineString crea y valida una secuencia de puntos para una polilínea
func NewLineString(coords []Coordinates) (LineString, error) {
	if len(coords) < 2 {
		return LineString{}, ErrInsufficientRoutePoints
	}
	for _, c := range coords {
		if !c.IsValid() {
			return LineString{}, ErrInvalidRouteCoordinates
		}
	}
	return LineString{Coordinates: coords}, nil
}

// ToWKTLineString convierte la polilínea a formato WKT: LINESTRING(lon1 lat1, lon2 lat2, ...)
func (ls LineString) ToWKTLineString() string {
	parts := make([]string, len(ls.Coordinates))
	for i, c := range ls.Coordinates {
		parts[i] = fmt.Sprintf("%.6f %.6f", c.Longitude, c.Latitude)
	}
	return fmt.Sprintf("LINESTRING(%s)", strings.Join(parts, ", "))
}

// TotalApproximatedDistanceKm calcula la distancia acumulada de la polilínea
func (ls LineString) TotalApproximatedDistanceKm() float64 {
	var total float64
	for i := 0; i < len(ls.Coordinates)-1; i++ {
		total += ls.Coordinates[i].DistanceHaversineKm(ls.Coordinates[i+1])
	}
	return total
}

// ParseWKTLineString descompone un WKT LINESTRING(lon lat, ...) a una estructura LineString
func ParseWKTLineString(wkt string) (LineString, error) {
	wkt = strings.TrimSpace(wkt)
	upper := strings.ToUpper(wkt)
	if !strings.HasPrefix(upper, "LINESTRING") {
		return LineString{}, ErrInvalidRouteCoordinates
	}

	start := strings.Index(wkt, "(")
	end := strings.LastIndex(wkt, ")")
	if start == -1 || end == -1 || start >= end {
		return LineString{}, ErrInvalidRouteCoordinates
	}

	rawPoints := strings.Split(wkt[start+1:end], ",")
	var coords []Coordinates
	for _, raw := range rawPoints {
		fields := strings.Fields(strings.TrimSpace(raw))
		if len(fields) < 2 {
			continue
		}
		lon, err1 := strconv.ParseFloat(fields[0], 64)
		lat, err2 := strconv.ParseFloat(fields[1], 64)
		if err1 != nil || err2 != nil {
			return LineString{}, ErrInvalidRouteCoordinates
		}
		c, err := NewCoordinates(lat, lon)
		if err != nil {
			return LineString{}, err
		}
		coords = append(coords, c)
	}

	if len(coords) < 2 {
		return LineString{}, ErrInsufficientRoutePoints
	}

	return LineString{Coordinates: coords}, nil
}
