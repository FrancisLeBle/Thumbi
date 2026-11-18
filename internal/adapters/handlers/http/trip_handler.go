package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/thumbi/backend/internal/adapters/handlers/http/middleware"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
)

// TripService define el contrato mínimo requerido por TripHandler (SOLID · ISP).
type TripService interface {
	CreateTrip(ctx context.Context, input ports.CreateTripInput) (*ports.TripDTO, error)
	SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error)
	GetTripByID(ctx context.Context, tripID uuid.UUID) (*ports.TripDetailDTO, error)
	CancelTrip(ctx context.Context, tripID, driverID uuid.UUID) error
	CompleteTrip(ctx context.Context, tripID, driverID uuid.UUID) (*ports.TripDTO, error)
}

type TripHandler struct {
	tripService TripService
}

// NewTripHandler crea un nuevo handler HTTP para gestión y búsqueda de viajes
func NewTripHandler(tripService TripService) *TripHandler {
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

// CreateTripRequest payload para la publicación de un viaje (driver_id se extrae exclusivamente del token autenticado)
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
	PricePerSeat     domain.Money            `json:"price_per_seat" binding:"required"`
	Currency         domain.Currency         `json:"currency,omitempty"`
	Stops            []CreateTripStopRequest `json:"stops"`
}

// TripSearchResultResponse DTO de respuesta HTTP para cada viaje coincidente en la búsqueda
type TripSearchResultResponse struct {
	ID                   uuid.UUID         `json:"id"`
	DriverID             uuid.UUID         `json:"driver_id"`
	DriverFullName       string            `json:"driver_full_name"`
	DriverAvatarURL      string            `json:"driver_avatar_url"`
	VehicleBrand         string            `json:"vehicle_brand"`
	VehicleModel         string            `json:"vehicle_model"`
	VehiclePlate         string            `json:"vehicle_plate"`
	OriginTitle          string            `json:"origin_title"`
	DestinationTitle     string            `json:"destination_title"`
	DepartureTime        string            `json:"departure_time"`
	EstimatedArrivalTime string            `json:"estimated_arrival_time"`
	TotalDistanceKm      float64           `json:"total_distance_km"`
	TotalDurationMinutes int               `json:"total_duration_minutes"`
	AvailableSeats       int               `json:"available_seats"`
	Currency             domain.Currency   `json:"currency"`
	PricePerSeat         domain.Money      `json:"price_per_seat"`
	CapPricePerSeat      domain.Money      `json:"cap_price_per_seat"`
	PickupWalkDistanceM  float64           `json:"pickup_walk_distance_m"`
	DropoffWalkDistanceM float64           `json:"dropoff_walk_distance_m"`
	RoutePreviewPolyline string            `json:"route_preview_polyline,omitempty"`
	Status               domain.TripStatus `json:"status"`
}

// SearchTripsResponse DTO de respuesta HTTP para la búsqueda de viajes
type SearchTripsResponse struct {
	Results []TripSearchResultResponse `json:"results"`
	Count   int                        `json:"count"`
}

// TripStopResponse DTO de respuesta HTTP para una parada intermedia del viaje
type TripStopResponse struct {
	ID                   uuid.UUID          `json:"id"`
	TripID               uuid.UUID          `json:"trip_id"`
	StopOrder            int                `json:"stop_order"`
	LocationTitle        string             `json:"location_title"`
	Coords               domain.Coordinates `json:"coords"`
	EstimatedArrivalTime string             `json:"estimated_arrival_time"`
	CreatedAt            string             `json:"created_at"`
}

