package pantry

import (
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestMatchConsumeTextItem(t *testing.T) {
	t.Parallel()

	productID := uuid.New()
	product := models.PantryProduct{ID: productID, NamePt: "Ovo"}
	byID := map[uuid.UUID]models.PantryProduct{productID: product}
	byName := map[string]models.PantryProduct{"ovo": product}
	aliases := map[string]models.PantryProductAlias{
		"ovos caipira": {ProductID: productID},
	}

	t.Run("matches by exact normalized name", func(t *testing.T) {
		t.Parallel()
		got, ok := matchConsumeTextItem("Ovo", aliases, byID, byName)
		if !ok || got.ID != productID {
			t.Errorf("matchConsumeTextItem(%q) = %v, %v, want product %v", "Ovo", got, ok, productID)
		}
	})

	t.Run("matches by alias", func(t *testing.T) {
		t.Parallel()
		got, ok := matchConsumeTextItem("Ovos Caipira", aliases, byID, byName)
		if !ok || got.ID != productID {
			t.Errorf("matchConsumeTextItem(alias) = %v, %v, want product %v", got, ok, productID)
		}
	})

	t.Run("no match returns ok=false", func(t *testing.T) {
		t.Parallel()
		_, ok := matchConsumeTextItem("Leite", aliases, byID, byName)
		if ok {
			t.Error("matchConsumeTextItem(unknown name) ok = true, want false")
		}
	})
}

func TestReconcileConsumeTextUnit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		quantity     float64
		fromUnit     string
		toUnit       string
		wantQuantity float64
		wantOK       bool
	}{
		{name: "same unit passes through", quantity: 2, fromUnit: "un", toUnit: "un", wantQuantity: 2, wantOK: true},
		{name: "kg to g converts", quantity: 1, fromUnit: "kg", toUnit: "g", wantQuantity: 1000, wantOK: true},
		{name: "incompatible units are rejected", quantity: 2, fromUnit: "kg", toUnit: "un", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := reconcileConsumeTextUnit(tt.quantity, tt.fromUnit, tt.toUnit)
			if ok != tt.wantOK {
				t.Fatalf("reconcileConsumeTextUnit() ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.wantQuantity {
				t.Errorf("reconcileConsumeTextUnit() = %v, want %v", got, tt.wantQuantity)
			}
		})
	}
}
