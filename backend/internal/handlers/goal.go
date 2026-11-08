package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type GoalHandler struct {
	repo        *repository.GoalRepository
	txRepo      *repository.TransactionRepository
	currencySvc *currency.Service
}

func NewGoalHandler(repo *repository.GoalRepository, txRepo *repository.TransactionRepository, currencySvc *currency.Service) *GoalHandler {
	return &GoalHandler{repo: repo, txRepo: txRepo, currencySvc: currencySvc}
}

func (h *GoalHandler) userCurrency(ctx context.Context) string {
	userID := auth.UserIDFromContext(ctx)
	c, err := h.currencySvc.GetUserBaseCurrency(ctx, userID.String())
	if err != nil || c == "" {
		return "BRL"
	}
	return c
}

func currencyToLocale(c string) string {
	switch c {
	case "EUR":
		return "PT"
	case "USD":
		return "US"
	case "GBP":
		return "GB"
	default:
		return "BR"
	}
}

// goalRequest is the shape accepted by Create and Update.
type goalRequest struct {
	Name                string  `json:"name"`
	TargetAmount        float64 `json:"target_amount"`
	InitialAmount       float64 `json:"initial_amount"`
	MonthlyContribution float64 `json:"monthly_contribution"`
	AnnualRate          float64 `json:"annual_rate"`
	AllocationPct       float64 `json:"allocation_pct"`
	GoalType            string  `json:"goal_type"`
	Institution         string  `json:"institution"`
	AccountType         string  `json:"account_type"`
	Deadline            string  `json:"deadline"`
	CoverImageURL       *string `json:"cover_image_url"`
}

// goalTransactionRequest is the shape accepted by AddTransaction.
type goalTransactionRequest struct {
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Source          string  `json:"source"`
	ExpenseCategory string  `json:"expense_category"` // required when source = "withdrawal"
}

