package handlers

import "testing"

func TestResolvePantryMovementsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		raw      string
		expected int
	}{
		{name: "empty defaults to 50", raw: "", expected: 50},
		{name: "valid value is kept", raw: "10", expected: 10},
		{name: "above cap is truncated", raw: "500", expected: 200},
		{name: "exactly the cap", raw: "200", expected: 200},
		{name: "zero falls back to default", raw: "0", expected: 50},
		{name: "negative falls back to default", raw: "-5", expected: 50},
		{name: "non-numeric falls back to default", raw: "abc", expected: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolvePantryMovementsLimit(tt.raw)
			if got != tt.expected {
				t.Errorf("resolvePantryMovementsLimit(%q) = %d, want %d", tt.raw, got, tt.expected)
			}
		})
	}
}

func TestPantryProductFromRequest(t *testing.T) {
	t.Parallel()

	t.Run("zero min level becomes nil", func(t *testing.T) {
		t.Parallel()

		p := pantryProductFromRequest(pantryProductRequest{
			NamePt:   "Arroz",
			Category: "food",
			Unit:     "kg",
			MinLevel: 0,
		})
		if p.MinLevel != nil {
			t.Errorf("expected nil MinLevel, got %v", *p.MinLevel)
		}
	})

	t.Run("positive min level is kept", func(t *testing.T) {
		t.Parallel()

		p := pantryProductFromRequest(pantryProductRequest{
			NamePt:   "Arroz",
			Category: "food",
			Unit:     "kg",
			MinLevel: 3.5,
		})
		if p.MinLevel == nil || *p.MinLevel != 3.5 {
			t.Errorf("expected MinLevel 3.5, got %v", p.MinLevel)
		}
	})

	t.Run("fields are copied", func(t *testing.T) {
		t.Parallel()

		p := pantryProductFromRequest(pantryProductRequest{
			NamePt:      "Arroz",
			NameDe:      "Reis",
			Category:    "food",
			Subcategory: "grains",
			Unit:        "kg",
			IsProduce:   true,
			TrackByUnit: true,
		})
		if p.NamePt != "Arroz" || p.NameDe != "Reis" || p.Category != "food" ||
			p.Subcategory != "grains" || p.DefaultUnit != "kg" || !p.IsProduce || !p.TrackByUnit {
			t.Errorf("unexpected product: %+v", p)
		}
	})
}
