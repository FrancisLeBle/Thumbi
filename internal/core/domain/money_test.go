package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestMoney_ConversionsAndFormatting(t *testing.T) {
	m := domain.MoneyFromCents(150050, domain.CurrencyARS)
	assert.Equal(t, domain.Money{Amount: 150050, Currency: domain.CurrencyARS}, m)
	assert.Equal(t, int64(150050), m.Cents())
	assert.Equal(t, domain.CurrencyARS, m.EffectiveCurrency())
	assert.Equal(t, domain.MoneyFromCents(150000), domain.MoneyFromMajor(1500))
	assert.Equal(t, "1500.50", m.DecimalString())
	assert.Equal(t, "0.05 ARS", domain.MoneyFromCents(5).String())
	assert.Equal(t, "-25.05", domain.MoneyFromCents(-2505).DecimalString())

	strictMoney, err := domain.NewMoney(2500, domain.Currency("usd"))
	require.NoError(t, err)
	assert.Equal(t, domain.CurrencyUSD, strictMoney.Currency)

	_, errInvalidCur := domain.NewMoney(2500, domain.Currency("US"))
	assert.ErrorIs(t, errInvalidCur, domain.ErrInvalidCurrency)
}

func TestMoney_ParseMoney(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		currency domain.Currency
		want     domain.Money
		wantErr  error
	}{
		{name: "entero positivo", input: "1500", want: domain.MoneyFromCents(150000)},
		{name: "dos decimales", input: "1500.50", want: domain.MoneyFromCents(150050)},
		{name: "un decimal con moneda explícita", input: "1500.5", currency: domain.CurrencyUSD, want: domain.MoneyFromCents(150050, domain.CurrencyUSD)},
		{name: "solo centavos", input: ".05", want: domain.MoneyFromCents(5)},
		{name: "negativo con signo", input: "-25.05", want: domain.MoneyFromCents(-2505)},
		{name: "con espacios", input: "  100.25  ", want: domain.MoneyFromCents(10025)},
		{name: "tres decimales rechazado", input: "1500.505", wantErr: domain.ErrInvalidMoneyFormat},
		{name: "punto sin decimales rechazado", input: "1500.", wantErr: domain.ErrInvalidMoneyFormat},
		{name: "cadena vacía rechazada", input: "", wantErr: domain.ErrInvalidMoneyFormat},
		{name: "alfabético rechazado", input: "15abc", wantErr: domain.ErrInvalidMoneyFormat},
		{name: "moneda inválida rechazada", input: "15.00", currency: domain.Currency("AR1"), wantErr: domain.ErrInvalidCurrency},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var (
				got domain.Money
				err error
			)
			if tc.currency != "" {
				got, err = domain.ParseMoney(tc.input, tc.currency)
			} else {
				got, err = domain.ParseMoney(tc.input)
			}
			if tc.wantErr != nil {
				assert.ErrorIs(t, err, tc.wantErr)
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

	t.Run("serializa como objeto estructurado con centavos y moneda ISO 4217", func(t *testing.T) {
		raw, err := json.Marshal(payload{Amount: domain.MoneyFromCents(150050, domain.CurrencyARS)})
		require.NoError(t, err)
		assert.Equal(t, `{"amount":{"amount":150050,"currency":"ARS"}}`, string(raw))
	})

	t.Run("acepta objeto JSON con centavos enteros y moneda", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":{"amount":150050,"currency":"USD"}}`), &p)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150050, domain.CurrencyUSD), p.Amount)
	})

	t.Run("acepta centavos enteros heredados en JSON asumiendo ARS", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":150050}`), &p)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150050, domain.CurrencyARS), p.Amount)
	})

	t.Run("acepta cadena decimal exacta en JSON sin pasar por float64", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":"1500.50"}`), &p)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150050, domain.CurrencyARS), p.Amount)
	})

	t.Run("rechaza floats JSON para impedir deriva binaria", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":1500.50}`), &p)
		assert.ErrorIs(t, err, domain.ErrInvalidMoneyFormat)

		errExp := json.Unmarshal([]byte(`{"amount":1.5e3}`), &p)
		assert.ErrorIs(t, errExp, domain.ErrInvalidMoneyFormat)
	})

	t.Run("rechaza monedas inválidas en objeto JSON", func(t *testing.T) {
		var p payload
		err := json.Unmarshal([]byte(`{"amount":{"amount":1500,"currency":"INVALID"}}`), &p)
		assert.ErrorIs(t, err, domain.ErrInvalidCurrency)
	})
}

