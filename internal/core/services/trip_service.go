package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// tripService implementa ports.TripService bajo Clean Architecture
type tripService struct {
	tripRepo          ports.TripRepository
	userRepo          ports.UserRepository
	vehicleRepo       ports.VehicleRepository
	routingProvider   ports.RoutingProvider
	pricingCalculator PricingCalculator
	bookingRepo       ports.BookingRepository
	escrowRepo        ports.EscrowRepository
	clock             func() time.Time
}

// NewTripService construye una nueva instancia inyectando los puertos y calculador requeridos
func NewTripService(
	tripRepo ports.TripRepository,
	userRepo ports.UserRepository,
	vehicleRepo ports.VehicleRepository,
	routingProvider ports.RoutingProvider,
	pricingCalculator PricingCalculator,
	optionalRepos ...any,
) ports.TripService {
	if pricingCalculator == nil {
		pricingCalculator = NewPricingCalculator()
	}
	svc := &tripService{
		tripRepo:          tripRepo,
		userRepo:          userRepo,
		vehicleRepo:       vehicleRepo,
		routingProvider:   routingProvider,
		pricingCalculator: pricingCalculator,
		clock:             time.Now,
	}

	for _, opt := range optionalRepos {
		switch r := opt.(type) {
		case ports.BookingRepository:
			svc.bookingRepo = r
		case ports.EscrowRepository:
			svc.escrowRepo = r
		}
	}

	return svc
}

