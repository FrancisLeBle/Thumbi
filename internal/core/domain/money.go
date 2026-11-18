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

// Currency representa un código de moneda ISO 4217 de 3 letras en mayúsculas (CHAR(3)).
type Currency string

const (
	// CurrencyARS corresponde al Peso Argentino (moneda base de referencia para el cálculo de combustible/peajes).
	CurrencyARS Currency = "ARS"
	// CurrencyUSD corresponde al Dólar Estadounidense.
	CurrencyUSD Currency = "USD"
	// DefaultCurrency define la moneda por defecto alineada con las constantes de precios del dominio (ARS).
	DefaultCurrency Currency = CurrencyARS
)

// IsValid verifica que el código de moneda cumpla exactamente el formato CHAR(3) de letras ASCII mayúsculas.
func (c Currency) IsValid() bool {
	if len(c) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		if c[i] < 'A' || c[i] > 'Z' {
			return false
		}
	}
	return true
}

// String retorna la representación textual ISO 4217 de la moneda.
func (c Currency) String() string {
	return string(c)
}

// ParseCurrency normaliza y valida una cadena hacia un Currency ISO 4217 de 3 caracteres.
func ParseCurrency(raw string) (Currency, error) {
	normalized := Currency(strings.ToUpper(strings.TrimSpace(raw)))
	if !normalized.IsValid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidCurrency, raw)
	}
	return normalized, nil
}

// Money es un value object que encapsula un importe monetario exacto en centavos (10^-2)
// junto con su código de moneda ISO 4217 (CHAR(3)), evitando tanto errores de precisión float64
// como discrepancias silenciosas de divisas entre cálculos de precios y pasarelas de pago.
type Money struct {
	Amount   int64    `json:"amount"`
	Currency Currency `json:"currency"`
}

var (
	// ErrInvalidMoneyFormat indica que el valor monetario recibido en JSON o texto no es válido.
	ErrInvalidMoneyFormat = errors.New("formato monetario inválido")
)

// NewMoney construye y valida un Money exigiendo un código de moneda ISO 4217 explícito válido.
func NewMoney(cents int64, currency Currency) (Money, error) {
	parsedCurrency, err := ParseCurrency(string(currency))
	if err != nil {
		return Money{}, err
	}
	return Money{
		Amount:   cents,
		Currency: parsedCurrency,
	}, nil
}

// MoneyFromCents construye un Money a partir de centavos enteros y una moneda opcional (por defecto ARS).
func MoneyFromCents(cents int64, currency ...Currency) Money {
	cur := DefaultCurrency
	if len(currency) > 0 && strings.TrimSpace(string(currency[0])) != "" {
		cur = Currency(strings.ToUpper(strings.TrimSpace(string(currency[0]))))
	}
	return Money{
		Amount:   cents,
		Currency: cur,
	}
}

// MoneyFromMajor construye un Money a partir de unidades principales enteras (ej: 1500 -> 150000 centavos = 1500.00).
func MoneyFromMajor(major int64, currency ...Currency) Money {
	return MoneyFromCents(major*100, currency...)
}

// ZeroMoney retorna un valor monetario cero para la moneda indicada (por defecto ARS).
func ZeroMoney(currency ...Currency) Money {
	return MoneyFromCents(0, currency...)
}

// EffectiveCurrency devuelve la moneda configurada en el value object o DefaultCurrency si estuviera vacía.
func (m Money) EffectiveCurrency() Currency {
	if strings.TrimSpace(string(m.Currency)) == "" {
		return DefaultCurrency
	}
	return Currency(strings.ToUpper(strings.TrimSpace(string(m.Currency))))
}

// WithCurrency retorna una copia de Money asociada a la moneda validada indicada.
func (m Money) WithCurrency(currency Currency) (Money, error) {
	cur, err := ParseCurrency(string(currency))
	if err != nil {
		return Money{}, err
	}
	return Money{
		Amount:   m.Amount,
		Currency: cur,
	}, nil
}

// Cents retorna el valor entero en centavos.
func (m Money) Cents() int64 {
	return m.Amount
}

