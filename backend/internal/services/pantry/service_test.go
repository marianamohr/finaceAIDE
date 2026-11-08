package pantry

import (
	"math"
	"testing"
	"time"

	"github.com/marianamohr/financas/internal/models"
)

func floatPtr(v float64) *float64 { return &v }

func TestComputeStatus(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	soon := now.Add(2 * 24 * time.Hour)
	far := now.Add(30 * 24 * time.Hour)

	tests := []struct {
		name                 string
		quantity             float64
		minLevel             *float64
		runOutAt             *time.Time
		shoppingIntervalDays int
		expected             string
	}{
		{
			name:     "zero quantity is out",
			quantity: 0,
			minLevel: nil,
			expected: StatusOut,
		},
		{
			name:     "negative quantity is out",
			quantity: -1,
			minLevel: nil,
			expected: StatusOut,
		},
		{
			name:     "below minimum is low",
			quantity: 2,
			minLevel: floatPtr(3),
			expected: StatusLow,
		},
		{
			name:     "equal to minimum is ok",
			quantity: 3,
			minLevel: floatPtr(3),
			expected: StatusOK,
		},
		{
			name:     "above minimum is ok",
			quantity: 5,
			minLevel: floatPtr(3),
			expected: StatusOK,
		},
		{
			name:     "nil min level with positive quantity is ok",
			quantity: 5,
			minLevel: nil,
			expected: StatusOK,
		},
		{
			name:                 "forecast to run out within shopping interval is running_out",
			quantity:             5,
			minLevel:             nil,
			runOutAt:             &soon,
			shoppingIntervalDays: 7,
			expected:             StatusRunningOut,
		},
		{
			name:                 "forecast to run out well after shopping interval is ok",
			quantity:             5,
			minLevel:             nil,
			runOutAt:             &far,
			shoppingIntervalDays: 7,
			expected:             StatusOK,
		},
		{
			name:                 "below minimum wins over running_out",
			quantity:             2,
			minLevel:             floatPtr(3),
			runOutAt:             &soon,
			shoppingIntervalDays: 7,
			expected:             StatusLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := computeStatus(tt.quantity, tt.minLevel, tt.runOutAt, tt.shoppingIntervalDays, now)
			if got != tt.expected {
				t.Errorf("computeStatus(%v, %v, %v, %v) = %q, want %q",
					tt.quantity, tt.minLevel, tt.runOutAt, tt.shoppingIntervalDays, got, tt.expected)
			}
		})
	}
}

func TestUnitConversionFactor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		from, to   string
		wantFactor float64
		wantOK     bool
	}{
		{name: "same unit is a no-op", from: "kg", to: "kg", wantFactor: 1, wantOK: true},
		{name: "kg to g multiplies by 1000", from: "kg", to: "g", wantFactor: 1000, wantOK: true},
		{name: "g to kg divides by 1000", from: "g", to: "kg", wantFactor: 1.0 / 1000, wantOK: true},
		{name: "l to ml multiplies by 1000", from: "l", to: "ml", wantFactor: 1000, wantOK: true},
		{name: "ml to l divides by 1000", from: "ml", to: "l", wantFactor: 1.0 / 1000, wantOK: true},
		{name: "weight to volume does not convert", from: "kg", to: "l", wantOK: false},
		{name: "weight to count does not convert", from: "kg", to: "un", wantOK: false},
		{name: "count to count does not convert", from: "un", to: "dz", wantOK: false},
		{name: "different count units never convert even in reverse", from: "dz", to: "un", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			factor, ok := UnitConversionFactor(tt.from, tt.to)
			if ok != tt.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if ok && math.Abs(factor-tt.wantFactor) > 1e-9 {
				t.Errorf("factor: got %v, want %v", factor, tt.wantFactor)
			}
		})
	}
}

func TestApplyWeightedAveragePrice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		currentAvg *float64
		currentQty float64
		newQty     float64
		newPrice   float64
		expected   float64
	}{
		{
			name:       "first entry has no current average",
			currentAvg: nil,
			currentQty: 0,
			newQty:     2,
			newPrice:   5.0,
			expected:   5.0,
		},
		{
			name:       "second entry weights by quantity",
			currentAvg: floatPtr(4.0),
			currentQty: 3,
			newQty:     1,
			newPrice:   8.0,
			// (4*3 + 8*1) / (3+1) = 20/4 = 5.0
			expected: 5.0,
		},
		{
			name:       "zero total quantity falls back to new price",
			currentAvg: floatPtr(4.0),
			currentQty: 0,
			newQty:     0,
			newPrice:   7.5,
			expected:   7.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := applyWeightedAveragePrice(tt.currentAvg, tt.currentQty, tt.newQty, tt.newPrice)
			if math.IsNaN(got) {
				t.Fatalf("applyWeightedAveragePrice returned NaN")
			}
			const epsilon = 1e-9
			if math.Abs(got-tt.expected) > epsilon {
				t.Errorf("applyWeightedAveragePrice(...) = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestClampConsume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   float64
		requested float64
		expected  float64
	}{
		{
			name:      "consume less than available",
			current:   10,
			requested: 4,
			expected:  6,
		},
		{
			name:      "consume exactly available",
			current:   5,
			requested: 5,
			expected:  0,
		},
		{
			name:      "consume more than available clamps to zero",
			current:   3,
			requested: 10,
			expected:  0,
		},
		{
			name:      "consume from empty stock stays zero",
			current:   0,
			requested: 2,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := clampConsume(tt.current, tt.requested)
			if got != tt.expected {
				t.Errorf("clampConsume(%v, %v) = %v, want %v", tt.current, tt.requested, got, tt.expected)
			}
		})
	}
}

func TestValidateProduct(t *testing.T) {
	t.Parallel()

	validProduct := func() *models.PantryProduct {
		return &models.PantryProduct{
			NamePt:      "Arroz",
			Category:    models.PantryCategoryFood,
			DefaultUnit: "kg",
		}
	}

	tests := []struct {
		name    string
		mutate  func(*models.PantryProduct)
		wantErr bool
	}{
		{
			name:    "valid product",
			mutate:  func(p *models.PantryProduct) {},
			wantErr: false,
		},
		{
			name:    "empty name is rejected",
			mutate:  func(p *models.PantryProduct) { p.NamePt = "  " },
			wantErr: true,
		},
		{
			name:    "invalid category is rejected",
			mutate:  func(p *models.PantryProduct) { p.Category = "toys" },
			wantErr: true,
		},
		{
			name:    "invalid unit is rejected",
			mutate:  func(p *models.PantryProduct) { p.DefaultUnit = "lbs" },
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			product := validProduct()
			tt.mutate(product)
			err := validateProduct(product)

			if tt.wantErr && err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}
