package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/pantry"
	"github.com/rs/zerolog/log"
)

// pantryShoppingListDefaultStatus is used when the "status" query param is
// absent: the shopping list screen's main view is always "o que falta
// comprar", so no filter means "open" rather than "everything". Pass
// status=all explicitly to see every item regardless of status.
const pantryShoppingListDefaultStatus = "open"

// PantryShoppingListHandler exposes the shopping list endpoints of the
// dispensa module: listing, manual add, status changes, a manual trigger for
// the automatic regeneration, and adding a recipe's missing ingredients.
type PantryShoppingListHandler struct {
	service       *pantry.ShoppingListService
	repo          *repository.PantryShoppingItemRepository
	recipeService *pantry.RecipeService
	recipeRepo    *repository.PantryRecipeRepository
}

func NewPantryShoppingListHandler(
	service *pantry.ShoppingListService,
	repo *repository.PantryShoppingItemRepository,
	recipeService *pantry.RecipeService,
	recipeRepo *repository.PantryRecipeRepository,
) *PantryShoppingListHandler {
	return &PantryShoppingListHandler{
		service:       service,
		repo:          repo,
		recipeService: recipeService,
		recipeRepo:    recipeRepo,
	}
}

// pantryShoppingListStatusFilter resolves the "status" query param into what
// the repository's List expects ("" = every status): an absent param
// defaults to "open" (see pantryShoppingListDefaultStatus), and "all" is the
// explicit way to ask for every status.
func pantryShoppingListStatusFilter(raw string) string {
	switch raw {
	case "":
		return pantryShoppingListDefaultStatus
	case "all":
		return ""
	default:
		return raw
	}
}

// pantryShoppingItemRequest is the body accepted by Create.
type pantryShoppingItemRequest struct {
	Name      string   `json:"name"`
	ProductID string   `json:"product_id"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
}

// pantryShoppingStatusRequest is the body accepted by UpdateStatus.
type pantryShoppingStatusRequest struct {
	Status   string   `json:"status"`
	Quantity *float64 `json:"quantity"`
}

// List godoc
// GET /pantry/shopping-list?status=open
func (h *PantryShoppingListHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	status := pantryShoppingListStatusFilter(r.URL.Query().Get("status"))

	items, err := h.repo.List(r.Context(), userID, status)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry shopping list: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}

// Create godoc
// POST /pantry/shopping-list
func (h *PantryShoppingListHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req pantryShoppingItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	var productID *uuid.UUID
	if req.ProductID != "" {
		id, err := uuid.Parse(req.ProductID)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid product_id")
			return
		}
		productID = &id
	}

	item, err := h.service.AddManual(r.Context(), userID, req.Name, productID, req.Quantity, req.Unit)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

// UpdateStatus godoc
// PATCH /pantry/shopping-list/{id}
func (h *PantryShoppingListHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid item id")
		return
	}

	var req pantryShoppingStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if err := h.service.SetStatus(r.Context(), id, userID, req.Status, req.Quantity); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "item não encontrado")
			return
		}
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"id": id.String(), "status": req.Status})
}

// pantryShoppingFromRecipeIngredient is one entry of the "ingredients" list
// accepted by FromRecipe when the caller passes a loose list instead of
// recipe_id.
type pantryShoppingFromRecipeIngredient struct {
	Name      string   `json:"name"`
	ProductID string   `json:"product_id"`
	Quantity  *float64 `json:"quantity"`
	Unit      string   `json:"unit"`
}

// pantryShoppingFromRecipeRequest is the body accepted by FromRecipe: either
// recipe_id (add a saved recipe's missing ingredients) or ingredients (add a
// loose list) — never both, never neither.
type pantryShoppingFromRecipeRequest struct {
	RecipeID    string                               `json:"recipe_id"`
	Ingredients []pantryShoppingFromRecipeIngredient `json:"ingredients"`
}

// FromRecipe godoc
// POST /pantry/shopping-list/from-recipe
//
// With recipe_id: loads the saved recipe, recomputes Have against live
// stock (a recipe saved days ago may no longer reflect what's in the
// pantry), and adds every ingredient with Have=false — with or without a
// linked product, since a missing ingredient still makes sense as a plain
// named item on the list. With ingredients: adds every item in the list as
// given, no have-filtering — the caller already decided what's missing.
func (h *PantryShoppingListHandler) FromRecipe(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	var req pantryShoppingFromRecipeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	hasRecipeID := strings.TrimSpace(req.RecipeID) != ""
	hasIngredients := len(req.Ingredients) > 0
	if hasRecipeID == hasIngredients {
		respondError(w, http.StatusUnprocessableEntity, "informe recipe_id ou ingredients, não os dois nem nenhum")
		return
	}

	if hasRecipeID {
		h.addMissingFromRecipe(w, r, userID, req.RecipeID)
		return
	}
	h.addLooseIngredients(w, r, userID, req.Ingredients)
}

// addMissingFromRecipe implements the recipe_id branch of FromRecipe.
func (h *PantryShoppingListHandler) addMissingFromRecipe(
	w http.ResponseWriter,
	r *http.Request,
	userID uuid.UUID,
	rawRecipeID string,
) {
	ctx := r.Context()

	recipeID, err := uuid.Parse(rawRecipeID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid recipe_id")
		return
	}

	recipe, err := h.recipeRepo.FindByID(ctx, recipeID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "receita não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry shopping list: from recipe: find recipe")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	ingredients, err := h.recipeService.RecomputeHave(ctx, userID, recipe.Ingredients)
	if err != nil {
		log.Error().Err(err).Str("recipe_id", recipeID.String()).
			Msg("pantry shopping list: from recipe: recompute have")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	added := 0
	for _, ingredient := range ingredients {
		if ingredient.Have {
			continue
		}
		var quantity *float64
		if ingredient.Quantity > 0 {
			q := ingredient.Quantity
			quantity = &q
		}
		if _, err := h.service.AddFromRecipe(
			ctx, userID, ingredient.Name, ingredient.ProductID, quantity, ingredient.Unit,
		); err != nil {
			log.Error().Err(err).Str("ingredient", ingredient.Name).Msg("pantry shopping list: from recipe: add")
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		added++
	}
	respondJSON(w, http.StatusCreated, map[string]any{"added": added})
}

// addLooseIngredients implements the ingredients branch of FromRecipe.
func (h *PantryShoppingListHandler) addLooseIngredients(
	w http.ResponseWriter,
	r *http.Request,
	userID uuid.UUID,
	items []pantryShoppingFromRecipeIngredient,
) {
	ctx := r.Context()

	added := 0
	for _, item := range items {
		var productID *uuid.UUID
		if item.ProductID != "" {
			id, err := uuid.Parse(item.ProductID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid product_id: "+item.ProductID)
				return
			}
			productID = &id
		}
		if _, err := h.service.AddFromRecipe(ctx, userID, item.Name, productID, item.Quantity, item.Unit); err != nil {
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		added++
	}
	respondJSON(w, http.StatusCreated, map[string]any{"added": added})
}

// Regenerate godoc
// POST /pantry/shopping-list/regenerate
func (h *PantryShoppingListHandler) Regenerate(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	if err := h.service.Regenerate(r.Context(), userID); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry shopping list: regenerate")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items, err := h.repo.List(r.Context(), userID, pantryShoppingListStatusFilter(""))
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry shopping list: list after regenerate")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"items": items})
}
