package pantry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
)

// ErrRecipeGenerationFailed indicates the AI could not produce a valid
// recipe even after a second attempt — the caller (handler) must translate
// this into a user-friendly message, not expose the technical error.
var ErrRecipeGenerationFailed = errors.New("pantry: não foi possível gerar a receita")

// recipeGenerationMaxTokens is the response budget for a single recipe
// generation call.
const recipeGenerationMaxTokens = 4000

// defaultRecipeServings is used when GenerateRecipeParams.Servings is not
// informed (0).
const defaultRecipeServings = 2

// RecipeService generates cooking recipes from the user's pantry via AI,
// and recomputes what the user actually has (have/have_quantity) against
// live stock — never trusting the AI's own guess about what is on hand.
type RecipeService struct {
	ai        ai.Provider
	products  *repository.PantryProductRepository
	stocks    *repository.PantryStockRepository
	settings  *repository.PantrySettingsRepository
	recipes   *repository.PantryRecipeRepository
	movements *repository.PantryMovementRepository
}

// NewRecipeService wires the pantry repositories and the AI provider used by
// the recipe generation flow.
func NewRecipeService(
	aiProvider ai.Provider,
	products *repository.PantryProductRepository,
	stocks *repository.PantryStockRepository,
	settings *repository.PantrySettingsRepository,
	recipes *repository.PantryRecipeRepository,
	movements *repository.PantryMovementRepository,
) *RecipeService {
	return &RecipeService{
		ai:        aiProvider,
		products:  products,
		stocks:    stocks,
		settings:  settings,
		recipes:   recipes,
		movements: movements,
	}
}

// GenerateRecipeParams are the inputs to Generate.
type GenerateRecipeParams struct {
	UserID       uuid.UUID
	Filters      []string
	Restrictions []string
	Servings     int // 0 = usar defaultRecipeServings
	FreeText     string
	Language     string // "" = usar o idioma das configurações do usuário (pantry_settings)
}

// Generate calls the AI and returns a NOT-YET-SAVED recipe (the caller
// decides whether to persist it, via a separate POST /pantry/recipes — see
// the handler of a later task). Every ingredient's have/have_quantity comes
// back recomputed from the user's real stock at call time, via
// RecomputeHave — never trusting the AI's own suggestion blindly, the same
// rule followed by every other AI-backed flow in this module.
func (s *RecipeService) Generate(ctx context.Context, params GenerateRecipeParams) (*models.PantryRecipe, error) {
	settings, err := s.settings.Get(ctx, params.UserID)
	if err != nil {
		return nil, fmt.Errorf("pantry recipe service: load settings: %w", err)
	}

	language := params.Language
	if language == "" {
		language = settings.Language
	}
	servings := params.Servings
	if servings <= 0 {
		servings = defaultRecipeServings
	}

	pantryContext, err := s.loadPantryContext(ctx, params.UserID, settings.ShoppingIntervalDays)
	if err != nil {
		return nil, fmt.Errorf("pantry recipe service: load pantry context: %w", err)
	}

	req := ai.StructuredRequest{
		Model:        ai.SelectModel(ai.TaskPantryRecipe),
		SystemPrompt: buildRecipeGenerationPrompt(params.Filters, params.Restrictions, servings, params.FreeText, language, pantryContext),
		UserText:     "Gere a receita.",
		Schema:       recipeGenerationSchema(),
		Effort:       "medium",
		MaxTokens:    recipeGenerationMaxTokens,
	}

	generated, resp, err := s.generateWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}

	ingredients := toRecipeIngredients(generated.Ingredients)
	ingredients, err = s.RecomputeHave(ctx, params.UserID, ingredients)
	if err != nil {
		return nil, fmt.Errorf("pantry recipe service: recompute have: %w", err)
	}

	recipe := &models.PantryRecipe{
		UserID:         params.UserID,
		Title:          generated.Title,
		Language:       generated.Language,
		Servings:       generated.Servings,
		PrepMinutes:    generated.PrepMinutes,
		Filters:        params.Filters,
		Ingredients:    ingredients,
		Steps:          generated.Steps,
		Tips:           generated.Tips,
		Source:         models.PantryRecipeSourceAI,
		AIModel:        ai.SelectModel(ai.TaskPantryRecipe),
		AIInputTokens:  resp.InputTokens,
		AIOutputTokens: resp.OutputTokens,
	}
	return recipe, nil
}

