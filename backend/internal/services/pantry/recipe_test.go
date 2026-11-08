package pantry

import (
	"math"
	"testing"
)

func TestConvertStockToIngredientUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		stockQty       float64
		stockUnit      string
		ingredientUnit string
		expected       float64
	}{
		{
			name:           "same unit is a no-op",
			stockQty:       2,
			stockUnit:      "kg",
			ingredientUnit: "kg",
			expected:       2,
		},
		{
			name:           "kg stock converts to g ingredient",
			stockQty:       1.5,
			stockUnit:      "kg",
			ingredientUnit: "g",
			expected:       1500,
		},
		{
			name:           "l stock converts to ml ingredient",
			stockQty:       0.5,
			stockUnit:      "l",
			ingredientUnit: "ml",
			expected:       500,
		},
		{
			name:           "non-convertible units fall back to the raw stock quantity",
			stockQty:       3,
			stockUnit:      "un",
			ingredientUnit: "xicara",
			expected:       3,
		},
		{
			name:           "count units never convert into each other",
			stockQty:       12,
			stockUnit:      "dz",
			ingredientUnit: "un",
			expected:       12,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := convertStockToIngredientUnit(tt.stockQty, tt.stockUnit, tt.ingredientUnit)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("convertStockToIngredientUnit(%v, %q, %q) = %v, want %v",
					tt.stockQty, tt.stockUnit, tt.ingredientUnit, got, tt.expected)
			}
		})
	}
}

func TestRecipeServingScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		recipeServings    int
		requestedServings int
		expected          float64
	}{
		{name: "same servings scale to 1", recipeServings: 2, requestedServings: 2, expected: 1},
		{name: "double servings scale to 2", recipeServings: 2, requestedServings: 4, expected: 2},
		{name: "half servings scale to 0.5", recipeServings: 4, requestedServings: 2, expected: 0.5},
		{name: "zero requested servings uses recipe as-is", recipeServings: 3, requestedServings: 0, expected: 1},
		{name: "non-positive recipe servings treated as 1", recipeServings: 0, requestedServings: 2, expected: 2},
		{name: "negative recipe servings treated as 1", recipeServings: -1, requestedServings: 3, expected: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := recipeServingScale(tt.recipeServings, tt.requestedServings)
			if math.Abs(got-tt.expected) > 1e-9 {
				t.Errorf("recipeServingScale(%d, %d) = %v, want %v",
					tt.recipeServings, tt.requestedServings, got, tt.expected)
			}
		})
	}
}
