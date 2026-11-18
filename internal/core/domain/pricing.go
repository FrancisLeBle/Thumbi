package domain

import (
	"fmt"
	"math"
)

// Constantes de referencia para economía no lucrativa de gastos compartidos
const (
	// DefaultFuelConsumptionLitersPer100Km promedio nacional de vehículos de combustión/mixtos (8.5L / 100km)
	DefaultFuelConsumptionLitersPer100Km = 8.5
	// DefaultFuelPricePerLiterCents referencia base de combustible en centavos de ARS (110000 centavos = $1100.00 ARS / litro)
	DefaultFuelPricePerLiterCents int64 = 110000
	// DefaultContingencyMargin margen de imprevistos permitido (10% según RF-06 sin generar lucro)
	DefaultContingencyMargin = 0.10
	// DefaultContingencyMarginBps margen de imprevistos en puntos básicos enteros (1000 bps = 10.00%)
	DefaultContingencyMarginBps int64 = 1000
)

// DefaultFuelPricePerLiter referencia base de combustible expresada en Money (1100.00 ARS / litro).
var DefaultFuelPricePerLiter = MoneyFromCents(DefaultFuelPricePerLiterCents, DefaultCurrency)

// FuelEstimate almacena el desglose del costo estimado de combustible en centavos exactos
type FuelEstimate struct {
	DistanceKm              float64 `json:"distance_km"`
	ConsumptionLitersPer100 float64 `json:"consumption_liters_per_100"`
	FuelPricePerLiter       Money   `json:"fuel_price_per_liter"`
	TotalFuelCost           Money   `json:"total_fuel_cost"`
}

// TollEstimate almacena el costo estimado de peajes a lo largo de la traza en centavos exactos
type TollEstimate struct {
	TollsCount    int   `json:"tolls_count"`
	TotalTollCost Money `json:"total_toll_cost"`
}

// CostMatrix matriz consolidada de costos operativos reales estimados en centavos exactos
type CostMatrix struct {
	Currency          Currency `json:"currency"`
	FuelCost          Money    `json:"fuel_cost"`
	TollCost          Money    `json:"toll_cost"`
	TotalSharedCost   Money    `json:"total_shared_cost"`
	SeatsOffered      int      `json:"seats_offered"`
	CostPerSeat       Money    `json:"cost_per_seat"`
	CapPricePerSeat   Money    `json:"cap_price_per_seat"`
	ContingencyMargin float64  `json:"contingency_margin"`
}

// CalculateFuelEstimate calcula el costo de combustible en base a la distancia y parámetros de consumo
func CalculateFuelEstimate(distanceKm, consumptionLiters float64, pricePerLiter Money) FuelEstimate {
	if consumptionLiters <= 0 {
		consumptionLiters = DefaultFuelConsumptionLitersPer100Km
	}
	if pricePerLiter.Amount <= 0 {
		cur := pricePerLiter.EffectiveCurrency()
		pricePerLiter = MoneyFromCents(DefaultFuelPricePerLiterCents, cur)
	}

	cur := pricePerLiter.EffectiveCurrency()
	pricePerLiter.Currency = cur

	litersNeeded := (distanceKm / 100.0) * consumptionLiters
	litersMicros := int64(math.Round(litersNeeded * 1_000_000))
	totalFuelCents := RoundDivHalfUp(litersMicros*pricePerLiter.Amount, 1_000_000)

	return FuelEstimate{
		DistanceKm:              math.Round(distanceKm*100) / 100,
		ConsumptionLitersPer100: consumptionLiters,
		FuelPricePerLiter:       pricePerLiter,
		TotalFuelCost:           MoneyFromCents(totalFuelCents, cur),
	}
}

// CalculateCapPrice calcula el precio tope máximo (Cap Price) por asiento en centavos exactos
// Fórmula EARS RF-06:
// CapPrice = ((CostFuel + CostTolls) / Seats) * (1 + 0.10)
func CalculateCapPrice(fuelCost, tollCost Money, seatsOffered int) CostMatrix {
	cur := fuelCost.EffectiveCurrency()
	if fuelCost.Currency == "" && tollCost.Currency != "" {
		cur = tollCost.EffectiveCurrency()
	}

	if seatsOffered <= 0 {
		seatsOffered = 1
	}
	if fuelCost.Amount < 0 {
		fuelCost.Amount = 0
	}
	if tollCost.Amount < 0 {
		tollCost.Amount = 0
	}

	fuelNormalized := MoneyFromCents(fuelCost.Amount, cur)
	tollNormalized := MoneyFromCents(tollCost.Amount, cur)

	totalSharedCents := fuelNormalized.Amount + tollNormalized.Amount
	totalSharedCost := MoneyFromCents(totalSharedCents, cur)
	costPerSeat := MoneyFromCents(RoundDivHalfUp(totalSharedCents, int64(seatsOffered)), cur)
	capPricePerSeat := MoneyFromCents(RoundDivHalfUp(
		totalSharedCents*(10000+DefaultContingencyMarginBps),
		int64(seatsOffered)*10000,
	), cur)

	return CostMatrix{
		Currency:          cur,
		FuelCost:          fuelNormalized,
		TollCost:          tollNormalized,
		TotalSharedCost:   totalSharedCost,
		SeatsOffered:      seatsOffered,
		CostPerSeat:       costPerSeat,
		CapPricePerSeat:   capPricePerSeat,
		ContingencyMargin: DefaultContingencyMargin,
	}
}

// CalculateCapPriceStrict valida coincidencia de monedas antes de computar la matriz de costos compartidos.
func CalculateCapPriceStrict(fuelCost, tollCost Money, seatsOffered int) (CostMatrix, error) {
	if fuelCost.Currency != "" && tollCost.Currency != "" && fuelCost.EffectiveCurrency() != tollCost.EffectiveCurrency() {
		return CostMatrix{}, fmt.Errorf("%w: combustible en %s y peajes en %s", ErrCurrencyMismatch, fuelCost.EffectiveCurrency(), tollCost.EffectiveCurrency())
	}
	return CalculateCapPrice(fuelCost, tollCost, seatsOffered), nil
}

// IsPricePermitted valida exactamente que el precio propuesto por el conductor coincida en moneda, no sea negativo ni supere el Cap Price.
func (cm CostMatrix) IsPricePermitted(proposedPrice Money) bool {
	if proposedPrice.Amount < 0 || proposedPrice.Amount > cm.CapPricePerSeat.Amount {
		return false
	}
	if cm.Currency != "" && proposedPrice.Currency != "" && proposedPrice.EffectiveCurrency() != cm.Currency {
		return false
	}
	return true
}