// generateWithRetry calls the AI once, and on any failure (transport error,
// non end_turn stop reason, invalid JSON, or a business-rule validation
// failure) tries exactly once more with the same request. If the second
// attempt also fails, it returns ErrRecipeGenerationFailed — same pattern as
// ReceiptService.extractWithRetry.
func (s *RecipeService) generateWithRetry(
	ctx context.Context,
	req ai.StructuredRequest,
) (*generatedRecipe, ai.StructuredResponse, error) {
	generated, resp, err := s.generateOnce(ctx, req)
	if err == nil {
		return generated, resp, nil
	}

	generated, resp, err = s.generateOnce(ctx, req)
	if err != nil {
		return nil, ai.StructuredResponse{}, fmt.Errorf("%w: %w", ErrRecipeGenerationFailed, err)
	}
	return generated, resp, nil
}

// generateOnce performs a single AI call and validates its response, without
// retrying.
func (s *RecipeService) generateOnce(
	ctx context.Context,
	req ai.StructuredRequest,
) (*generatedRecipe, ai.StructuredResponse, error) {
	resp, err := s.ai.ChatStructured(ctx, req)
	if err != nil {
		return nil, ai.StructuredResponse{}, fmt.Errorf("pantry recipe service: chat structured: %w", err)
	}
	if resp.StopReason != "end_turn" {
		return nil, resp, fmt.Errorf("pantry recipe service: stop_reason inesperado: %q", resp.StopReason)
	}

	var generated generatedRecipe
	if err := json.Unmarshal([]byte(resp.JSON), &generated); err != nil {
		return nil, resp, fmt.Errorf("pantry recipe service: unmarshal generation: %w", err)
	}
	if err := validateGeneratedRecipe(&generated); err != nil {
		return nil, resp, fmt.Errorf("pantry recipe service: validate generation: %w", err)
	}
	return &generated, resp, nil
}

// loadPantryContext loads the user's catalog, stock and movement history and
// joins them into the shape buildRecipeGenerationPrompt needs, with the same
// status/forecast computation used by Service.ListWithStock — so the
// use_running_out/use_expiring filters see the same running_out/expiring
// flags the catalog screen shows.
func (s *RecipeService) loadPantryContext(
	ctx context.Context,
	userID uuid.UUID,
	shoppingIntervalDays int,
) ([]pantryIngredientContext, error) {
	products, err := s.products.List(ctx, userID, "", "")
	if err != nil {
		return nil, fmt.Errorf("load user catalog: %w", err)
	}
	stocksByProduct, err := s.stocks.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user stock: %w", err)
	}
	movements, err := s.movements.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load user movements: %w", err)
	}
	now := time.Now().UTC()
	forecasts := buildForecasts(products, stocksByProduct, movements, shoppingIntervalDays, now)

	pantryItems := make([]pantryIngredientContext, 0, len(products))
	for _, product := range products {
		stock, ok := stocksByProduct[product.ID]
		if !ok {
			stock = *emptyStock(product.ID, userID, product.DefaultUnit)
		}
		forecast := forecasts[product.ID]
		pantryItems = append(pantryItems, pantryIngredientContext{
			ProductID: product.ID,
			Name:      product.NamePt,
			Quantity:  stock.Quantity,
			Unit:      stock.Unit,
			Status:    computeStatus(stock.Quantity, forecast.EffectiveMinLevel, forecast.RunOutAt, shoppingIntervalDays, now),
			Expiring:  isExpiring(stock.ExpiresAt, now),
		})
	}
	return pantryItems, nil
}

// toRecipeIngredients converts the AI's generated ingredients into the
// stored shape, parsing product_id when present. An unparseable product_id
// (should not happen, the schema constrains it to a string or null, but the
// AI could still return garbage) is treated the same as a missing one.
func toRecipeIngredients(generated []generatedIngredient) []models.PantryRecipeIngredient {
	ingredients := make([]models.PantryRecipeIngredient, 0, len(generated))
	for _, g := range generated {
		var productID *uuid.UUID
		if g.ProductID != nil {
			if parsed, err := uuid.Parse(*g.ProductID); err == nil {
				productID = &parsed
			}
		}
		ingredients = append(ingredients, models.PantryRecipeIngredient{
			Name:      g.Name,
			ProductID: productID,
			Quantity:  g.Quantity,
			Unit:      g.Unit,
			Have:      g.Have,
			Optional:  g.Optional,
		})
	}
	return ingredients
}

