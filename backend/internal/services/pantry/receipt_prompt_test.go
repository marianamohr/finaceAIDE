package pantry

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestBuildReceiptExtractionPrompt(t *testing.T) {
	t.Parallel()

	t.Run("includes the informed country and a catalog product", func(t *testing.T) {
		t.Parallel()

		productID := uuid.New()
		catalog := []models.PantryProduct{
			{ID: productID, NamePt: "Banana prata", UpdatedAt: time.Now()},
		}

		prompt := buildReceiptExtractionPrompt("DE", catalog)

		if !strings.Contains(prompt, "DE") {
			t.Error("prompt does not mention the informed country")
		}
		if !strings.Contains(prompt, productID.String()) {
			t.Error("prompt does not mention the catalog product id")
		}
		if !strings.Contains(prompt, "Banana prata") {
			t.Error("prompt does not mention the catalog product name")
		}
	})

	t.Run("falls back to detection and empty-catalog text", func(t *testing.T) {
		t.Parallel()

		prompt := buildReceiptExtractionPrompt("", nil)

		if !strings.Contains(prompt, "não informado") {
			t.Error("prompt does not contain the country fallback text")
		}
		if !strings.Contains(prompt, "catálogo vazio") {
			t.Error("prompt does not contain the empty-catalog fallback text")
		}
	})

	t.Run("caps the catalog at the prompt limit, most recently updated first", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		catalog := make([]models.PantryProduct, maxCatalogProductsInPrompt+5)
		for i := range catalog {
			catalog[i] = models.PantryProduct{
				ID:        uuid.New(),
				NamePt:    "produto",
				UpdatedAt: now.Add(time.Duration(i) * time.Minute),
			}
		}
		// The 5 oldest entries (index 0..4) must be excluded.
		excluded := catalog[:5]
		included := catalog[5]

		prompt := buildReceiptExtractionPrompt("BR", catalog)

		if !strings.Contains(prompt, included.ID.String()) {
			t.Error("prompt is missing a product that should have made the cutoff")
		}
		for _, product := range excluded {
			if strings.Contains(prompt, product.ID.String()) {
				t.Errorf("prompt contains product %s, which should have been dropped by the cap", product.ID)
			}
		}
	})
}

func TestReceiptExtractionSchemaIsValidJSON(t *testing.T) {
	t.Parallel()

	schema := receiptExtractionSchema()
	if !json.Valid(schema) {
		t.Fatalf("receiptExtractionSchema() is not valid JSON: %s", schema)
	}
}
