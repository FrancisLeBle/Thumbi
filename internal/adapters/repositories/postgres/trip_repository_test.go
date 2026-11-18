package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/adapters/repositories/postgres"
	"github.com/thumbi/backend/internal/core/domain"
	"github.com/thumbi/backend/internal/core/ports"
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
		case *uuid.UUID:
			switch v := val.(type) {
			case uuid.UUID:
				*target = v
			case string:
				*target = uuid.MustParse(v)
			default:
				return fmt.Errorf("unsupported mock uuid source type: %T", val)
			}
		case **uuid.UUID:
			if val == nil {
				*target = nil
			} else if u, ok := val.(uuid.UUID); ok {
				*target = &u
			} else if up, ok := val.(*uuid.UUID); ok {
				*target = up
			} else {
				return fmt.Errorf("unsupported mock *uuid source type: %T", val)
			}
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
		case *pgtype.Numeric:
			switch v := val.(type) {
			case pgtype.Numeric:
				*target = v
			case domain.Money:
				*target = pgtype.Numeric{Int: big.NewInt(v.Amount), Exp: -2, Valid: true}
			case int64:
				*target = pgtype.Numeric{Int: big.NewInt(v), Exp: -2, Valid: true}
			default:
				return fmt.Errorf("unsupported mock pgtype.Numeric source type: %T", val)
			}
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
		case *[]float64:
			if val == nil {
				*target = []float64{}
			} else if arr, ok := val.([]float64); ok {
				*target = append([]float64(nil), arr...)
			} else {
				return fmt.Errorf("unsupported mock []float64 source type: %T", val)
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

	stopID := uuid.MustParse("01933e3d-0000-7000-8000-000000000010")
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000002")
	vehicleID := uuid.MustParse("01933e3d-0000-7000-8000-000000000003")

	stopCoord, _ := domain.NewCoordinates(-33.8000, -59.5000)
	stop, _ := domain.NewTripStop(stopID, tripID, 1, "San Nicolás", stopCoord, now.Add(2*time.Hour))

	params := domain.TripCreationParams{
		ID:                   tripID,
		DriverID:             driverID,
		VehicleID:            vehicleID,
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
		Currency:             domain.CurrencyARS,
		PricePerSeat:         domain.MoneyFromCents(1000000, domain.CurrencyARS),
		EstimatedFuelCost:    domain.MoneyFromCents(2805000, domain.CurrencyARS),
		EstimatedTollCost:    domain.MoneyFromCents(300000, domain.CurrencyARS),
		Stops:                []domain.TripStop{*stop},
	}

	trip, err := domain.NewTrip(params, now)
	require.NoError(t, err)

	t.Run("inserta atómicamente el viaje y sus paradas intermedias en transacción y hace Commit", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		beginner.On("Begin", ctx).Return(tx, nil)

		// 1. Verificación del query de trips con constructores nativos PostGIS
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trips") &&
				strings.Contains(sql, "ST_SetSRID(ST_MakePoint($5, $6), 4326)") &&
				strings.Contains(sql, "ST_SetSRID(ST_MakePoint($8, $9), 4326)") &&
				strings.Contains(sql, "ST_MakeLine") &&
				!strings.Contains(sql, "ST_GeomFromText")
		}), mock.MatchedBy(func(args []any) bool {
			origLon, ok1 := args[4].(float64)
			origLat, ok2 := args[5].(float64)
			destLon, ok3 := args[7].(float64)
			destLat, ok4 := args[8].(float64)
			routeLons, ok5 := args[9].([]float64)
			routeLats, ok6 := args[10].([]float64)

			return ok1 && ok2 && ok3 && ok4 && ok5 && ok6 &&
				origLon == orig.Longitude && origLat == orig.Latitude &&
				destLon == dest.Longitude && destLat == dest.Latitude &&
				len(routeLons) == 2 && len(routeLats) == 2
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		// 2. Verificación del query de trip_stops con ST_MakePoint nativo
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trip_stops") &&
				strings.Contains(sql, "ST_SetSRID(ST_MakePoint($5, $6), 4326)") &&
				!strings.Contains(sql, "ST_GeomFromText")
		}), mock.MatchedBy(func(args []any) bool {
			stopLon, ok1 := args[4].(float64)
			stopLat, ok2 := args[5].(float64)
			return ok1 && ok2 && stopLon == stopCoord.Longitude && stopLat == stopCoord.Latitude
		})).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)

		txExec.On("Commit", ctx).Return(nil)

		repo := postgres.NewTripRepository(poolDB, uow)
		err := repo.Save(ctx, trip)

		require.NoError(t, err)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("ejecuta Rollback si falla la inserción de paradas intermedias", func(t *testing.T) {
		poolDB := new(MockDBExecutor)
		txExec := new(MockDBExecutor)
		tx := &MockTx{MockDBExecutor: txExec}
		beginner := new(MockTxBeginner)
		uow := postgres.NewUnitOfWork(beginner)

		stopErr := errors.New("foreign key violation on trip_stops")
		beginner.On("Begin", ctx).Return(tx, nil)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trips")
		}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 0 1"), nil)
		txExec.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "INSERT INTO trip_stops")
		}), mock.Anything).Return(pgconn.NewCommandTag("INSERT 0 0"), stopErr)
		txExec.On("Rollback", ctx).Return(nil)

		repo := postgres.NewTripRepository(poolDB, uow)
		err := repo.Save(ctx, trip)

		assert.ErrorIs(t, err, stopErr)
		txExec.AssertNotCalled(t, "Commit", mock.Anything)
		poolDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
		beginner.AssertExpectations(t)
		txExec.AssertExpectations(t)
	})

	t.Run("falla explícitamente si no hay UnitOfWork ni TxBeginner configurado", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewTripRepository(mockDB)

		err := repo.Save(ctx, trip)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "UnitOfWork")
		mockDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	})
}

