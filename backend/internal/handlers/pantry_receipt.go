package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/pantry"
	"github.com/marianamohr/financas/internal/services/storage"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/rs/zerolog/log"
)

// PantryReceiptHandler exposes the receipt reading flow of the dispensa
// module: upload+extraction, review (list/get/edit) and confirmation into
// stock and, optionally, a financial transaction.
type PantryReceiptHandler struct {
	receiptService  *pantry.ReceiptService
	receiptRepo     *repository.PantryReceiptRepository
	itemRepo        *repository.PantryReceiptItemRepository
	aliasRepo       *repository.PantryProductAliasRepository
	productRepo     *repository.PantryProductRepository
	pantryService   *pantry.Service
	shoppingListSvc *pantry.ShoppingListService
	storage         storage.Storage
	userRepo        *repository.UserRepository
	catRepo         *repository.CategoryRepository
	pmRepo          *repository.PaymentMethodRepository
	txRepo          *repository.TransactionRepository
	currencySvc     *currency.Service
	validator       *transactions.Validator
}

func NewPantryReceiptHandler(
	receiptService *pantry.ReceiptService,
	receiptRepo *repository.PantryReceiptRepository,
	itemRepo *repository.PantryReceiptItemRepository,
	aliasRepo *repository.PantryProductAliasRepository,
	productRepo *repository.PantryProductRepository,
	pantryService *pantry.Service,
	shoppingListSvc *pantry.ShoppingListService,
	storageClient storage.Storage,
	userRepo *repository.UserRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
	txRepo *repository.TransactionRepository,
	currencySvc *currency.Service,
	validator *transactions.Validator,
) *PantryReceiptHandler {
	return &PantryReceiptHandler{
		receiptService:  receiptService,
		receiptRepo:     receiptRepo,
		itemRepo:        itemRepo,
		aliasRepo:       aliasRepo,
		productRepo:     productRepo,
		pantryService:   pantryService,
		shoppingListSvc: shoppingListSvc,
		storage:         storageClient,
		userRepo:        userRepo,
		catRepo:         catRepo,
		pmRepo:          pmRepo,
		txRepo:          txRepo,
		currencySvc:     currencySvc,
		validator:       validator,
	}
}

// pantryReceiptItemUpdate is the shape of one item in the body of UpdateItems
// and (optionally) Confirm. The frontend review screen always sends the full
// object for every item it wants to change.
type pantryReceiptItemUpdate struct {
	ID          string   `json:"id"`
	NamePt      string   `json:"name_pt"`
	NameDe      string   `json:"name_de"`
	Category    string   `json:"category"`
	Subcategory string   `json:"subcategory"`
	Quantity    float64  `json:"quantity"`
	Unit        string   `json:"unit"`
	UnitCount   *float64 `json:"unit_count"`
	KeepWeight  bool     `json:"keep_weight"`
	Ignored     bool     `json:"ignored"`
}

// pantryReceiptConfirmRequest is the body of Confirm.
type pantryReceiptConfirmRequest struct {
	CreateTransaction bool                      `json:"create_transaction"`
	PaymentMethodID   *string                   `json:"payment_method_id"`
	Items             []pantryReceiptItemUpdate `json:"items,omitempty"`
}

// resolveCountryHint accepts only the two country codes the extraction prompt
// understands; anything else (including empty) is treated as "no hint".
func resolveCountryHint(raw string) string {
	switch raw {
	case models.PantryCountryBR, models.PantryCountryDE:
		return raw
	default:
		return ""
	}
}

// receiptImageURL builds the public image proxy URL for a stored receipt
// photo. Mirrors the same construction in upload.go (scheme detection via
// X-Forwarded-Proto, host via X-Forwarded-Host) — duplicated rather than
// shared because it is a handful of lines and pulling it into a shared helper
// would couple this file to upload.go for no real benefit.
func receiptImageURL(r *http.Request, key string) string {
	scheme := "https"
	if r.Header.Get("X-Forwarded-Proto") != "https" && r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	return fmt.Sprintf("%s://%s/image/%s", scheme, host, key)
}