func TestMoney_ExactFinancialArithmetic(t *testing.T) {
	t.Run("Add y Sub exigen coincidencia de moneda", func(t *testing.T) {
		ars1 := domain.MoneyFromCents(1000, domain.CurrencyARS)
		ars2 := domain.MoneyFromCents(500, domain.CurrencyARS)
		usd := domain.MoneyFromCents(500, domain.CurrencyUSD)

		sum, err := ars1.Add(ars2)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(1500, domain.CurrencyARS), sum)

		_, errAddMismatch := ars1.Add(usd)
		assert.ErrorIs(t, errAddMismatch, domain.ErrCurrencyMismatch)

		_, errSubMismatch := ars1.Sub(usd)
		assert.ErrorIs(t, errSubMismatch, domain.ErrCurrencyMismatch)
	})

	t.Run("RoundDivHalfUp redondea mitades exactas al centavo más cercano", func(t *testing.T) {
		assert.Equal(t, int64(3334), domain.RoundDivHalfUp(10000, 3))
		assert.Equal(t, int64(3333), domain.RoundDivHalfUp(9999, 3))
		assert.Equal(t, int64(2), domain.RoundDivHalfUp(3, 2))
		assert.Equal(t, int64(-2), domain.RoundDivHalfUp(-3, 2))
		assert.Equal(t, int64(0), domain.RoundDivHalfUp(100, 0))
	})

	t.Run("SplitHalf conserva la suma exacta ante centavos impares y la moneda", func(t *testing.T) {
		pRefund, dComp := domain.MoneyFromCents(101, domain.CurrencyARS).SplitHalf()
		assert.Equal(t, domain.MoneyFromCents(51, domain.CurrencyARS), pRefund)
		assert.Equal(t, domain.MoneyFromCents(50, domain.CurrencyARS), dComp)
		total, err := pRefund.Add(dComp)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(101, domain.CurrencyARS), total)

		pEven, dEven := domain.MoneyFromCents(10000, domain.CurrencyARS).SplitHalf()
		assert.Equal(t, domain.MoneyFromCents(5000, domain.CurrencyARS), pEven)
		assert.Equal(t, domain.MoneyFromCents(5000, domain.CurrencyARS), dEven)
	})

	t.Run("CalculateCapPrice y IsPricePermitted no sufren deriva de centavo ni tolerancia float", func(t *testing.T) {
		// 10000 centavos ($100.00 ARS) / 3 asientos = 3333.33... -> CostPerSeat = 3333 ($33.33)
		// CapPrice = (10000 * 11000) / (3 * 10000) = 11000 / 3 = 3666.66... -> 3667 centavos ($36.67)
		matrix := domain.CalculateCapPrice(domain.MoneyFromCents(10000, domain.CurrencyARS), domain.MoneyFromCents(0, domain.CurrencyARS), 3)
		assert.Equal(t, domain.CurrencyARS, matrix.Currency)
		assert.Equal(t, domain.MoneyFromCents(3333, domain.CurrencyARS), matrix.CostPerSeat)
		assert.Equal(t, domain.MoneyFromCents(3667, domain.CurrencyARS), matrix.CapPricePerSeat)
		assert.True(t, matrix.IsPricePermitted(domain.MoneyFromCents(3667, domain.CurrencyARS)))
		assert.False(t, matrix.IsPricePermitted(domain.MoneyFromCents(3668, domain.CurrencyARS)))
		assert.False(t, matrix.IsPricePermitted(domain.MoneyFromCents(3667, domain.CurrencyUSD)))
	})

	t.Run("NewBooking multiplica UnitPrice por asientos de forma exacta en centavos", func(t *testing.T) {
		b, err := domain.NewBooking(domain.BookingParams{
			ID:          domain.NewUUIDv7(),
			TripID:      domain.NewUUIDv7(),
			PassengerID: domain.NewUUIDv7(),
			DriverID:    domain.NewUUIDv7(),
			SeatsBooked: 3,
			Currency:    domain.CurrencyARS,
			UnitPrice:   domain.MoneyFromCents(1999, domain.CurrencyARS), // $19.99 ARS
			TTL:         15 * time.Minute,
		})
		require.NoError(t, err)
		assert.Equal(t, domain.CurrencyARS, b.Currency)
		assert.Equal(t, domain.MoneyFromCents(5997, domain.CurrencyARS), b.TotalPrice)
	})
}