func TestTripRepository_FindByID(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000123")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000124")
	vehicleID := uuid.MustParse("01933e3d-0000-7000-8000-000000000125")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	t.Run("recupera y reconstruye geometrías desde coordenadas y vértices nativos de PostGIS", func(t *testing.T) {
		mockDB := new(MockDBExecutor)

		row := &MockRow{
			scanFn: func(dest ...any) error {
				*(dest[0].(*uuid.UUID)) = tripID
				*(dest[1].(*uuid.UUID)) = driverID
				*(dest[2].(*uuid.UUID)) = vehicleID
				*(dest[3].(*string)) = "Buenos Aires"
				*(dest[4].(*float64)) = -58.381600
				*(dest[5].(*float64)) = -34.603700
				*(dest[6].(*string)) = "Rosario"
				*(dest[7].(*float64)) = -60.639300
				*(dest[8].(*float64)) = -32.946800
				*(dest[9].(*[]float64)) = []float64{-58.381600, -60.639300}
				*(dest[10].(*[]float64)) = []float64{-34.603700, -32.946800}
				*(dest[11].(*time.Time)) = now.Add(24 * time.Hour)
				*(dest[12].(*time.Time)) = now.Add(28 * time.Hour)
				*(dest[13].(*float64)) = 300.0
				*(dest[14].(*int)) = 240
				*(dest[15].(*int)) = 3
				*(dest[16].(*int)) = 3
				*(dest[17].(*string)) = "ARS"
				*(dest[18].(*pgtype.Numeric)) = pgtype.Numeric{Int: big.NewInt(1100000), Exp: -2, Valid: true}
				*(dest[19].(*pgtype.Numeric)) = pgtype.Numeric{Int: big.NewInt(1138500), Exp: -2, Valid: true}
				*(dest[20].(*pgtype.Numeric)) = pgtype.Numeric{Int: big.NewInt(2805000), Exp: -2, Valid: true}
				*(dest[21].(*pgtype.Numeric)) = pgtype.Numeric{Int: big.NewInt(300000), Exp: -2, Valid: true}
				*(dest[22].(*string)) = "PUBLISHED"
				*(dest[23].(*time.Time)) = now
				*(dest[24].(*time.Time)) = now
				return nil
			},
		}

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") &&
				strings.Contains(sql, "ST_X(origin_geom), ST_Y(origin_geom)") &&
				strings.Contains(sql, "ST_DumpPoints(route_path)") &&
				!strings.Contains(sql, "ST_AsText")
		}), mock.Anything).Return(row)

		emptyStopsRows := NewMockRows([][]any{})
		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trip_stops") &&
				strings.Contains(sql, "ST_X(location_geom), ST_Y(location_geom)") &&
				!strings.Contains(sql, "ST_AsText")
		}), mock.Anything).Return(emptyStopsRows, nil)

		repo := postgres.NewTripRepository(mockDB)
		trip, err := repo.FindByID(ctx, tripID)

		require.NoError(t, err)
		assert.NotNil(t, trip)
		assert.Equal(t, tripID, trip.ID)
		assert.Equal(t, domain.CurrencyARS, trip.Currency)
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
		trip, err := repo.FindByID(ctx, missingID)

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

	t.Run("ejecuta consulta CTE con ST_DWithin usando ST_X/ST_Y y polilínea simplificada sin WKT", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		encodedPreview := "p`~rE~lxbJc~kH|m}M"
		matchedTripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000777")
		driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000099")

		mockData := [][]any{
			{
				matchedTripID,
				driverID,
				"Esteban",
				"Quito",
				"https://example.com/pic.jpg",
				"Toyota",
				"Corolla",
				"AF123JK",
				"Buenos Aires",
				"Rosario",
				-58.381600,
				-34.603700,
				-60.639300,
				-32.946800,
				encodedPreview,
				now.Add(24 * time.Hour),
				now.Add(28 * time.Hour),
				300.0,
				240,
				3,
				2,
				"ARS",
				domain.MoneyFromCents(1050000, domain.CurrencyARS),
				domain.MoneyFromCents(1138500, domain.CurrencyARS),
				domain.MoneyFromCents(2805000, domain.CurrencyARS),
				domain.MoneyFromCents(300000, domain.CurrencyARS),
				350.0,
				720.0,
				"PUBLISHED",
				now,
				now,
			},
		}

		mockRows := NewMockRows(mockData)

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "passenger_input AS NOT MATERIALIZED") &&
				strings.Contains(sql, "ST_DWithin(t.route_path::geography, p.orig_pt::geography, $5)") &&
				strings.Contains(sql, "ST_DWithin(t.route_path::geography, p.dest_pt::geography, $5)") &&
				strings.Contains(sql, "ST_X(r.origin_geom)") &&
				strings.Contains(sql, "ST_Y(r.origin_geom)") &&
				strings.Contains(sql, "ST_AsEncodedPolyline(ST_Simplify(r.route_path, 0.001), 5)") &&
				!strings.Contains(sql, "ST_AsText") &&
				strings.Contains(sql, "ranked_trips") &&
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
		assert.Equal(t, matchedTripID, rec.Trip.ID)
		assert.Equal(t, domain.CurrencyARS, rec.Trip.Currency)
		assert.InDelta(t, -34.6037, rec.Trip.OriginCoords.Latitude, 0.001)
		assert.InDelta(t, -58.3816, rec.Trip.OriginCoords.Longitude, 0.001)
		assert.Equal(t, "Esteban", rec.DriverFirstName)
		assert.Equal(t, "Quito", rec.DriverLastName)
		assert.Equal(t, "Toyota", rec.VehicleBrand)
		assert.Equal(t, 350.0, rec.PickupWalkDistanceM)
		assert.Equal(t, 720.0, rec.DropoffWalkDistanceM)
		assert.Equal(t, encodedPreview, rec.RoutePreviewPolyline)
		assert.Empty(t, rec.Trip.RoutePath.Coordinates, "El listado de búsqueda no debe inflarse con la polilínea completa")
		assert.Equal(t, domain.TripStatusPublished, rec.Trip.Status)
	})
}