// Create godoc
// POST /pantry/receipts
func (h *PantryReceiptHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)
	accountID := auth.AccountIDFromContext(ctx)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		log.Warn().Err(err).Msg("pantry receipt: parse multipart failed (file too large?)")
		respondError(w, http.StatusRequestEntityTooLarge, "Arquivo muito grande (máximo 10 MB).")
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		respondError(w, http.StatusBadRequest, "Campo 'image' é obrigatório.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		log.Error().Err(err).Msg("pantry receipt: read file body failed")
		respondError(w, http.StatusInternalServerError, "Erro ao ler o arquivo enviado.")
		return
	}

	mimeType := http.DetectContentType(data)
	if !allowedMIMETypes[mimeType] {
		respondError(w, http.StatusUnsupportedMediaType,
			fmt.Sprintf("Tipo de arquivo não suportado (%s). Envie JPEG, PNG ou WebP.", mimeType))
		return
	}

	key, err := h.storage.Upload(ctx, data, mimeType)
	if err != nil {
		log.Error().Err(err).Msg("pantry receipt: storage upload failed")
		respondError(w, http.StatusInternalServerError, "Falha ao salvar a imagem. Tente novamente.")
		return
	}

	instruction := strings.TrimSpace(r.FormValue("instruction"))
	country := resolveCountryHint(strings.TrimSpace(r.FormValue("country")))

	receipt, items, err := h.receiptService.ExtractAndSave(ctx, pantry.ExtractReceiptParams{
		UserID:      userID,
		AccountID:   accountID,
		ImageData:   data,
		MIMEType:    mimeType,
		ImageKey:    key,
		Instruction: instruction,
		CountryHint: country,
	})
	if err != nil {
		if errors.Is(err, pantry.ErrExtractionFailed) {
			respondError(w, http.StatusBadGateway,
				"Não consegui ler a nota. Tente uma foto mais nítida, com a nota inteira e sem dobras.")
			return
		}
		log.Error().Err(err).Msg("pantry receipt: extract and save failed")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"receipt": receipt, "items": items})
}

// List godoc
// GET /pantry/receipts?status=pending_review
func (h *PantryReceiptHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	status := r.URL.Query().Get("status")

	receipts, err := h.receiptRepo.List(r.Context(), userID, status)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry receipt: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"receipts": receipts})
}

// Get godoc
// GET /pantry/receipts/{id}
func (h *PantryReceiptHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id")
		return
	}

	receipt, err := h.receiptRepo.FindByID(r.Context(), id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "nota não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry receipt: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	items, err := h.itemRepo.ListByReceipt(r.Context(), id)
	if err != nil {
		log.Error().Err(err).Msg("pantry receipt: get items")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"receipt": receipt, "items": items})
}

// applyReceiptItemUpdates validates that every update refers to an item that
// actually belongs to the receipt (present in existing) and applies the
// editable fields on top of a copy of that item. Returns an error naming the
// offending id on the first mismatch.
func applyReceiptItemUpdates(
	existing []models.PantryReceiptItem,
	updates []pantryReceiptItemUpdate,
) ([]models.PantryReceiptItem, error) {
	byID := make(map[uuid.UUID]models.PantryReceiptItem, len(existing))
	for _, item := range existing {
		byID[item.ID] = item
	}

	result := make([]models.PantryReceiptItem, 0, len(updates))
	for _, upd := range updates {
		itemID, err := uuid.Parse(upd.ID)
		if err != nil {
			return nil, fmt.Errorf("id de item inválido: %q", upd.ID)
		}
		item, ok := byID[itemID]
		if !ok {
			return nil, fmt.Errorf("item %s não pertence a esta nota", upd.ID)
		}
		applyReceiptItemUpdate(&item, upd)
		result = append(result, item)
	}
	return result, nil
}