// CreateTrip publica un nuevo viaje en ruta verificando el rol de conductor y el tope de Cap Pricing
func (s *tripService) CreateTrip(ctx context.Context, input ports.CreateTripInput) (*domain.Trip, error) {
	now := s.clock().UTC()

	// 1. Validar que el usuario exista y tenga el rol de conductor activo y verificado (RF-01)
	driver, err := s.userRepo.GetByID(ctx, input.DriverID)
	if err != nil || driver == nil {
		return nil, domain.ErrUserNotFound
	}
	if !driver.IsDriverActive {
		return nil, domain.ErrDriverNotActive
	}

	// 2. Validar que el vehículo exista, pertenezca al conductor y esté en estado APPROVED (RF-02)
	vehicle, err := s.vehicleRepo.GetByID(ctx, input.VehicleID)
	if err != nil || vehicle == nil {
		return nil, domain.ErrVehicleNotFound
	}
	if vehicle.Status != domain.VehicleStatusApproved {
		return nil, domain.ErrVehicleNotAvailable
	}
	if vehicle.UserID != input.DriverID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Validar coordenadas de origen y destino
	if !input.OriginCoords.IsValid() || !input.DestinationCoords.IsValid() {
		return nil, domain.ErrInvalidRouteCoordinates
	}

	// 4. Parsear y validar fecha de salida
	departureTime, err := parseFlexibleTimestamp(input.DepartureTime)
	if err != nil {
		return nil, domain.ErrDepartureTimeMustBeFuture
	}
	if !departureTime.After(now) {
		return nil, domain.ErrDepartureTimeMustBeFuture
	}

	// 5. Preparar waypoints para el cálculo de ruteo
	var waypoints []domain.Coordinates
	for _, stop := range input.Stops {
		if !stop.Coords.IsValid() {
			return nil, domain.ErrInvalidRouteCoordinates
		}
		waypoints = append(waypoints, stop.Coords)
	}

	// 6. Consultar trazado y distancias al motor cartográfico (RoutingProvider)
	routeReq := ports.RouteCalculationRequest{
		Origin:      input.OriginCoords,
		Destination: input.DestinationCoords,
		Waypoints:   waypoints,
	}

	routeDetails, err := s.routingProvider.CalculateRoute(ctx, routeReq)
	if err != nil {
		return nil, fmt.Errorf("error al calcular trazado de ruta: %w", err)
	}

	// Si el proveedor no entregó trazado válido, construir polilínea directa
	routePath := routeDetails.RoutePath
	if len(routePath.Coordinates) < 2 {
		points := []domain.Coordinates{input.OriginCoords}
		points = append(points, waypoints...)
		points = append(points, input.DestinationCoords)
		routePath, err = domain.NewLineString(points)
		if err != nil {
			return nil, err
		}
	}

	distanceKm := routeDetails.DistanceKm
	if distanceKm <= 0 {
		distanceKm = routePath.TotalApproximatedDistanceKm()
	}

	durationMinutes := routeDetails.DurationMinutes
	if durationMinutes <= 0 {
		// Estimación fallback a 80 km/h promedio
		durationMinutes = int((distanceKm / 80.0) * 60)
		if durationMinutes < 15 {
			durationMinutes = 15
		}
	}

	estimatedArrival := departureTime.Add(time.Duration(durationMinutes) * time.Minute)

	// 7. Calcular matriz de costos compartidos y Cap Price (RF-05, RF-06)
	costMatrix := s.pricingCalculator.CalculatePricing(distanceKm, routeDetails.EstimatedTollCost, input.SeatsOffered)

	// Invariante de negocio: No superar el Cap Price (RF-07)
	if !costMatrix.IsPricePermitted(input.PricePerSeat) {
		return nil, domain.ErrPriceExceedsCapPrice
	}

	tripID := uuid.New().String()

	// 8. Construir paradas intermedias
	var domainStops []domain.TripStop
	for i, stopInput := range input.Stops {
		stopETA, errParse := parseFlexibleTimestamp(stopInput.EstimatedArrivalTime)
		if errParse != nil {
			// Aproximar proporcionalmente
			fraction := float64(i+1) / float64(len(input.Stops)+1)
			stopETA = departureTime.Add(time.Duration(float64(durationMinutes)*fraction) * time.Minute)
		}
		order := stopInput.StopOrder
		if order <= 0 {
			order = i + 1
		}
		stopObj, errStop := domain.NewTripStop(uuid.New().String(), tripID, order, stopInput.LocationTitle, stopInput.Coords, stopETA)
		if errStop != nil {
			return nil, errStop
		}
		domainStops = append(domainStops, *stopObj)
	}

	// 9. Instanciar la entidad Trip con validación estricta de invariantes
	params := domain.TripCreationParams{
		ID:                   tripID,
		DriverID:             input.DriverID,
		VehicleID:            input.VehicleID,
		OriginTitle:          input.OriginTitle,
		OriginCoords:         input.OriginCoords,
		DestinationTitle:     input.DestinationTitle,
		DestinationCoords:    input.DestinationCoords,
		RoutePath:            routePath,
		DepartureTime:        departureTime,
		EstimatedArrivalTime: estimatedArrival,
		TotalDistanceKm:      distanceKm,
		TotalDurationMinutes: durationMinutes,
		TotalSeatsOffered:    input.SeatsOffered,
		MaxVehicleSeats:      vehicle.SeatCapacity,
		PricePerSeat:         input.PricePerSeat,
		EstimatedFuelCost:    costMatrix.FuelCost,
		EstimatedTollCost:    costMatrix.TollCost,
		Stops:                domainStops,
	}

	trip, err := domain.NewTrip(params, now)
	if err != nil {
		return nil, err
	}

	// 10. Persistir en base de datos con PostGIS (RF-08, RF-09)
	if err := s.tripRepo.Save(ctx, trip); err != nil {
		return nil, fmt.Errorf("error al persistir el viaje: %w", err)
	}

	return trip, nil
}

