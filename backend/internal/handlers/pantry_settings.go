package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// PantrySettingsHandler exposes the per-user preferences of the dispensa module.
type PantrySettingsHandler struct {
	repo *repository.PantrySettingsRepository
}

func NewPantrySettingsHandler(repo *repository.PantrySettingsRepository) *PantrySettingsHandler {
	return &PantrySettingsHandler{repo: repo}
}

// pantrySettingsRequest is a partial update: nil fields keep the current value.
type pantrySettingsRequest struct {
	Language             *string `json:"language"`
	Country              *string `json:"country"`
	Currency             *string `json:"currency"`
	ShoppingIntervalDays *int    `json:"shopping_interval_days"`
	AskProduceUnits      *bool   `json:"ask_produce_units"`
}

// Get returns the settings, creating the defaults on first call.
func (h *PantrySettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	settings, err := h.repo.Get(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry settings: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

// Update applies a partial update and returns the resulting settings.
func (h *PantrySettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req pantrySettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	settings, err := h.repo.Get(r.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry settings: get before update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	applyPantrySettingsRequest(settings, req)
	if err := validatePantrySettings(settings); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.Save(r.Context(), settings); err != nil {
		log.Error().Err(err).Str("user_id", userID.String()).Msg("pantry settings: save")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, settings)
}

func applyPantrySettingsRequest(settings *models.PantrySettings, req pantrySettingsRequest) {
	if req.Language != nil {
		settings.Language = *req.Language
	}
	if req.Country != nil {
		settings.Country = *req.Country
	}
	if req.Currency != nil {
		settings.Currency = *req.Currency
	}
	if req.ShoppingIntervalDays != nil {
		settings.ShoppingIntervalDays = *req.ShoppingIntervalDays
	}
	if req.AskProduceUnits != nil {
		settings.AskProduceUnits = *req.AskProduceUnits
	}
}

// validatePantrySettings checks the enum-like fields and ranges. Messages are
// shown to the user, so they are in Portuguese like the rest of the API.
func validatePantrySettings(settings *models.PantrySettings) error {
	if settings == nil {
		return errors.New("configurações ausentes")
	}

	switch settings.Language {
	case models.PantryLanguagePtBR, models.PantryLanguageDe:
	default:
		return fmt.Errorf("idioma inválido %q (use pt-BR ou de)", settings.Language)
	}

	switch settings.Country {
	case models.PantryCountryBR, models.PantryCountryDE:
	default:
		return fmt.Errorf("país inválido %q (use BR ou DE)", settings.Country)
	}

	switch settings.Currency {
	case models.PantryCurrencyBRL, models.PantryCurrencyEUR:
	default:
		return fmt.Errorf("moeda inválida %q (use BRL ou EUR)", settings.Currency)
	}

	if settings.ShoppingIntervalDays < 1 || settings.ShoppingIntervalDays > models.PantryMaxShoppingIntervalDays {
		return fmt.Errorf(
			"intervalo de compras deve ficar entre 1 e %d dias",
			models.PantryMaxShoppingIntervalDays,
		)
	}

	return nil
}
