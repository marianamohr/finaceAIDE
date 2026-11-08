package pantry

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

// recipeUnits lists the units a recipe ingredient quantity can be expressed
// in. It is a superset of models.PantryUnits with a few units specific to
// cooking measurements (tablespoon, teaspoon, cup, "to taste") that make no
// sense for pantry stock tracking itself.
var recipeUnits = []string{
	"un", "kg", "g", "l", "ml", "pct", "maco", "dz",
	"colher_sopa", "colher_cha", "xicara", "a_gosto",
}

// pantryIngredientContext is the minimal shape of a user's pantry item needed
// to build the recipe generation prompt, decoupled from
// pantry.ProductWithStock so this file does not depend on the stock service's
// return type.
type pantryIngredientContext struct {
	ProductID uuid.UUID
	Name      string
	Quantity  float64
	Unit      string
	Status    string // StatusOK | StatusLow | StatusOut | StatusRunningOut
	Expiring  bool
}

const recipeGenerationPromptTemplate = `Você é um chef caseiro prático que ajuda a cozinhar com o que já tem em casa.

Crie UMA receita usando prioritariamente os ingredientes disponíveis na dispensa do usuário, listada abaixo, respeitando os filtros e restrições informados. Escreva o título, os ingredientes e o modo de preparo no idioma pedido (pt-BR ou de). Use nomes de ingredientes comuns no país informado — não sugira algo difícil de achar lá.

Quando o filtro pedir para priorizar o que está acabando ou vencendo, dê preferência aos itens da dispensa marcados como tal na lista abaixo. Mantenha as quantidades realistas para o número de porções pedido.

Para cada ingrediente da receita, diga se o usuário provavelmente tem (compare com a dispensa listada abaixo) e, quando o ingrediente corresponder a um produto da dispensa, preencha product_id com o id exato da lista — senão devolva null. NÃO invente que o usuário tem algo que não está na lista. Sal, água, óleo, pimenta e temperos básicos podem ser assumidos como sempre disponíveis (product_id null, have true) mesmo que não estejam na lista.

FILTROS APLICADOS: %s
RESTRIÇÕES ALIMENTARES: %s
PORÇÕES: %d
TEXTO LIVRE DO USUÁRIO: %s

DISPENSA DO USUÁRIO (id → nome, quantidade, status):
%s
`

// buildRecipeGenerationPrompt assembles the system prompt sent to the AI for
// recipe generation: the requested filters, dietary restrictions, servings
// and free text (each with a fallback when empty), plus the user's pantry
// formatted with formatPantryForPrompt. When language is "pt-BR" or "de", an
// extra line reinforces the response language beyond the schema's own
// "language" field.
func buildRecipeGenerationPrompt(
	filters, restrictions []string,
	servings int,
	freeText, language string,
	pantry []pantryIngredientContext,
) string {
	prompt := fmt.Sprintf(
		recipeGenerationPromptTemplate,
		joinOrFallback(filters, "(nenhum)"),
		joinOrFallback(restrictions, "(nenhuma)"),
		servings,
		stringOrFallback(freeText, "(nenhum)"),
		formatPantryForPrompt(pantry),
	)

	switch language {
	case "de":
		prompt += "\nResponda em alemão (título, ingredientes e modo de preparo)."
	case "pt-BR":
		prompt += "\nResponda em português do Brasil (título, ingredientes e modo de preparo)."
	}
	return prompt
}

// joinOrFallback joins values with ", ", or returns fallback when values is
// empty.
func joinOrFallback(values []string, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	return strings.Join(values, ", ")
}

