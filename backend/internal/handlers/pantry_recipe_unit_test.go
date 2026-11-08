package handlers

import (
	"testing"

	"github.com/marianamohr/financas/internal/models"
)

func TestReconcileRecipeDeductionUnit(t *testing.T) {
	t.Parallel()

	t.Run("same unit passes through unchanged", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "kg"}
		qty, unit, err := reconcileRecipeDeductionUnit(product, 0.5, "kg")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 0.5 || unit != "kg" {
			t.Errorf("got qty=%v unit=%q, want 0.5/kg", qty, unit)
		}
	})

	t.Run("converts g deduction into a product tracked in kg", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "kg"}
		qty, unit, err := reconcileRecipeDeductionUnit(product, 500, "g")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 0.5 || unit != "kg" {
			t.Errorf("got qty=%v unit=%q, want 0.5/kg", qty, unit)
		}
	})

	t.Run("converts ml deduction into a product tracked in l", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Óleo", DefaultUnit: "l"}
		qty, unit, err := reconcileRecipeDeductionUnit(product, 900, "ml")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if qty != 0.9 || unit != "l" {
			t.Errorf("got qty=%v unit=%q, want 0.9/l", qty, unit)
		}
	})

	t.Run("rejects a weight deduction against a product tracked by unit count", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Arroz", DefaultUnit: "un"}
		_, _, err := reconcileRecipeDeductionUnit(product, 10, "kg")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("rejects two incompatible count units", func(t *testing.T) {
		t.Parallel()

		product := &models.PantryProduct{NamePt: "Ovos", DefaultUnit: "dz"}
		_, _, err := reconcileRecipeDeductionUnit(product, 6, "un")
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}

func TestCountRecipeIngredients(t *testing.T) {
	t.Parallel()

	ingredients := []models.PantryRecipeIngredient{
		{Name: "Arroz", Have: true},
		{Name: "Feijão", Have: true},
		{Name: "Alho", Have: false},
	}

	have, missing := countRecipeIngredients(ingredients)
	if have != 2 || missing != 1 {
		t.Errorf("got have=%d missing=%d, want have=2 missing=1", have, missing)
	}
}

func TestValidateRecipeForSave(t *testing.T) {
	t.Parallel()

	t.Run("rejects empty title", func(t *testing.T) {
		t.Parallel()

		recipe := &models.PantryRecipe{
			Ingredients: []models.PantryRecipeIngredient{{Name: "Arroz"}},
		}
		if err := validateRecipeForSave(recipe); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("rejects no ingredients", func(t *testing.T) {
		t.Parallel()

		recipe := &models.PantryRecipe{Title: "Arroz com feijão"}
		if err := validateRecipeForSave(recipe); err == nil {
			t.Fatal("expected an error, got nil")
		}
	})

	t.Run("accepts a title and at least one ingredient", func(t *testing.T) {
		t.Parallel()

		recipe := &models.PantryRecipe{
			Title:       "Arroz com feijão",
			Ingredients: []models.PantryRecipeIngredient{{Name: "Arroz"}},
		}
		if err := validateRecipeForSave(recipe); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
