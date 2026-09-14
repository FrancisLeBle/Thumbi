package http

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/thumbi/auth-kyc-service/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

type TripHandler struct {
	tripService ports.TripService
}

// NewTripHandler crea un nuevo handler HTTP para gestión y búsqueda de viajes
func NewTripHandler(tripService ports.TripService) *TripHandler {
	return &TripHandler{tripService: tripService}
}

// CreateTripStopRequest representa una parada intermedia en la solicitud de creación
type CreateTripStopRequest struct {
	StopOrder            int     `json:"stop_order"`
	LocationTitle        string  `json:"location_title" binding:"required"`
	Lat                  float64 `json:"lat" binding:"required"`
	Lon                  float64 `json:"lon" binding:"required"`
	EstimatedArrivalTime string  `json:"estimated_arrival_time"`
}

// CreateTripRequest payload para la publicación de un viaje
type CreateTripRequest struct {
	VehicleID        string                  `json:"vehicle_id" binding:"required"`
	OriginTitle      string                  `json:"origin_title" binding:"required"`
	OriginLat        float64                 `json:"origin_lat" binding:"required"`
	OriginLon        float64                 `json:"origin_lon" binding:"required"`
	DestinationTitle string                  `json:"destination_title" binding:"required"`
	DestinationLat   float64                 `json:"destination_lat" binding:"required"`
	DestinationLon   float64                 `json:"destination_lon" binding:"required"`
	DepartureTime    string                  `json:"departure_time" binding:"required"`
	SeatsOffered     int                     `json:"seats_offered" binding:"required,min=1,max=8"`
	PricePerSeat     float64                 `json:"price_per_seat" binding:"required,gte=0"`
	Stops            []CreateTripStopRequest `json:"stops"`
}

// CreateTrip publica un nuevo viaje en ruta verificando conductor activo y límite Cap Pricing
// POST /v1/trips ó POST /api/v1/trips
func (h *TripHandler) CreateTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	var req CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "datos de publicación de viaje incompletos o inválidos",
			"details": err.Error(),
			"code":    "INVALID_REQUEST_PAYLOAD",
		})
		return
	}

	origCoords, err := domain.NewCoordinates(req.OriginLat, req.OriginLon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "coordenadas de origen inválidas",
			"code":  "INVALID_COORDINATES",
		})
		return
	}

	destCoords, err := domain.NewCoordinates(req.DestinationLat, req.DestinationLon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "coordenadas de destino inválidas",
			"code":  "INVALID_COORDINATES",
		})
		return
	}

	var stops []ports.CreateTripStopInput
	for _, s := range req.Stops {
		stopCoords, errStop := domain.NewCoordinates(s.Lat, s.Lon)
		if errStop != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "coordenadas de parada intermedia inválidas",
				"code":  "INVALID_COORDINATES",
			})
			return
		}
		stops = append(stops, ports.CreateTripStopInput{
			StopOrder:            s.StopOrder,
			LocationTitle:        s.LocationTitle,
			Coords:               stopCoords,
			EstimatedArrivalTime: s.EstimatedArrivalTime,
		})
	}

	input := ports.CreateTripInput{
		DriverID:          driverID,
		VehicleID:         req.VehicleID,
		OriginTitle:       req.OriginTitle,
		OriginCoords:      origCoords,
		DestinationTitle:  req.DestinationTitle,
		DestinationCoords: destCoords,
		DepartureTime:     req.DepartureTime,
		SeatsOffered:      req.SeatsOffered,
		PricePerSeat:      req.PricePerSeat,
		Stops:             stops,
	}

	trip, err := h.tripService.CreateTrip(c.Request.Context(), input)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrDriverNotActive):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "El usuario no cuenta con rol de conductor activo o no ha completado la verificación KYC.",
				"code":  "DRIVER_NOT_ACTIVE",
			})
			return

		case errors.Is(err, domain.ErrPriceExceedsCapPrice):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "El precio fijado excede el límite máximo permitido (Cap Price) para el trayecto según la normativa de economía colaborativa no lucrativa.",
				"code":  "PRICE_EXCEEDS_CAP_PRICE",
			})
			return

		case errors.Is(err, domain.ErrVehicleNotAvailable):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "El vehículo no se encuentra aprobado para realizar viajes.",
				"code":  "VEHICLE_NOT_APPROVED",
			})
			return

		case errors.Is(err, domain.ErrVehicleNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Vehículo no encontrado.",
				"code":  "VEHICLE_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "El vehículo no pertenece al usuario autenticado.",
				"code":  "FORBIDDEN",
			})
			return

		case errors.Is(err, domain.ErrDepartureTimeMustBeFuture):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "La fecha y hora de salida debe ser estrictamente en el futuro.",
				"code":  "INVALID_DEPARTURE_TIME",
			})
			return

		case errors.Is(err, domain.ErrInvalidRouteCoordinates):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Las coordenadas geográficas proporcionadas son inválidas.",
				"code":  "INVALID_COORDINATES",
			})
			return

		case errors.Is(err, domain.ErrSeatsExceedCapacity):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "La cantidad de asientos ofertados supera la capacidad del vehículo.",
				"code":  "SEATS_EXCEED_CAPACITY",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
				"code":  "INTERNAL_SERVER_ERROR",
			})
			return
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Viaje publicado con éxito",
		"trip":    trip,
	})
}

