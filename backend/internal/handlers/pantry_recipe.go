package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/pantry"
	"github.com/rs/zerolog/log"
)

// PantryRecipeHandler exposes the recipe endpoints of the dispensa module:
// AI generation, save/favorite, listing, and the "vou fazer" cook flow that
// discounts the used ingredients from stock.
type PantryRecipeHandler struct {
	recipeService   *pantry.RecipeService
	recipeRepo      *repository.PantryRecipeRepository
	recipeLogRepo   *repository.PantryRecipeLogRepository
	productRepo     *repository.PantryProductRepository
	pantryService   *pantry.Service
	shoppingListSvc *pantry.ShoppingListService
}

func NewPantryRecipeHandler(
	recipeService *pantry.RecipeService,
	recipeRepo *repository.PantryRecipeRepository,
	recipeLogRepo *repository.PantryRecipeLogRepository,
	productRepo *repository.PantryProductRepository,
	pantryService *pantry.Service,
	shoppingListSvc *pantry.ShoppingListService,
) *PantryRecipeHandler {
	return &PantryRecipeHandler{
		recipeService:   recipeService,
		recipeRepo:      recipeRepo,
		recipeLogRepo:   recipeLogRepo,
		productRepo:     productRepo,
		pantryService:   pantryService,
		shoppingListSvc: shoppingListSvc,
	}
}

