package handlers

import (
	"net/http"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// PantrySeedProductHandler exposes the global, read-only seed catalog —
// used by the frontend's initial-inventory checklist ("Fazer inventário")
// to let a new user quickly mark what they already have at home, instead of
// typing every product by hand. Internally the same catalog already backs
// receipt-item matching (pantry.ReceiptService); this just makes it readable
// over HTTP too.
type PantrySeedProductHandler struct {
	repo *repository.PantrySeedProductRepository
}

func NewPantrySeedProductHandler(repo *repository.PantrySeedProductRepository) *PantrySeedProductHandler {
	return &PantrySeedProductHandler{repo: repo}
}

// List godoc
// GET /pantry/seed-products
func (h *PantrySeedProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.repo.ListAll(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("pantry seed product: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"seed_products": products})
}
