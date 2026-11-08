package currency

import (
	"encoding/json"
	"net/http"

	"github.com/marianamohr/financas/internal/auth"
)

// Handler exposes currency preference and exchange rate endpoints.
type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetCurrency handles GET /settings/currency
func (h *Handler) GetCurrency(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	base, err := h.service.GetUserBaseCurrency(r.Context(), userID.String())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]string{"base_currency": base})
}

// SetCurrency handles PUT /settings/currency
func (h *Handler) SetCurrency(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var body struct {
		BaseCurrency string `json:"base_currency"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.BaseCurrency != "BRL" && body.BaseCurrency != "EUR" {
		respondError(w, http.StatusBadRequest, "base_currency must be BRL or EUR")
		return
	}

	if err := h.service.SetUserBaseCurrency(r.Context(), userID.String(), body.BaseCurrency); err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetExchangeRate handles GET /exchange-rate?from=BRL&to=EUR
func (h *Handler) GetExchangeRate(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")

	if from == "" || to == "" {
		respondError(w, http.StatusBadRequest, "query params 'from' and 'to' are required")
		return
	}

	rate, err := h.service.GetRate(r.Context(), from, to)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not fetch exchange rate")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"from": from,
		"to":   to,
		"rate": rate,
	})
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func respondError(w http.ResponseWriter, status int, msg string) {
	respondJSON(w, status, map[string]string{"error": msg})
}
