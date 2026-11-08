package pantry

import (
	"math"
	"testing"

	"github.com/marianamohr/financas/internal/models"
)

func TestComputeShoppingNeed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		quantity     float64
		minLevel     *float64
		runningOut   bool
		unit         string
		wantNeeded   bool
		wantReason   string
		wantQuantity float64
	}{
		{
			name:         "zero quantity without min level is out with suggestion one",
			quantity:     0,
			minLevel:     nil,
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonOut,
			wantQuantity: 1,
		},
		{
			name:         "negative quantity without min level is out with suggestion one",
			quantity:     -2,
			minLevel:     nil,
			unit:         "kg",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonOut,
			wantQuantity: 1,
		},
		{
			name:         "zero quantity with min level is out with suggestion equal to min level",
			quantity:     0,
			minLevel:     floatPtr(5),
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonOut,
			wantQuantity: 5,
		},
		{
			name:         "positive quantity below min level is below_min with formula",
			quantity:     2,
			minLevel:     floatPtr(5),
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonBelowMin,
			wantQuantity: 8, // 5*2 - 2
		},
		{
			name:       "quantity equal to min level does not need restocking",
			quantity:   3,
			minLevel:   floatPtr(3),
			unit:       "un",
			wantNeeded: false,
		},
		{
			name:       "quantity above min level does not need restocking",
			quantity:   10,
			minLevel:   floatPtr(3),
			unit:       "un",
			wantNeeded: false,
		},
		{
			name:       "positive quantity without min level does not need restocking",
			quantity:   1,
			minLevel:   nil,
			unit:       "un",
			wantNeeded: false,
		},
		{
			name:         "count unit rounds suggestion up",
			quantity:     0.5,
			minLevel:     floatPtr(1.4),
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonBelowMin,
			wantQuantity: 3, // (1.4*2 - 0.5) = 2.3 -> ceil -> 3
		},
		{
			name:         "weight unit keeps decimal suggestion",
			quantity:     0.5,
			minLevel:     floatPtr(1.4),
			unit:         "kg",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonBelowMin,
			wantQuantity: 2.3,
		},
		{
			name:         "running out with no min level suggests one",
			quantity:     3,
			minLevel:     nil,
			runningOut:   true,
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonRunningOut,
			wantQuantity: 1,
		},
		{
			name:         "running out with a min level uses the below_min formula",
			quantity:     4,
			minLevel:     floatPtr(3),
			runningOut:   true,
			unit:         "un",
			wantNeeded:   true,
			wantReason:   models.PantryShoppingReasonRunningOut,
			wantQuantity: 2, // 3*2 - 4
		},
		{
			name:       "not running out and above min level does not need restocking",
			quantity:   10,
			minLevel:   floatPtr(3),
			runningOut: false,
			unit:       "un",
			wantNeeded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := computeShoppingNeed(tt.quantity, tt.minLevel, tt.runningOut, tt.unit)
			if got.Needed != tt.wantNeeded {
				t.Fatalf("computeShoppingNeed(%v, %v, %q).Needed = %v, want %v",
					tt.quantity, tt.minLevel, tt.unit, got.Needed, tt.wantNeeded)
			}
			if !tt.wantNeeded {
				return
			}
			if got.Reason != tt.wantReason {
				t.Errorf("computeShoppingNeed(%v, %v, %q).Reason = %q, want %q",
					tt.quantity, tt.minLevel, tt.unit, got.Reason, tt.wantReason)
			}
			if math.Abs(got.Quantity-tt.wantQuantity) > 1e-9 {
				t.Errorf("computeShoppingNeed(%v, %v, %q).Quantity = %v, want %v",
					tt.quantity, tt.minLevel, tt.unit, got.Quantity, tt.wantQuantity)
			}
		})
	}
}

func TestComputeShoppingNeedClampsBelowMinSuggestionToOne(t *testing.T) {
	t.Parallel()

	// minLevel*2 - quantity must be clamped to 1 when it would fall under it —
	// happens for small minLevel values close to quantity:
	// 0.2*2 - 0.15 = 0.25, which is below one.
	minLevel := 0.2
	got := computeShoppingNeed(0.15, &minLevel, false, "kg")

	if !got.Needed || got.Reason != models.PantryShoppingReasonBelowMin {
		t.Fatalf("got %+v, want needed below_min", got)
	}
	if math.Abs(got.Quantity-1) > 1e-9 {
		t.Errorf("Quantity = %v, want 1 (clamped)", got.Quantity)
	}
}

func TestRoundShoppingSuggestion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		quantity float64
		unit     string
		want     float64
	}{
		{name: "count unit rounds up fractional", quantity: 2.1, unit: "un", want: 3},
		{name: "count unit keeps whole number", quantity: 4, unit: "dz", want: 4},
		{name: "weight unit keeps two decimals", quantity: 2.345, unit: "kg", want: 2.35},
		{name: "volume unit keeps two decimals", quantity: 1.001, unit: "l", want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := roundShoppingSuggestion(tt.quantity, tt.unit)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Errorf("roundShoppingSuggestion(%v, %q) = %v, want %v", tt.quantity, tt.unit, got, tt.want)
			}
		})
	}
}