// TripResponse DTO de respuesta HTTP para la representación de un viaje
type TripResponse struct {
	ID                   uuid.UUID          `json:"id"`
	DriverID             uuid.UUID          `json:"driver_id"`
	VehicleID            uuid.UUID          `json:"vehicle_id"`
	OriginTitle          string             `json:"origin_title"`
	OriginCoords         domain.Coordinates `json:"origin_coords"`
	DestinationTitle     string             `json:"destination_title"`
	DestinationCoords    domain.Coordinates `json:"destination_coords"`
	RoutePath            domain.LineString  `json:"route_path,omitempty"`
	RoutePreviewPolyline string             `json:"route_preview_polyline,omitempty"`
	DepartureTime        string             `json:"departure_time"`
	EstimatedArrivalTime string             `json:"estimated_arrival_time"`
	TotalDistanceKm      float64            `json:"total_distance_km"`
	TotalDurationMinutes int                `json:"total_duration_minutes"`
	TotalSeatsOffered    int                `json:"total_seats_offered"`
	AvailableSeats       int                `json:"available_seats"`
	Currency             domain.Currency    `json:"currency"`
	PricePerSeat         domain.Money       `json:"price_per_seat"`
	CapPricePerSeat      domain.Money       `json:"cap_price_per_seat"`
	EstimatedFuelCost    domain.Money       `json:"estimated_fuel_cost"`
	EstimatedTollCost    domain.Money       `json:"estimated_toll_cost"`
	Status               domain.TripStatus  `json:"status"`
	Stops                []TripStopResponse `json:"stops,omitempty"`
	CreatedAt            string             `json:"created_at"`
	UpdatedAt            string             `json:"updated_at"`
}

// TripDetailResponse DTO de respuesta HTTP con el detalle completo del viaje y matriz de Cap Pricing
type TripDetailResponse struct {
	Trip       *TripResponse     `json:"trip"`
	Driver     *AuthUserResponse `json:"driver"`
	Vehicle    *VehicleResponse  `json:"vehicle"`
	CostMatrix domain.CostMatrix `json:"cost_matrix"`
}

// CreateTripResponse DTO de respuesta HTTP al publicar un viaje
type CreateTripResponse struct {
	Message string        `json:"message"`
	Trip    *TripResponse `json:"trip"`
}

// CancelTripResponse DTO de respuesta HTTP al cancelar un viaje
type CancelTripResponse struct {
	Message string            `json:"message"`
	Status  domain.TripStatus `json:"status"`
}

// CompleteTripResponse DTO de respuesta HTTP al finalizar un viaje
type CompleteTripResponse struct {
	Message string            `json:"message"`
	Status  domain.TripStatus `json:"status"`
	Trip    *TripResponse     `json:"trip"`
}

func toTripSearchResultResponseList(results []ports.TripSearchResult) []TripSearchResultResponse {
	out := make([]TripSearchResultResponse, 0, len(results))
	for _, r := range results {
		out = append(out, TripSearchResultResponse{
			ID:                   r.ID,
			DriverID:             r.DriverID,
			DriverFullName:       r.DriverFullName,
			DriverAvatarURL:      r.DriverAvatarURL,
			VehicleBrand:         r.VehicleBrand,
			VehicleModel:         r.VehicleModel,
			VehiclePlate:         r.VehiclePlate,
			OriginTitle:          r.OriginTitle,
			DestinationTitle:     r.DestinationTitle,
			DepartureTime:        r.DepartureTime,
			EstimatedArrivalTime: r.EstimatedArrivalTime,
			TotalDistanceKm:      r.TotalDistanceKm,
			TotalDurationMinutes: r.TotalDurationMinutes,
			AvailableSeats:       r.AvailableSeats,
			Currency:             r.Currency,
			PricePerSeat:         r.PricePerSeat,
			CapPricePerSeat:      r.CapPricePerSeat,
			PickupWalkDistanceM:  r.PickupWalkDistanceM,
			DropoffWalkDistanceM: r.DropoffWalkDistanceM,
			RoutePreviewPolyline: r.RoutePreviewPolyline,
			Status:               r.Status,
		})
	}
	return out
}

