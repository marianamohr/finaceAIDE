package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// UserHandler handles user profile and settings endpoints.
type UserHandler struct {
	repo *repository.UserRepository
}

func NewUserHandler(repo *repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// maskAPIKey returns a masked version of the key so the frontend can
// tell "configured / not configured" without receiving the full secret.
// Returns "" if the key is empty, or "sk-ant-****...****" otherwise,
// keeping the first 10 and last 4 characters visible.
func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	const visible = 10
	const tail = 4
	if len(key) <= visible+tail {
		return "****"
	}
	return key[:visible] + "****..." + key[len(key)-tail:]
}

// Me godoc
// GET /user/me
// Returns the authenticated user's profile including settings (category_context, show_goals, etc.).
func (h *UserHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.repo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("user: me")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type meResponse struct {
		ID                   string  `json:"id"`
		Email                string  `json:"email"`
		Role                 string  `json:"role"`
		Status               string  `json:"status"`
		AccountID            string  `json:"account_id"`
		CategoryContext      string  `json:"category_context"`
		ShowGoals            bool    `json:"show_goals"`
		UserAPIKey           string  `json:"user_api_key"`
		ShowLimitsCategories bool    `json:"show_limits_categories"`
		ShowTools            bool    `json:"show_tools"`
		BaseCurrency         string  `json:"base_currency"`
		DistributionMethod   string  `json:"distribution_method"`
		FuturePct            float64 `json:"future_pct"`
		OnboardingSeen       bool    `json:"onboarding_seen"`
	}

	respondJSON(w, http.StatusOK, meResponse{
		ID:                   user.ID.String(),
		Email:                user.Email,
		Role:                 user.Role,
		Status:               user.Status,
		AccountID:            user.AccountID.String(),
		CategoryContext:      user.CategoryContext,
		ShowGoals:            user.ShowGoals,
		UserAPIKey:           maskAPIKey(user.UserAPIKey),
		ShowLimitsCategories: user.ShowLimitsCategories,
		ShowTools:            user.ShowTools,
		BaseCurrency:         user.BaseCurrency,
		DistributionMethod:   user.DistributionMethod,
		FuturePct:            user.FuturePct,
		OnboardingSeen:       user.OnboardingSeen,
	})
}

// UpdateSettings godoc
// PATCH /user/settings
// Body: any subset of { category_context, show_goals, user_api_key, show_limits_categories,
//
//	base_currency, distribution_method, future_pct }
func (h *UserHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	// Load current settings first so we can do a partial update (PATCH semantics).
	current, err := h.repo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("user: update settings fetch current")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req struct {
		CategoryContext      *string  `json:"category_context"`
		ShowGoals            *bool    `json:"show_goals"`
		UserAPIKey           *string  `json:"user_api_key"`
		ShowLimitsCategories *bool    `json:"show_limits_categories"`
		ShowTools            *bool    `json:"show_tools"`
		BaseCurrency         *string  `json:"base_currency"`
		DistributionMethod   *string  `json:"distribution_method"`
		FuturePct            *float64 `json:"future_pct"`
		OnboardingSeen       *bool    `json:"onboarding_seen"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Merge patch onto current values.
	updated := models.UserSettings{
		CategoryContext:      current.CategoryContext,
		ShowGoals:            current.ShowGoals,
		UserAPIKey:           current.UserAPIKey,
		ShowLimitsCategories: current.ShowLimitsCategories,
		ShowTools:            current.ShowTools,
		BaseCurrency:         current.BaseCurrency,
		DistributionMethod:   current.DistributionMethod,
		FuturePct:            current.FuturePct,
	}
	if req.CategoryContext != nil {
		if *req.CategoryContext != "standard" && *req.CategoryContext != "food_detail" {
			respondError(w, http.StatusBadRequest, "category_context must be 'standard' or 'food_detail'")
			return
		}
		updated.CategoryContext = *req.CategoryContext
	}
	if req.ShowGoals != nil {
		if err := h.repo.UpdateShowGoals(userID, *req.ShowGoals); err != nil {
			log.Error().Err(err).Msg("user: update show_goals")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		updated.ShowGoals = *req.ShowGoals
	}
	if req.UserAPIKey != nil {
		updated.UserAPIKey = *req.UserAPIKey
	}
	if req.ShowLimitsCategories != nil {
		updated.ShowLimitsCategories = *req.ShowLimitsCategories
	}
	if req.ShowTools != nil {
		updated.ShowTools = *req.ShowTools
	}
	if req.OnboardingSeen != nil {
		updated.OnboardingSeen = *req.OnboardingSeen
	}
	if req.BaseCurrency != nil {
		updated.BaseCurrency = *req.BaseCurrency
	}
	if req.DistributionMethod != nil {
		if *req.DistributionMethod != "70-30" && *req.DistributionMethod != "60-20-20" {
			respondError(w, http.StatusBadRequest, "distribution_method must be '70-30' or '60-20-20'")
			return
		}
		updated.DistributionMethod = *req.DistributionMethod
	}
	if req.FuturePct != nil {
		updated.FuturePct = *req.FuturePct
	}

	if err := h.repo.UpdateSettings(userID, updated); err != nil {
		log.Error().Err(err).Msg("user: update settings")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	type settingsResponse struct {
		CategoryContext      string  `json:"category_context"`
		ShowGoals            bool    `json:"show_goals"`
		UserAPIKey           string  `json:"user_api_key"`
		ShowLimitsCategories bool    `json:"show_limits_categories"`
		ShowTools            bool    `json:"show_tools"`
		BaseCurrency         string  `json:"base_currency"`
		DistributionMethod   string  `json:"distribution_method"`
		FuturePct            float64 `json:"future_pct"`
		OnboardingSeen       bool    `json:"onboarding_seen"`
	}

	respondJSON(w, http.StatusOK, settingsResponse{
		CategoryContext:      updated.CategoryContext,
		ShowGoals:            updated.ShowGoals,
		UserAPIKey:           maskAPIKey(updated.UserAPIKey),
		ShowLimitsCategories: updated.ShowLimitsCategories,
		ShowTools:            updated.ShowTools,
		BaseCurrency:         updated.BaseCurrency,
		DistributionMethod:   updated.DistributionMethod,
		FuturePct:            updated.FuturePct,
		OnboardingSeen:       updated.OnboardingSeen,
	})
}

// ChangePassword godoc
// POST /user/password
// Body: { "current_password": string, "new_password": string }
// Returns 204 No Content on success.
func (h *UserHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	user, err := h.repo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("user: change password fetch")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.NewPassword) < 8 {
		respondError(w, http.StatusBadRequest, "nova senha deve ter no mínimo 8 caracteres")
		return
	}
	if len(req.NewPassword) > 72 {
		respondError(w, http.StatusBadRequest, "nova senha deve ter no máximo 72 caracteres")
		return
	}

	if err := auth.CheckPassword(user.PasswordHash, req.CurrentPassword); err != nil {
		respondError(w, http.StatusBadRequest, "senha atual incorreta")
		return
	}

	if req.NewPassword == req.CurrentPassword {
		respondError(w, http.StatusBadRequest, "nova senha deve ser diferente da senha atual")
		return
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		log.Error().Err(err).Msg("user: change password hash")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.repo.UpdatePassword(userID, newHash); err != nil {
		log.Error().Err(err).Msg("user: change password update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
