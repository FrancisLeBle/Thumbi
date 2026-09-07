package postgres_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/adapters/repositories/postgres"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
	"github.com/thumbi/auth-kyc-service/internal/core/ports"
)

// MockDBExecutor implementa la interfaz postgres.DBExecutor para tests unitarios/aislados
type MockDBExecutor struct {
	mock.Mock
}

func (m *MockDBExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	args := m.Called(ctx, sql, arguments)
	return args.Get(0).(pgconn.CommandTag), args.Error(1)
}

func (m *MockDBExecutor) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	callArgs := m.Called(ctx, sql, args)
	if callArgs.Get(0) == nil {
		return nil, callArgs.Error(1)
	}
	return callArgs.Get(0).(pgx.Rows), callArgs.Error(1)
}

func (m *MockDBExecutor) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	callArgs := m.Called(ctx, sql, args)
	return callArgs.Get(0).(pgx.Row)
}

// MockRow simula el escaneo de una fila pgx.Row
type MockRow struct {
	scanFn func(dest ...any) error
}

func (r *MockRow) Scan(dest ...any) error {
	return r.scanFn(dest...)
}

// MockRows simula pgx.Rows para consultas de múltiples filas
type MockRows struct {
	data    [][]any
	index   int
	closed  bool
	errVal  error
}

func NewMockRows(data [][]any) *MockRows {
	return &MockRows{
		data:  data,
		index: -1,
	}
}

func (r *MockRows) Close() {
	r.closed = true
}

func (r *MockRows) Err() error {
	return r.errVal
}

func (r *MockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("")
}

func (r *MockRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (r *MockRows) Next() bool {
	r.index++
	return r.index < len(r.data)
}

func (r *MockRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.data) {
		return fmt.Errorf("no more rows to scan")
	}
	row := r.data[r.index]
	for i, val := range row {
		if i >= len(dest) {
			break
		}
		switch target := dest[i].(type) {
		case *string:
			*target = fmt.Sprintf("%v", val)
		case **string:
			if val == nil {
				*target = nil
			} else if s, ok := val.(string); ok {
				*target = &s
			} else if sp, ok := val.(*string); ok {
				*target = sp
			}
		case *float64:
			*target = val.(float64)
		case *int:
			*target = val.(int)
		case *time.Time:
			*target = val.(time.Time)
		case **time.Time:
			if val == nil {
				*target = nil
			} else if t, ok := val.(time.Time); ok {
				*target = &t
			} else if tp, ok := val.(*time.Time); ok {
				*target = tp
			}
		case *[]string:
			if val == nil {
				*target = []string{}
			} else if arr, ok := val.([]string); ok {
				*target = arr
			}
		default:
			return fmt.Errorf("unsupported mock scan destination type: %T", target)
		}
	}
	return nil
}

func (r *MockRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.data) {
		return nil, fmt.Errorf("index out of range")
	}
	return r.data[r.index], nil
}

func (r *MockRows) RawValues() [][]byte {
	return nil
}

func (r *MockRows) Conn() *pgx.Conn {
	return nil
}

