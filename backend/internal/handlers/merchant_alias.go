package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type MerchantAliasHandler struct {
	repo *repository.MerchantAliasRepository
}

func NewMerchantAliasHandler(repo *repository.MerchantAliasRepository) *MerchantAliasHandler {
	return &MerchantAliasHandler{repo: repo}
}

// List godoc
// GET /merchant-aliases
func (h *MerchantAliasHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	aliases, err := h.repo.List(accountID)
	if err != nil {
		log.Error().Err(err).Msg("merchant_alias: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, aliases)
}

type merchantAliasRequest struct {
	RawName     string `json:"raw_name"`
	DisplayName string `json:"display_name"`
}

// Create godoc
// POST /merchant-aliases
// Body: { "raw_name": "PETZ COMÉRCIO LTDA", "display_name": "Petz" }
func (h *MerchantAliasHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var req merchantAliasRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RawName == "" || req.DisplayName == "" {
		respondError(w, http.StatusBadRequest, "raw_name and display_name are required")
		return
	}

	alias := &models.MerchantAlias{
		AccountID:   accountID,
		RawName:     req.RawName,
		DisplayName: req.DisplayName,
	}
	if err := h.repo.Create(alias); err != nil {
		if repository.IsUniqueViolation(err) {
			respondError(w, http.StatusConflict, "alias already exists for this merchant")
			return
		}
		log.Error().Err(err).Msg("merchant_alias: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusCreated, alias)
}

// Update godoc
// PUT /merchant-aliases/{id}
// Body: { "display_name": "Novo nome" }
func (h *MerchantAliasHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid alias id")
		return
	}

	var req struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DisplayName == "" {
		respondError(w, http.StatusBadRequest, "display_name is required")
		return
	}

	if err := h.repo.Update(id, accountID, req.DisplayName); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "alias not found")
			return
		}
		log.Error().Err(err).Msg("merchant_alias: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"id":           id,
		"display_name": req.DisplayName,
	})
}

// ApplyToTransactions godoc
// POST /merchant-aliases/apply
// Updates merchant field on all existing transactions using the current alias dictionary.
func (h *MerchantAliasHandler) ApplyToTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	result := h.repo.ApplyToTransactions(accountID)
	respondJSON(w, http.StatusOK, map[string]any{"updated": result})
}

// PendingCount godoc
// GET /merchant-aliases/pending-count
// Returns the count of aliases not yet reviewed by the user (is_custom=false).
func (h *MerchantAliasHandler) PendingCount(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	count, err := h.repo.CountPending(accountID)
	if err != nil {
		log.Error().Err(err).Msg("merchant_alias: pending count")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"count": count})
}

// Delete godoc
// DELETE /merchant-aliases/{id}
func (h *MerchantAliasHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid alias id")
		return
	}

	if err := h.repo.Delete(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "alias not found")
			return
		}
		log.Error().Err(err).Msg("merchant_alias: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