// SearchTrips ejecuta la búsqueda geoespacial sobre trazados vectoriales en PostGIS
// GET /v1/trips/search ó GET /api/v1/trips/search
func (h *TripHandler) SearchTrips(c *gin.Context) {
	origLatStr := c.Query("orig_lat")
	origLonStr := c.Query("orig_lon")
	destLatStr := c.Query("dest_lat")
	destLonStr := c.Query("dest_lon")

	if origLatStr == "" || origLonStr == "" || destLatStr == "" || destLonStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Se requieren los parámetros de coordenadas: orig_lat, orig_lon, dest_lat, dest_lon",
			"code":  "MISSING_COORDINATES",
		})
		return
	}

	origLat, err1 := strconv.ParseFloat(origLatStr, 64)
	origLon, err2 := strconv.ParseFloat(origLonStr, 64)
	destLat, err3 := strconv.ParseFloat(destLatStr, 64)
	destLon, err4 := strconv.ParseFloat(destLonStr, 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Los valores de coordenadas deben ser números decimales válidos",
			"code":  "INVALID_COORDINATES",
		})
		return
	}

	origCoords, err := domain.NewCoordinates(origLat, origLon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Coordenadas de origen fuera de rango",
			"code":  "INVALID_COORDINATES",
		})
		return
	}

	destCoords, err := domain.NewCoordinates(destLat, destLon)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Coordenadas de destino fuera de rango",
			"code":  "INVALID_COORDINATES",
		})
		return
	}

	// Parámetros opcionales
	dateStr := strings.TrimSpace(c.Query("date"))

	seats := 1
	if seatsStr := c.Query("seats"); seatsStr != "" {
		if s, err := strconv.Atoi(seatsStr); err == nil && s > 0 {
			seats = s
		}
	}

	radiusMeters := 10000.0 // 10 km por defecto
	if rStr := c.Query("radius_meters"); rStr != "" {
		if r, err := strconv.ParseFloat(rStr, 64); err == nil && r > 0 {
			radiusMeters = r
		}
	} else if rStr := c.Query("tolerance_meters"); rStr != "" {
		if r, err := strconv.ParseFloat(rStr, 64); err == nil && r > 0 {
			radiusMeters = r
		}
	}

	limit := 20
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := c.Query("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	query := ports.SearchTripsQuery{
		OriginCoords:    origCoords,
		DestCoords:      destCoords,
		DepartureDate:   dateStr,
		RequiredSeats:   seats,
		ToleranceMeters: radiusMeters,
		Limit:           limit,
		Offset:          offset,
	}

	results, err := h.tripService.SearchTrips(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "SEARCH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
	})
}

// GetTripByID obtiene el detalle integral del viaje con desglose de costos
// GET /v1/trips/:id ó GET /api/v1/trips/:id
func (h *TripHandler) GetTripByID(c *gin.Context) {
	tripID := strings.TrimSpace(c.Param("id"))
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de viaje no proporcionado",
			"code":  "MISSING_TRIP_ID",
		})
		return
	}

	detail, err := h.tripService.GetTripByID(c.Request.Context(), tripID)
	if err != nil {
		if errors.Is(err, domain.ErrTripNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Viaje no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
			"code":  "INTERNAL_SERVER_ERROR",
		})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// CancelTrip cancela un viaje publicado si pertenece al conductor autenticado
// DELETE /v1/trips/:id ó DELETE /api/v1/trips/:id
func (h *TripHandler) CancelTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	tripID := strings.TrimSpace(c.Param("id"))
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de viaje no proporcionado",
			"code":  "MISSING_TRIP_ID",
		})
		return
	}

	err := h.tripService.CancelTrip(c.Request.Context(), tripID, driverID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTripNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Viaje no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No estás autorizado para cancelar este viaje",
				"code":  "FORBIDDEN",
			})
			return

		case errors.Is(err, domain.ErrCannotCancelStartedTrip):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "No es posible cancelar un viaje que ya se encuentra en curso o completado",
				"code":  "CANNOT_CANCEL_STARTED_TRIP",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
				"code":  "INTERNAL_SERVER_ERROR",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Viaje cancelado exitosamente",
		"status":  "CANCELLED",
	})
}

// CompleteTrip concluye el viaje por parte del conductor titular, completa las reservas y libera custodia
// POST /v1/trips/:id/complete ó POST /api/v1/trips/:id/complete
func (h *TripHandler) CompleteTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "usuario no autenticado",
			"code":  "UNAUTHORIZED",
		})
		return
	}

	tripID := strings.TrimSpace(c.Param("id"))
	if tripID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "ID de viaje no proporcionado",
			"code":  "MISSING_TRIP_ID",
		})
		return
	}

	_, err := h.tripService.CompleteTrip(c.Request.Context(), tripID, driverID)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrTripNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Viaje no encontrado",
				"code":  "TRIP_NOT_FOUND",
			})
			return

		case errors.Is(err, domain.ErrUnauthorized):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "No estás autorizado para finalizar este viaje",
				"code":  "FORBIDDEN",
			})
			return

		case errors.Is(err, domain.ErrInvalidTripStatusChange):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "El viaje no se encuentra en un estado que permita su finalización",
				"code":  "INVALID_TRIP_STATUS",
			})
			return

		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
				"code":  "INTERNAL_SERVER_ERROR",
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Viaje finalizado exitosamente y liquidación procesada",
		"status":  "COMPLETED",
	})
}