// decodeOptionalJSON decodes r.Body into dst, treating a completely empty
// body (io.EOF right away) the same as "nothing was sent" instead of an
// error — used by endpoints whose body is entirely optional (Generate,
// CookPreview), where the zero value of every field is a valid default.
func decodeOptionalJSON(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// countRecipeIngredients tallies how many ingredients of a just-generated
// recipe the user already has vs. is missing, for the Generate response —
// a simple count over the Have field RecomputeHave already filled in.
func countRecipeIngredients(ingredients []models.PantryRecipeIngredient) (have, missing int) {
	for _, ingredient := range ingredients {
		if ingredient.Have {
			have++
			continue
		}
		missing++
	}
	return have, missing
}

// validateRecipeForSave checks the minimum a recipe needs before it can be
// persisted: a title and at least one ingredient. Everything else (steps,
// tips, servings) is free-form and the AI/user can leave it empty.
func validateRecipeForSave(recipe *models.PantryRecipe) error {
	if strings.TrimSpace(recipe.Title) == "" {
		return errors.New("título da receita é obrigatório")
	}
	if len(recipe.Ingredients) == 0 {
		return errors.New("receita precisa de ao menos um ingrediente")
	}
	return nil
}

// reconcileRecipeDeductionUnit adapts a confirmed cook deduction
// (quantity/unit, as the user confirmed on the "vou fazer" screen —
// pre-filled from CookPreview, possibly edited) to product's already
// established stock unit before it reaches pantry.Service.RecipeOut, which
// trusts its unit argument completely and never converts itself. Mirrors
// reconcileReceiptEntryUnit in pantry_receipt.go — same contract, same
// pattern, adapted to a deduction (no unit price to convert). When the two
// units differ and are not the same physical dimension
// (pantry.UnitConversionFactor), the deduction is rejected with a
// Portuguese message naming the offending product, meant to reach the user.
func reconcileRecipeDeductionUnit(
	product *models.PantryProduct,
	quantity float64,
	unit string,
) (float64, string, error) {
	if unit == product.DefaultUnit {
		return quantity, unit, nil
	}

	factor, ok := pantry.UnitConversionFactor(unit, product.DefaultUnit)
	if !ok {
		return 0, "", fmt.Errorf(
			"%q está cadastrado em %q, mas a dedução veio em %q — ajuste a unidade antes de confirmar",
			product.NamePt, product.DefaultUnit, unit,
		)
	}
	return quantity * factor, product.DefaultUnit, nil
}

// pantryRecipeGenerateRequest is the body accepted by Generate. Every field
// is optional — even an entirely empty body is accepted, the AI is expected
// to work with whatever criteria it is given (see the prompt builder).
type pantryRecipeGenerateRequest struct {
	Filters      []string `json:"filters"`
	Restrictions []string `json:"restrictions"`
	Servings     int      `json:"servings"`
	FreeText     string   `json:"free_text"`
	Language     string   `json:"language"`
}

// Generate godoc
// POST /pantry/recipes/generate
func (h *PantryRecipeHandler) Generate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	var req pantryRecipeGenerateRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	recipe, err := h.recipeService.Generate(ctx, pantry.GenerateRecipeParams{
		UserID:       userID,
		Filters:      req.Filters,
		Restrictions: req.Restrictions,
		Servings:     req.Servings,
		FreeText:     req.FreeText,
		Language:     req.Language,
	})
	if err != nil {
		if errors.Is(err, pantry.ErrRecipeGenerationFailed) {
			respondError(w, http.StatusBadGateway,
				"Não consegui gerar uma receita agora. Tente de novo com outros filtros ou daqui a pouco.")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry recipe: generate")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	haveCount, missingCount := countRecipeIngredients(recipe.Ingredients)
	respondJSON(w, http.StatusOK, map[string]any{
		"recipe":        recipe,
		"have_count":    haveCount,
		"missing_count": missingCount,
	})
}

// Create godoc
// POST /pantry/recipes
//
// The body is a full recipe (the same shape Generate returns, possibly
// edited by the user before saving). Saving is the only way to keep a
// recipe in this module, so it is always favorited — is_favorite in the
// body, if any, is ignored.
func (h *PantryRecipeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	var recipe models.PantryRecipe
	if err := json.NewDecoder(r.Body).Decode(&recipe); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := validateRecipeForSave(&recipe); err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// ID/CreatedAt are left at their zero value so the database defaults
	// (gen_random_uuid(), now()) apply — any id the caller sent in the body
	// belongs to whatever draft Generate returned, not to a row that exists
	// yet, and is discarded here.
	recipe.ID = uuid.Nil
	recipe.UserID = userID
	recipe.IsFavorite = true
	recipe.CreatedAt = time.Time{}

	if err := h.recipeRepo.Create(ctx, &recipe); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry recipe: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, recipe)
}

// List godoc
// GET /pantry/recipes?favorite=true
func (h *PantryRecipeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)
	favoriteOnly := r.URL.Query().Get("favorite") == "true"

	recipes, err := h.recipeRepo.List(ctx, userID, favoriteOnly)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry recipe: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	for i := range recipes {
		ingredients, err := h.recipeService.RecomputeHave(ctx, userID, recipes[i].Ingredients)
		if err != nil {
			log.Error().Err(err).Str("recipe_id", recipes[i].ID.String()).
				Msg("pantry recipe: list: recompute have")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		recipes[i].Ingredients = ingredients
	}

	respondJSON(w, http.StatusOK, map[string]any{"recipes": recipes})
}

// Get godoc
// GET /pantry/recipes/{id}
func (h *PantryRecipeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe id")
		return
	}

	recipe, err := h.recipeRepo.FindByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receita não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry recipe: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	ingredients, err := h.recipeService.RecomputeHave(ctx, userID, recipe.Ingredients)
	if err != nil {
		log.Error().Err(err).Str("recipe_id", id.String()).Msg("pantry recipe: get: recompute have")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	recipe.Ingredients = ingredients

	respondJSON(w, http.StatusOK, recipe)
}

// Delete godoc
// DELETE /pantry/recipes/{id}
func (h *PantryRecipeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe id")
		return
	}

	if err := h.recipeRepo.Delete(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receita não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry recipe: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// pantryRecipeCookPreviewRequest is the body accepted by CookPreview.
// Servings=0 (or an absent body) means "use the recipe's own serving count".
type pantryRecipeCookPreviewRequest struct {
	Servings int `json:"servings"`
}

// CookPreview godoc
// POST /pantry/recipes/{id}/cook/preview
func (h *PantryRecipeHandler) CookPreview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe id")
		return
	}

	var req pantryRecipeCookPreviewRequest
	if err := decodeOptionalJSON(r, &req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	recipe, err := h.recipeRepo.FindByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receita não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry recipe: cook preview: find recipe")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Refreshed against live stock first, so "missing" reflects what is
	// actually in the pantry right now, not whatever was true when the
	// recipe was saved.
	ingredients, err := h.recipeService.RecomputeHave(ctx, userID, recipe.Ingredients)
	if err != nil {
		log.Error().Err(err).Str("recipe_id", id.String()).Msg("pantry recipe: cook preview: recompute have")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	recipe.Ingredients = ingredients

	deductions, missing, err := h.recipeService.PreviewCook(ctx, userID, recipe, req.Servings)
	if err != nil {
		log.Error().Err(err).Str("recipe_id", id.String()).Msg("pantry recipe: cook preview")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"deductions": deductions, "missing": missing})
}

// pantryRecipeCookDeductionRequest is one confirmed deduction in the body of
// Cook — the user's final decision, possibly adjusted from what
// CookPreview suggested.
type pantryRecipeCookDeductionRequest struct {
	ProductID string  `json:"product_id"`
	Quantity  float64 `json:"quantity"`
	Unit      string  `json:"unit"`
}

// pantryRecipeCookRequest is the body accepted by Cook.
type pantryRecipeCookRequest struct {
	Servings   int                                `json:"servings"`
	Deductions []pantryRecipeCookDeductionRequest `json:"deductions"`
}

// Cook godoc
// POST /pantry/recipes/{id}/cook
//
// Applies exactly the deductions the caller confirmed — this handler does
// NOT recompute them from the recipe, that already happened in
// CookPreview. If a deduction fails partway through (product not found,
// incompatible unit, or a RecipeOut business error), processing stops
// immediately: deductions already applied to stock before the failing one
// remain in place (the same tolerance already accepted by the receipt
// confirmation's Confirm handler) and no PantryRecipeLog is created for a
// cook that didn't fully go through.
func (h *PantryRecipeHandler) Cook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe id")
		return
	}

	recipe, err := h.recipeRepo.FindByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receita não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry recipe: cook: find recipe")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req pantryRecipeCookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	servings := req.Servings
	if servings <= 0 {
		servings = recipe.Servings
	}

	// The log id is generated up front so it can be used as the
	// reference_id of every recipe_out movement created below; the log row
	// itself is only inserted once every deduction below has succeeded (see
	// the doc comment above).
	logID := uuid.New()
	deductions := make([]models.PantryRecipeDeduction, 0, len(req.Deductions))
	for _, d := range req.Deductions {
		productID, err := uuid.Parse(d.ProductID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid product_id: "+d.ProductID)
			return
		}

		product, err := h.productRepo.FindByID(ctx, productID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "produto não encontrado: "+d.ProductID)
				return
			}
			log.Error().Err(err).Str("product_id", d.ProductID).Msg("pantry recipe: cook: find product")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}

		quantity, unit, err := reconcileRecipeDeductionUnit(product, d.Quantity, d.Unit)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		if _, err := h.pantryService.RecipeOut(ctx, productID, userID, quantity, unit, logID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "produto não encontrado: "+d.ProductID)
				return
			}
			log.Error().Err(err).Str("product_id", d.ProductID).Msg("pantry recipe: cook: recipe out")
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		deductions = append(deductions, models.PantryRecipeDeduction{
			ProductID: productID,
			Quantity:  quantity,
			Unit:      unit,
		})
	}

	cookLog := &models.PantryRecipeLog{
		ID:         logID,
		UserID:     userID,
		RecipeID:   recipe.ID,
		CookedAt:   time.Now().UTC(),
		Servings:   servings,
		Deductions: deductions,
	}
	if err := h.recipeLogRepo.Create(ctx, cookLog); err != nil {
		log.Error().Err(err).Str("recipe_id", id.String()).Msg("pantry recipe: cook: create log")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Non-fatal, same pattern as every other stock-affecting pantry
	// endpoint: the deductions already succeeded above, a regeneration
	// hiccup shouldn't turn that into an error response.
	if err := h.shoppingListSvc.Regenerate(ctx, userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Msg("pantry recipe: cook: regenerate shopping list")
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"log":               cookLog,
		"movements_created": len(deductions),
	})
}
