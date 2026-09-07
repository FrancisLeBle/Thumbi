package domain

import (
	"math"
)

// Constantes de referencia para economía no lucrativa de gastos compartidos
const (
	// DefaultFuelConsumptionLitersPer100Km promedio nacional de vehículos de combustión/mixtos (8.5L / 100km)
	DefaultFuelConsumptionLitersPer100Km = 8.5
	// DefaultFuelPricePerLiter referencia base de combustible en moneda local (ej: $1100 ARS / litro)
	DefaultFuelPricePerLiter = 1100.0
	// DefaultContingencyMargin margen de imprevistos permitido (10% según RF-06 sin generar lucro)
	DefaultContingencyMargin = 0.10
)

// FuelEstimate almacena el desglose del costo estimado de combustible
type FuelEstimate struct {
	DistanceKm              float64 `json:"distance_km"`
	ConsumptionLitersPer100 float64 `json:"consumption_liters_per_100"`
	FuelPricePerLiter       float64 `json:"fuel_price_per_liter"`
	TotalFuelCost           float64 `json:"total_fuel_cost"`
}

// TollEstimate almacena el costo estimado de peajes a lo largo de la traza
type TollEstimate struct {
	TollsCount    int     `json:"tolls_count"`
	TotalTollCost float64 `json:"total_toll_cost"`
}

// CostMatrix matriz consolidada de costos operativos reales estimados
type CostMatrix struct {
	FuelCost          float64 `json:"fuel_cost"`
	TollCost          float64 `json:"toll_cost"`
	TotalSharedCost   float64 `json:"total_shared_cost"`
	SeatsOffered      int     `json:"seats_offered"`
	CostPerSeat       float64 `json:"cost_per_seat"`
	CapPricePerSeat   float64 `json:"cap_price_per_seat"`
	ContingencyMargin float64 `json:"contingency_margin"`
}

// CalculateFuelEstimate calcula el costo de combustible en base a la distancia y parámetros de consumo
func CalculateFuelEstimate(distanceKm float64, consumptionLiters, pricePerLiter float64) FuelEstimate {
	if consumptionLiters <= 0 {
		consumptionLiters = DefaultFuelConsumptionLitersPer100Km
	}
	if pricePerLiter <= 0 {
		pricePerLiter = DefaultFuelPricePerLiter
	}

	litersNeeded := (distanceKm / 100.0) * consumptionLiters
	totalCost := litersNeeded * pricePerLiter

	return FuelEstimate{
		DistanceKm:              math.Round(distanceKm*100) / 100,
		ConsumptionLitersPer100: consumptionLiters,
		FuelPricePerLiter:       pricePerLiter,
		TotalFuelCost:           math.Round(totalCost*100) / 100,
	}
}

// CalculateCapPrice calcula el precio tope máximo (Cap Price) por asiento garantizando el modelo no lucrativo
// Fórmula EARS RF-06:
// CapPrice = ((CostFuel + CostTolls) / Seats) * (1 + 0.10)
func CalculateCapPrice(fuelCost, tollCost float64, seatsOffered int) CostMatrix {
	if seatsOffered <= 0 {
		seatsOffered = 1
	}

	totalSharedCost := fuelCost + tollCost
	costPerSeat := totalSharedCost / float64(seatsOffered)
	rawCapPrice := costPerSeat * (1.0 + DefaultContingencyMargin)

	// Redondeo a 2 decimales
	capPriceRounded := math.Round(rawCapPrice*100) / 100
	costPerSeatRounded := math.Round(costPerSeat*100) / 100
	totalSharedRounded := math.Round(totalSharedCost*100) / 100

	return CostMatrix{
		FuelCost:          math.Round(fuelCost*100) / 100,
		TollCost:          math.Round(tollCost*100) / 100,
		TotalSharedCost:   totalSharedRounded,
		SeatsOffered:      seatsOffered,
		CostPerSeat:       costPerSeatRounded,
		CapPricePerSeat:   capPriceRounded,
		ContingencyMargin: DefaultContingencyMargin,
	}
}

// IsPricePermitted valida que el precio propuesto por el conductor no supere el Cap Price
func (cm CostMatrix) IsPricePermitted(proposedPrice float64) bool {
	// Tolerancia de precisión en punto flotante
	return proposedPrice <= (cm.CapPricePerSeat + 0.001)
}
