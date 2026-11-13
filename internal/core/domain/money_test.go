package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestMoney_ConversionsAndFormatting(t *testing.T) {
	assert.Equal(t, domain.Money(150050), domain.MoneyFromCents(150050))
	assert.Equal(t, int64(150050), domain.MoneyFromCents(150050).Cents())
	assert.Equal(t, domain.Money(150000), domain.MoneyFromMajor(1500))
	assert.Equal(t, "1500.50", domain.Money(150050).DecimalString())
	assert.Equal(t, "0.05", domain.Money(5).String())
	assert.Equal(t, "-25.05", domain.Money(-2505).DecimalString())
}

func TestMoney_ParseMoney(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    domain.Money
		wantErr bool
	}{
		{name: "entero positivo", input: "1500", want: domain.Money(150000)},
		{name: "dos decimales", input: "1500.50", want: domain.Money(150050)},
		{name: "un decimal", input: "1500.5", want: domain.Money(150050)},
		{name: "solo centavos", input: ".05", want: domain.Money(5)},
		{name: "negativo con signo", input: "-25.05", want: domain.Money(-2505)},
		{name: "con espacios", input: "  100.25  ", want: domain.Money(10025)},
		{name: "tres decimales rechazado", input: "1500.505", wantErr: true},
		{name: "punto sin decimales rechazado", input: "1500.", wantErr: true},
		{name: "cadena vacía rechazada", input: "", wantErr: true},
		{name: "alfabético rechazado", input: "15abc", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ParseMoney(tc.input)
			if tc.wantErr {
				assert.ErrorIs(t, err, domain.ErrInvalidMoneyFormat)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestMoney_JSONSerialization(t *testing.T) {
	type payload struct {
		Amount domain.Money `json:"amount"`
	}

	t.Run("serializa como centavos enteros exactos", func(t *testing.T) {
		raw, err := json.Marshal(payload{Amount: domain.Money(150050)})
		require.NoError(t, err)
		assert.Equal(t, `{"amount":150050}`, string(raw))
	})

	t.Run("acepta centavos enteros en JSON", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":150050}`), &p)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(150050), p.Amount)
	})

	t.Run("acepta cadena decimal exacta en JSON sin pasar por float64", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":"1500.50"}`), &p)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(150050), p.Amount)
	})

	t.Run("rechaza floats JSON para impedir deriva binaria", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":1500.50}`), &p)
		assert.ErrorIs(t, err, domain.ErrInvalidMoneyFormat)

		errExp := json.Unmarshal([]byte(`{"amount":1.5e3}`), &p)
		assert.ErrorIs(t, errExp, domain.ErrInvalidMoneyFormat)
	})
}

func TestMoney_ExactFinancialArithmetic(t *testing.T) {
	t.Run("RoundDivHalfUp redondea mitades exactas al centavo más cercano", func(t *testing.T) {
		assert.Equal(t, int64(3334), domain.RoundDivHalfUp(10000, 3))
		assert.Equal(t, int64(3333), domain.RoundDivHalfUp(9999, 3))
		assert.Equal(t, int64(2), domain.RoundDivHalfUp(3, 2))
		assert.Equal(t, int64(-2), domain.RoundDivHalfUp(-3, 2))
		assert.Equal(t, int64(0), domain.RoundDivHalfUp(100, 0))
	})

	t.Run("SplitHalf conserva la suma exacta ante centavos impares", func(t *testing.T) {
		pRefund, dComp := domain.Money(101).SplitHalf()
		assert.Equal(t, domain.Money(51), pRefund)
		assert.Equal(t, domain.Money(50), dComp)
		assert.Equal(t, domain.Money(101), pRefund+dComp)

		pEven, dEven := domain.Money(10000).SplitHalf()
		assert.Equal(t, domain.Money(5000), pEven)
		assert.Equal(t, domain.Money(5000), dEven)
	})

	t.Run("CalculateCapPrice y IsPricePermitted no sufren deriva de centavo ni tolerancia float", func(t *testing.T) {
		// 10000 centavos ($100.00) / 3 asientos = 3333.33... -> CostPerSeat = 3333 ($33.33)
		// CapPrice = (10000 * 11000) / (3 * 10000) = 11000 / 3 = 3666.66... -> 3667 centavos ($36.67)
		matrix := domain.CalculateCapPrice(domain.Money(10000), domain.Money(0), 3)
		assert.Equal(t, domain.Money(3333), matrix.CostPerSeat)
		assert.Equal(t, domain.Money(3667), matrix.CapPricePerSeat)
		assert.True(t, matrix.IsPricePermitted(domain.Money(3667)))
		assert.False(t, matrix.IsPricePermitted(domain.Money(3668)))
	})

	t.Run("NewBooking multiplica UnitPrice por asientos de forma exacta en centavos", func(t *testing.T) {
		b, err := domain.NewBooking(domain.BookingParams{
			ID:          "b-exact",
			TripID:      "t-exact",
			PassengerID: "p-1",
			DriverID:    "d-1",
			SeatsBooked: 3,
			UnitPrice:   domain.Money(1999), // $19.99
			TTL:         15 * time.Minute,
		})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(5997), b.TotalPrice)
	})
}
