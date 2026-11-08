// Package pantry contains the business rules of the dispensa inteligente module:
// product catalog validation, stock math (consume, adjust, mark out) and the
// weighted average price used when a new purchase enters the stock.
package pantry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/gorm"
)

// Pantry stock status values, computed from the current quantity and the
// product's configured minimum level.
const (
	StatusOK         = "ok"
	StatusLow        = "low"
	StatusOut        = "out"
	StatusRunningOut = "running_out"
)

// Service implements the business rules of the dispensa inteligente module on
// top of the pantry repositories.
type Service struct {
	products  *repository.PantryProductRepository
	stocks    *repository.PantryStockRepository
	movements *repository.PantryMovementRepository
	db        *gorm.DB
}

// NewService wires the pantry repositories and the DB handle used to run
// stock+movement writes atomically.
func NewService(
	products *repository.PantryProductRepository,
	stocks *repository.PantryStockRepository,
	movements *repository.PantryMovementRepository,
	db *gorm.DB,
) *Service {
	return &Service{
		products:  products,
		stocks:    stocks,
		movements: movements,
		db:        db,
	}
}

// ProductWithStock joins a catalog product with its current stock for the
// listing and mutation endpoints. Stock fields are flattened (not nested)
// because a product may not have a stock row yet — see ListWithStock.
type ProductWithStock struct {
	models.PantryProduct
	Quantity        float64    `json:"quantity"`
	Unit            string     `json:"unit"`
	AvgPrice        *float64   `json:"avg_price,omitempty"`
	Currency        string     `json:"currency"`
	LastPurchasedAt *time.Time `json:"last_purchased_at,omitempty"`
	Status          string     `json:"status"`
	// MinLevel shadows models.PantryProduct.MinLevel: it carries the
	// EFFECTIVE minimum (the user's configured value, or the auto-suggested
	// one from the consumption rate when the user never set one), tagged by
	// MinLevelSource.
	MinLevel       *float64   `json:"min_level,omitempty"`
	MinLevelSource string     `json:"min_level_source"` // "manual" | "auto"
	DailyRate      *float64   `json:"daily_rate,omitempty"`
	RunOutAt       *time.Time `json:"run_out_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	Expiring       bool       `json:"expiring"`
}

// validateProduct checks the fields shared by CreateProduct and UpdateProduct.
// Messages are in Portuguese: they reach the user as-is through the handler.
func validateProduct(p *models.PantryProduct) error {
	if p == nil {
		return errors.New("produto ausente")
	}
	if strings.TrimSpace(p.NamePt) == "" {
		return errors.New("nome do produto é obrigatório")
	}
	switch p.Category {
	case models.PantryCategoryFood, models.PantryCategoryCleaning, models.PantryCategoryHygiene, models.PantryCategoryGeneral:
	default:
		return fmt.Errorf("categoria inválida: %q", p.Category)
	}
	if !slices.Contains(models.PantryUnits, p.DefaultUnit) {
		return fmt.Errorf("unidade inválida: %q", p.DefaultUnit)
	}
	return nil
}

// computeStatus derives the display status of a product from its current
// quantity, its effective minimum level (nil when none is configured or
// suggested) and its forecasted run-out date, in priority order: out (no
// stock) > low (below minimum) > running_out (forecast to run out before the
// next shopping trip) > ok. runOutAt/shoppingIntervalDays may be zero-valued
// (nil / 0) when no forecast was computed for this call site — running_out
// is then simply never reported.
func computeStatus(quantity float64, minLevel *float64, runOutAt *time.Time, shoppingIntervalDays int, now time.Time) string {
	if quantity <= 0 {
		return StatusOut
	}
	if minLevel != nil && quantity < *minLevel {
		return StatusLow
	}
	if isRunningOut(runOutAt, shoppingIntervalDays, now) {
		return StatusRunningOut
	}
	return StatusOK
}

// toProductWithStock assembles the listing/mutation return value from a
// product and its stock row. forecast is nil at mutation call sites that
// don't have (and don't need) the user's full movement history and
// shopping-interval setting on hand — daily_rate/run_out_at/running_out are
// simply omitted there, but expiry (a plain stock field, no movements
// needed) is always populated.
func toProductWithStock(
	product *models.PantryProduct,
	stock *models.PantryStock,
	forecast *productForecast,
	shoppingIntervalDays int,
) ProductWithStock {
	now := time.Now().UTC()
	minLevel := product.MinLevel
	minLevelSource := "manual"
	var dailyRate *float64
	var runOutAt *time.Time
	if forecast != nil {
		minLevel = forecast.EffectiveMinLevel
		minLevelSource = forecast.MinLevelSource
		dailyRate = forecast.DailyRate
		runOutAt = forecast.RunOutAt
	}

	return ProductWithStock{
		PantryProduct:   *product,
		Quantity:        stock.Quantity,
		Unit:            stock.Unit,
		AvgPrice:        stock.AvgPrice,
		Currency:        stock.Currency,
		LastPurchasedAt: stock.LastPurchasedAt,
		MinLevel:        minLevel,
		MinLevelSource:  minLevelSource,
		DailyRate:       dailyRate,
		RunOutAt:        runOutAt,
		ExpiresAt:       stock.ExpiresAt,
		Expiring:        isExpiring(stock.ExpiresAt, now),
		Status:          computeStatus(stock.Quantity, minLevel, runOutAt, shoppingIntervalDays, now),
	}
}

// emptyStock builds the zero-value stock row used when a product has no
// pantry_stock entry yet (no movement has ever been recorded for it).
func emptyStock(productID, userID uuid.UUID, unit string) *models.PantryStock {
	return &models.PantryStock{
		ProductID: productID,
		UserID:    userID,
		Unit:      unit,
		Currency:  models.PantryCurrencyBRL,
	}
}

// clampConsume returns the stock quantity remaining after discounting
// requested from current, never going below zero — consuming more than what
// is on hand simply zeroes the stock instead of failing.
func clampConsume(current, requested float64) float64 {
	remaining := current - requested
	if remaining < 0 {
		return 0
	}
	return remaining
}

// applyWeightedAveragePrice computes the new weighted average unit price
// after a stock entry of newQty at newPrice joins an existing currentQty at
// currentAvg. currentAvg is nil for a product's first ever entry.
//
// No caller uses this yet: it lands with the receipt-entry flow in Stage 2,
// which is the first place a purchase price is known. Written and tested now
// because the pure math is what the plan asks to cover in this stage.
func applyWeightedAveragePrice(currentAvg *float64, currentQty, newQty, newPrice float64) float64 {
	if currentAvg == nil {
		return newPrice
	}
	totalQty := currentQty + newQty
	if totalQty == 0 {
		return newPrice
	}
	return (*currentAvg*currentQty + newPrice*newQty) / totalQty
}

// CreateProduct validates and inserts a new catalog product. When
// initialQuantity is positive, it also opens the stock row and records a
// manual-in movement for it, atomically.
func (s *Service) CreateProduct(
	ctx context.Context,
	p *models.PantryProduct,
	initialQuantity float64,
) (*ProductWithStock, error) {
	if err := validateProduct(p); err != nil {
		return nil, err
	}
	if err := s.products.Create(ctx, p); err != nil {
		return nil, err
	}

	stock := emptyStock(p.ID, p.UserID, p.DefaultUnit)
	if initialQuantity <= 0 {
		result := toProductWithStock(p, stock, nil, 0)
		return &result, nil
	}

	stock.Quantity = initialQuantity
	occurredAt := time.Now().UTC()
	applyEntryExpiry(stock, p, occurredAt)
	movement := &models.PantryMovement{
		UserID:     p.UserID,
		ProductID:  p.ID,
		Type:       models.PantryMovementManualIn,
		Quantity:   initialQuantity,
		Unit:       p.DefaultUnit,
		OccurredAt: occurredAt,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: create product: initial stock: %w", err)
	}

	result := toProductWithStock(p, stock, nil, 0)
	return &result, nil
}

// UpdateProduct validates and persists changes to an existing catalog product.
func (s *Service) UpdateProduct(ctx context.Context, p *models.PantryProduct) error {
	if err := validateProduct(p); err != nil {
		return err
	}
	return s.products.Update(ctx, p)
}

// GetProduct loads a single catalog product, enforcing ownership. Thin
// delegate to the repository — exists so handlers don't need their own
// PantryProductRepository just to read a product back before validating a
// request against it (see PantryProductHandler.Update's unit-change guard).
func (s *Service) GetProduct(ctx context.Context, id, userID uuid.UUID) (*models.PantryProduct, error) {
	return s.products.FindByID(ctx, id, userID)
}

// DeleteProduct removes a catalog product. Its stock row is removed by the
// database's ON DELETE CASCADE; its movement history is kept as an orphaned
// audit trail on purpose.
func (s *Service) DeleteProduct(ctx context.Context, id, userID uuid.UUID) error {
	return s.products.Delete(ctx, id, userID)
}

// ListWithStock joins the catalog with current stock, computing each
// product's status and Etapa-5 forecast (daily_rate, run_out_at, expiring).
// Products without a stock row yet are reported as empty (StatusOut).
// statusFilter matches against Status ("ok"|"low"|"out"|"running_out") when
// non-empty; the special value "expiring" filters by the Expiring flag
// instead, since expiry is independent of the stock-level status.
func (s *Service) ListWithStock(
	ctx context.Context,
	userID uuid.UUID,
	query, category, statusFilter string,
	shoppingIntervalDays int,
) ([]ProductWithStock, error) {
	products, err := s.products.List(ctx, userID, query, category)
	if err != nil {
		return nil, err
	}
	stocksByProduct, err := s.stocks.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	movements, err := s.movements.ListByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	forecasts := buildForecasts(products, stocksByProduct, movements, shoppingIntervalDays, time.Now().UTC())

	result := make([]ProductWithStock, 0, len(products))
	for _, product := range products {
		stock, ok := stocksByProduct[product.ID]
		if !ok {
			stock = *emptyStock(product.ID, userID, product.DefaultUnit)
		}
		forecast := forecasts[product.ID]

		item := toProductWithStock(&product, &stock, &forecast, shoppingIntervalDays)
		if statusFilter == "expiring" {
			if !item.Expiring {
				continue
			}
		} else if statusFilter != "" && item.Status != statusFilter {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

// findProductAndStock loads a product (enforcing ownership) and its stock
// row, treating a missing stock row as empty stock rather than an error.
func (s *Service) findProductAndStock(
	ctx context.Context,
	productID, userID uuid.UUID,
) (*models.PantryProduct, *models.PantryStock, error) {
	product, err := s.products.FindByID(ctx, productID, userID)
	if err != nil {
		return nil, nil, err
	}

	stock, err := s.stocks.Get(ctx, productID)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			return nil, nil, err
		}
		stock = emptyStock(productID, userID, product.DefaultUnit)
	}
	return product, stock, nil
}

// Consume discounts quantity from productID's current stock, clamped at
// zero: asking to consume more than what is on hand is not an error, it just
// zeroes the stock. Records a manual-out movement for the amount actually
// discounted.
func (s *Service) Consume(
	ctx context.Context,
	productID, userID uuid.UUID,
	quantity float64,
	note string,
) (*ProductWithStock, error) {
	if quantity <= 0 {
		return nil, errors.New("quantidade a consumir deve ser maior que zero")
	}

	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	newQuantity := clampConsume(stock.Quantity, quantity)
	discounted := stock.Quantity - newQuantity
	stock.Quantity = newQuantity

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		movement := &models.PantryMovement{
			UserID:     userID,
			ProductID:  productID,
			Type:       models.PantryMovementManualOut,
			Quantity:   -discounted,
			Unit:       stock.Unit,
			Note:       note,
			OccurredAt: time.Now().UTC(),
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: consume: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}

// MarkOut zeroes productID's stock in one step ("acabou"). If the stock is
// already zero this is a no-op that still returns the current state without
// error or a zero-quantity movement.
func (s *Service) MarkOut(ctx context.Context, productID, userID uuid.UUID) (*ProductWithStock, error) {
	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	if stock.Quantity <= 0 {
		result := toProductWithStock(product, stock, nil, 0)
		return &result, nil
	}

	discounted := stock.Quantity
	stock.Quantity = 0

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		movement := &models.PantryMovement{
			UserID:     userID,
			ProductID:  productID,
			Type:       models.PantryMovementManualOut,
			Quantity:   -discounted,
			Unit:       stock.Unit,
			OccurredAt: time.Now().UTC(),
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: mark out: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}

// ReceiptIn adds a stock entry coming from a confirmed pantry receipt: it
// always increases the current quantity (never subtracts), refreshes the
// weighted average price when unitPrice is known, stamps last_purchased_at,
// and records a receipt_in movement referencing the receipt. Used by the
// receipt confirmation flow, once per confirmed item.
//
// unit is expected to already match the product's established stock unit —
// ReceiptIn itself does not reconcile or convert between units, and does not
// overwrite the stock's stored unit, precisely to avoid silently corrupting
// the quantity when a receipt line ends up matched (by name) to an existing
// product tracked in a different unit. Reconciling a mismatch — converting
// when the two units are the same physical dimension (UnitConversionFactor),
// rejecting the entry otherwise — is the caller's responsibility; so is the
// case where the unit legitimately changes going forward, a product switched
// to per-unit tracking (see PantryReceiptHandler.ensureTrackByUnit and its
// unit-conversion check right before each ReceiptIn call).
func (s *Service) ReceiptIn(
	ctx context.Context,
	productID, userID uuid.UUID,
	quantity float64,
	unit string,
	unitPrice *float64,
	currency string,
	referenceID uuid.UUID,
) (*ProductWithStock, error) {
	if quantity <= 0 {
		return nil, errors.New("quantidade de entrada deve ser maior que zero")
	}

	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	if unitPrice != nil {
		stock.AvgPrice = float64Ptr(applyWeightedAveragePrice(stock.AvgPrice, stock.Quantity, quantity, *unitPrice))
	}
	stock.Quantity += quantity
	if currency != "" {
		stock.Currency = currency
	}
	now := time.Now().UTC()
	stock.LastPurchasedAt = &now
	applyEntryExpiry(stock, product, now)

	referenceType := "receipt"
	movement := &models.PantryMovement{
		UserID:        userID,
		ProductID:     productID,
		Type:          models.PantryMovementReceiptIn,
		Quantity:      quantity,
		Unit:          unit,
		UnitPrice:     unitPrice,
		ReferenceType: &referenceType,
		ReferenceID:   &referenceID,
		OccurredAt:    now,
	}
	if currency != "" {
		movement.Currency = &currency
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: receipt in: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}

// RecipeOut discounts quantity from productID's stock as a consequence of
// cooking a recipe, clamped at zero exactly like Consume, and records a
// recipe_out movement referencing the recipe log. Used by the recipe
// confirmation flow ("vou fazer"), once per deducted ingredient.
//
// unit is expected to already match the product's established stock unit —
// same contract as ReceiptIn: RecipeOut does not reconcile or convert units
// itself; that is the caller's responsibility (pantry.UnitConversionFactor +
// the same reconciliation pattern used before every ReceiptIn call).
func (s *Service) RecipeOut(
	ctx context.Context,
	productID, userID uuid.UUID,
	quantity float64,
	unit string,
	referenceID uuid.UUID,
) (*ProductWithStock, error) {
	if quantity <= 0 {
		return nil, errors.New("quantidade a descontar deve ser maior que zero")
	}

	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	newQuantity := clampConsume(stock.Quantity, quantity)
	discounted := stock.Quantity - newQuantity
	stock.Quantity = newQuantity

	referenceType := "recipe_log"
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		movement := &models.PantryMovement{
			UserID:        userID,
			ProductID:     productID,
			Type:          models.PantryMovementRecipeOut,
			Quantity:      -discounted,
			Unit:          unit,
			ReferenceType: &referenceType,
			ReferenceID:   &referenceID,
			OccurredAt:    time.Now().UTC(),
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: recipe out: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}

// ResetStockForUnitTracking zeroes productID's stock and switches its
// tracked unit to "un". Called by PantryReceiptHandler.ensureTrackByUnit the
// moment a product switches from weight-based to per-unit tracking: a
// leftover weight-based quantity (e.g. 1.24 kg of carrots already on hand)
// cannot be reliably turned into a unit count — there is no fixed weight per
// unit — so tracking restarts at zero in the new unit instead of silently
// adding a unit count on top of a kg quantity (the visible symptom was stock
// showing something like "3.24 kg" after answering "2" units for produce
// that already had loose stock on hand). The zeroed-out quantity is recorded
// as an adjust movement, in its original unit, for the audit trail.
//
// A no-op (returns nil) when the product has no stock row yet, or when stock
// is already zero and tracked in "un".
func (s *Service) ResetStockForUnitTracking(ctx context.Context, productID, userID uuid.UUID) error {
	stock, err := s.stocks.Get(ctx, productID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("pantry service: reset stock for unit tracking: %w", err)
	}
	if stock.Quantity == 0 && stock.Unit == "un" {
		return nil
	}

	previousQuantity := stock.Quantity
	previousUnit := stock.Unit
	stock.Quantity = 0
	stock.Unit = "un"

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		if previousQuantity == 0 {
			return nil
		}
		movement := &models.PantryMovement{
			UserID:     userID,
			ProductID:  productID,
			Type:       models.PantryMovementAdjust,
			Quantity:   -previousQuantity,
			Unit:       previousUnit,
			Note:       "estoque zerado ao trocar para rastreio por unidade",
			OccurredAt: time.Now().UTC(),
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return fmt.Errorf("pantry service: reset stock for unit tracking: %w", err)
	}
	return nil
}

// float64Ptr returns a pointer to v — used by ReceiptIn to store the result
// of applyWeightedAveragePrice (always a concrete value, never nil) into the
// *float64 stock field.
func float64Ptr(v float64) *float64 {
	return &v
}

// UnitConversionFactor returns f such that a quantity expressed in from
// equals quantity*f once expressed in to, and ok=true when the two units are
// the same physical dimension. Only the SI weight pair (kg, g) and the SI
// volume pair (l, ml) convert; count-like units (un, dz, pct, maco) never
// convert into anything, including each other, since "1 un" carries no fixed
// weight or volume. ReceiptIn does not call this itself — see its doc comment
// — callers reconcile a receipt line's unit against a product's established
// unit before calling ReceiptIn, and reject the entry when ok is false.
func UnitConversionFactor(from, to string) (factor float64, ok bool) {
	if from == to {
		return 1, true
	}
	switch {
	case from == "kg" && to == "g":
		return 1000, true
	case from == "g" && to == "kg":
		return 1.0 / 1000, true
	case from == "l" && to == "ml":
		return 1000, true
	case from == "ml" && to == "l":
		return 1.0 / 1000, true
	default:
		return 0, false
	}
}

// ChangeUnit switches productID's tracked unit (both pantry_products.default_unit
// and pantry_stock.unit — changing only one and not the other is exactly the
// bug ResetStockForUnitTracking exists to prevent, see its doc comment) to
// newUnit, from the Dispensa product screen (e.g. "Arroz" from kg to g).
//
//   - When newUnit shares a physical dimension with the current stock unit
//     (kg<->g, l<->ml, or newUnit == the current unit), the quantity is
//     converted exactly via UnitConversionFactor and no movement is recorded:
//     it is a redenomination, not a real change in what's on hand.
//   - Otherwise (e.g. kg -> un) there is no sound conversion — "1 un" carries
//     no fixed weight — so the stock is zeroed and, if it wasn't already
//     zero, an adjust movement documents the old quantity for the audit
//     trail. Same tradeoff ResetStockForUnitTracking makes for hortifruti.
//
// For a produce product, TrackByUnit is kept in sync with the new unit
// (true when newUnit == "un", false otherwise) since that flag only means
// "this produce item is tracked by count instead of weight" — non-produce
// products never use it and are left untouched.
func (s *Service) ChangeUnit(
	ctx context.Context,
	productID, userID uuid.UUID,
	newUnit string,
) (*ProductWithStock, error) {
	if !slices.Contains(models.PantryUnits, newUnit) {
		return nil, fmt.Errorf("unidade inválida: %q", newUnit)
	}

	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	if stock.Unit != newUnit {
		factor, convertible := UnitConversionFactor(stock.Unit, newUnit)
		previousQuantity := stock.Quantity
		previousUnit := stock.Unit

		if convertible {
			stock.Quantity *= factor
		} else {
			stock.Quantity = 0
		}
		stock.Unit = newUnit

		err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
				return err
			}
			if convertible || previousQuantity == 0 {
				return nil
			}
			movement := &models.PantryMovement{
				UserID:     userID,
				ProductID:  productID,
				Type:       models.PantryMovementAdjust,
				Quantity:   -previousQuantity,
				Unit:       previousUnit,
				Note:       fmt.Sprintf("estoque zerado ao trocar unidade de %s para %s", previousUnit, newUnit),
				OccurredAt: time.Now().UTC(),
			}
			return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
		})
		if err != nil {
			return nil, fmt.Errorf("pantry service: change unit: %w", err)
		}
	}

	product.DefaultUnit = newUnit
	if product.IsProduce {
		product.TrackByUnit = newUnit == "un"
	}
	if err := s.products.Update(ctx, product); err != nil {
		return nil, fmt.Errorf("pantry service: change unit: update product: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}

// Adjust sets productID's stock to an absolute quantity (a manual correction,
// e.g. a physical count). Records the difference against the previous
// quantity as an adjust movement; a zero difference is a no-op.
func (s *Service) Adjust(
	ctx context.Context,
	productID, userID uuid.UUID,
	quantity float64,
	note string,
) (*ProductWithStock, error) {
	if quantity < 0 {
		return nil, errors.New("quantidade ajustada não pode ser negativa")
	}

	product, stock, err := s.findProductAndStock(ctx, productID, userID)
	if err != nil {
		return nil, err
	}

	diff := quantity - stock.Quantity
	if diff == 0 {
		result := toProductWithStock(product, stock, nil, 0)
		return &result, nil
	}
	stock.Quantity = quantity

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryStockRepository(tx).Save(ctx, stock); err != nil {
			return err
		}
		movement := &models.PantryMovement{
			UserID:     userID,
			ProductID:  productID,
			Type:       models.PantryMovementAdjust,
			Quantity:   diff,
			Unit:       stock.Unit,
			Note:       note,
			OccurredAt: time.Now().UTC(),
		}
		return repository.NewPantryMovementRepository(tx).Create(ctx, movement)
	})
	if err != nil {
		return nil, fmt.Errorf("pantry service: adjust: %w", err)
	}

	result := toProductWithStock(product, stock, nil, 0)
	return &result, nil
}
