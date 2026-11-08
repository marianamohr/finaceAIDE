package pantry

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// normalizeMatchTerm normalizes a term for comparison: lowercase, no accents,
// collapsed whitespace. Same idea as normalizePantryRawName in
// internal/repository/pantry_product_alias.go; duplicated here (rather than
// exported from there) because it is ~10 lines and the two packages have no
// other coupling that would justify a shared helper package for this alone.
func normalizeMatchTerm(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	}), norm.NFC)
	result, _, _ := transform.String(t, strings.ToLower(strings.TrimSpace(s)))
	return strings.Join(strings.Fields(result), " ")
}

// matchResult is the outcome of matching one receipt item against the user's
// catalog and the global seed catalog.
type matchResult struct {
	ProductID   *uuid.UUID // non-nil when an EXISTING product (from the user's catalog) was found
	MatchSource string     // models.PantryMatchAlias | Product | Seed | AI | None
	NamePt      string     // canonical name suggested (from the seed/product on a match; otherwise the AI's name_pt)
	NameDe      string
	Category    string // suggested category when it comes from the seed (overrides the AI's, which is less reliable); otherwise the AI's
	Subcategory string
	DefaultUnit string // suggested unit when it comes from the seed/existing product; otherwise the item's unit
	IsProduce   bool
}

// resolveMatch decides the origin of the match, in priority order: (1) an
// alias already registered by the user for this exact raw_name, (2) a
// product in the user's catalog with an equal (normalized) name_pt, (3) the
// global seed catalog: matching name_pt/name_de or present in
// aliases_pt/aliases_de, (4) the product_id the AI suggested, if it really
// exists in the user's catalog, (5) no match at all — the item becomes a "new
// product" in the review screen.
func resolveMatch(
	item extractedItem,
	userAliases map[string]models.PantryProductAlias,
	userProducts map[uuid.UUID]models.PantryProduct,
	userProductsByName map[string]models.PantryProduct,
	seedProducts []models.PantrySeedProduct,
) matchResult {
	rawNameNorm := normalizeMatchTerm(item.RawName)
	namePtNorm := normalizeMatchTerm(item.NamePt)
	nameDeNorm := normalizeMatchTerm(item.NameDe)

	if alias, ok := userAliases[rawNameNorm]; ok {
		if product, ok := userProducts[alias.ProductID]; ok {
			return matchFromProduct(&product, models.PantryMatchAlias)
		}
		productID := alias.ProductID
		result := matchFromItem(item, models.PantryMatchAlias)
		result.ProductID = &productID
		return result
	}

	if product, ok := userProductsByName[namePtNorm]; ok {
		return matchFromProduct(&product, models.PantryMatchProduct)
	}

	if seed := findSeedMatch(seedProducts, rawNameNorm, namePtNorm, nameDeNorm); seed != nil {
		return matchFromSeed(seed)
	}

	if item.ProductID != nil {
		if id, err := uuid.Parse(*item.ProductID); err == nil {
			if product, ok := userProducts[id]; ok {
				return matchFromProduct(&product, models.PantryMatchAI)
			}
		}
	}

	return matchFromItem(item, models.PantryMatchNone)
}

// matchFromProduct builds a matchResult around an existing user catalog
// product, whose canonical fields take priority over the AI's reading.
func matchFromProduct(product *models.PantryProduct, source string) matchResult {
	id := product.ID
	return matchResult{
		ProductID:   &id,
		MatchSource: source,
		NamePt:      product.NamePt,
		NameDe:      product.NameDe,
		Category:    product.Category,
		Subcategory: product.Subcategory,
		DefaultUnit: product.DefaultUnit,
		IsProduce:   product.IsProduce,
	}
}

// matchFromSeed builds a matchResult around a global seed catalog entry. No
// user product exists yet, so ProductID stays nil.
func matchFromSeed(seed *models.PantrySeedProduct) matchResult {
	return matchResult{
		ProductID:   nil,
		MatchSource: models.PantryMatchSeed,
		NamePt:      seed.NamePt,
		NameDe:      seed.NameDe,
		Category:    seed.Category,
		Subcategory: seed.Subcategory,
		DefaultUnit: seed.DefaultUnit,
		IsProduce:   seed.IsProduce,
	}
}

// matchFromItem builds a matchResult purely from the AI's own reading of the
// item, used when no catalog entry backs the match (or as a defensive
// fallback when an alias points at a product id no longer in userProducts).
func matchFromItem(item extractedItem, source string) matchResult {
	return matchResult{
		ProductID:   nil,
		MatchSource: source,
		NamePt:      item.NamePt,
		NameDe:      item.NameDe,
		Category:    item.Category,
		Subcategory: item.Subcategory,
		DefaultUnit: item.Unit,
		IsProduce:   item.IsProduce,
	}
}

// findSeedMatch linearly scans the global seed catalog (~150 rows, small
// enough that this is simpler and fast enough compared to a JSONB query) for
// an entry whose name_pt/name_de or aliases_pt/aliases_de match either the
// item's raw_name or its AI-read name_pt/name_de, all normalized.
func findSeedMatch(seedProducts []models.PantrySeedProduct, rawNameNorm, namePtNorm, nameDeNorm string) *models.PantrySeedProduct {
	for i := range seedProducts {
		seed := &seedProducts[i]
		if seedMatchesTerm(seed.NamePt, rawNameNorm, namePtNorm) {
			return seed
		}
		if seedMatchesTerm(seed.NameDe, rawNameNorm, nameDeNorm) {
			return seed
		}
		for _, alias := range seed.AliasesPt {
			if seedMatchesTerm(alias, rawNameNorm, namePtNorm) {
				return seed
			}
		}
		for _, alias := range seed.AliasesDe {
			if seedMatchesTerm(alias, rawNameNorm, nameDeNorm) {
				return seed
			}
		}
	}
	return nil
}

// seedMatchesTerm reports whether candidate, once normalized, equals either
// of the two already-normalized terms (empty terms never match).
func seedMatchesTerm(candidate, termA, termB string) bool {
	if candidate == "" {
		return false
	}
	normalized := normalizeMatchTerm(candidate)
	return (termA != "" && normalized == termA) || (termB != "" && normalized == termB)
}
