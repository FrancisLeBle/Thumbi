package postgres

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/backend/internal/core/domain"
)

func TestMoneyPGNumericConversion(t *testing.T) {
	t.Run("moneyToPGNumeric preserva centavos exactos con escala -2", func(t *testing.T) {
		num := moneyToPGNumeric(domain.MoneyFromCents(150050, domain.CurrencyARS))
		assert.True(t, num.Valid)
		assert.Equal(t, int32(-2), num.Exp)
		require.NotNil(t, num.Int)
		assert.Equal(t, 0, num.Int.Cmp(big.NewInt(150050)))

		neg := moneyToPGNumeric(domain.MoneyFromCents(-2505, domain.CurrencyARS))
		assert.True(t, neg.Valid)
		assert.Equal(t, int32(-2), neg.Exp)
		assert.Equal(t, 0, neg.Int.Cmp(big.NewInt(-2505)))
	})

	t.Run("pgNumericToMoney convierte distintas escalas decimales sin float64", func(t *testing.T) {
		// 1500.50 (Exp: -2)
		m1, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(150050), Exp: -2, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150050, domain.CurrencyARS), m1)

		// 1500 (Exp: 0) -> 150000 centavos
		m2, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(1500), Exp: 0, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150000, domain.CurrencyARS), m2)

		// 1500.5 (Exp: -1) -> 150050 centavos
		m3, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(15005), Exp: -1, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(150050, domain.CurrencyARS), m3)

		// 15.005 (Exp: -3) -> redondeo half-up a 1501 centavos
		m4, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(15005), Exp: -3, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(1501, domain.CurrencyARS), m4)

		// -15.005 (Exp: -3) -> redondeo half-up simétrico a -1501 centavos
		m5, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(-15005), Exp: -3, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(-1501, domain.CurrencyARS), m5)

		// Moneda explícita ARS
		m6, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(50000), Exp: -2, Valid: true}, domain.CurrencyARS)
		require.NoError(t, err)
		assert.Equal(t, domain.MoneyFromCents(50000, domain.CurrencyARS), m6)
	})

	t.Run("pgNumericToMoney rechaza valores nulos, NaN, infinitos o con desbordamiento", func(t *testing.T) {
		_, errNull := pgNumericToMoney(pgtype.Numeric{Valid: false})
		assert.Error(t, errNull)

		_, errNaN := pgNumericToMoney(pgtype.Numeric{Valid: true, NaN: true})
		assert.Error(t, errNaN)

		_, errInf := pgNumericToMoney(pgtype.Numeric{Valid: true, InfinityModifier: pgtype.Infinity})
		assert.Error(t, errInf)

		huge, _ := new(big.Int).SetString("999999999999999999999999999999", 10)
		_, errOverflow := pgNumericToMoney(pgtype.Numeric{Int: huge, Exp: 0, Valid: true})
		assert.Error(t, errOverflow)
	})
}