func TestTripRepository_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	t.Run("actualiza el estado exitosamente", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips SET status = $1")
		}), mock.Anything).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		repo := postgres.NewTripRepository(mockDB)
		err := repo.UpdateStatus(ctx, tripID, domain.TripStatusCancelled)
		require.NoError(t, err)
	})

	t.Run("retorna ErrTripNotFound si no se modificó ninguna fila", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.Anything, mock.Anything).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		repo := postgres.NewTripRepository(mockDB)
		err := repo.UpdateStatus(ctx, missingID, domain.TripStatusCancelled)
		assert.ErrorIs(t, err, domain.ErrTripNotFound)
	})
}

func TestTripRepository_ReleaseSeats(t *testing.T) {
	ctx := context.Background()
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	missingID := uuid.MustParse("01933e3d-0000-7000-8000-000000000404")

	t.Run("aplica atómicamente la restitución y transición decidida por el dominio reportando applied = true", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips") &&
				strings.Contains(sql, "available_seats = available_seats + $1") &&
				strings.Contains(sql, "status = $2") &&
				strings.Contains(sql, "WHERE id = $3") &&
				strings.Contains(sql, "AND status = $4") &&
				strings.Contains(sql, "available_seats + $1 <= total_seats_offered") &&
				!strings.Contains(sql, "CASE WHEN")
		}), []any{2, "PUBLISHED", tripID, "FULL"}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		repo := postgres.NewTripRepository(mockDB)
		applied, err := repo.ReleaseSeats(ctx, domain.SeatTransition{
			TripID:         tripID,
			SeatsDelta:     2,
			ExpectedStatus: domain.TripStatusFull,
			NewStatus:      domain.TripStatusPublished,
		})
		require.NoError(t, err)
		assert.True(t, applied)
		mockDB.AssertExpectations(t)
	})

	t.Run("rechaza cantidad de asientos no positiva sin tocar la base de datos", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewTripRepository(mockDB)

		applied, err := repo.ReleaseSeats(ctx, domain.SeatTransition{
			TripID:         tripID,
			SeatsDelta:     0,
			ExpectedStatus: domain.TripStatusPublished,
			NewStatus:      domain.TripStatusPublished,
		})
		assert.False(t, applied)
		assert.ErrorIs(t, err, domain.ErrInvalidSeatCount)
		mockDB.AssertNotCalled(t, "Exec", mock.Anything, mock.Anything, mock.Anything)
	})

	t.Run("reporta applied = false si la guarda no coincidió o el viaje no existe", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		mockDB.On("Exec", ctx, mock.Anything, []any{2, "PUBLISHED", missingID, "FULL"}).Return(pgconn.NewCommandTag("UPDATE 0"), nil)

		repo := postgres.NewTripRepository(mockDB)
		applied, err := repo.ReleaseSeats(ctx, domain.SeatTransition{
			TripID:         missingID,
			SeatsDelta:     2,
			ExpectedStatus: domain.TripStatusFull,
			NewStatus:      domain.TripStatusPublished,
		})
		require.NoError(t, err)
		assert.False(t, applied)
	})
}

