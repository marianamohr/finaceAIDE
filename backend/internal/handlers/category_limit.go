package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type CategoryLimitHandler struct {
	limitRepo *repository.CategoryLimitRepository
	groupRepo *repository.LimitGroupRepository
	txRepo    *repository.TransactionRepository
	catRepo   *repository.CategoryRepository
}

func NewCategoryLimitHandler(
	limitRepo *repository.CategoryLimitRepository,
	groupRepo *repository.LimitGroupRepository,
	txRepo    *repository.TransactionRepository,
	catRepo   *repository.CategoryRepository,
) *CategoryLimitHandler {
	return &CategoryLimitHandler{limitRepo: limitRepo, groupRepo: groupRepo, txRepo: txRepo, catRepo: catRepo}
}

// GET /account/limits
func (h *CategoryLimitHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	individualLimits, err := h.limitRepo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("category_limit: list individual")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	groups, err := h.groupRepo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("category_limit: list groups")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	now := time.Now()
	summary, err := h.txRepo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		log.Error().Err(err).Msg("category_limit: summary")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Collect all category IDs to look up slugs in one query
	var catIDs []uuid.UUID
	for _, l := range individualLimits {
		catIDs = append(catIDs, l.CategoryID)
	}

	// Fetch group member limits
	groupMembersMap := make(map[uuid.UUID][]models.CategoryLimit, len(groups))
	for _, g := range groups {
		members, err := h.limitRepo.ListByGroupID(g.ID)
		if err != nil {
			log.Error().Err(err).Msg("category_limit: list group members")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		groupMembersMap[g.ID] = members
		for _, m := range members {
			catIDs = append(catIDs, m.CategoryID)
		}
	}

	slugByID := make(map[uuid.UUID]string, len(catIDs))
	if len(catIDs) > 0 {
		cats, err := h.catRepo.FindByIDs(catIDs)
		if err != nil {
			log.Error().Err(err).Msg("category_limit: find categories")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		for _, c := range cats {
			slugByID[c.ID] = c.Slug
		}
	}

	var spentMap map[string]float64
	if summary != nil {
		spentMap = summary.Categories
	}

	// Build individual results
	individual := make([]models.LimitWithProgress, 0, len(individualLimits))
	for _, l := range individualLimits {
		slug := slugByID[l.CategoryID]
		spent := math.Abs(spentMap[slug])
		pct := 0.0
		if l.MonthlyLimit > 0 {
			pct = spent / l.MonthlyLimit * 100
		}
		individual = append(individual, models.LimitWithProgress{
			ID:           l.ID,
			CategoryID:   l.CategoryID,
			CategorySlug: slug,
			MonthlyLimit: l.MonthlyLimit,
			Spent:        spent,
			Pct:          pct,
		})
	}

	// Build group results
	groupResults := make([]models.GroupWithProgress, 0, len(groups))
	for _, g := range groups {
		members := groupMembersMap[g.ID]
		memberProgress := make([]models.LimitWithProgress, 0, len(members))
		var totalLimit, totalSpent float64
		for _, m := range members {
			slug := slugByID[m.CategoryID]
			spent := math.Abs(spentMap[slug])
			pct := 0.0
			if m.MonthlyLimit > 0 {
				pct = spent / m.MonthlyLimit * 100
			}
			memberProgress = append(memberProgress, models.LimitWithProgress{
				ID:           m.ID,
				CategoryID:   m.CategoryID,
				CategorySlug: slug,
				MonthlyLimit: m.MonthlyLimit,
				Spent:        spent,
				Pct:          pct,
			})
			totalLimit += m.MonthlyLimit
			totalSpent += spent
		}
		groupPct := 0.0
		if totalLimit > 0 {
			groupPct = totalSpent / totalLimit * 100
		}
		groupResults = append(groupResults, models.GroupWithProgress{
			ID:           g.ID,
			Label:        g.Label,
			MonthlyLimit: totalLimit,
			Spent:        totalSpent,
			Pct:          groupPct,
			Members:      memberProgress,
		})
	}

	respondJSON(w, http.StatusOK, models.LimitsResponse{
		Individual: individual,
		Groups:     groupResults,
	})
}

// PUT /account/limits/:category_id
func (h *CategoryLimitHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(chi.URLParam(r, "category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	var body struct {
		MonthlyLimit float64 `json:"monthly_limit"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.MonthlyLimit <= 0 {
		respondError(w, http.StatusBadRequest, "monthly_limit deve ser maior que zero")
		return
	}

	limit := &models.CategoryLimit{
		AccountID:    accountID,
		CategoryID:   categoryID,
		MonthlyLimit: body.MonthlyLimit,
	}
	if err := h.limitRepo.Upsert(limit); err != nil {
		log.Error().Err(err).Msg("category_limit: upsert")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, limit)
}

// DELETE /account/limits/:category_id
func (h *CategoryLimitHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(chi.URLParam(r, "category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	if err := h.limitRepo.Delete(accountID, categoryID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "limite não encontrado")
			return
		}
		log.Error().Err(err).Msg("category_limit: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /account/limits/check?category_id=X&amount=Y
func (h *CategoryLimitHandler) Check(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	categoryID, err := uuid.Parse(r.URL.Query().Get("category_id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "category_id inválido")
		return
	}

	var amount float64
	if _, err := fmt.Sscanf(r.URL.Query().Get("amount"), "%f", &amount); err != nil || amount <= 0 {
		respondError(w, http.StatusBadRequest, "amount inválido")
		return
	}

	limits, err := h.limitRepo.ListByAccount(accountID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var matchedLimit *models.CategoryLimit
	for _, l := range limits {
		if l.CategoryID == categoryID {
			cl := l
			matchedLimit = &cl
			break
		}
	}

	if matchedLimit == nil {
		respondJSON(w, http.StatusOK, map[string]any{"exceeds": false, "current": 0, "limit": 0})
		return
	}

	cats, err := h.catRepo.FindByIDs([]uuid.UUID{categoryID})
	if err != nil || len(cats) == 0 {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	slug := cats[0].Slug

	now := time.Now()
	summary, err := h.txRepo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var current float64
	if summary != nil {
		current = math.Abs(summary.Categories[slug])
	}
	exceeds := (current + amount) > matchedLimit.MonthlyLimit

	respondJSON(w, http.StatusOK, map[string]any{
		"exceeds": exceeds,
		"current": current,
		"limit":   matchedLimit.MonthlyLimit,
	})
}