func TestTripRepository_Save(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	orig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	dest, _ := domain.NewCoordinates(-32.9468, -60.6393)
	routeLine, _ := domain.NewLineString([]domain.Coordinates{orig, dest})

	stopCoord, _ := domain.NewCoordinates(-33.8000, -59.5000)
	stop, _ := domain.NewTripStop("stop-1", "trip-1", 1, "San Nicolás", stopCoord, now.Add(2*time.Hour))

	params := domain.TripCreationParams{
		ID:                   "trip-1",
		DriverID:             "driver-1",
		VehicleID:            "vehicle-1",
		OriginTitle:          "Buenos Aires",
		OriginCoords:         orig,
		DestinationTitle:     "Rosario",
		DestinationCoords:    dest,
		RoutePath:            routeLine,
		DepartureTime:        now.Add(24 * time.Hour),
		EstimatedArrivalTime: now.Add(28 * time.Hour),
		TotalDistanceKm:      300.0,
		TotalDurationMinutes: 240,
		TotalSeatsOffered:    3,
		MaxVehicleSeats:      4,
		PricePerSeat:         10000.0,
		EstimatedFuelCost:    28050.0,
		EstimatedTollCost:    3000.0,
		Stops:                []domain.TripStop{*stop},
	}

	trip, err := domain.NewTrip(params, now)
	require.NoError(t, err)

	t.Run("inserta atómicamente el viaje y sus paradas intermedias convirtiendo WKT a PostGIS", func(t *testing.T) {
		mockDB := new(MockDBExecutor)

		// 1. Verificación del query de trips
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trips") &&
				strings.Contains(sql, "ST_GeomFromText($5, 4326)") &&
				strings.Contains(sql, "ST_GeomFromText($7, 4326)") &&
				strings.Contains(sql, "ST_GeomFromText($8, 4326)")
		}), mock.MatchedBy(func(args []any) bool {
			// Validar conversión a WKT
			origWKT, ok1 := args[4].(string)
			destWKT, ok2 := args[6].(string)
			routeWKT, ok3 := args[7].(string)

			return ok1 && ok2 && ok3 &&
				strings.HasPrefix(origWKT, "POINT(") &&
				strings.HasPrefix(destWKT, "POINT(") &&
				strings.HasPrefix(routeWKT, "LINESTRING(")
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		// 2. Verificación del query de trip_stops
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trip_stops") &&
				strings.Contains(sql, "ST_GeomFromText($5, 4326)")
		}), mock.MatchedBy(func(args []any) bool {
			stopWKT, ok := args[4].(string)
			return ok && strings.HasPrefix(stopWKT, "POINT(")
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		repo := postgres.NewTripRepository(mockDB)
		err := repo.Save(ctx, trip)

		require.NoError(t, err)
		mockDB.AssertExpectations(t)
	})
}

func TestTripRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	t.Run("recupera y reconstruye geometrías desde WKT de PostGIS", func(t *testing.T) {
		mockDB := new(MockDBExecutor)

		row := &MockRow{
			scanFn: func(dest ...any) error {
				*(dest[0].(*string)) = "trip-123"
				*(dest[1].(*string)) = "driver-123"
				*(dest[2].(*string)) = "vehicle-123"
				*(dest[3].(*string)) = "Buenos Aires"
				*(dest[4].(*string)) = "POINT(-58.381600 -34.603700)"
				*(dest[5].(*string)) = "Rosario"
				*(dest[6].(*string)) = "POINT(-60.639300 -32.946800)"
				*(dest[7].(*string)) = "LINESTRING(-58.381600 -34.603700, -60.639300 -32.946800)"
				*(dest[8].(*time.Time)) = now.Add(24 * time.Hour)
				*(dest[9].(*time.Time)) = now.Add(28 * time.Hour)
				*(dest[10].(*float64)) = 300.0
				*(dest[11].(*int)) = 240
				*(dest[12].(*int)) = 3
				*(dest[13].(*int)) = 3
				*(dest[14].(*float64)) = 11000.0
				*(dest[15].(*float64)) = 11385.0
				*(dest[16].(*float64)) = 28050.0
				*(dest[17].(*float64)) = 3000.0
				*(dest[18].(*string)) = "PUBLISHED"
				*(dest[19].(*time.Time)) = now
				*(dest[20].(*time.Time)) = now
				return nil
			},
		}

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") &&
				strings.Contains(sql, "ST_AsText(origin_geom)") &&
				strings.Contains(sql, "ST_AsText(route_path)")
		}), mock.Anything).Return(row)

		emptyStopsRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trip_stops")
		}), mock.Anything).Return(emptyStopsRows, nil)

		repo := postgres.NewTripRepository(mockDB)
		trip, err := repo.FindByID(ctx, "trip-123")

		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, "trip-123", trip.ID)
		assert.InDelta(t, -34.6037, trip.OriginCoords.Latitude, 0.001)
		assert.InDelta(t, -58.3816, trip.OriginCoords.Longitude, 0.001)
		assert.InDelta(t, -32.9468, trip.DestinationCoords.Latitude, 0.001)
		assert.InDelta(t, -60.6393, trip.DestinationCoords.Longitude, 0.001)
		assert.Len(t, trip.RoutePath.Coordinates, 2)
		assert.Equal(t, domain.TripStatusPublished, trip.Status)
	})

	t.Run("devuelve ErrTripNotFound si la fila no existe", func(t *testing.T) {
		mockDB := new(MockDBExecutor)

		row := &MockRow{
			scanFn: func(dest ...any) error {
				return pgx.ErrNoRows
			},
		}

		mockDB.On("QueryRow", ctx, mock.Anything, mock.Anything).Return(row)

		repo := postgres.NewTripRepository(mockDB)
		trip, err := repo.FindByID(ctx, "non-existent")

		assert.Nil(t, trip)
		assert.ErrorIs(t, err, domain.ErrTripNotFound)
	})
}