// SearchTrips ejecuta la búsqueda geoespacial sobre polilíneas en PostGIS (RF-11 a RF-15)
func (s *tripService) SearchTrips(ctx context.Context, query ports.SearchTripsQuery) ([]ports.TripSearchResult, error) {
	// 1. Validar coordenadas de consulta
	if !query.OriginCoords.IsValid() || !query.DestCoords.IsValid() {
		return nil, domain.ErrInvalidRouteCoordinates
	}

	// 2. Parámetros de tolerancia y paginación con valores seguros por defecto
	toleranceMeters := query.ToleranceMeters
	if toleranceMeters <= 0 {
		toleranceMeters = 10000.0 // 10 km por defecto
	}

	requiredSeats := query.RequiredSeats
	if requiredSeats <= 0 {
		requiredSeats = 1
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}

	offset := query.Offset
	if offset < 0 {
		offset = 0
	}

	// 3. Establecer ventana horaria de partida
	var departureFrom, departureTo time.Time
	if strings.TrimSpace(query.DepartureDate) != "" {
		parsedDate, err := time.Parse("2006-01-02", strings.TrimSpace(query.DepartureDate))
		if err == nil {
			departureFrom = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 0, 0, 0, 0, time.UTC)
			departureTo = time.Date(parsedDate.Year(), parsedDate.Month(), parsedDate.Day(), 23, 59, 59, 999999999, time.UTC)
		}
	}

	// Si no se especificó fecha válida, buscar a partir del instante actual hasta 30 días
	if departureFrom.IsZero() {
		now := s.clock().UTC()
		departureFrom = now
		departureTo = now.Add(30 * 24 * time.Hour)
	}

	// 4. Invocar el repositorio con la consulta PostGIS
	searchParams := ports.SearchTripsParams{
		OriginCoords:    query.OriginCoords,
		DestCoords:      query.DestCoords,
		ToleranceMeters: toleranceMeters,
		DepartureFrom:   departureFrom,
		DepartureTo:     departureTo,
		RequiredSeats:   requiredSeats,
		Limit:           limit,
		Offset:          offset,
	}

	matchedRecords, err := s.tripRepo.SearchMatchingTrips(ctx, searchParams)
	if err != nil {
		return nil, fmt.Errorf("error en la búsqueda geoespacial: %w", err)
	}

	// 5. Mapear resultados a DTOs de salida
	results := make([]ports.TripSearchResult, len(matchedRecords))
	for i, r := range matchedRecords {
		fullName := strings.TrimSpace(fmt.Sprintf("%s %s", r.DriverFirstName, r.DriverLastName))
		results[i] = ports.TripSearchResult{
			ID:                   r.Trip.ID,
			DriverID:             r.Trip.DriverID,
			DriverFullName:       fullName,
			DriverAvatarURL:      r.DriverAvatarURL,
			VehicleBrand:         r.VehicleBrand,
			VehicleModel:         r.VehicleModel,
			VehiclePlate:         r.VehiclePlate,
			OriginTitle:          r.Trip.OriginTitle,
			DestinationTitle:     r.Trip.DestinationTitle,
			DepartureTime:        r.Trip.DepartureTime.Format(time.RFC3339),
			EstimatedArrivalTime: r.Trip.EstimatedArrivalTime.Format(time.RFC3339),
			TotalDistanceKm:      r.Trip.TotalDistanceKm,
			TotalDurationMinutes: r.Trip.TotalDurationMinutes,
			AvailableSeats:       r.Trip.AvailableSeats,
			PricePerSeat:         r.Trip.PricePerSeat,
			CapPricePerSeat:      r.Trip.CapPricePerSeat,
			PickupWalkDistanceM:  r.PickupWalkDistanceM,
			DropoffWalkDistanceM: r.DropoffWalkDistanceM,
			Status:               r.Trip.Status,
		}
	}

	return results, nil
}