func toTripResponse(t *ports.TripDTO) *TripResponse {
	if t == nil {
		return nil
	}
	var stops []TripStopResponse
	if len(t.Stops) > 0 {
		stops = make([]TripStopResponse, 0, len(t.Stops))
		for _, st := range t.Stops {
			stops = append(stops, TripStopResponse{
				ID:                   st.ID,
				TripID:               st.TripID,
				StopOrder:            st.StopOrder,
				LocationTitle:        st.LocationTitle,
				Coords:               st.Coords,
				EstimatedArrivalTime: st.EstimatedArrivalTime,
				CreatedAt:            st.CreatedAt,
			})
		}
	}
	return &TripResponse{
		ID:                   t.ID,
		DriverID:             t.DriverID,
		VehicleID:            t.VehicleID,
		OriginTitle:          t.OriginTitle,
		OriginCoords:         t.OriginCoords,
		DestinationTitle:     t.DestinationTitle,
		DestinationCoords:    t.DestinationCoords,
		RoutePath:            t.RoutePath,
		RoutePreviewPolyline: t.RoutePreviewPolyline,
		DepartureTime:        t.DepartureTime,
		EstimatedArrivalTime: t.EstimatedArrivalTime,
		TotalDistanceKm:      t.TotalDistanceKm,
		TotalDurationMinutes: t.TotalDurationMinutes,
		TotalSeatsOffered:    t.TotalSeatsOffered,
		AvailableSeats:       t.AvailableSeats,
		Currency:             t.Currency,
		PricePerSeat:         t.PricePerSeat,
		CapPricePerSeat:      t.CapPricePerSeat,
		EstimatedFuelCost:    t.EstimatedFuelCost,
		EstimatedTollCost:    t.EstimatedTollCost,
		Status:               t.Status,
		Stops:                stops,
		CreatedAt:            t.CreatedAt,
		UpdatedAt:            t.UpdatedAt,
	}
}

func toTripDetailResponse(detail *ports.TripDetailDTO) *TripDetailResponse {
	if detail == nil {
		return nil
	}
	return &TripDetailResponse{
		Trip:       toTripResponse(detail.Trip),
		Driver:     toAuthUserResponse(detail.Driver),
		Vehicle:    toVehicleResponse(detail.Vehicle),
		CostMatrix: detail.CostMatrix,
	}
}

// CreateTrip publica un nuevo viaje en ruta verificando conductor activo y límite Cap Pricing
// POST /v1/trips ó POST /api/v1/trips
func (h *TripHandler) CreateTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	var req CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondBadRequest(c, "datos de publicación de viaje incompletos o inválidos", "INVALID_REQUEST_PAYLOAD")
		return
	}

	vehicleID, err := domain.ParseUUID(req.VehicleID)
	if err != nil {
		RespondBadRequest(c, "id de viaje o vehículo con formato UUID inválido", "INVALID_VEHICLE_ID")
		return
	}

	if req.PricePerSeat.Amount < 0 {
		RespondBadRequest(c, "el precio por asiento no puede ser negativo", "INVALID_PRICE")
		return
	}

	tripCurrency := req.PricePerSeat.Currency
	if strings.TrimSpace(string(req.Currency)) != "" {
		parsedCur, errCur := domain.ParseCurrency(string(req.Currency))
		if errCur != nil {
			respondError(c, errCur)
			return
		}
		if tripCurrency != "" && tripCurrency != parsedCur {
			respondError(c, domain.ErrCurrencyMismatch)
			return
		}
		tripCurrency = parsedCur
	}
	if tripCurrency != "" {
		req.PricePerSeat.Currency = tripCurrency
	}

	origCoords, err := domain.NewCoordinates(req.OriginLat, req.OriginLon)
	if err != nil {
		RespondBadRequest(c, "coordenadas de origen inválidas", "INVALID_COORDINATES")
		return
	}

	destCoords, err := domain.NewCoordinates(req.DestinationLat, req.DestinationLon)
	if err != nil {
		RespondBadRequest(c, "coordenadas de destino inválidas", "INVALID_COORDINATES")
		return
	}

	var stops []ports.CreateTripStopInput
	for _, s := range req.Stops {
		stopCoords, errStop := domain.NewCoordinates(s.Lat, s.Lon)
		if errStop != nil {
			RespondBadRequest(c, "coordenadas de parada intermedia inválidas", "INVALID_COORDINATES")
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
		VehicleID:         vehicleID,
		OriginTitle:       req.OriginTitle,
		OriginCoords:      origCoords,
		DestinationTitle:  req.DestinationTitle,
		DestinationCoords: destCoords,
		DepartureTime:     req.DepartureTime,
		SeatsOffered:      req.SeatsOffered,
		PricePerSeat:      req.PricePerSeat,
		Currency:          tripCurrency,
		Stops:             stops,
	}

	trip, err := h.tripService.CreateTrip(c.Request.Context(), input)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, CreateTripResponse{
		Message: "Viaje publicado con éxito",
		Trip:    toTripResponse(trip),
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
		RespondBadRequest(c, "Se requieren los parámetros de coordenadas: orig_lat, orig_lon, dest_lat, dest_lon", "MISSING_COORDINATES")
		return
	}

	origLat, err1 := strconv.ParseFloat(origLatStr, 64)
	origLon, err2 := strconv.ParseFloat(origLonStr, 64)
	destLat, err3 := strconv.ParseFloat(destLatStr, 64)
	destLon, err4 := strconv.ParseFloat(destLonStr, 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		RespondBadRequest(c, "Los valores de coordenadas deben ser números decimales válidos", "INVALID_COORDINATES")
		return
	}

	origCoords, err := domain.NewCoordinates(origLat, origLon)
	if err != nil {
		RespondBadRequest(c, "Coordenadas de origen fuera de rango", "INVALID_COORDINATES")
		return
	}

	destCoords, err := domain.NewCoordinates(destLat, destLon)
	if err != nil {
		RespondBadRequest(c, "Coordenadas de destino fuera de rango", "INVALID_COORDINATES")
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
		respondError(c, err)
		return
	}

	mappedResults := toTripSearchResultResponseList(results)
	c.JSON(http.StatusOK, SearchTripsResponse{
		Results: mappedResults,
		Count:   len(mappedResults),
	})
}

