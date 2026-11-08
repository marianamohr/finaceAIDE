package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/pantry"
	"github.com/rs/zerolog/log"
)

// Movements query defaults: a page small enough for a product's history to
// render instantly, capped so a caller can't force an unbounded scan.
const (
	defaultPantryMovementsLimit = 50
	maxPantryMovementsLimit     = 200
)

// PantryProductHandler exposes the catalog and stock endpoints of the
// dispensa module. The pantry is scoped per user, not per account.
type PantryProductHandler struct {
	service         *pantry.Service
	movements       *repository.PantryMovementRepository
	shoppingListSvc *pantry.ShoppingListService
	settings        *repository.PantrySettingsRepository
	consumeText     *pantry.ConsumeTextService
}

func NewPantryProductHandler(
	service *pantry.Service,
	movements *repository.PantryMovementRepository,
	shoppingListSvc *pantry.ShoppingListService,
	settings *repository.PantrySettingsRepository,
	consumeText *pantry.ConsumeTextService,
) *PantryProductHandler {
	return &PantryProductHandler{
		service:         service,
		movements:       movements,
		shoppingListSvc: shoppingListSvc,
		settings:        settings,
		consumeText:     consumeText,
	}
}

// regenerateShoppingList re-runs the automatic shopping list regeneration
// after a stock-affecting operation succeeded. Failure here is never fatal to
// the caller: the stock operation already succeeded, and a shopping list
// hiccup shouldn't turn that into an error response.
func (h *PantryProductHandler) regenerateShoppingList(r *http.Request, userID uuid.UUID) {
	if err := h.shoppingListSvc.Regenerate(r.Context(), userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Msg("pantry product: regenerate shopping list")
	}
}

// pantryProductRequest is the shape accepted by Create and Update.
// MinLevel of 0 is treated the same as an absent field: a minimum of zero
// makes no sense, so a real minimum is always positive.
type pantryProductRequest struct {
	NamePt          string  `json:"name_pt"`
	NameDe          string  `json:"name_de"`
	Category        string  `json:"category"`
	Subcategory     string  `json:"subcategory"`
	Unit            string  `json:"unit"`
	IsProduce       bool    `json:"is_produce"`
	TrackByUnit     bool    `json:"track_by_unit"`
	MinLevel        float64 `json:"min_level"`
	InitialQuantity float64 `json:"initial_quantity"`
	ShelfLifeDays   *int    `json:"shelf_life_days"`
}

// pantryQuantityRequest is the shape accepted by Consume and Adjust.
type pantryQuantityRequest struct {
	Quantity float64 `json:"quantity"`
	Note     string  `json:"note"`
}

// pantryProductFromRequest builds the catalog product to persist. ID and
// UserID are filled in by the caller once known.
func pantryProductFromRequest(req pantryProductRequest) *models.PantryProduct {
	p := &models.PantryProduct{
		NamePt:      req.NamePt,
		NameDe:      req.NameDe,
		Category:    req.Category,
		Subcategory: req.Subcategory,
		DefaultUnit: req.Unit,
		IsProduce:   req.IsProduce,
		TrackByUnit: req.TrackByUnit,
	}
	if req.MinLevel > 0 {
		minLevel := req.MinLevel
		p.MinLevel = &minLevel
	}
	if req.ShelfLifeDays != nil && *req.ShelfLifeDays > 0 {
		p.ShelfLifeDays = req.ShelfLifeDays
	}
	return p
}

// resolvePantryMovementsLimit parses the "limit" query param: empty or
// invalid values fall back to the default, and anything above the cap is
// truncated to it.
func resolvePantryMovementsLimit(raw string) int {
	if raw == "" {
		return defaultPantryMovementsLimit
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultPantryMovementsLimit
	}
	if n > maxPantryMovementsLimit {
		return maxPantryMovementsLimit
	}
	return n
}

