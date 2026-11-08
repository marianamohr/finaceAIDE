package handlers

import (
	"net/http"
	"time"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// claude-sonnet-5 pricing used by every AI call in the dispensa module (see
// design doc §7) — used only to give the user a rough monthly cost estimate,
// not for billing.
const (
	pantryAIUsageCostPerInputTokenUSD  = 2.0 / 1_000_000
	pantryAIUsageCostPerOutputTokenUSD = 10.0 / 1_000_000
)

// PantryAIUsageHandler exposes a rough token/cost summary of the dispensa's
// AI calls (nota fiscal + receitas) for the current calendar month, so the
// user can see what the module is actually costing them (design doc's
// "métricas de tokens no admin" — surfaced in Ajustes, this being a
// single-user app with no separate admin screen).
type PantryAIUsageHandler struct {
	receipts *repository.PantryReceiptRepository
	recipes  *repository.PantryRecipeRepository
}

func NewPantryAIUsageHandler(
	receipts *repository.PantryReceiptRepository,
	recipes *repository.PantryRecipeRepository,
) *PantryAIUsageHandler {
	return &PantryAIUsageHandler{receipts: receipts, recipes: recipes}
}

// pantryAIUsageBreakdown is the token/count summary of one AI-backed
// resource (receipts or recipes).
type pantryAIUsageBreakdown struct {
	Count        int `json:"count"`
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Get godoc
// GET /pantry/ai-usage
//
// Sums ai_input_tokens/ai_output_tokens across every receipt and recipe
// created in the current calendar month (UTC) — a full re-scan of the
// month's rows in Go rather than a SQL aggregate, consistent with this
// module's personal-scale tradeoff (see PantryMovementRepository.ListByUser).
func (h *PantryAIUsageHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	ctx := r.Context()

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	receipts, err := h.receipts.List(ctx, userID, "")
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry ai usage: list receipts")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	recipes, err := h.recipes.List(ctx, userID, false)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry ai usage: list recipes")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	receiptsUsage := pantryAIUsageBreakdown{}
	for _, receipt := range receipts {
		if receipt.CreatedAt.Before(monthStart) {
			continue
		}
		receiptsUsage.Count++
		receiptsUsage.InputTokens += receipt.AIInputTokens
		receiptsUsage.OutputTokens += receipt.AIOutputTokens
	}

	recipesUsage := pantryAIUsageBreakdown{}
	for _, recipe := range recipes {
		if recipe.CreatedAt.Before(monthStart) {
			continue
		}
		recipesUsage.Count++
		recipesUsage.InputTokens += recipe.AIInputTokens
		recipesUsage.OutputTokens += recipe.AIOutputTokens
	}

	totalInput := receiptsUsage.InputTokens + recipesUsage.InputTokens
	totalOutput := receiptsUsage.OutputTokens + recipesUsage.OutputTokens
	estimatedCost := float64(totalInput)*pantryAIUsageCostPerInputTokenUSD + float64(totalOutput)*pantryAIUsageCostPerOutputTokenUSD

	respondJSON(w, http.StatusOK, map[string]any{
		"period":              monthStart.Format("2006-01"),
		"receipts":            receiptsUsage,
		"recipes":             recipesUsage,
		"total_input_tokens":  totalInput,
		"total_output_tokens": totalOutput,
		"estimated_cost_usd":  estimatedCost,
	})
}