func TestTripRepository_BatchSeatTransitions(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	tripID := uuid.MustParse("01933e3d-0000-7000-8000-000000000100")
	driverID := uuid.MustParse("01933e3d-0000-7000-8000-000000000099")
	vehicleID := uuid.MustParse("01933e3d-0000-7000-8000-000000000001")

	t.Run("FindByIDsForUpdate bloquea viajes en orden ascendente de ID", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		ids := []uuid.UUID{tripID}

		mockRows := NewMockRows([][]any{
			{
				tripID,
				driverID,
				vehicleID,
				"Buenos Aires",
				-58.381600,
				-34.603700,
				"Rosario",
				-60.639300,
				-32.946800,
				now.Add(24 * time.Hour),
				now.Add(28 * time.Hour),
				300.0,
				240,
				3,
				0,
				"ARS",
				domain.MoneyFromCents(1050000, domain.CurrencyARS),
				domain.MoneyFromCents(1138500, domain.CurrencyARS),
				domain.MoneyFromCents(2805000, domain.CurrencyARS),
				domain.MoneyFromCents(300000, domain.CurrencyARS),
				"FULL",
				now,
				now,
			},
		})

		mockDB.On("Query", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "FROM trips") &&
				strings.Contains(sql, "ST_X(origin_geom), ST_Y(origin_geom)") &&
				!strings.Contains(sql, "ST_AsText") &&
				strings.Contains(sql, "WHERE id = ANY($1)") &&
				strings.Contains(sql, "ORDER BY id ASC") &&
				strings.Contains(sql, "FOR UPDATE")
		}), []any{ids}).Return(mockRows, nil)

		repo := postgres.NewTripRepository(mockDB)
		trips, err := repo.FindByIDsForUpdate(ctx, ids)
		require.NoError(t, err)
		require.Len(t, trips, 1)
		assert.Equal(t, domain.TripStatusFull, trips[0].Status)
		assert.Equal(t, 0, trips[0].AvailableSeats)
		assert.InDelta(t, -34.6037, trips[0].OriginCoords.Latitude, 0.001)
		mockDB.AssertExpectations(t)
	})

	t.Run("ApplySeatTransitionsBatch aplica en lote usando UNNEST sin CASE WHEN en SQL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		transitions := []domain.SeatTransition{
			{
				TripID:         tripID,
				SeatsDelta:     2,
				ExpectedStatus: domain.TripStatusFull,
				NewStatus:      domain.TripStatusPublished,
			},
		}

		mockDB.On("Exec", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "UPDATE trips t") &&
				strings.Contains(sql, "FROM UNNEST") &&
				strings.Contains(sql, "t.status::text = r.expected_status") &&
				strings.Contains(sql, "t.available_seats + r.seats_to_restore <= t.total_seats_offered") &&
				!strings.Contains(sql, "CASE WHEN")
		}), []any{now.UTC(), []uuid.UUID{tripID}, []int32{2}, []string{"FULL"}, []string{"PUBLISHED"}}).Return(pgconn.NewCommandTag("UPDATE 1"), nil)

		repo := postgres.NewTripRepository(mockDB)
		updated, err := repo.ApplySeatTransitionsBatch(ctx, transitions, now)
		require.NoError(t, err)
		assert.Equal(t, 1, updated)
		mockDB.AssertExpectations(t)
	})
}