// List godoc
// GET /pantry/products
func (h *PantryProductHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	query := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	status := r.URL.Query().Get("status")

	settings, err := h.settings.Get(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: list: load settings")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	products, err := h.service.ListWithStock(r.Context(), userID, query, category, status, settings.ShoppingIntervalDays)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"products": products})
}

// Create godoc
// POST /pantry/products
func (h *PantryProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req pantryProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	product := pantryProductFromRequest(req)
	product.UserID = userID

	result, err := h.service.CreateProduct(r.Context(), product, req.InitialQuantity)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: create")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusCreated, result)
}

// Update godoc
// PUT /pantry/products/{id}
func (h *PantryProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req pantryProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	product := pantryProductFromRequest(req)
	product.ID = id
	product.UserID = userID

	// Changing the unit here would update pantry_products.default_unit
	// without touching pantry_stock.unit/quantity — exactly the
	// inconsistency ResetStockForUnitTracking exists to prevent for the
	// hortifruti flow (see memory.md). Unit changes go through ChangeUnit
	// instead, which converts (or safely zeroes) the stock alongside it.
	existing, err := h.service.GetProduct(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: update: load existing")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if existing.DefaultUnit != product.DefaultUnit {
		respondError(w, http.StatusUnprocessableEntity,
			"para trocar a unidade do produto, use a opção de trocar unidade (ela converte o estoque também)")
		return
	}

	if err := h.service.UpdateProduct(r.Context(), product); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: update")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, map[string]any{"id": id.String(), "updated": true})
}

// pantryChangeUnitRequest is the shape accepted by ChangeUnit.
type pantryChangeUnitRequest struct {
	Unit string `json:"unit"`
}

// ChangeUnit godoc
// POST /pantry/products/{id}/change-unit
//
// Troca a unidade rastreada do produto (ex: "Arroz" de kg para g),
// convertendo a quantidade em estoque quando as unidades são da mesma
// dimensão física, ou zerando o estoque (com movimento de ajuste
// registrado) quando não há conversão possível — ver pantry.Service.ChangeUnit.
func (h *PantryProductHandler) ChangeUnit(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req pantryChangeUnitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := h.service.ChangeUnit(r.Context(), id, userID, req.Unit)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: change unit")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, result)
}

// Delete godoc
// DELETE /pantry/products/{id}
func (h *PantryProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	if err := h.service.DeleteProduct(r.Context(), id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.regenerateShoppingList(r, userID)
	w.WriteHeader(http.StatusNoContent)
}

// Movements godoc
// GET /pantry/products/{id}/movements?limit=50
func (h *PantryProductHandler) Movements(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	limit := resolvePantryMovementsLimit(r.URL.Query().Get("limit"))
	movements, err := h.movements.ListByProduct(r.Context(), id, userID, limit)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: movements")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"movements": movements})
}

// Consume godoc
// POST /pantry/products/{id}/consume
func (h *PantryProductHandler) Consume(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req pantryQuantityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := h.service.Consume(r.Context(), id, userID, req.Quantity, req.Note)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: consume")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, result)
}

// pantryConsumeTextRequest is the shape accepted by ConsumeText.
type pantryConsumeTextRequest struct {
	Text string `json:"text"`
}

// ConsumeText godoc
// POST /pantry/products/consume-text
//
// Consumo por texto livre (Etapa 5): a usuária escreve algo como "usei 2
// ovos" e a IA extrai os itens; cada um é casado contra o catálogo (só por
// alias/nome exato — não cria produto novo) e baixado do estoque. A resposta
// sempre 200 (mesmo com itens não reconhecidos): "unmatched" avisa quais
// nomes a usuária precisa ajustar manualmente, sem falhar a requisição
// inteira por causa de um item.
func (h *PantryProductHandler) ConsumeText(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req pantryConsumeTextRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := h.consumeText.ConsumeFromText(r.Context(), userID, req.Text)
	if err != nil {
		if errors.Is(err, pantry.ErrConsumeTextFailed) {
			respondError(w, http.StatusUnprocessableEntity, "Não consegui entender o texto. Tente descrever de outro jeito, ex: \"usei 2 ovos\".")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: consume text")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, result)
}

// Out godoc
// POST /pantry/products/{id}/out
func (h *PantryProductHandler) Out(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	result, err := h.service.MarkOut(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: mark out")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, result)
}

// Adjust godoc
// POST /pantry/products/{id}/adjust
func (h *PantryProductHandler) Adjust(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid product id")
		return
	}

	var req pantryQuantityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	result, err := h.service.Adjust(r.Context(), id, userID, req.Quantity, req.Note)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "product not found")
			return
		}
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry product: adjust")
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	h.regenerateShoppingList(r, userID)
	respondJSON(w, http.StatusOK, result)
}