// applyReceiptItemUpdate overwrites the editable fields of item from upd.
// Ignored/IgnoreReason get a small extra rule: turning an item into "ignored"
// through this endpoint (rather than the AI extraction) stamps
// IgnoreReason=user when it did not already carry one, and un-ignoring an
// item always clears its reason, since a non-ignored item has none.
func applyReceiptItemUpdate(item *models.PantryReceiptItem, upd pantryReceiptItemUpdate) {
	item.NamePt = upd.NamePt
	item.NameDe = upd.NameDe
	item.Category = upd.Category
	item.Subcategory = upd.Subcategory
	item.Quantity = upd.Quantity
	item.Unit = upd.Unit
	item.UnitCount = upd.UnitCount
	item.KeepWeight = upd.KeepWeight

	switch {
	case upd.Ignored && !item.Ignored && item.IgnoreReason == "":
		item.IgnoreReason = models.PantryIgnoreUser
	case !upd.Ignored:
		item.IgnoreReason = ""
	}
	item.Ignored = upd.Ignored
}

// UpdateItems godoc
// PUT /pantry/receipts/{id}/items
func (h *PantryReceiptHandler) UpdateItems(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id")
		return
	}

	if _, err := h.receiptRepo.FindByID(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "nota não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry receipt: update items: find receipt")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req struct {
		Items []pantryReceiptItemUpdate `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	existing, err := h.itemRepo.ListByReceipt(ctx, id)
	if err != nil {
		log.Error().Err(err).Msg("pantry receipt: update items: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	updated, err := applyReceiptItemUpdates(existing, req.Items)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.itemRepo.UpdateBatch(ctx, updated); err != nil {
		log.Error().Err(err).Msg("pantry receipt: update items: save")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{"items": updated})
}

// pendingUnitCountItemIDs lists the ids of non-ignored produce items sold by
// weight that still need the user to say how many individual units they
// correspond to, before the receipt can be confirmed.
func pendingUnitCountItemIDs(items []models.PantryReceiptItem) []string {
	ids := []string{}
	for _, item := range items {
		if item.Ignored {
			continue
		}
		if item.IsProduce && item.NeedsUnitCount && item.UnitCount == nil && !item.KeepWeight {
			ids = append(ids, item.ID.String())
		}
	}
	return ids
}

// receiptItemProductName is the name_pt a new catalog product would be
// created with for item, falling back to the raw receipt text when the
// extraction/matching step left name_pt empty.
func receiptItemProductName(item models.PantryReceiptItem) string {
	if item.NamePt != "" {
		return item.NamePt
	}
	return item.RawName
}

// newProductFromReceiptItem builds the catalog product to create for a
// receipt item that did not match any existing product. When the item was
// resolved into a per-unit count on the review screen, the new product
// starts tracked by unit ("un") rather than by the receipt's original weight
// unit.
func newProductFromReceiptItem(item models.PantryReceiptItem, userID uuid.UUID) *models.PantryProduct {
	p := &models.PantryProduct{
		UserID:      userID,
		NamePt:      receiptItemProductName(item),
		NameDe:      item.NameDe,
		Category:    item.Category,
		Subcategory: item.Subcategory,
		DefaultUnit: item.Unit,
		IsProduce:   item.IsProduce,
	}
	if item.UnitCount != nil {
		p.DefaultUnit = "un"
		p.TrackByUnit = true
	}
	return p
}

// receiptEntryQuantity determines the quantity/unit/unit price to record in
// the stock entry for item. When the user resolved a weight-sold item into a
// unit count, the entry switches to that count in "un" — and the per-unit
// price is recomputed from the line's total (not the original per-kg price).
func receiptEntryQuantity(item models.PantryReceiptItem) (quantity float64, unit string, unitPrice *float64) {
	if item.UnitCount != nil {
		quantity = *item.UnitCount
		unit = "un"
		if item.TotalPrice != nil && quantity > 0 {
			price := *item.TotalPrice / quantity
			unitPrice = &price
		}
		return quantity, unit, unitPrice
	}
	return item.Quantity, item.Unit, item.UnitPrice
}

// reconcileReceiptEntryUnit adapts a receipt entry (quantity/unit/unitPrice,
// as decided by receiptEntryQuantity) to product's already-established
// tracking unit before it reaches pantry.Service.ReceiptIn, which trusts its
// unit argument completely and never converts (see its doc comment). Product
// name/subcategory naming here always match what the AI printed on the
// receipt, in kg/g or l/ml, when the package declared a weight or volume —
// see the extraction prompt's grocery-packaging rule — but the product's
// canonical unit was set whenever it was first created, possibly from an
// earlier receipt that never printed a weight at all. When the two units
// differ, this converts if they are the same physical dimension
// (pantry.UnitConversionFactor) and errors out — in Portuguese, meant to
// reach the user — otherwise, so the confirmation stops instead of adding a
// weight-based quantity to a unit-based stock or vice versa.
func reconcileReceiptEntryUnit(
	product *models.PantryProduct,
	quantity float64,
	unit string,
	unitPrice *float64,
) (float64, string, *float64, error) {
	if unit == product.DefaultUnit {
		return quantity, unit, unitPrice, nil
	}

	factor, ok := pantry.UnitConversionFactor(unit, product.DefaultUnit)
	if !ok {
		return 0, "", nil, fmt.Errorf(
			"%q está cadastrado em %q, mas esta nota traz %q — ajuste a unidade do item na revisão antes de confirmar",
			product.NamePt, product.DefaultUnit, unit,
		)
	}

	convertedQuantity := quantity * factor
	var convertedPrice *float64
	if unitPrice != nil {
		price := *unitPrice / factor
		convertedPrice = &price
	}
	return convertedQuantity, product.DefaultUnit, convertedPrice, nil
}

// receiptItemAmount is the monetary amount of item in the receipt's currency,
// used to build the financial transaction(s) on confirmation.
func receiptItemAmount(item models.PantryReceiptItem) float64 {
	if item.TotalPrice != nil {
		return *item.TotalPrice
	}
	if item.UnitPrice != nil {
		return item.Quantity * (*item.UnitPrice)
	}
	return 0
}

// receiptCategoryGroup accumulates the items of one financial category slug
// into a single transaction: the summed amount and a human-readable
// description listing what was bought.
type receiptCategoryGroup struct {
	amount      float64
	description string
}

// groupReceiptItemsByFinancialCategory sums non-ignored items by financial
// category slug (see financialCategorySlug), the same idea as the
// food_detail aggregation in upload.go: one transaction per category instead
// of one per line.
func groupReceiptItemsByFinancialCategory(
	items []models.PantryReceiptItem,
	categoryContext string,
) map[string]*receiptCategoryGroup {
	groups := map[string]*receiptCategoryGroup{}
	for _, item := range items {
		if item.Ignored {
			continue
		}
		amount := receiptItemAmount(item)
		if amount <= 0 {
			continue
		}
		slug := financialCategorySlug(item.Category, item.Subcategory, categoryContext)
		name := item.NamePt
		if name == "" {
			name = item.RawName
		}
		group, ok := groups[slug]
		if !ok {
			groups[slug] = &receiptCategoryGroup{amount: amount, description: name}
			continue
		}
		group.amount += amount
		group.description += ", " + name
	}
	return groups
}

// applyReceiptCurrency fills the currency/conversion fields on tx from a raw
// amount in the receipt's currency, converting to baseCurrency when they
// differ. Mirrors UploadHandler.applyCurrency in upload.go — duplicated
// (not called directly) to avoid coupling the pantry handler to the upload
// handler for a dozen lines of shared logic.
func (h *PantryReceiptHandler) applyReceiptCurrency(
	ctx context.Context,
	tx *models.Transaction,
	rawAmount float64,
	receiptCurrency, baseCurrency string,
) {
	tx.Currency = receiptCurrency
	tx.OriginalAmount = math.Round(rawAmount*100) / 100

	if receiptCurrency == baseCurrency {
		return
	}

	converted, rate, err := h.currencySvc.Convert(ctx, rawAmount, receiptCurrency, baseCurrency)
	if err != nil {
		log.Warn().Err(err).
			Str("from", receiptCurrency).Str("to", baseCurrency).
			Msg("pantry receipt: currency conversion failed, storing original amount")
		return
	}

	now := time.Now().UTC()
	tx.Amount = -converted
	tx.ConvertedAmount = &converted
	tx.ExchangeRate = &rate
	tx.ConvertedAt = &now
}

// findProductByExactName looks up an existing catalog product of userID whose
// name_pt matches name exactly (case-insensitively). Used by Confirm to avoid
// creating a duplicate product — and hitting the (user_id, name_pt) unique
// constraint — when a receipt line has no product_id yet but a product with
// that exact name already exists (created earlier for another line of the
// same receipt matched to the same seed/AI name, or from a previous receipt
// entirely). Returns nil, nil when there is no exact match.
func (h *PantryReceiptHandler) findProductByExactName(
	ctx context.Context,
	userID uuid.UUID,
	name string,
) (*uuid.UUID, error) {
	candidates, err := h.productRepo.List(ctx, userID, name, "")
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.NamePt, name) {
			id := candidate.ID
			return &id, nil
		}
	}
	return nil, nil
}

// resolveReceiptItemProduct assigns item.ProductID for an item that had none
// after extraction. It reuses a product already created for the same name_pt
// earlier in this same confirmation (createdByName), or one that already
// exists in the user's catalog (findProductByExactName), before falling back
// to creating a brand new product. Returns whether a new product was
// created, for the caller's products_created counter.
func (h *PantryReceiptHandler) resolveReceiptItemProduct(
	ctx context.Context,
	userID uuid.UUID,
	item *models.PantryReceiptItem,
	createdByName map[string]uuid.UUID,
) (created bool, err error) {
	name := receiptItemProductName(*item)
	if existingID, ok := createdByName[name]; ok {
		item.ProductID = &existingID
		return false, nil
	}

	foundID, err := h.findProductByExactName(ctx, userID, name)
	if err != nil {
		return false, fmt.Errorf("buscar produto %q: %w", name, err)
	}
	if foundID != nil {
		item.ProductID = foundID
		createdByName[name] = *foundID
		return false, nil
	}

	product := newProductFromReceiptItem(*item, userID)
	result, err := h.pantryService.CreateProduct(ctx, product, 0)
	if err != nil {
		return false, fmt.Errorf("criar produto %q: %w", item.RawName, err)
	}
	item.ProductID = &result.ID
	createdByName[name] = result.ID
	return true, nil
}

// ensureTrackByUnit switches an already-existing product to unit tracking
// when the review screen resolved one of its receipt lines into a unit
// count, preserving every other field of the product. Also resets the
// product's stock to zero in the new unit (ResetStockForUnitTracking) —
// without this, the stock row keeps its old weight-based unit forever (only
// the product's own default_unit changes here), and the ReceiptIn call right
// after this one would add a unit count on top of a leftover kg quantity,
// e.g. 1.24 kg of carrots plus a count of 2 silently becoming "3.24 kg".
func (h *PantryReceiptHandler) ensureTrackByUnit(ctx context.Context, productID, userID uuid.UUID) error {
	product, err := h.productRepo.FindByID(ctx, productID, userID)
	if err != nil {
		return fmt.Errorf("find product: %w", err)
	}
	if product.TrackByUnit {
		return nil
	}
	if err := h.pantryService.ResetStockForUnitTracking(ctx, productID, userID); err != nil {
		return fmt.Errorf("reset stock: %w", err)
	}
	product.TrackByUnit = true
	product.DefaultUnit = "un"
	if err := h.pantryService.UpdateProduct(ctx, product); err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	return nil
}

// Confirm godoc
// POST /pantry/receipts/{id}/confirm
//
//nolint:gocyclo // orchestrates a multi-step domain flow described end-to-end in the spec; splitting further would scatter the sequencing.
func (h *PantryReceiptHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)
	accountID := auth.AccountIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id")
		return
	}

	receipt, err := h.receiptRepo.FindByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "nota não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry receipt: confirm: find receipt")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if receipt.Status != models.PantryReceiptPendingReview {
		respondError(w, http.StatusConflict, "nota já foi confirmada ou descartada")
		return
	}

	var req pantryReceiptConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if len(req.Items) > 0 {
		existing, err := h.itemRepo.ListByReceipt(ctx, id)
		if err != nil {
			log.Error().Err(err).Msg("pantry receipt: confirm: list items for update")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		updated, err := applyReceiptItemUpdates(existing, req.Items)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.itemRepo.UpdateBatch(ctx, updated); err != nil {
			log.Error().Err(err).Msg("pantry receipt: confirm: save item updates")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	items, err := h.itemRepo.ListByReceipt(ctx, id)
	if err != nil {
		log.Error().Err(err).Msg("pantry receipt: confirm: list items")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if pending := pendingUnitCountItemIDs(items); len(pending) > 0 {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error": "alguns itens de hortifrúti vendidos por peso precisam da quantidade de unidades " +
				"antes de confirmar",
			"pending_unit_count_item_ids": pending,
		})
		return
	}

	// Two lines of the same receipt can both be seed-matched to the same
	// canonical name (e.g. two different brands of "sabão em pó") without
	// either resolving to an existing user product — createdByName makes the
	// second one reuse the product the first one just created instead of
	// tripping the (user_id, name_pt) unique constraint.
	createdByName := make(map[string]uuid.UUID)
	// Two different lines can also share the exact same raw receipt text
	// (e.g. the same product bought twice) — registeredRawNames avoids
	// tripping the (user_id, raw_name) unique constraint on the alias table
	// by only registering each raw name once per confirmation.
	registeredRawNames := make(map[string]bool)
	productsCreated := 0
	movementsCreated := 0
	for i := range items {
		item := &items[i]
		if item.Ignored {
			continue
		}

		productJustCreated := false
		if item.ProductID == nil {
			created, err := h.resolveReceiptItemProduct(ctx, userID, item, createdByName)
			if err != nil {
				log.Error().Err(err).Str("raw_name", item.RawName).Msg("pantry receipt: confirm: resolve product")
				respondError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			productJustCreated = created
			if created {
				productsCreated++
			}

			if !registeredRawNames[item.RawName] {
				alias := &models.PantryProductAlias{
					UserID:    userID,
					ProductID: *item.ProductID,
					RawName:   item.RawName,
					Source:    models.PantryAliasSourceUser,
				}
				if err := h.aliasRepo.Create(ctx, alias); err != nil {
					// Non-fatal: the stock entry is what matters, the alias
					// is just a shortcut for the next receipt with this name.
					log.Warn().Err(err).Str("raw_name", item.RawName).Msg("pantry receipt: confirm: create alias failed")
				}
				registeredRawNames[item.RawName] = true
			}
		}

		// A brand new product is already created with TrackByUnit set (see
		// newProductFromReceiptItem) when its own item carries a unit count.
		// Every other case — a pre-existing product, or one just reused from
		// another line of this same receipt via resolveReceiptItemProduct —
		// needs the explicit upgrade.
		if item.UnitCount != nil && !productJustCreated {
			if err := h.ensureTrackByUnit(ctx, *item.ProductID, userID); err != nil {
				log.Error().Err(err).Str("product_id", item.ProductID.String()).
					Msg("pantry receipt: confirm: ensure track by unit")
				respondError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
		}

		quantity, unit, unitPrice := receiptEntryQuantity(*item)

		product, err := h.productRepo.FindByID(ctx, *item.ProductID, userID)
		if err != nil {
			log.Error().Err(err).Str("product_id", item.ProductID.String()).Msg("pantry receipt: confirm: load product for unit check")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		quantity, unit, unitPrice, err = reconcileReceiptEntryUnit(product, quantity, unit, unitPrice)
		if err != nil {
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}

		if _, err := h.pantryService.ReceiptIn(ctx, *item.ProductID, userID, quantity, unit, unitPrice, receipt.Currency, receipt.ID); err != nil {
			log.Error().Err(err).Str("product_id", item.ProductID.String()).Msg("pantry receipt: confirm: receipt in")
			respondError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		movementsCreated++

		// Non-fatal: an open shopping list item for this product should be
		// marked bought automatically, but a failure here shouldn't undo the
		// stock entry that already succeeded above.
		if _, err := h.shoppingListSvc.MarkBoughtByProduct(ctx, userID, *item.ProductID, item.ID); err != nil {
			log.Warn().Err(err).Str("product_id", item.ProductID.String()).
				Msg("pantry receipt: confirm: mark shopping list item bought")
		}
	}

	// Non-fatal, same as the per-item marking above: the stock entries already
	// succeeded, a shopping list regeneration hiccup shouldn't block the
	// response. Run once for the whole receipt, not once per item.
	if err := h.shoppingListSvc.Regenerate(ctx, userID); err != nil {
		log.Warn().Err(err).Str("user_id", userID.String()).Msg("pantry receipt: confirm: regenerate shopping list")
	}

	createdTxs := []models.Transaction{}
	if req.CreateTransaction {
		user, err := h.userRepo.FindByID(userID)
		if err != nil {
			log.Error().Err(err).Msg("pantry receipt: confirm: find user")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}

		baseCurrency, err := h.currencySvc.GetUserBaseCurrency(ctx, userID.String())
		if err != nil {
			log.Warn().Err(err).Msg("pantry receipt: confirm: could not get user base currency, defaulting to BRL")
			baseCurrency = "BRL"
		}

		activePMs, err := h.pmRepo.ListActive(accountID)
		if err != nil {
			log.Warn().Err(err).Msg("pantry receipt: confirm: could not list active payment methods")
		}

		var hint transactions.PaymentHint
		if req.PaymentMethodID != nil && strings.TrimSpace(*req.PaymentMethodID) != "" {
			pmID, err := uuid.Parse(*req.PaymentMethodID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "payment_method_id inválido")
				return
			}
			hint.PaymentMethodID = &pmID
		}

		imageURL := receiptImageURL(r, receipt.ImageKey)
		groups := groupReceiptItemsByFinancialCategory(items, user.CategoryContext)
		slugs := make([]string, 0, len(groups))
		for slug := range groups {
			slugs = append(slugs, slug)
		}
		sort.Strings(slugs)

		for _, slug := range slugs {
			group := groups[slug]
			draft := &models.Transaction{
				UserID:      userID,
				AccountID:   accountID,
				Amount:      -group.amount,
				Category:    slug,
				Merchant:    receipt.Merchant,
				Description: group.description,
				Date:        receipt.PurchasedAt,
				Type:        "expense",
				Source:      "photo",
				ImageURL:    imageURL,
			}
			h.applyReceiptCurrency(ctx, draft, group.amount, receipt.Currency, baseCurrency)

			need, verr := h.validator.ValidateAndEnrich(ctx, accountID, draft, hint)
			if verr != nil {
				log.Warn().Err(verr).Str("category", slug).Msg("pantry receipt: confirm: validate transaction")
				respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("categoria %q: %v", slug, verr))
				return
			}
			if need != nil {
				// Etapa 2: sem fluxo de PendingAction para a dispensa ainda —
				// o caller deve resolver a forma de pagamento e chamar
				// /confirm novamente (os itens de estoque já processados
				// acima permanecem válidos).
				respondJSON(w, http.StatusUnprocessableEntity, need)
				return
			}
			if draft.PaymentMethodID != nil {
				applyBillingMonth(draft, *draft.PaymentMethodID, activePMs)
			}

			if err := h.txRepo.Create(draft); err != nil {
				log.Error().Err(err).Msg("pantry receipt: confirm: create transaction")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			createdTxs = append(createdTxs, *draft)
			receipt.TransactionIDs = append(receipt.TransactionIDs, draft.ID.String())
		}
	}

	now := time.Now().UTC()
	receipt.Status = models.PantryReceiptConfirmed
	receipt.ConfirmedAt = &now
	if err := h.receiptRepo.Update(ctx, receipt); err != nil {
		log.Error().Err(err).Msg("pantry receipt: confirm: save receipt")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"receipt":           receipt,
		"movements_created": movementsCreated,
		"products_created":  productsCreated,
		"transactions":      createdTxs,
	})
}

// Discard godoc
// DELETE /pantry/receipts/{id}
func (h *PantryReceiptHandler) Discard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := auth.UserIDFromContext(ctx)

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid receipt id")
		return
	}

	receipt, err := h.receiptRepo.FindByID(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "nota não encontrada")
			return
		}
		log.Error().Err(err).Msg("pantry receipt: discard: find receipt")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	receipt.Status = models.PantryReceiptDiscarded
	if err := h.receiptRepo.Update(ctx, receipt); err != nil {
		log.Error().Err(err).Msg("pantry receipt: discard: save receipt")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
