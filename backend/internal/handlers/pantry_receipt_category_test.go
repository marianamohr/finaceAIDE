package handlers

import (
	"testing"

	"github.com/marianamohr/financas/internal/models"
)

func TestFinancialCategorySlug(t *testing.T) {
	tests := []struct {
		name            string
		category        string
		subcategory     string
		categoryContext string
		expected        string
	}{
		{
			name:            "food produce food_detail",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryProduce,
			categoryContext: "food_detail",
			expected:        finCategoryFrutaria,
		},
		{
			name:            "food produce standard",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryProduce,
			categoryContext: "standard",
			expected:        finCategoryAlimentacao,
		},
		{
			name:            "food beverages non alcohol food_detail",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryBeverages,
			categoryContext: "food_detail",
			expected:        finCategoryBebidasNaoAlcoolicas,
		},
		{
			name:            "food beverages non alcohol standard",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryBeverages,
			categoryContext: "standard",
			expected:        finCategoryAlimentacao,
		},
		{
			name:            "food alcohol food_detail",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryAlcohol,
			categoryContext: "food_detail",
			expected:        finCategoryBebidasAlcoolicas,
		},
		{
			name:            "food alcohol standard",
			category:        models.PantryCategoryFood,
			subcategory:     pantrySubcategoryAlcohol,
			categoryContext: "standard",
			expected:        finCategoryAlimentacao,
		},
		{
			name:            "food other subcategory food_detail",
			category:        models.PantryCategoryFood,
			subcategory:     "grains",
			categoryContext: "food_detail",
			expected:        finCategoryComida,
		},
		{
			name:            "food empty subcategory food_detail",
			category:        models.PantryCategoryFood,
			subcategory:     "",
			categoryContext: "food_detail",
			expected:        finCategoryComida,
		},
		{
			name:            "food other subcategory standard",
			category:        models.PantryCategoryFood,
			subcategory:     "grains",
			categoryContext: "standard",
			expected:        finCategoryAlimentacao,
		},
		{
			name:            "hygiene food_detail",
			category:        models.PantryCategoryHygiene,
			subcategory:     "anything",
			categoryContext: "food_detail",
			expected:        finCategoryHigienePessoal,
		},
		{
			name:            "hygiene standard",
			category:        models.PantryCategoryHygiene,
			subcategory:     "anything",
			categoryContext: "standard",
			expected:        finCategoryOutros,
		},
		{
			name:            "cleaning food_detail",
			category:        models.PantryCategoryCleaning,
			subcategory:     "anything",
			categoryContext: "food_detail",
			expected:        finCategoryLimpezaDaCasa,
		},
		{
			name:            "cleaning standard",
			category:        models.PantryCategoryCleaning,
			subcategory:     "anything",
			categoryContext: "standard",
			expected:        finCategoryCasa,
		},
		{
			name:            "general food_detail",
			category:        models.PantryCategoryGeneral,
			subcategory:     "anything",
			categoryContext: "food_detail",
			expected:        finCategoryOutros,
		},
		{
			name:            "general standard",
			category:        models.PantryCategoryGeneral,
			subcategory:     "anything",
			categoryContext: "standard",
			expected:        finCategoryOutros,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := financialCategorySlug(tt.category, tt.subcategory, tt.categoryContext)
			if got != tt.expected {
				t.Errorf("financialCategorySlug(%q, %q, %q) = %q, want %q",
					tt.category, tt.subcategory, tt.categoryContext, got, tt.expected)
			}
		})
	}
}
