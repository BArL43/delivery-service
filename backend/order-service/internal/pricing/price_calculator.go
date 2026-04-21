package pricing

import (
	"math"
	"os"
	"strconv"
)

// Config holds pricing rate parameters, loaded from environment variables.
type Config struct {
	BaseRate  float64 // Base delivery fee (default: 150)
	PerKmRate float64 // Price per kilometer (default: 20)
	PerKgRate float64 // Price per kilogram (default: 50)
}

// DefaultConfig returns Config with built-in defaults.
func DefaultConfig() Config {
	return Config{
		BaseRate:  150,
		PerKmRate: 20,
		PerKgRate: 50,
	}
}

// LoadConfig reads pricing rates from environment variables.
// Falls back to defaults for any missing or invalid values.
func LoadConfig() Config {
	cfg := DefaultConfig()

	if v := os.Getenv("PRICING_BASE_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.BaseRate = f
		}
	}
	if v := os.Getenv("PRICING_PER_KM_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.PerKmRate = f
		}
	}
	if v := os.Getenv("PRICING_PER_KG_RATE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			cfg.PerKgRate = f
		}
	}

	return cfg
}

// Calculator computes delivery price based on distance and weight.
type Calculator struct {
	cfg Config
}

// NewCalculator creates a new Calculator with the given config.
func NewCalculator(cfg Config) *Calculator {
	return &Calculator{cfg: cfg}
}

// Calculate returns the delivery price in rubles.
// Formula: base_rate + (distance_km * per_km_rate) + (weight_kg * per_kg_rate)
// Distance is rounded up to the nearest kilometer for billing.
func (c *Calculator) Calculate(distanceKm, weightKg float64) float64 {
	// Round distance up to nearest km for billing
	distanceBilled := math.Ceil(distanceKm)

	price := c.cfg.BaseRate +
		(distanceBilled * c.cfg.PerKmRate) +
		(weightKg * c.cfg.PerKgRate)

	// Round to 2 decimal places
	return math.Round(price*100) / 100
}
