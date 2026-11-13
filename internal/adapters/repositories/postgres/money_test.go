package postgres

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

func TestMoneyPGNumericConversion(t *testing.T) {
	t.Run("moneyToPGNumeric preserva centavos exactos con escala -2", func(t *testing.T) {
		num := moneyToPGNumeric(domain.Money(150050))
		assert.True(t, num.Valid)
		assert.Equal(t, int32(-2), num.Exp)
		require.NotNil(t, num.Int)
		assert.Equal(t, 0, num.Int.Cmp(big.NewInt(150050)))

		neg := moneyToPGNumeric(domain.Money(-2505))
		assert.True(t, neg.Valid)
		assert.Equal(t, int32(-2), neg.Exp)
		assert.Equal(t, 0, neg.Int.Cmp(big.NewInt(-2505)))
	})

	t.Run("pgNumericToMoney convierte distintas escalas decimales sin float64", func(t *testing.T) {
		// 1500.50 (Exp: -2)
		m1, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(150050), Exp: -2, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(150050), m1)

		// 1500 (Exp: 0) -> 150000 centavos
		m2, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(1500), Exp: 0, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(150000), m2)

		// 1500.5 (Exp: -1) -> 150050 centavos
		m3, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(15005), Exp: -1, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(150050), m3)

		// 15.005 (Exp: -3) -> redondeo half-up a 1501 centavos
		m4, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(15005), Exp: -3, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(1501), m4)

		// -15.005 (Exp: -3) -> redondeo half-up simétrico a -1501 centavos
		m5, err := pgNumericToMoney(pgtype.Numeric{Int: big.NewInt(-15005), Exp: -3, Valid: true})
		require.NoError(t, err)
		assert.Equal(t, domain.Money(-1501), m5)
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
