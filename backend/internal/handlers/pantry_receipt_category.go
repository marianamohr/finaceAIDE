package handlers

import "github.com/marianamohr/financas/internal/models"

// Financial category slugs a confirmed pantry receipt item can map to. These
// mirror the slugs seeded by migrations.Run (see migrations/migrate.go) — they
// are duplicated here (not imported) because the migrations package seeds the
// categories table and has no exported constants for the slugs.
const (
	finCategoryAlimentacao          = "alimentacao"
	finCategoryCasa                 = "casa"
	finCategoryOutros               = "outros"
	finCategoryComida               = "comida"
	finCategoryFrutaria             = "frutaria"
	finCategoryBebidasNaoAlcoolicas = "bebidas_nao_alcoolicas"
	finCategoryBebidasAlcoolicas    = "bebidas_alcoolicas"
	finCategoryHigienePessoal       = "higiene_pessoal"
	finCategoryLimpezaDaCasa        = "limpeza_da_casa"
)

// pantrySubcategoryProduce, pantrySubcategoryBeverages and
// pantrySubcategoryAlcohol are the fixed pantry subcategories relevant to the
// financial category mapping (see the "dispensa -> financeiro" table in the
// stage 2 spec). Every other subcategory of "food" falls into the generic
// comida/alimentacao bucket.
const (
	pantrySubcategoryProduce   = "produce"
	pantrySubcategoryBeverages = "beverages"
	pantrySubcategoryAlcohol   = "alcohol"
)

// financialCategorySlug maps a pantry receipt item's category/subcategory to
// the financial category slug used when confirming a receipt into a
// transaction. The mapping depends on the user's category_context
// ("food_detail" gets a finer breakdown; anything else, including "standard",
// gets the coarser buckets).
func financialCategorySlug(category, subcategory, categoryContext string) string {
	isFoodDetail := categoryContext == "food_detail"

	switch category {
	case models.PantryCategoryFood:
		return financialFoodCategorySlug(subcategory, isFoodDetail)
	case models.PantryCategoryHygiene:
		if isFoodDetail {
			return finCategoryHigienePessoal
		}
		return finCategoryOutros
	case models.PantryCategoryCleaning:
		if isFoodDetail {
			return finCategoryLimpezaDaCasa
		}
		return finCategoryCasa
	default:
		return finCategoryOutros
	}
}

// financialFoodCategorySlug resolves the "food" branch of financialCategorySlug.
func financialFoodCategorySlug(subcategory string, isFoodDetail bool) string {
	if !isFoodDetail {
		return finCategoryAlimentacao
	}
	switch subcategory {
	case pantrySubcategoryProduce:
		return finCategoryFrutaria
	case pantrySubcategoryBeverages:
		return finCategoryBebidasNaoAlcoolicas
	case pantrySubcategoryAlcohol:
		return finCategoryBebidasAlcoolicas
	default:
		return finCategoryComida
	}
}
