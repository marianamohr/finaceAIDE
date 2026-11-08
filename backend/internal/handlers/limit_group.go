package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type LimitGroupHandler struct {
	groupRepo *repository.LimitGroupRepository
}

func NewLimitGroupHandler(groupRepo *repository.LimitGroupRepository) *LimitGroupHandler {
	return &LimitGroupHandler{groupRepo: groupRepo}
}

type memberInput struct {
	CategoryID   string  `json:"category_id"`
	MonthlyLimit float64 `json:"monthly_limit"`
}

type groupBody struct {
	Label   string        `json:"label"`
	Members []memberInput `json:"members"`
}

// POST /account/limits/groups
func (h *LimitGroupHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var body groupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Label == "" {
		respondError(w, http.StatusBadRequest, "label é obrigatório")
		return
	}
	if len(body.Members) < 2 {
		respondError(w, http.StatusBadRequest, "um grupo precisa de ao menos 2 categorias")
		return
	}

	members, err := parseMemberInputs(body.Members)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	group, err := h.groupRepo.CreateGroup(accountID, body.Label, members)
	if err != nil {
		log.Error().Err(err).Msg("limit_group: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, group)
}

// PUT /account/limits/groups/:group_id
func (h *LimitGroupHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	groupID, err := uuid.Parse(chi.URLParam(r, "group_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "group_id inválido")
		return
	}

	var body groupBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Label == "" {
		respondError(w, http.StatusBadRequest, "label é obrigatório")
		return
	}
	if len(body.Members) < 2 {
		respondError(w, http.StatusBadRequest, "um grupo precisa de ao menos 2 categorias")
		return
	}

	members, err := parseMemberInputs(body.Members)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.groupRepo.UpdateGroup(accountID, groupID, body.Label, members); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "grupo não encontrado")
			return
		}
		log.Error().Err(err).Msg("limit_group: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// DELETE /account/limits/groups/:group_id
func (h *LimitGroupHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	groupID, err := uuid.Parse(chi.URLParam(r, "group_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "group_id inválido")
		return
	}

	if err := h.groupRepo.DeleteGroup(accountID, groupID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "grupo não encontrado")
			return
		}
		log.Error().Err(err).Msg("limit_group: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func parseMemberInputs(inputs []memberInput) ([]repository.GroupMemberInput, error) {
	result := make([]repository.GroupMemberInput, 0, len(inputs))
	for _, m := range inputs {
		catID, err := uuid.Parse(m.CategoryID)
		if err != nil {
			return nil, fmt.Errorf("category_id inválido: %s", m.CategoryID)
		}
		if m.MonthlyLimit <= 0 {
			return nil, fmt.Errorf("monthly_limit deve ser maior que zero")
		}
		result = append(result, repository.GroupMemberInput{CategoryID: catID, MonthlyLimit: m.MonthlyLimit})
	}
	return result, nil
}
