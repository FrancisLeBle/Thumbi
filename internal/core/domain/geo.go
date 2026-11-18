package domain

import "strings"

// Coordinates representa una coordenada geográfica en latitud y longitud (WGS 84, SRID 4326)
type Coordinates struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// NewCoordinates crea y valida coordenadas geográficas en grados decimales
func NewCoordinates(lat, lon float64) (Coordinates, error) {
	c := Coordinates{
		Latitude:  lat,
		Longitude: lon,
	}
	if !c.IsValid() {
		return Coordinates{}, ErrInvalidRouteCoordinates
	}
	return c, nil
}

// IsValid verifica si las coordenadas se encuentran dentro del rango geodésico admisible
func (c Coordinates) IsValid() bool {
	return c.Latitude >= -90.0 && c.Latitude <= 90.0 && c.Longitude >= -180.0 && c.Longitude <= 180.0
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

// LineString representa una secuencia ordenada de coordenadas que forman el trazado de una ruta
type LineString struct {
	Coordinates []Coordinates `json:"coordinates,omitempty"`
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
