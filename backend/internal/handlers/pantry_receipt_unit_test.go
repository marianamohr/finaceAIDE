package handlers

import (
	"testing"

	"github.com/marianamohr/financas/internal/models"
)

func TestReconcileReceiptEntryUnit(t *testing.T) {
	t.Parallel()

	price := func(v float64) *float64 { return &v }

	t.Run("same unit passes through unchanged", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "kg"}
		qty, unit, unitPrice, err := reconcileReceiptEntryUnit(product, 10, "kg", price(5))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 10 || unit != "kg" || unitPrice == nil || *unitPrice != 5 {
			t.Errorf("got qty=%v unit=%q unitPrice=%v", qty, unit, unitPrice)
		}
	})

	t.Run("converts kg entry into a product tracked in g", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "g"}
		qty, unit, unitPrice, err := reconcileReceiptEntryUnit(product, 2, "kg", price(10))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 2000 || unit != "g" {
			t.Errorf("qty/unit: got %v/%q, want 2000/g", qty, unit)
		}
		if unitPrice == nil || *unitPrice != 0.01 {
			t.Errorf("unitPrice: got %v, want 0.01", unitPrice)
		}
	})

	t.Run("converts ml entry into a product tracked in l, keeping nil price nil", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Óleo", DefaultUnit: "l"}
		qty, unit, unitPrice, err := reconcileReceiptEntryUnit(product, 900, "ml", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 0.9 || unit != "l" {
			t.Errorf("qty/unit: got %v/%q, want 0.9/l", qty, unit)
		}
		if unitPrice != nil {
			t.Errorf("unitPrice: got %v, want nil", unitPrice)
		}
	})

	t.Run("rejects a weight entry against a product tracked by unit count", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "un"}
		_, _, _, err := reconcileReceiptEntryUnit(product, 10, "kg", price(5))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("rejects two incompatible count units", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Ovos", DefaultUnit: "dz"}
		_, _, _, err := reconcileReceiptEntryUnit(product, 6, "un", nil)
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