// RecomputeHave refreshes have/have_quantity of every ingredient against the
// user's CURRENT stock — called both right after generation and whenever a
// saved recipe is read back (GET), since stock changes over time and a
// recipe saved days ago may no longer reflect what's actually in the pantry.
//
// For ingredients with a product_id that exists in the user's catalog,
// have_quantity is the current stock converted to the ingredient's unit
// (best-effort via UnitConversionFactor; when the two units are not the same
// physical dimension, the stock quantity is used as-is, in its own unit, as
// an approximate indicator), and have = have_quantity > 0. For ingredients
// without a usable product_id — no match, or a basic seasoning the AI marked
// as always-available — the AI's own have is kept (there is nothing to
// verify it against) and have_quantity is reported as 0.
func (s *RecipeService) RecomputeHave(
	ctx context.Context,
	userID uuid.UUID,
	ingredients []models.PantryRecipeIngredient,
) ([]models.PantryRecipeIngredient, error) {
	recomputed := make([]models.PantryRecipeIngredient, len(ingredients))
	for i, ingredient := range ingredients {
		recomputed[i] = ingredient
		if ingredient.ProductID == nil {
			recomputed[i].HaveQuantity = 0
			continue
		}

		stock, err := s.stocks.Get(ctx, *ingredient.ProductID)
		if err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return nil, fmt.Errorf("get stock for product %s: %w", *ingredient.ProductID, err)
			}
			recomputed[i].Have = false
			recomputed[i].HaveQuantity = 0
			continue
		}

		haveQuantity := convertStockToIngredientUnit(stock.Quantity, stock.Unit, ingredient.Unit)
		recomputed[i].HaveQuantity = haveQuantity
		recomputed[i].Have = haveQuantity > 0
	}
	return recomputed, nil
}

// convertStockToIngredientUnit converts a stock quantity into an ingredient's
// unit via UnitConversionFactor when the two units share a physical
// dimension; otherwise it returns stockQty as-is (an approximate indicator
// only — e.g. a stock kept in "un" compared against a recipe asking for
// "xicara").
func convertStockToIngredientUnit(stockQty float64, stockUnit, ingredientUnit string) float64 {
	factor, ok := UnitConversionFactor(stockUnit, ingredientUnit)
	if !ok {
		return stockQty
	}
	return stockQty * factor
}

// CookDeduction is one ingredient's stock impact if the user confirms
// cooking a recipe now, scaled to the requested servings.
type CookDeduction struct {
	ProductID  uuid.UUID `json:"product_id"`
	Name       string    `json:"name"`
	Quantity   float64   `json:"quantity"` // já escalado pelas porções pedidas
	Unit       string    `json:"unit"`
	Available  float64   `json:"available"`   // estoque atual, na mesma unidade (convertido quando possível)
	WillDeduct float64   `json:"will_deduct"` // min(quantity, available) — nunca deixa o estoque negativo
}

// PreviewCook calculates, for a recipe and a desired number of servings
// (requestedServings — 0 means use recipe.Servings), what would be
// discounted from stock if the user confirmed "vou fazer" now. Ingredients
// without a product_id, or marked optional=true and have=false (using the
// Have field already stored on the recipe — presumed already refreshed by a
// prior RecomputeHave call), go to missing instead of deductions: there is
// nothing to deduct from them.
func (s *RecipeService) PreviewCook(
	ctx context.Context,
	userID uuid.UUID,
	recipe *models.PantryRecipe,
	requestedServings int,
) (deductions []CookDeduction, missing []models.PantryRecipeIngredient, err error) {
	scale := recipeServingScale(recipe.Servings, requestedServings)

	deductions = []CookDeduction{}
	missing = []models.PantryRecipeIngredient{}
	for _, ingredient := range recipe.Ingredients {
		skippable := ingredient.Optional && !ingredient.Have
		if ingredient.ProductID == nil || skippable {
			missing = append(missing, ingredient)
			continue
		}

		stock, err := s.stocks.Get(ctx, *ingredient.ProductID)
		if err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return nil, nil, fmt.Errorf("pantry recipe service: preview cook: get stock: %w", err)
			}
			stock = emptyStock(*ingredient.ProductID, userID, ingredient.Unit)
		}

		quantity := ingredient.Quantity * scale
		available := convertStockToIngredientUnit(stock.Quantity, stock.Unit, ingredient.Unit)
		deductions = append(deductions, CookDeduction{
			ProductID:  *ingredient.ProductID,
			Name:       ingredient.Name,
			Quantity:   quantity,
			Unit:       ingredient.Unit,
			Available:  available,
			WillDeduct: min(quantity, available),
		})
	}
	return deductions, missing, nil
}

// recipeServingScale computes requestedServings/recipeServings, treating a
// non-positive recipeServings (should not happen, but a defensively invalid
// stored recipe must not divide by zero) as 1, and requestedServings <= 0 as
// "use the recipe's own serving count" (scale 1).
func recipeServingScale(recipeServings, requestedServings int) float64 {
	if recipeServings <= 0 {
		recipeServings = 1
	}
	if requestedServings <= 0 {
		return 1
	}
	return float64(requestedServings) / float64(recipeServings)
}
