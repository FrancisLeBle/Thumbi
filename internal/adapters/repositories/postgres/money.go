package postgres

import (
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// moneyToPGNumeric convierte domain.Money (centavos exactos) en pgtype.Numeric con escala fija -2
// para su almacenamiento exacto en columnas PostgreSQL NUMERIC(10, 2) sin pasar por float64.
func moneyToPGNumeric(m domain.Money) pgtype.Numeric {
	return pgtype.Numeric{
		Int:   big.NewInt(int64(m)),
		Exp:   -2,
		Valid: true,
	}
}

// pgNumericToMoney convierte un pgtype.Numeric devuelto por PostgreSQL a domain.Money (centavos exactos)
// utilizando aritmética decimal arbitraria (math/big.Int) sin conversión intermedia a float64.
func pgNumericToMoney(n pgtype.Numeric) (domain.Money, error) {
	if !n.Valid || n.NaN {
		return 0, fmt.Errorf("valor NUMERIC nulo o NaN para campo monetario")
	}
	if n.InfinityModifier != pgtype.Finite {
		return 0, fmt.Errorf("valor NUMERIC infinito para campo monetario")
	}
	if n.Int == nil {
		return 0, nil
	}

	// Valor en centavos = n.Int * 10^(n.Exp + 2)
	val := new(big.Int).Set(n.Int)
	shift := int64(n.Exp) + 2

	ten := big.NewInt(10)
	if shift > 0 {
		mul := new(big.Int).Exp(ten, big.NewInt(shift), nil)
		val.Mul(val, mul)
	} else if shift < 0 {
		div := new(big.Int).Exp(ten, big.NewInt(-shift), nil)
		half := new(big.Int).Rsh(div, 1)
		if val.Sign() >= 0 {
			val.Add(val, half)
		} else {
			val.Sub(val, half)
		}
		val.Quo(val, div)
	}

	if !val.IsInt64() {
		return 0, fmt.Errorf("desbordamiento de int64 al convertir NUMERIC a centavos: %s", val.String())
	}
	return domain.Money(val.Int64()), nil
}
