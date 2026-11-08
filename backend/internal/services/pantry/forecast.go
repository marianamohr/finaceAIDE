package pantry

import (
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

// pantryForecastLookbackDays bounds how far back outgoing movements
// (manual_out/recipe_out) count toward the consumption rate (§6.1 of the
// design doc: "últimos 60 dias").
const pantryForecastLookbackDays = 60

// pantryForecastMinObservedDays is the minimum number of days the outgoing
// rate is divided by, even when the earliest observed exit is more recent
// than that — avoids wildly overestimating the daily rate from a single
// exit a day or two ago.
const pantryForecastMinObservedDays = 14

// pantryExpiringWithinDays is the "vencendo" threshold: a stock entry is
// flagged as expiring when its estimated expiry falls within this many days.
const pantryExpiringWithinDays = 3

// pantryMinLevelSuggestionFactor is the safety margin applied over the
// expected consumption during one shopping interval when suggesting a
// minimum level from the daily rate (§6.1: "daily_rate × shopping_interval ×
// 1.2").
const pantryMinLevelSuggestionFactor = 1.2

// productForecast holds the Etapa-5 stock-forecast numbers for one product,
// computed once per request and shared between the catalog listing
// (Service.ListWithStock), the shopping list (ShoppingListService.Regenerate)
// and the recipe-generation pantry context (RecipeService.loadPantryContext)
// — all three need the same daily consumption rate and effective minimum
// level, so it is computed in one place.
type productForecast struct {
	DailyRate         *float64
	RunOutAt          *time.Time
	EffectiveMinLevel *float64 // product.MinLevel when set, otherwise the auto-suggested one (may still be nil)
	MinLevelSource    string   // "manual" | "auto"
}

// buildForecasts computes a productForecast for every product in products,
// from the current stock (stocksByProduct) and the user's full movement
// history (movements, any order, not pre-filtered by date — the rate_in
// fallback needs entries older than the 60-day outgoing window).
func buildForecasts(
	products []models.PantryProduct,
	stocksByProduct map[uuid.UUID]models.PantryStock,
	movements []models.PantryMovement,
	shoppingIntervalDays int,
	now time.Time,
) map[uuid.UUID]productForecast {
	movementsByProduct := make(map[uuid.UUID][]models.PantryMovement, len(products))
	for _, m := range movements {
		movementsByProduct[m.ProductID] = append(movementsByProduct[m.ProductID], m)
	}

	result := make(map[uuid.UUID]productForecast, len(products))
	for _, product := range products {
		dailyRate := computeDailyRate(movementsByProduct[product.ID], now)

		quantity := 0.0
		if stock, ok := stocksByProduct[product.ID]; ok {
			quantity = stock.Quantity
		}
		var runOutAt *time.Time
		if dailyRate != nil && *dailyRate > 0 && quantity > 0 {
			days := quantity / *dailyRate
			t := now.Add(time.Duration(days * 24 * float64(time.Hour)))
			runOutAt = &t
		}

		effectiveMinLevel := product.MinLevel
		minLevelSource := "manual"
		if effectiveMinLevel == nil {
			if suggested := suggestedMinLevel(dailyRate, shoppingIntervalDays); suggested != nil {
				effectiveMinLevel = suggested
				minLevelSource = "auto"
			}
		}

		result[product.ID] = productForecast{
			DailyRate:         dailyRate,
			RunOutAt:          runOutAt,
			EffectiveMinLevel: effectiveMinLevel,
			MinLevelSource:    minLevelSource,
		}
	}
	return result
}

// computeDailyRate implements § 6.1: rate_out from outgoing movements in the
// last pantryForecastLookbackDays days when there are any; otherwise
// rate_in from the purchase history between the first and second-to-last
// entry (needs at least two); otherwise nil (no forecast possible).
func computeDailyRate(productMovements []models.PantryMovement, now time.Time) *float64 {
	since := now.AddDate(0, 0, -pantryForecastLookbackDays)

	var outSum float64
	var earliestOut *time.Time
	entries := make([]models.PantryMovement, 0, len(productMovements))

	for _, m := range productMovements {
		switch m.Type {
		case models.PantryMovementManualOut, models.PantryMovementRecipeOut:
			if m.OccurredAt.Before(since) {
				continue
			}
			outSum += -m.Quantity // quantity is negative for exits
			if earliestOut == nil || m.OccurredAt.Before(*earliestOut) {
				t := m.OccurredAt
				earliestOut = &t
			}
		case models.PantryMovementReceiptIn, models.PantryMovementManualIn:
			entries = append(entries, m)
		}
	}

	if outSum > 0 {
		observedDays := float64(pantryForecastMinObservedDays)
		if earliestOut != nil {
			if d := now.Sub(*earliestOut).Hours() / 24; d > observedDays {
				observedDays = d
			}
		}
		rate := outSum / observedDays
		return &rate
	}

	if len(entries) < 2 {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].OccurredAt.Before(entries[j].OccurredAt) })
	first := entries[0]
	penultimate := entries[len(entries)-2]
	days := penultimate.OccurredAt.Sub(first.OccurredAt).Hours() / 24
	if days <= 0 {
		return nil
	}
	var sum float64
	for _, e := range entries[:len(entries)-1] {
		sum += e.Quantity
	}
	rate := sum / days
	if rate <= 0 {
		return nil
	}
	return &rate
}

// suggestedMinLevel implements the "mínimo sugerido" formula of § 6.1,
// rounded up (minimum 1). Returns nil when there is no daily rate to base a
// suggestion on.
func suggestedMinLevel(dailyRate *float64, shoppingIntervalDays int) *float64 {
	if dailyRate == nil || *dailyRate <= 0 {
		return nil
	}
	suggested := math.Ceil(*dailyRate * float64(shoppingIntervalDays) * pantryMinLevelSuggestionFactor)
	if suggested < 1 {
		suggested = 1
	}
	return &suggested
}

// applyEntryExpiry sets stock.ExpiresAt from product.ShelfLifeDays whenever a
// new "in" movement (a purchase, whether via nota fiscal or manual entry)
// happens, at occurredAt + shelf life. When the product has no shelf life
// configured, stock.ExpiresAt is left untouched (it keeps whatever an
// earlier entry set, or stays nil). This is a simplification of the design's
// literal "expires_at por entrada": the stock model has no per-lot/FIFO
// tracking, so the newest entry always overwrites the estimate rather than
// each batch tracking its own expiry.
func applyEntryExpiry(stock *models.PantryStock, product *models.PantryProduct, occurredAt time.Time) {
	if product.ShelfLifeDays == nil {
		return
	}
	expiresAt := occurredAt.AddDate(0, 0, *product.ShelfLifeDays)
	stock.ExpiresAt = &expiresAt
}

// isExpiring reports whether expiresAt falls within pantryExpiringWithinDays
// of now. A nil expiresAt (no shelf life data, or never purchased) is never
// expiring.
func isExpiring(expiresAt *time.Time, now time.Time) bool {
	if expiresAt == nil {
		return false
	}
	return !expiresAt.After(now.AddDate(0, 0, pantryExpiringWithinDays))
}

// isRunningOut reports whether runOutAt falls within shoppingIntervalDays of
// now — the product would run out before the user's next planned shopping
// trip. A nil runOutAt (no forecast) is never running out.
func isRunningOut(runOutAt *time.Time, shoppingIntervalDays int, now time.Time) bool {
	if runOutAt == nil {
		return false
	}
	return !runOutAt.After(now.AddDate(0, 0, shoppingIntervalDays))
}
