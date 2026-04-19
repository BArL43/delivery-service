package pricing

import (
	"os"
	"testing"
)

func TestCalculate(t *testing.T) {
	cfg := Config{
		BaseRate:  150,
		PerKmRate: 20,
		PerKgRate: 50,
	}
	calc := NewCalculator(cfg)

	tests := []struct {
		name       string
		distanceKm float64
		weightKg   float64
		want       float64
	}{
		{
			name:       "zero distance and weight",
			distanceKm: 0,
			weightKg:   0,
			want:       150, // base rate only
		},
		{
			name:       "5 km, 2 kg",
			distanceKm: 5,
			weightKg:   2,
			want:       150 + 5*20 + 2*50, // 300
		},
		{
			name:       "7.3 km (rounds up to 8), 2.5 kg",
			distanceKm: 7.3,
			weightKg:   2.5,
			want:       150 + 8*20 + 2.5*50, // 405
		},
		{
			name:       "0.1 km (rounds up to 1), 0.5 kg",
			distanceKm: 0.1,
			weightKg:   0.5,
			want:       150 + 1*20 + 0.5*50, // 195
		},
		{
			name:       "10 km, 0 kg",
			distanceKm: 10,
			weightKg:   0,
			want:       150 + 10*20, // 350
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calc.Calculate(tt.distanceKm, tt.weightKg)
			if got != tt.want {
				t.Errorf("Calculate(%v, %v) = %v, want %v", tt.distanceKm, tt.weightKg, got, tt.want)
			}
		})
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	// Save and restore original env
	origBase := os.Getenv("PRICING_BASE_RATE")
	origKm := os.Getenv("PRICING_PER_KM_RATE")
	origKg := os.Getenv("PRICING_PER_KG_RATE")
	defer func() {
		os.Setenv("PRICING_BASE_RATE", origBase)
		os.Setenv("PRICING_PER_KM_RATE", origKm)
		os.Setenv("PRICING_PER_KG_RATE", origKg)
	}()

	// Test custom values
	os.Setenv("PRICING_BASE_RATE", "200")
	os.Setenv("PRICING_PER_KM_RATE", "25")
	os.Setenv("PRICING_PER_KG_RATE", "60")

	cfg := LoadConfig()
	if cfg.BaseRate != 200 {
		t.Errorf("BaseRate = %v, want 200", cfg.BaseRate)
	}
	if cfg.PerKmRate != 25 {
		t.Errorf("PerKmRate = %v, want 25", cfg.PerKmRate)
	}
	if cfg.PerKgRate != 60 {
		t.Errorf("PerKgRate = %v, want 60", cfg.PerKgRate)
	}
}

func TestLoadConfigInvalidEnv(t *testing.T) {
	// Invalid values should fall back to defaults
	os.Setenv("PRICING_BASE_RATE", "not-a-number")
	os.Setenv("PRICING_PER_KM_RATE", "-10")
	os.Setenv("PRICING_PER_KG_RATE", "abc")

	cfg := LoadConfig()
	if cfg.BaseRate != 150 {
		t.Errorf("BaseRate = %v, want 150 (default)", cfg.BaseRate)
	}
	if cfg.PerKmRate != 20 {
		t.Errorf("PerKmRate = %v, want 20 (default)", cfg.PerKmRate)
	}
	if cfg.PerKgRate != 50 {
		t.Errorf("PerKgRate = %v, want 50 (default)", cfg.PerKgRate)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseRate != 150 || cfg.PerKmRate != 20 || cfg.PerKgRate != 50 {
		t.Errorf("DefaultConfig = %+v, expected defaults", cfg)
	}
}