func TestTripRepository_CalculateRouteDistanceKm(t *testing.T) {
	ctx := context.Background()

	t.Run("calcula distancia geodésica en PostGIS usando ST_Length y ST_MakeLine sin WKT", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		orig, _ := domain.NewCoordinates(-34.6037, -58.3816)
		dest, _ := domain.NewCoordinates(-32.9468, -60.6393)
		routeLine, _ := domain.NewLineString([]domain.Coordinates{orig, dest})

		row := &MockRow{
			scanFn: func(destScan ...any) error {
				*(destScan[0].(*float64)) = 278.42
				return nil
			},
		}

		mockDB.On("QueryRow", ctx, mock.MatchedBy(func(sql string) bool {
			return strings.Contains(sql, "ST_Length(") &&
				strings.Contains(sql, "ST_MakeLine(") &&
				strings.Contains(sql, "::geography") &&
				!strings.Contains(sql, "ST_GeomFromText")
		}), []any{[]float64{-58.3816, -60.6393}, []float64{-34.6037, -32.9468}}).Return(row)

		repo := postgres.NewTripRepository(mockDB)
		km, err := repo.CalculateRouteDistanceKm(ctx, routeLine)
		require.NoError(t, err)
		assert.InDelta(t, 278.42, km, 0.001)
		mockDB.AssertExpectations(t)
	})

	t.Run("rechaza polilínea con menos de dos puntos sin consultar PostgreSQL", func(t *testing.T) {
		mockDB := new(MockDBExecutor)
		repo := postgres.NewTripRepository(mockDB)

		km, err := repo.CalculateRouteDistanceKm(ctx, domain.LineString{})
		assert.Zero(t, km)
		assert.ErrorIs(t, err, domain.ErrInsufficientRoutePoints)
		mockDB.AssertNotCalled(t, "QueryRow", mock.Anything, mock.Anything, mock.Anything)
	})
}