// List godoc
// GET /goals
func (h *GoalHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	goals, err := h.repo.List(accountID)
	if err != nil {
		log.Error().Err(err).Msg("goal: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, goals)
}

// Create godoc
// POST /goals
func (h *GoalHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var req goalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.TargetAmount <= 0 {
		respondError(w, http.StatusBadRequest, "target_amount must be positive")
		return
	}

	goalType := req.GoalType
	if goalType == "" {
		goalType = "other"
	}

	validGoalTypes := map[string]bool{
		"emergency_global": true, "travel": true, "car": true,
		"house": true, "emergency": true, "other": true,
	}
	if !validGoalTypes[goalType] {
		respondError(w, http.StatusBadRequest, "invalid goal_type")
		return
	}

	goal := &models.Goal{
		AccountID:           accountID,
		Name:                req.Name,
		TargetAmount:        req.TargetAmount,
		InitialAmount:       req.InitialAmount,
		CurrentAmount:       req.InitialAmount,
		MonthlyContribution: req.MonthlyContribution,
		AnnualRate:          req.AnnualRate,
		AllocationPct:       req.AllocationPct,
		GoalType:            goalType,
		Institution:         req.Institution,
		AccountType:         req.AccountType,
		CoverImageURL:       req.CoverImageURL,
	}
	if req.Deadline != "" {
		d, err := time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			respondError(w, http.StatusBadRequest, "deadline must be YYYY-MM-DD")
			return
		}
		goal.Deadline = &d
	}

	if err := h.repo.Create(goal); err != nil {
		log.Error().Err(err).Msg("goal: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	log.Info().Str("goal_id", goal.ID.String()).Str("name", goal.Name).Msg("goal: created successfully")
	respondJSON(w, http.StatusCreated, goal)
}

// Update godoc
// PUT /goals/{id}
func (h *GoalHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid goal id")
		return
	}

	var req goalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	goal := &models.Goal{
		ID:                  id,
		Name:                req.Name,
		TargetAmount:        req.TargetAmount,
		MonthlyContribution: req.MonthlyContribution,
		AnnualRate:          req.AnnualRate,
		AllocationPct:       req.AllocationPct,
		GoalType:            req.GoalType,
		Institution:         req.Institution,
		AccountType:         req.AccountType,
		CoverImageURL:       req.CoverImageURL,
	}
	if req.Deadline != "" {
		d, err := time.Parse("2006-01-02", req.Deadline)
		if err != nil {
			respondError(w, http.StatusBadRequest, "deadline must be YYYY-MM-DD")
			return
		}
		goal.Deadline = &d
	}

	if req.GoalType != "" {
		validGoalTypes := map[string]bool{
			"emergency_global": true, "travel": true, "car": true,
			"house": true, "emergency": true, "other": true,
		}
		if !validGoalTypes[req.GoalType] {
			respondError(w, http.StatusBadRequest, "invalid goal_type")
			return
		}
	}

	if err := h.repo.Update(goal, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "goal not found")
			return
		}
		log.Error().Err(err).Msg("goal: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, goal)
}

// ListTransactions godoc
// GET /goals/{id}/transactions
func (h *GoalHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	goalID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid goal id")
		return
	}

	txs, err := h.repo.ListTransactions(goalID, accountID)
	if err != nil {
		log.Error().Err(err).Msg("goal: list transactions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, txs)
}

// AddTransaction godoc
// POST /goals/{id}/transactions
// When source = "withdrawal", also creates entry+expense in main account.
func (h *GoalHandler) AddTransaction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := auth.AccountIDFromContext(ctx)
	userID := auth.UserIDFromContext(ctx)

	goalID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid goal id")
		return
	}

	var req goalTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Amount == 0 {
		respondError(w, http.StatusBadRequest, "amount must be non-zero")
		return
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	txDate := time.Now().UTC()
	if req.Date != "" {
		parsed, err := time.Parse("2006-01-02", req.Date)
		if err != nil {
			respondError(w, http.StatusBadRequest, "date must be YYYY-MM-DD")
			return
		}
		txDate = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 12, 0, 0, 0, time.UTC)
	}

	// Fetch goal name for auto-generated descriptions
	goals, err := h.repo.List(accountID)
	if err != nil {
		log.Error().Err(err).Msg("goal: add transaction: fetch goal")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	goalName := ""
	for _, g := range goals {
		if g.ID == goalID {
			goalName = g.Name
			break
		}
	}
	if goalName == "" {
		respondError(w, http.StatusNotFound, "goal not found")
		return
	}

	gtx := &models.GoalTransaction{
		GoalID:      goalID,
		AccountID:   accountID,
		Amount:      req.Amount,
		Description: req.Description,
		Date:        txDate,
		Source:      source,
	}

	if source == "withdrawal" {
		if req.Amount >= 0 {
			respondError(w, http.StatusBadRequest, "withdrawal amount must be negative")
			return
		}
		expCat := req.ExpenseCategory
		if expCat == "" {
			expCat = models.CategoryOutros
		}
		absAmt := -req.Amount

		entryDesc := fmt.Sprintf("Saque reserva %s", goalName)
		if req.Description != "" {
			entryDesc = fmt.Sprintf("Saque reserva %s — %s", goalName, req.Description)
		}
		cur := h.userCurrency(ctx)
		loc := currencyToLocale(cur)
		entry := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         absAmt,
			Category:       models.CategoryReserva,
			Description:    entryDesc,
			Date:           txDate,
			Source:         "reserve",
			Type:           "transfer",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: absAmt,
		}
		expense := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         -absAmt,
			Category:       expCat,
			Description:    entryDesc,
			Date:           txDate,
			Source:         "reserve",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: absAmt,
		}

		if err := h.repo.WithdrawAndRecord(gtx, entry, expense); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "goal not found")
				return
			}
			log.Error().Err(err).Msg("goal: withdrawal")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else if source == "manual" || source == "chat" {
		if req.Amount <= 0 {
			respondError(w, http.StatusBadRequest, "deposit amount must be positive")
			return
		}
		expCat := req.ExpenseCategory
		if expCat == "" {
			expCat = models.CategoryReserva
		}
		expenseDesc := fmt.Sprintf("Aporte reserva %s", goalName)
		if req.Description != "" {
			expenseDesc = fmt.Sprintf("Aporte reserva %s — %s", goalName, req.Description)
		}
		cur := h.userCurrency(ctx)
		loc := currencyToLocale(cur)
		expense := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         -req.Amount,
			Category:       expCat,
			Description:    expenseDesc,
			Date:           txDate,
			Source:         "reserve",
			Type:           "transfer",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: req.Amount,
		}
		if err := h.repo.DepositAndRecord(gtx, expense); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "goal not found")
				return
			}
			log.Error().Err(err).Msg("goal: deposit")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else {
		if err := h.repo.AddTransaction(gtx); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "goal not found")
				return
			}
			log.Error().Err(err).Msg("goal: add transaction")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	respondJSON(w, http.StatusCreated, gtx)
}

// MonthlyHistory godoc
// GET /goals/monthly-history?months=12
func (h *GoalHandler) MonthlyHistory(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	months := 12
	if m := r.URL.Query().Get("months"); m != "" {
		if v, err := fmt.Sscanf(m, "%d", &months); err != nil || v == 0 || months < 1 || months > 60 {
			respondError(w, http.StatusBadRequest, "invalid months")
			return
		}
	}

	points, err := h.repo.GoalMonthlyHistory(accountID, months)
	if err != nil {
		log.Error().Err(err).Msg("goal: monthly history")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, points)
}

// Suggestions godoc
// GET /goals/suggestions?year=2026&month=5
func (h *GoalHandler) Suggestions(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var year, month int
	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")

	now := time.Now()
	year = now.Year()
	month = int(now.Month())

	if yearStr != "" {
		if _, err := fmt.Sscanf(yearStr, "%d", &year); err != nil {
			respondError(w, http.StatusBadRequest, "invalid year")
			return
		}
	}
	if monthStr != "" {
		if _, err := fmt.Sscanf(monthStr, "%d", &month); err != nil || month < 1 || month > 12 {
			respondError(w, http.StatusBadRequest, "invalid month")
			return
		}
	}

	suggestions, err := h.repo.SuggestedContributions(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("goal: suggestions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, suggestions)
}
