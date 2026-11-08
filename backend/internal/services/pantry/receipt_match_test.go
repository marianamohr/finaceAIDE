package pantry

import (
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestNormalizeMatchTerm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "lowercases", input: "Banana Prata", expected: "banana prata"},
		{name: "strips accents", input: "Açúcar Refinado", expected: "acucar refinado"},
		{name: "collapses whitespace", input: "  Leite   Integral  ", expected: "leite integral"},
		{name: "german umlaut", input: "Spülmittel", expected: "spulmittel"},
		{name: "empty string", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeMatchTerm(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeMatchTerm(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveMatch(t *testing.T) {
	t.Parallel()

	userProductID := uuid.New()
	aliasedProductID := uuid.New()
	aiSuggestedID := uuid.New()

	userProducts := map[uuid.UUID]models.PantryProduct{
		userProductID: {
			ID:          userProductID,
			NamePt:      "Banana prata",
			NameDe:      "Banane",
			Category:    models.PantryCategoryFood,
			Subcategory: "produce",
			DefaultUnit: "kg",
			IsProduce:   true,
		},
		aliasedProductID: {
			ID:          aliasedProductID,
			NamePt:      "Detergente lava-louças",
			NameDe:      "Spülmittel",
			Category:    models.PantryCategoryCleaning,
			Subcategory: "kitchen",
			DefaultUnit: "un",
			IsProduce:   false,
		},
		aiSuggestedID: {
			ID:          aiSuggestedID,
			NamePt:      "Arroz branco",
			NameDe:      "Reis",
			Category:    models.PantryCategoryFood,
			Subcategory: "grains",
			DefaultUnit: "kg",
			IsProduce:   false,
		},
	}

	userProductsByName := map[string]models.PantryProduct{
		normalizeMatchTerm("Banana prata"): userProducts[userProductID],
	}

	userAliases := map[string]models.PantryProductAlias{
		normalizeMatchTerm("SPUM LEMON 500ML"): {
			ProductID: aliasedProductID,
			RawName:   "SPUM LEMON 500ML",
		},
	}

	seedProducts := []models.PantrySeedProduct{
		{
			ID:          uuid.New(),
			NamePt:      "Leite integral",
			NameDe:      "Vollmilch",
			AliasesPt:   []string{"leite tipo a"},
			AliasesDe:   []string{"milch"},
			Category:    models.PantryCategoryFood,
			Subcategory: "dairy",
			DefaultUnit: "l",
			IsProduce:   false,
		},
	}

	tests := []struct {
		name                string
		item                extractedItem
		userAliases         map[string]models.PantryProductAlias
		userProducts        map[uuid.UUID]models.PantryProduct
		userProductsByName  map[string]models.PantryProduct
		seedProducts        []models.PantrySeedProduct
		expectedProductID   *uuid.UUID
		expectedMatchSource string
		expectedNamePt      string
	}{
		{
			name: "priority 1: user alias matches raw_name exactly",
			item: extractedItem{
				RawName:     "SPUM LEMON 500ML",
				NamePt:      "Detergente",
				NameDe:      "Spülmittel",
				Category:    models.PantryCategoryGeneral,
				Subcategory: "other",
				Unit:        "un",
			},
			userAliases:         userAliases,
			userProducts:        userProducts,
			userProductsByName:  userProductsByName,
			seedProducts:        seedProducts,
			expectedProductID:   &aliasedProductID,
			expectedMatchSource: models.PantryMatchAlias,
			expectedNamePt:      "Detergente lava-louças",
		},
		{
			name: "priority 2: user product with matching name_pt",
			item: extractedItem{
				RawName:     "BANANA PRATA KG",
				NamePt:      "Banana prata",
				NameDe:      "Banane",
				Category:    models.PantryCategoryFood,
				Subcategory: "produce",
				Unit:        "kg",
			},
			userAliases:         userAliases,
			userProducts:        userProducts,
			userProductsByName:  userProductsByName,
			seedProducts:        seedProducts,
			expectedProductID:   &userProductID,
			expectedMatchSource: models.PantryMatchProduct,
			expectedNamePt:      "Banana prata",
		},
		{
			name: "priority 3: seed catalog matches via alias",
			item: extractedItem{
				RawName:     "LEITE TIPO A 1L",
				NamePt:      "Leite tipo a",
				NameDe:      "Milch",
				Category:    models.PantryCategoryFood,
				Subcategory: "dairy",
				Unit:        "l",
			},
			userAliases:         map[string]models.PantryProductAlias{},
			userProducts:        map[uuid.UUID]models.PantryProduct{},
			userProductsByName:  map[string]models.PantryProduct{},
			seedProducts:        seedProducts,
			expectedProductID:   nil,
			expectedMatchSource: models.PantryMatchSeed,
			expectedNamePt:      "Leite integral",
		},
		{
			name: "priority 4: ai-suggested product_id exists in user catalog",
			item: extractedItem{
				RawName:     "ARROZ BRANCO 5KG",
				NamePt:      "Arroz branco tipo 1",
				NameDe:      "Reis",
				ProductID:   uuidPtr(aiSuggestedID),
				Category:    models.PantryCategoryFood,
				Subcategory: "grains",
				Unit:        "kg",
			},
			userAliases:         map[string]models.PantryProductAlias{},
			userProducts:        userProducts,
			userProductsByName:  map[string]models.PantryProduct{},
			seedProducts:        []models.PantrySeedProduct{},
			expectedProductID:   &aiSuggestedID,
			expectedMatchSource: models.PantryMatchAI,
			expectedNamePt:      "Arroz branco",
		},
		{
			name: "priority 5: no match at all",
			item: extractedItem{
				RawName:     "PRODUTO DESCONHECIDO XYZ",
				NamePt:      "Produto desconhecido",
				NameDe:      "Unbekanntes produkt",
				Category:    models.PantryCategoryGeneral,
				Subcategory: "other",
				Unit:        "un",
			},
			userAliases:         map[string]models.PantryProductAlias{},
			userProducts:        map[uuid.UUID]models.PantryProduct{},
			userProductsByName:  map[string]models.PantryProduct{},
			seedProducts:        seedProducts,
			expectedProductID:   nil,
			expectedMatchSource: models.PantryMatchNone,
			expectedNamePt:      "Produto desconhecido",
		},
		{
			name: "alias wins over a name and seed match that would also apply",
			item: extractedItem{
				RawName:     "SPUM LEMON 500ML",
				NamePt:      "Banana prata", // would hit priority 2 if alias were absent
				NameDe:      "Spülmittel",
				ProductID:   uuidPtr(aiSuggestedID), // would hit priority 4 if higher priorities were absent
				Category:    models.PantryCategoryGeneral,
				Subcategory: "other",
				Unit:        "un",
			},
			userAliases:         userAliases,
			userProducts:        userProducts,
			userProductsByName:  userProductsByName,
			seedProducts:        seedProducts,
			expectedProductID:   &aliasedProductID,
			expectedMatchSource: models.PantryMatchAlias,
			expectedNamePt:      "Detergente lava-louças",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveMatch(tt.item, tt.userAliases, tt.userProducts, tt.userProductsByName, tt.seedProducts)

			if got.MatchSource != tt.expectedMatchSource {
				t.Errorf("MatchSource = %q, want %q", got.MatchSource, tt.expectedMatchSource)
			}
			if got.NamePt != tt.expectedNamePt {
				t.Errorf("NamePt = %q, want %q", got.NamePt, tt.expectedNamePt)
			}
			if tt.expectedProductID == nil {
				if got.ProductID != nil {
					t.Errorf("ProductID = %v, want nil", *got.ProductID)
				}
				return
			}
			if got.ProductID == nil {
				t.Fatalf("ProductID = nil, want %v", *tt.expectedProductID)
			}
			if *got.ProductID != *tt.expectedProductID {
				t.Errorf("ProductID = %v, want %v", *got.ProductID, *tt.expectedProductID)
			}
		})
	}
}

func uuidPtr(id uuid.UUID) *string {
	s := id.String()
	return &s
}
