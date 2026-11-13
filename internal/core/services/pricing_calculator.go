package services

import (
	"github.com/thumbi/auth-kyc-service/internal/core/domain"
)

// PricingCalculator define el contrato del motor de cálculo de costos de combustible, peajes y Cap Pricing
type PricingCalculator interface {
	CalculatePricing(distanceKm float64, tollCost domain.Money, seatsOffered int) domain.CostMatrix
	CalculatePricingWithCustomRates(distanceKm float64, tollCost domain.Money, seatsOffered int, consumptionLiters float64, pricePerLiter domain.Money) domain.CostMatrix
}

type pricingCalculator struct {
	defaultConsumption float64
	defaultFuelPrice   domain.Money
}

// NewPricingCalculator inicializa el calculador con valores de referencia estándar
func NewPricingCalculator() PricingCalculator {
	return &pricingCalculator{
		defaultConsumption: domain.DefaultFuelConsumptionLitersPer100Km,
		defaultFuelPrice:   domain.DefaultFuelPricePerLiter,
	}
}

// NewPricingCalculatorWithRates inicializa el calculador con tarifas personalizadas de combustible
func NewPricingCalculatorWithRates(consumptionLiters float64, pricePerLiter domain.Money) PricingCalculator {
	if consumptionLiters <= 0 {
		consumptionLiters = domain.DefaultFuelConsumptionLitersPer100Km
	}
	if pricePerLiter <= 0 {
		pricePerLiter = domain.DefaultFuelPricePerLiter
	}
	return &pricingCalculator{
		defaultConsumption: consumptionLiters,
		defaultFuelPrice:   pricePerLiter,
	}
}

// CalculatePricing calcula la matriz de costos y el Cap Price con las tasas vigentes
func (c *pricingCalculator) CalculatePricing(distanceKm float64, tollCost domain.Money, seatsOffered int) domain.CostMatrix {
	return c.CalculatePricingWithCustomRates(distanceKm, tollCost, seatsOffered, c.defaultConsumption, c.defaultFuelPrice)
}

// CalculatePricingWithCustomRates calcula los costos admitiendo parámetros de consumo específicos
func (c *pricingCalculator) CalculatePricingWithCustomRates(distanceKm float64, tollCost domain.Money, seatsOffered int, consumptionLiters float64, pricePerLiter domain.Money) domain.CostMatrix {
	fuelEstimate := domain.CalculateFuelEstimate(distanceKm, consumptionLiters, pricePerLiter)
	return domain.CalculateCapPrice(fuelEstimate.TotalFuelCost, tollCost, seatsOffered)
}
