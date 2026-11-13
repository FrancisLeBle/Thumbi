package domain

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money representa un importe monetario exacto en centavos (unidades menores, 10^-2).
// Almacenar valores monetarios como int64 elimina la pérdida de precisión binaria de float64
// y se corresponde de forma exacta con columnas NUMERIC(10, 2) en PostgreSQL.
type Money int64

var (
	// ErrInvalidMoneyFormat indica que el valor monetario recibido en JSON o texto no es válido.
	ErrInvalidMoneyFormat = errors.New("formato monetario inválido")
)

// MoneyFromCents construye un Money a partir de una cantidad entera de centavos.
func MoneyFromCents(cents int64) Money {
	return Money(cents)
}

// MoneyFromMajor construye un Money a partir de unidades enteras principales (ej: 1500 -> 150000 centavos = 1500.00).
func MoneyFromMajor(major int64) Money {
	return Money(major * 100)
}

// Cents retorna el valor entero en centavos.
func (m Money) Cents() int64 {
	return int64(m)
}

// DecimalString formatea el monto como cadena decimal exacta de 2 decimales (ej: "1500.50", "-25.05").
func (m Money) DecimalString() string {
	v := int64(m)
	sign := ""
	if v < 0 {
		sign = "-"
		if v == math.MinInt64 {
			return "-92233720368547758.08"
		}
		v = -v
	}
	return fmt.Sprintf("%s%d.%02d", sign, v/100, v%100)
}

// String implementa fmt.Stringer devolviendo la representación decimal exacta.
func (m Money) String() string {
	return m.DecimalString()
}

// ParseMoney parsea una representación decimal exacta ("1500.50", "1500", "-25.5") a Money en centavos
// sin pasar por aritmética binaria float64. Rechaza más de 2 decimales para evitar truncamientos silenciosos.
func ParseMoney(raw string) (Money, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return 0, fmt.Errorf("%w: cadena vacía", ErrInvalidMoneyFormat)
	}

	neg := false
	switch s[0] {
	case '-':
		neg = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("%w: sin dígitos", ErrInvalidMoneyFormat)
	}

	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("%w: múltiples puntos decimales", ErrInvalidMoneyFormat)
	}

	intPartStr := parts[0]
	if intPartStr == "" {
		intPartStr = "0"
	}
	for i := 0; i < len(intPartStr); i++ {
		if intPartStr[i] < '0' || intPartStr[i] > '9' {
			return 0, fmt.Errorf("%w: dígito entero inválido", ErrInvalidMoneyFormat)
		}
	}

	intPart, err := strconv.ParseInt(intPartStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidMoneyFormat, err)
	}

	var fracPart int64
	if len(parts) == 2 {
		fracStr := parts[1]
		if fracStr == "" || len(fracStr) > 2 {
			return 0, fmt.Errorf("%w: se requieren 1 o 2 dígitos decimales", ErrInvalidMoneyFormat)
		}
		for i := 0; i < len(fracStr); i++ {
			if fracStr[i] < '0' || fracStr[i] > '9' {
				return 0, fmt.Errorf("%w: dígito decimal inválido", ErrInvalidMoneyFormat)
			}
		}
		if len(fracStr) == 1 {
			fracStr += "0"
		}
		fracPart, err = strconv.ParseInt(fracStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("%w: %v", ErrInvalidMoneyFormat, err)
		}
	}

	if intPart > math.MaxInt64/100 {
		return 0, fmt.Errorf("%w: desbordamiento numérico", ErrInvalidMoneyFormat)
	}
	total := intPart*100 + fracPart
	if neg {
		total = -total
	}
	return Money(total), nil
}

// MarshalJSON serializa Money como entero exacto en centavos para preservar precisión en JSON.
func (m Money) MarshalJSON() ([]byte, error) {
	return json.Marshal(int64(m))
}

// UnmarshalJSON acepta un entero JSON exacto en centavos (ej: 150000) o una cadena decimal exacta (ej: "1500.00").
// Rechaza números JSON con punto flotante o notación científica para impedir pérdida de precisión.
func (m *Money) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*m = 0
		return nil
	}

	if trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(trimmed, &str); err != nil {
			return err
		}
		parsed, err := ParseMoney(str)
		if err != nil {
			return err
		}
		*m = parsed
		return nil
	}

	if bytes.ContainsAny(trimmed, ".eE") {
		return fmt.Errorf("%w: use centavos enteros o cadena decimal exacta en lugar de float JSON", ErrInvalidMoneyFormat)
	}

	val, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidMoneyFormat, err)
	}
	*m = Money(val)
	return nil
}

// RoundDivHalfUp divide num / den en aritmética entera exacta aplicando redondeo half-up (al centavo más cercano).
func RoundDivHalfUp(num, den int64) int64 {
	if den == 0 {
		return 0
	}
	if den < 0 {
		num = -num
		den = -den
	}
	if num >= 0 {
		return (num + den/2) / den
	}
	return -((-num + den/2) / den)
}

// SplitHalf divide un monto en dos mitades exactas preservando la conservación de suma (a + b == m).
// Ante centavos impares (ej. 101 centavos), la primera mitad recibe el redondeo hacia arriba (51)
// y la segunda mitad recibe el remanente exacto (50).
func (m Money) SplitHalf() (Money, Money) {
	if m <= 0 {
		return 0, 0
	}
	first := Money((int64(m) + 1) / 2)
	second := m - first
	return first, second
}