// DecimalString formatea el monto como cadena decimal exacta de 2 decimales (ej: "1500.50", "-25.05").
func (m Money) DecimalString() string {
	v := m.Amount
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

// String implementa fmt.Stringer devolviendo la representación decimal exacta con su moneda.
func (m Money) String() string {
	return fmt.Sprintf("%s %s", m.DecimalString(), m.EffectiveCurrency())
}

// IsZero indica si el monto es exactamente 0.
func (m Money) IsZero() bool {
	return m.Amount == 0
}

// IsPositive indica si el monto es estrictamente mayor a 0.
func (m Money) IsPositive() bool {
	return m.Amount > 0
}

// IsNegative indica si el monto es estrictamente menor a 0.
func (m Money) IsNegative() bool {
	return m.Amount < 0
}

// Add suma dos montos exigiendo coincidencia estricta de moneda.
func (m Money) Add(other Money) (Money, error) {
	c1 := m.EffectiveCurrency()
	c2 := other.EffectiveCurrency()
	if c1 != c2 {
		return Money{}, fmt.Errorf("%w: no se puede sumar %s con %s", ErrCurrencyMismatch, c1, c2)
	}
	return Money{
		Amount:   m.Amount + other.Amount,
		Currency: c1,
	}, nil
}

// Sub resta dos montos exigiendo coincidencia estricta de moneda.
func (m Money) Sub(other Money) (Money, error) {
	c1 := m.EffectiveCurrency()
	c2 := other.EffectiveCurrency()
	if c1 != c2 {
		return Money{}, fmt.Errorf("%w: no se puede restar %s con %s", ErrCurrencyMismatch, c2, c1)
	}
	return Money{
		Amount:   m.Amount - other.Amount,
		Currency: c1,
	}, nil
}

// Mul multiplica el monto por un factor entero conservando moneda y precisión exacta.
func (m Money) Mul(factor int64) Money {
	return Money{
		Amount:   m.Amount * factor,
		Currency: m.EffectiveCurrency(),
	}
}

// ParseMoney parsea una representación decimal exacta ("1500.50", "1500", "-25.5") a Money en centavos
// sin pasar por aritmética binaria float64. Rechaza más de 2 decimales para evitar truncamientos silenciosos.
func ParseMoney(raw string, currency ...Currency) (Money, error) {
	cents, err := parseDecimalToCents(raw)
	if err != nil {
		return Money{}, err
	}
	cur := DefaultCurrency
	if len(currency) > 0 && strings.TrimSpace(string(currency[0])) != "" {
		parsedCur, err := ParseCurrency(string(currency[0]))
		if err != nil {
			return Money{}, err
		}
		cur = parsedCur
	}
	return Money{
		Amount:   cents,
		Currency: cur,
	}, nil
}

func parseDecimalToCents(raw string) (int64, error) {
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
	return total, nil
}

func parseAmountToken(raw []byte) (int64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return 0, nil
	}
	if trimmed[0] == '"' {
		var str string
		if err := json.Unmarshal(trimmed, &str); err != nil {
			return 0, err
		}
		return parseDecimalToCents(str)
	}
	if bytes.ContainsAny(trimmed, ".eE") {
		return 0, fmt.Errorf("%w: use centavos enteros o cadena decimal exacta en lugar de float JSON", ErrInvalidMoneyFormat)
	}
	val, err := strconv.ParseInt(string(trimmed), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidMoneyFormat, err)
	}
	return val, nil
}

// MarshalJSON serializa Money como objeto estructurado {"amount": <centavos>, "currency": "<ISO4217>"}.
func (m Money) MarshalJSON() ([]byte, error) {
	type moneyWire struct {
		Amount   int64    `json:"amount"`
		Currency Currency `json:"currency"`
	}
	return json.Marshal(moneyWire{
		Amount:   m.Amount,
		Currency: m.EffectiveCurrency(),
	})
}

// UnmarshalJSON acepta:
// 1. Un objeto JSON {"amount": 150000, "currency": "ARS"} (o "amount" como string decimal exacta "1500.00"),
// 2. Un entero JSON exacto en centavos (ej: 150000) o una cadena decimal exacta (ej: "1500.00") asumiendo DefaultCurrency.
// Rechaza números JSON con punto flotante o notación científica para impedir pérdida de precisión.
func (m *Money) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*m = Money{}
		return nil
	}

	if trimmed[0] == '{' {
		var obj struct {
			Amount   json.RawMessage `json:"amount"`
			Currency *string         `json:"currency"`
		}
		if err := json.Unmarshal(trimmed, &obj); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidMoneyFormat, err)
		}
		if len(obj.Amount) == 0 {
			return fmt.Errorf("%w: campo amount requerido", ErrInvalidMoneyFormat)
		}
		cents, err := parseAmountToken(obj.Amount)
		if err != nil {
			return err
		}
		cur := DefaultCurrency
		if obj.Currency != nil && strings.TrimSpace(*obj.Currency) != "" {
			parsedCur, err := ParseCurrency(*obj.Currency)
			if err != nil {
				return err
			}
			cur = parsedCur
		}
		*m = Money{
			Amount:   cents,
			Currency: cur,
		}
		return nil
	}

	cents, err := parseAmountToken(trimmed)
	if err != nil {
		return err
	}
	*m = Money{
		Amount:   cents,
		Currency: DefaultCurrency,
	}
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

// SplitHalf divide un monto en dos mitades exactas preservando la conservación de suma (a + b == m) y la moneda.
// Ante centavos impares (ej. 101 centavos), la primera mitad recibe el redondeo hacia arriba (51)
// y la segunda mitad recibe el remanente exacto (50).
func (m Money) SplitHalf() (Money, Money) {
	cur := m.EffectiveCurrency()
	if m.Amount <= 0 {
		return Money{Amount: 0, Currency: cur}, Money{Amount: 0, Currency: cur}
	}
	first := (m.Amount + 1) / 2
	second := m.Amount - first
	return Money{Amount: first, Currency: cur}, Money{Amount: second, Currency: cur}
}