// GetTripByID obtiene el detalle integral del viaje y el desglose de costos
func (s *tripService) GetTripByID(ctx context.Context, tripID string) (*ports.TripDetailDTO, error) {
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	driver, _ := s.userRepo.GetByID(ctx, trip.DriverID)
	vehicle, _ := s.vehicleRepo.GetByID(ctx, trip.VehicleID)

	costMatrix := domain.CalculateCapPrice(trip.EstimatedFuelCost, trip.EstimatedTollCost, trip.TotalSeatsOffered)

	return &ports.TripDetailDTO{
		Trip:       trip,
		Driver:     driver,
		Vehicle:    vehicle,
		CostMatrix: costMatrix,
	}, nil
}

// CancelTrip cancela un viaje publicado si pertenece al conductor (RF-16)
func (s *tripService) CancelTrip(ctx context.Context, tripID, driverID string) error {
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil || trip == nil {
		return domain.ErrTripNotFound
	}

	if trip.DriverID != driverID {
		return domain.ErrUnauthorized
	}

	if err := trip.Cancel(); err != nil {
		return err
	}

	if err := s.tripRepo.UpdateStatus(ctx, tripID, domain.TripStatusCancelled); err != nil {
		return fmt.Errorf("error al actualizar estado de cancelación: %w", err)
	}

	return nil
}

// CompleteTrip finaliza un viaje activo, actualiza las reservas confirmadas a COMPLETED y libera los fondos en custodia hacia el conductor (Módulo 4: RF-01 a RF-05)
func (s *tripService) CompleteTrip(ctx context.Context, tripID, driverID string) (*domain.Trip, error) {
	now := s.clock().UTC()

	// 1. Validar que el viaje exista
	trip, err := s.tripRepo.FindByID(ctx, tripID)
	if err != nil || trip == nil {
		return nil, domain.ErrTripNotFound
	}

	// 2. Validar que el ejecutor sea el conductor del viaje
	if trip.DriverID != driverID {
		return nil, domain.ErrUnauthorized
	}

	// 3. Validar que el viaje esté en un estado finalizable (IN_PROGRESS, PUBLISHED o FULL)
	if trip.Status != domain.TripStatusInProgress && trip.Status != domain.TripStatusPublished && trip.Status != domain.TripStatusFull {
		return nil, domain.ErrInvalidTripStatusChange
	}

	// 4. Transicionar estado del viaje a COMPLETED
	if err := trip.TransitionStatus(domain.TripStatusCompleted); err != nil {
		return nil, err
	}

	if err := s.tripRepo.UpdateStatus(ctx, tripID, domain.TripStatusCompleted); err != nil {
		return nil, fmt.Errorf("error al actualizar estado del viaje a finalizado: %w", err)
	}

	// 5. Gestionar reservas asociadas y liberación de fondos en custodia
	if s.bookingRepo != nil {
		bookings, err := s.bookingRepo.GetActiveBookingsByTrip(ctx, tripID)
		if err == nil {
			for _, booking := range bookings {
				if booking.Status == domain.BookingStatusConfirmed {
					// Actualizar estado de la reserva a COMPLETED
					_ = booking.MarkCompleted(now)
					_ = s.bookingRepo.UpdateStatus(ctx, booking.ID, domain.BookingStatusCompleted, nil)

					// Liberación automática de fondos en custodia (HELD -> RELEASED)
					if s.escrowRepo != nil {
						escrow, err := s.escrowRepo.FindByBookingID(ctx, booking.ID)
						if err == nil && escrow != nil && (escrow.Status == domain.EscrowStatusHeld || escrow.Status == domain.EscrowStatusDisputed) {
							if err := escrow.Release(now); err == nil {
								_ = s.escrowRepo.UpdateEscrowStatus(ctx, escrow.ID, domain.EscrowStatusReleased)
							}
						}
					}
				}
			}
		}
	}

	return trip, nil
}

// parseFlexibleTimestamp intenta parsear cadenas de tiempo en varios formatos usuales
func parseFlexibleTimestamp(val string) (time.Time, error) {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return time.Time{}, domain.ErrDepartureTimeMustBeFuture
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, trimmed); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, domain.ErrDepartureTimeMustBeFuture
}
