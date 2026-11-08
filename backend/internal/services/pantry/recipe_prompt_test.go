package pantry

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestBuildRecipeGenerationPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes filters, restrictions, servings, free text and a pantry item", func(t *testing.T) {
		t.Parallel()

		productID := uuid.New()
		pantry := []pantryIngredientContext{
			{ProductID: productID, Name: "Arroz", Quantity: 2, Unit: "kg", Status: StatusOK},
		}

		prompt := buildRecipeGenerationPrompt(
			[]string{"vegetarian", "quick"},
			[]string{"gluten_free"},
			4,
			"algo com frango",
			models.PantryLanguagePtBR,
			pantry,
		)

		for _, want := range []string{"vegetarian", "quick", "gluten_free", "4", "algo com frango", productID.String(), "Arroz"} {
			if !strings.Contains(prompt, want) {
				t.Errorf("prompt does not contain %q", want)
			}
		}
		if !strings.Contains(prompt, "português do Brasil") {
			t.Error("prompt does not reinforce pt-BR as the response language")
		}
	})

	t.Run("adds the german language reinforcement line", func(t *testing.T) {
		t.Parallel()

		prompt := buildRecipeGenerationPrompt(nil, nil, 2, "", models.PantryLanguageDe, nil)

		if !strings.Contains(prompt, "alemão") {
			t.Error("prompt does not reinforce de as the response language")
		}
	})

	t.Run("falls back to placeholder text when parameters are empty", func(t *testing.T) {
		t.Parallel()

		prompt := buildRecipeGenerationPrompt(nil, nil, 2, "", "", nil)

		if !strings.Contains(prompt, "(nenhum)") {
			t.Error("prompt does not contain the filters/free-text fallback")
		}
		if !strings.Contains(prompt, "(nenhuma)") {
			t.Error("prompt does not contain the restrictions fallback")
		}
		if !strings.Contains(prompt, "(dispensa vazia)") {
			t.Error("prompt does not contain the empty-pantry fallback")
		}
	})
}

func TestFormatPantryForPrompt(t *testing.T) {
	t.Parallel()

	okID := uuid.New()
	lowID := uuid.New()
	outID := uuid.New()
	pantry := []pantryIngredientContext{
		{ProductID: okID, Name: "Arroz", Quantity: 5, Unit: "kg", Status: StatusOK},
		{ProductID: lowID, Name: "Feijão", Quantity: 0.2, Unit: "kg", Status: StatusLow},
		{ProductID: outID, Name: "Leite", Quantity: 0, Unit: "l", Status: StatusOut},
	}

	formatted := formatPantryForPrompt(pantry)

	outPos := strings.Index(formatted, outID.String())
	lowPos := strings.Index(formatted, lowID.String())
	okPos := strings.Index(formatted, okID.String())
	if outPos == -1 || lowPos == -1 || okPos == -1 {
		t.Fatalf("formatted pantry is missing an entry: %s", formatted)
	}
	if !(outPos < lowPos && lowPos < okPos) {
		t.Errorf("expected out < low < ok ordering, got positions out=%d low=%d ok=%d", outPos, lowPos, okPos)
	}
}

func TestRecipeGenerationSchemaIsValidJSON(t *testing.T) {
	t.Parallel()

	schema := recipeGenerationSchema()
	if !json.Valid(schema) {
		t.Fatalf("recipeGenerationSchema() is not valid JSON: %s", schema)
	}
}

func TestValidateGeneratedRecipe(t *testing.T) {
	t.Parallel()

	validRecipe := func() generatedRecipe {
		return generatedRecipe{
			Title:       "Arroz com feijão",
			Language:    models.PantryLanguagePtBR,
			Servings:    2,
			PrepMinutes: 30,
			Ingredients: []generatedIngredient{
				{Name: "Arroz", Quantity: 1, Unit: "xicara", Have: true, Optional: false},
			},
			Steps: []string{"Cozinhe o arroz."},
			Tips:  "",
		}
	}

	tests := []struct {
		name    string
		mutate  func(r *generatedRecipe)
		wantErr bool
	}{
		{
			name:    "valid recipe",
			mutate:  func(r *generatedRecipe) {},
			wantErr: false,
		},
		{
			name:    "empty title",
			mutate:  func(r *generatedRecipe) { r.Title = "  " },
			wantErr: true,
		},
		{
			name:    "invalid language",
			mutate:  func(r *generatedRecipe) { r.Language = "en" },
			wantErr: true,
		},
		{
			name:    "zero servings",
			mutate:  func(r *generatedRecipe) { r.Servings = 0 },
			wantErr: true,
		},
		{
			name:    "negative prep minutes",
			mutate:  func(r *generatedRecipe) { r.PrepMinutes = -1 },
			wantErr: true,
		},
		{
			name:    "no ingredients",
			mutate:  func(r *generatedRecipe) { r.Ingredients = nil },
			wantErr: true,
		},
		{
			name: "invalid ingredient unit",
			mutate: func(r *generatedRecipe) {
				r.Ingredients[0].Unit = "invalid"
			},
			wantErr: true,
		},
		{
			name: "negative ingredient quantity",
			mutate: func(r *generatedRecipe) {
				r.Ingredients[0].Quantity = -1
			},
			wantErr: true,
		},
		{
			name: "recipe-specific unit is accepted",
			mutate: func(r *generatedRecipe) {
				r.Ingredients[0].Unit = "colher_sopa"
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := validRecipe()
			tt.mutate(&r)

			err := validateGeneratedRecipe(&r)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateGeneratedRecipe() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("nil recipe", func(t *testing.T) {
		t.Parallel()

		if err := validateGeneratedRecipe(nil); err == nil {
			t.Error("expected error for nil recipe, got nil")
		}
	})
}