// GetTripByID obtiene el detalle integral del viaje con desglose de costos
// GET /v1/trips/:id ó GET /api/v1/trips/:id
func (h *TripHandler) GetTripByID(c *gin.Context) {
	rawTripID := strings.TrimSpace(c.Param("id"))
	if rawTripID == "" {
		RespondBadRequest(c, "ID de viaje no proporcionado", "MISSING_TRIP_ID")
		return
	}
	tripID, err := domain.ParseUUID(rawTripID)
	if err != nil {
		RespondBadRequest(c, "ID de viaje con formato UUID inválido", "INVALID_TRIP_ID")
		return
	}

	detail, err := h.tripService.GetTripByID(c.Request.Context(), tripID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, toTripDetailResponse(detail))
}

// CancelTrip cancela un viaje publicado si pertenece al conductor autenticado
// DELETE /v1/trips/:id ó DELETE /api/v1/trips/:id
func (h *TripHandler) CancelTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	rawTripID := strings.TrimSpace(c.Param("id"))
	if rawTripID == "" {
		RespondBadRequest(c, "ID de viaje no proporcionado", "MISSING_TRIP_ID")
		return
	}
	tripID, err := domain.ParseUUID(rawTripID)
	if err != nil {
		RespondBadRequest(c, "ID de viaje con formato UUID inválido", "INVALID_TRIP_ID")
		return
	}

	if err := h.tripService.CancelTrip(c.Request.Context(), tripID, driverID); err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, CancelTripResponse{
		Message: "Viaje cancelado exitosamente",
		Status:  domain.TripStatusCancelled,
	})
}

// CompleteTrip concluye el viaje por parte del conductor titular, completa las reservas y libera custodia
// POST /v1/trips/:id/complete ó POST /api/v1/trips/:id/complete
func (h *TripHandler) CompleteTrip(c *gin.Context) {
	driverID, exists := middleware.GetUserID(c)
	if !exists {
		RespondUnauthorized(c, "usuario no autenticado", "UNAUTHORIZED")
		return
	}

	rawTripID := strings.TrimSpace(c.Param("id"))
	if rawTripID == "" {
		RespondBadRequest(c, "ID de viaje no proporcionado", "MISSING_TRIP_ID")
		return
	}
	tripID, err := domain.ParseUUID(rawTripID)
	if err != nil {
		RespondBadRequest(c, "ID de viaje con formato UUID inválido", "INVALID_TRIP_ID")
		return
	}

	trip, err := h.tripService.CompleteTrip(c.Request.Context(), tripID, driverID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, CompleteTripResponse{
		Message: "Viaje finalizado exitosamente y liquidación procesada",
		Status:  domain.TripStatusCompleted,
		Trip:    toTripResponse(trip),
	})
}
