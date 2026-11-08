package handlers

import (
	"net/http"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type PaymentModeHandler struct {
	repo *repository.PaymentModeRepository
}

func NewPaymentModeHandler(repo *repository.PaymentModeRepository) *PaymentModeHandler {
	return &PaymentModeHandler{repo: repo}
}

func (h *PaymentModeHandler) List(w http.ResponseWriter, r *http.Request) {
	modes, err := h.repo.ListAll()
	if err != nil {
		log.Error().Err(err).Msg("payment-modes: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, modes)
}