// stringOrFallback returns value trimmed, or fallback when it is empty.
func stringOrFallback(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

// pantryStatusPromptPriority orders the pantry listing so low/out items (the
// ones a "use what's running out" filter should prioritize) come first.
var pantryStatusPromptPriority = map[string]int{
	StatusOut:        0,
	StatusLow:        1,
	StatusRunningOut: 2,
	StatusOK:         3,
}

// formatPantryForPrompt renders the "- <uuid>: <name> (<quantity> <unit>,
// status: <status>)" list embedded in the recipe prompt, ordered with
// low/out items first (recipes should prefer what's running out) and capped
// at maxCatalogProductsInPrompt entries, same cap used for the receipt
// extraction prompt.
func formatPantryForPrompt(pantry []pantryIngredientContext) string {
	if len(pantry) == 0 {
		return "(dispensa vazia)"
	}

	sorted := make([]pantryIngredientContext, len(pantry))
	copy(sorted, pantry)
	sort.SliceStable(sorted, func(i, j int) bool {
		return pantryStatusPromptPriority[sorted[i].Status] < pantryStatusPromptPriority[sorted[j].Status]
	})
	if len(sorted) > maxCatalogProductsInPrompt {
		sorted = sorted[:maxCatalogProductsInPrompt]
	}

	var b strings.Builder
	for i, item := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- %s: %s (%v %s, status: %s", item.ProductID, item.Name, item.Quantity, item.Unit, item.Status)
		if item.Expiring {
			b.WriteString(", vencendo")
		}
		b.WriteString(")")
	}
	return b.String()
}

// recipeGenerationSchema returns the JSON Schema the AI's response must match
// (Anthropic output_config.format = json_schema). Written as a literal, same
// rationale as receiptExtractionSchema: the schema is fixed and deeply
// nested, so a literal is easier to compare against the spec than a struct
// definition would be. Its validity is checked in
// TestRecipeGenerationSchemaIsValidJSON.
func recipeGenerationSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["title", "language", "servings", "prep_minutes", "ingredients", "steps", "tips"],
  "properties": {
    "title": {"type": "string"},
    "language": {"type": "string", "enum": ["pt-BR", "de"]},
    "servings": {"type": "integer"},
    "prep_minutes": {"type": "integer"},
    "ingredients": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["name", "product_id", "quantity", "unit", "have", "optional"],
        "properties": {
          "name": {"type": "string"},
          "product_id": {"type": ["string", "null"]},
          "quantity": {"type": "number"},
          "unit": {"type": "string", "enum": ["un", "kg", "g", "l", "ml", "pct", "maco", "dz", "colher_sopa", "colher_cha", "xicara", "a_gosto"]},
          "have": {"type": "boolean"},
          "optional": {"type": "boolean"}
        }
      }
    },
    "steps": {"type": "array", "items": {"type": "string"}},
    "tips": {"type": "string"}
  }
}`)
}

// generatedRecipe is the shape of the AI's structured response for recipe
// generation, matching recipeGenerationSchema exactly.
type generatedRecipe struct {
	Title       string                `json:"title"`
	Language    string                `json:"language"`
	Servings    int                   `json:"servings"`
	PrepMinutes int                   `json:"prep_minutes"`
	Ingredients []generatedIngredient `json:"ingredients"`
	Steps       []string              `json:"steps"`
	Tips        string                `json:"tips"`
}

// generatedIngredient is one ingredient of generatedRecipe.
type generatedIngredient struct {
	Name      string  `json:"name"`
	ProductID *string `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
	Have      bool    `json:"have"`
	Optional  bool    `json:"optional"`
}

// validateGeneratedRecipe checks that r is internally consistent enough to
// use. Its error is never shown to the end user directly — it only drives
// the retry/failure logic in RecipeService.Generate.
func validateGeneratedRecipe(r *generatedRecipe) error {
	if r == nil {
		return fmt.Errorf("pantry: receita ausente")
	}
	if strings.TrimSpace(r.Title) == "" {
		return fmt.Errorf("pantry: título da receita ausente")
	}
	if r.Language != models.PantryLanguagePtBR && r.Language != models.PantryLanguageDe {
		return fmt.Errorf("pantry: language inválido: %q", r.Language)
	}
	if r.Servings < 1 {
		return fmt.Errorf("pantry: servings inválido: %d", r.Servings)
	}
	if r.PrepMinutes < 0 {
		return fmt.Errorf("pantry: prep_minutes inválido: %d", r.PrepMinutes)
	}
	if len(r.Ingredients) == 0 {
		return fmt.Errorf("pantry: receita sem ingredientes")
	}
	for i, ingredient := range r.Ingredients {
		if err := validateGeneratedIngredient(ingredient); err != nil {
			return fmt.Errorf("pantry: ingrediente %d: %w", i, err)
		}
	}
	return nil
}

// validateGeneratedIngredient checks a single generated ingredient.
func validateGeneratedIngredient(ingredient generatedIngredient) error {
	if !slices.Contains(recipeUnits, ingredient.Unit) {
		return fmt.Errorf("unidade inválida: %q", ingredient.Unit)
	}
	if ingredient.Quantity < 0 {
		return fmt.Errorf("quantidade negativa: %v", ingredient.Quantity)
	}
	return nil
}
