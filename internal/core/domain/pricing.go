package domain

import (
	"math"
)

// Constantes de referencia para economía no lucrativa de gastos compartidos
const (
	// DefaultFuelConsumptionLitersPer100Km promedio nacional de vehículos de combustión/mixtos (8.5L / 100km)
	DefaultFuelConsumptionLitersPer100Km = 8.5
	// DefaultFuelPricePerLiter referencia base de combustible en centavos (110000 centavos = $1100.00 ARS / litro)
	DefaultFuelPricePerLiter Money = 110000
	// DefaultContingencyMargin margen de imprevistos permitido (10% según RF-06 sin generar lucro)
	DefaultContingencyMargin = 0.10
	// DefaultContingencyMarginBps margen de imprevistos en puntos básicos enteros (1000 bps = 10.00%)
	DefaultContingencyMarginBps int64 = 1000
)

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
	FuelCost          Money   `json:"fuel_cost"`
	TollCost          Money   `json:"toll_cost"`
	TotalSharedCost   Money   `json:"total_shared_cost"`
	SeatsOffered      int     `json:"seats_offered"`
	CostPerSeat       Money   `json:"cost_per_seat"`
	CapPricePerSeat   Money   `json:"cap_price_per_seat"`
	ContingencyMargin float64 `json:"contingency_margin"`
}

// CalculateFuelEstimate calcula el costo de combustible en base a la distancia y parámetros de consumo
func CalculateFuelEstimate(distanceKm, consumptionLiters float64, pricePerLiter Money) FuelEstimate {
	if consumptionLiters <= 0 {
		consumptionLiters = DefaultFuelConsumptionLitersPer100Km
	}
	if pricePerLiter <= 0 {
		pricePerLiter = DefaultFuelPricePerLiter
	}

	litersNeeded := (distanceKm / 100.0) * consumptionLiters
	litersMicros := int64(math.Round(litersNeeded * 1_000_000))
	totalFuelCents := RoundDivHalfUp(litersMicros*int64(pricePerLiter), 1_000_000)

	return FuelEstimate{
		DistanceKm:              math.Round(distanceKm*100) / 100,
		ConsumptionLitersPer100: consumptionLiters,
		FuelPricePerLiter:       pricePerLiter,
		TotalFuelCost:           Money(totalFuelCents),
	}
}

// CalculateCapPrice calcula el precio tope máximo (Cap Price) por asiento en centavos exactos
// Fórmula EARS RF-06:
// CapPrice = ((CostFuel + CostTolls) / Seats) * (1 + 0.10)
func CalculateCapPrice(fuelCost, tollCost Money, seatsOffered int) CostMatrix {
	if seatsOffered <= 0 {
		seatsOffered = 1
	}
	if fuelCost < 0 {
		fuelCost = 0
	}
	if tollCost < 0 {
		tollCost = 0
	}

	totalSharedCost := fuelCost + tollCost
	costPerSeat := Money(RoundDivHalfUp(int64(totalSharedCost), int64(seatsOffered)))
	capPricePerSeat := Money(RoundDivHalfUp(
		int64(totalSharedCost)*(10000+DefaultContingencyMarginBps),
		int64(seatsOffered)*10000,
	))

	return CostMatrix{
		FuelCost:          fuelCost,
		TollCost:          tollCost,
		TotalSharedCost:   totalSharedCost,
		SeatsOffered:      seatsOffered,
		CostPerSeat:       costPerSeat,
		CapPricePerSeat:   capPricePerSeat,
		ContingencyMargin: DefaultContingencyMargin,
	}
}

// IsPricePermitted valida exactamente que el precio propuesto por el conductor no sea negativo ni supere el Cap Price
func (cm CostMatrix) IsPricePermitted(proposedPrice Money) bool {
	return proposedPrice >= 0 && proposedPrice <= cm.CapPricePerSeat
}