func TestTripRepository_SearchMatchingTrips(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()

	orig, _ := domain.NewCoordinates(-34.6037, -58.3816)
	dest, _ := domain.NewCoordinates(-32.9468, -60.6393)

	params := ports.SearchTripsParams{
		OriginCoords:    orig,
		DestCoords:      dest,
		ToleranceMeters: 10000.0,
		DepartureFrom:   now,
		DepartureTo:     now.Add(48 * time.Hour),
		RequiredSeats:   1,
		Limit:           10,
		Offset:          0,
	}

	t.Run("ejecuta consulta CTE con ST_DWithin y mapea los resultados correctamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)

		mockData := [][]any{
			{
				"trip-matched-1",
				"driver-99",
				"Esteban",
				"Quito",
				"https://example.com/pic.jpg",
				"Toyota",
				"Corolla",
				"AF123JK",
				"Buenos Aires",
				"Rosario",
				"POINT(-58.381600 -34.603700)",
				"POINT(-60.639300 -32.946800)",
				"LINESTRING(-58.381600 -34.603700, -60.639300 -32.946800)",
				now.Add(24 * time.Hour),
				now.Add(28 * time.Hour),
				300.0,
				240,
				3,
				2,
				10500.0,
				11385.0,
				28050.0,
				3000.0,
				350.0,
				720.0,
				"PUBLISHED",
				now,
				now,
			},
		}

		mockRows := NewMockRows(mockData)

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "passenger_input") &&
				strings.Contains(sql, "ST_DWithin(t.route_path::geography, p.orig_pt::geography, $5)") &&
				strings.Contains(sql, "ST_LineLocatePoint") &&
				strings.Contains(sql, "ORDER BY")
		}), mock.MatchedBy(func(args []any) bool {
			// Validar paso de longitud/latitud para ST_MakePoint ($1: lon, $2: lat)
			lon1 := args[0].(float64)
			lat1 := args[1].(float64)
			return lon1 == orig.Longitude && lat1 == orig.Latitude
		})).Return(mockRows, nil)

		repo := postgres.NewTripRepository(mockDB)
		records, err := repo.SearchMatchingTrips(ctx, params)

		require.NoError(t, err)
		assert.Len(t, records, 1)

		rec := records[0]
		assert.Equal(t, "trip-matched-1", rec.Trip.ID)
		assert.Equal(t, "Esteban", rec.DriverFirstName)
		assert.Equal(t, "Quito", rec.DriverLastName)
		assert.Equal(t, "Toyota", rec.VehicleBrand)
		assert.Equal(t, 350.0, rec.PickupWalkDistanceM)
		assert.Equal(t, 720.0, rec.DropoffWalkDistanceM)
		assert.Equal(t, domain.TripStatusPublished, rec.Trip.Status)
		assert.Len(t, rec.Trip.RoutePath.Coordinates, 2)
	})
}

func TestTripRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()

	t.Run("actualiza el estado exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips SET status = $1")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		repo := postgres.NewTripRepository(mockDB)
		err := repo.UpdateStatus(ctx, "trip-1", domain.TripStatusCancelled)
		require.NoError(t, err)
	})

	t.Run("retorna ErrTripNotFound si no se modificó ninguna fila", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		repo := postgres.NewTripRepository(mockDB)
		err := repo.UpdateStatus(ctx, "trip-non-existent", domain.TripStatusCancelled)
		assert.ErrorIs(t, err, domain.ErrTripNotFound)
	})
}
