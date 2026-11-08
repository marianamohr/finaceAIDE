package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/rs/zerolog/log"
)

type RecurringExpenseHandler struct {
	repo      *repository.RecurringExpenseRepository
	txRepo    *repository.TransactionRepository
	catRepo   *repository.CategoryRepository
	pmRepo    *repository.PaymentMethodRepository
	validator *transactions.Validator
}

func NewRecurringExpenseHandler(
	repo *repository.RecurringExpenseRepository,
	txRepo *repository.TransactionRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
	validator *transactions.Validator,
) *RecurringExpenseHandler {
	return &RecurringExpenseHandler{
		repo:      repo,
		txRepo:    txRepo,
		catRepo:   catRepo,
		pmRepo:    pmRepo,
		validator: validator,
	}
}

// GET /recurring-expenses
func (h *RecurringExpenseHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	items, err := h.repo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("recurring expense: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, items)
}

// resolveStart returns the start year/month to persist for a recurring expense.
// When the caller omits either field, it defaults to the current month so the
// expense applies from now on and never backfills past (already-closed) invoices.
func resolveStart(startYear, startMonth *int, now time.Time) (year, month *int) {
	if startYear != nil && startMonth != nil {
		return startYear, startMonth
	}
	y, m := now.Year(), int(now.Month())
	return &y, &m
}

// POST /recurring-expenses
func (h *RecurringExpenseHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var body struct {
		Name            string     `json:"name"`
		Amount          float64    `json:"amount"`
		CategoryID      *uuid.UUID `json:"category_id"`
		PaymentMethodID *uuid.UUID `json:"payment_method_id"`
		DueDay          *int       `json:"due_day"`
		StartMonth      *int       `json:"start_month"`
		StartYear       *int       `json:"start_year"`
		EndMonth        *int       `json:"end_month"`
		EndYear         *int       `json:"end_year"`
		IsVariable      bool       `json:"is_variable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Name == "" {
		respondError(w, http.StatusBadRequest, "name é obrigatório")
		return
	}
	if body.Amount <= 0 {
		respondError(w, http.StatusBadRequest, "amount deve ser maior que zero")
		return
	}
	if body.DueDay != nil && (*body.DueDay < 1 || *body.DueDay > 28) {
		respondError(w, http.StatusBadRequest, "due_day deve estar entre 1 e 28")
		return
	}
	if err := validateDateRange(body.StartYear, body.StartMonth, body.EndYear, body.EndMonth); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Sem início explícito, a recorrência passa a valer a partir do mês atual,
	// evitando lançamentos retroativos em meses (e faturas) já fechados.
	startYear, startMonth := resolveStart(body.StartYear, body.StartMonth, time.Now().UTC())

	item := &models.RecurringExpense{
		AccountID:       accountID,
		Name:            body.Name,
		Amount:          body.Amount,
		CategoryID:      body.CategoryID,
		PaymentMethodID: body.PaymentMethodID,
		DueDay:          body.DueDay,
		StartMonth:      startMonth,
		StartYear:       startYear,
		EndMonth:        body.EndMonth,
		EndYear:         body.EndYear,
		IsVariable:      body.IsVariable,
		Active:          true,
	}
	if err := h.repo.Create(item); err != nil {
		log.Error().Err(err).Msg("recurring expense: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, item)
}

// PUT /recurring-expenses/{id}
func (h *RecurringExpenseHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var body struct {
		Name            string     `json:"name"`
		Amount          float64    `json:"amount"`
		CategoryID      *uuid.UUID `json:"category_id"`
		PaymentMethodID *uuid.UUID `json:"payment_method_id"`
		DueDay          *int       `json:"due_day"`
		Active          *bool      `json:"active"`
		StartMonth      *int       `json:"start_month"`
		StartYear       *int       `json:"start_year"`
		EndMonth        *int       `json:"end_month"`
		EndYear         *int       `json:"end_year"`
		IsVariable      *bool      `json:"is_variable"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Amount < 0 {
		respondError(w, http.StatusBadRequest, "amount deve ser maior que zero")
		return
	}
	if body.DueDay != nil && (*body.DueDay < 1 || *body.DueDay > 28) {
		respondError(w, http.StatusBadRequest, "due_day deve estar entre 1 e 28")
		return
	}
	if err := validateDateRange(body.StartYear, body.StartMonth, body.EndYear, body.EndMonth); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	existing, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gasto fixo não encontrado")
			return
		}
		log.Error().Err(err).Msg("recurring expense: find for update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if body.Name != "" {
		existing.Name = body.Name
	}
	if body.Amount > 0 {
		existing.Amount = body.Amount
	}
	if body.CategoryID != nil {
		existing.CategoryID = body.CategoryID
	}
	if body.PaymentMethodID != nil {
		existing.PaymentMethodID = body.PaymentMethodID
	}
	existing.DueDay = body.DueDay
	existing.StartMonth = body.StartMonth
	existing.StartYear = body.StartYear
	existing.EndMonth = body.EndMonth
	existing.EndYear = body.EndYear
	if body.Active != nil {
		existing.Active = *body.Active
	}
	if body.IsVariable != nil {
		existing.IsVariable = *body.IsVariable
	}

	if err := h.repo.Update(existing, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gasto fixo não encontrado")
			return
		}
		log.Error().Err(err).Msg("recurring expense: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, existing)
}

// DELETE /recurring-expenses/{id}
func (h *RecurringExpenseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}

	if err := h.repo.Delete(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gasto fixo não encontrado")
			return
		}
		log.Error().Err(err).Msg("recurring expense: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /recurring-expenses/reminders?year=&month=
func (h *RecurringExpenseHandler) Reminders(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if y := r.URL.Query().Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := r.URL.Query().Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			month = v
		}
	}

	items, err := h.repo.ListActiveForMonth(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("recurring expense: reminders list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Assinaturas de cartão são cobradas via fatura — não aparecem como pendentes.
	billOnly := items[:0]
	for _, it := range items {
		if it.PaymentMethodID == nil {
			billOnly = append(billOnly, it)
		}
	}
	items = billOnly

	paidMap, err := h.repo.FindPaidInMonth(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("recurring expense: reminders paid lookup")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Batch-fetch category slugs for all items
	catIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item.CategoryID != nil {
			catIDs = append(catIDs, *item.CategoryID)
		}
	}
	slugByID := make(map[uuid.UUID]string, len(catIDs))
	if len(catIDs) > 0 {
		cats, err := h.catRepo.FindByIDs(catIDs)
		if err != nil {
			log.Error().Err(err).Msg("recurring expense: reminders category lookup")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		for _, c := range cats {
			slugByID[c.ID] = c.Slug
		}
	}

	result := make([]models.RecurringExpenseWithStatus, 0, len(items))
	for _, item := range items {
		status := models.RecurringExpenseWithStatus{
			ID:         item.ID,
			Name:       item.Name,
			Amount:     item.Amount,
			CategoryID: item.CategoryID,
			DueDay:     item.DueDay,
			StartMonth: item.StartMonth,
			StartYear:  item.StartYear,
			EndMonth:   item.EndMonth,
			EndYear:    item.EndYear,
			IsVariable: item.IsVariable,
		}
		if item.CategoryID != nil {
			status.CategorySlug = slugByID[*item.CategoryID]
		}
		if tx, paid := paidMap[item.ID]; paid {
			status.Paid = true
			status.TransactionID = &tx.ID
			paidAmount := math.Abs(tx.Amount)
			status.PaidAmount = &paidAmount
		}
		result = append(result, status)
	}

	// Sort: unpaid first, then by DueDay asc (nil last), then alphabetical
	sort.Slice(result, func(i, j int) bool {
		if result[i].Paid != result[j].Paid {
			return !result[i].Paid
		}
		di := result[i].DueDay
		dj := result[j].DueDay
		if di == nil && dj == nil {
			return result[i].Name < result[j].Name
		}
		if di == nil {
			return false
		}
		if dj == nil {
			return true
		}
		if *di != *dj {
			return *di < *dj
		}
		return result[i].Name < result[j].Name
	})

	respondJSON(w, http.StatusOK, result)
}

func validateDateRange(startYear, startMonth, endYear, endMonth *int) error {
	if (startYear == nil) != (startMonth == nil) {
		return fmt.Errorf("start_year e start_month devem ser fornecidos juntos")
	}
	if (endYear == nil) != (endMonth == nil) {
		return fmt.Errorf("end_year e end_month devem ser fornecidos juntos")
	}
	if startMonth != nil && (*startMonth < 1 || *startMonth > 12) {
		return fmt.Errorf("start_month deve estar entre 1 e 12")
	}
	if endMonth != nil && (*endMonth < 1 || *endMonth > 12) {
		return fmt.Errorf("end_month deve estar entre 1 e 12")
	}
	if startYear != nil && endYear != nil {
		if *endYear < *startYear || (*endYear == *startYear && *endMonth < *startMonth) {
			return fmt.Errorf("data de fim deve ser igual ou posterior à data de início")
		}
	}
	return nil
}

// POST /recurring-expenses/{id}/pay
func (h *RecurringExpenseHandler) MarkPaid(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var body struct {
		Year            int        `json:"year"`
		Month           int        `json:"month"`
		PaymentMethodID *uuid.UUID `json:"payment_method_id"`
		Amount          *float64   `json:"amount"` // valor real do mês; sobrescreve o estimado
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	if body.Year == 0 || body.Month < 1 || body.Month > 12 {
		respondError(w, http.StatusBadRequest, "year e month são obrigatórios")
		return
	}

	expense, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "gasto fixo não encontrado")
			return
		}
		log.Error().Err(err).Msg("recurring expense: find for pay")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if expense.PaymentMethodID != nil {
		respondError(w, http.StatusBadRequest, "assinaturas de cartão são pagas via fatura do cartão")
		return
	}

	// Variable bills (água, luz) have no meaningful fixed value — the real amount
	// must be informed at pay time.
	if expense.IsVariable && (body.Amount == nil || *body.Amount <= 0) {
		respondError(w, http.StatusBadRequest, "valor é obrigatório para gasto de valor variável")
		return
	}

	// Determine the amount to charge: the informed value, or the registered estimate.
	amount := expense.Amount
	if body.Amount != nil && *body.Amount > 0 {
		amount = *body.Amount
	}

	// Check idempotency — if already paid this month, return the existing transaction
	paidMap, err := h.repo.FindPaidInMonth(accountID, body.Year, body.Month)
	if err != nil {
		log.Error().Err(err).Msg("recurring expense: check paid")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	if tx, alreadyPaid := paidMap[id]; alreadyPaid {
		respondJSON(w, http.StatusOK, tx)
		return
	}

	date := time.Now().UTC()
	tx := &models.Transaction{
		UserID:             userID,
		AccountID:          accountID,
		Amount:             -math.Abs(amount),
		CategoryID:         expense.CategoryID,
		Merchant:           expense.Name,
		Description:        expense.Name,
		Date:               date,
		Source:             "recurring",
		Type:               "regular",
		RecurringExpenseID: &expense.ID,
		PaymentMethodID:    body.PaymentMethodID,
		Locale:             "BR",
		Currency:           "BRL",
	}

	// Populate legacy Category string for backward compatibility
	if expense.CategoryID != nil {
		cats, err := h.catRepo.FindByIDs([]uuid.UUID{*expense.CategoryID})
		if err == nil && len(cats) > 0 {
			tx.Category = cats[0].Slug
		}
	}

	hint := transactions.PaymentHint{PaymentMethodID: tx.PaymentMethodID}
	need, verr := h.validator.ValidateAndEnrich(r.Context(), accountID, tx, hint)
	if verr != nil {
		log.Warn().
			Err(verr).
			Str("recurring_expense_id", expense.ID.String()).
			Msg("recurring expense: validator rejected")
		respondError(w, http.StatusUnprocessableEntity, verr.Error())
		return
	}
	if need != nil {
		log.Warn().
			Str("recurring_expense_id", expense.ID.String()).
			Str("clarification_kind", string(need.Kind)).
			Msg("recurring expense: payment method needs clarification")
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"type":          "needs_clarification",
			"clarification": need,
		})
		return
	}

	if err := h.txRepo.Create(tx); err != nil {
		log.Error().Err(err).Msg("recurring expense: create payment transaction")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, tx)
}

// DELETE /recurring-expenses/{id}/pay?year=&month=
func (h *RecurringExpenseHandler) UnmarkPaid(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}

	var year, month int
	if y, err := strconv.Atoi(r.URL.Query().Get("year")); err == nil {
		year = y
	}
	if m, err := strconv.Atoi(r.URL.Query().Get("month")); err == nil {
		month = m
	}
	if year == 0 || month < 1 || month > 12 {
		respondError(w, http.StatusBadRequest, "year e month são obrigatórios")
		return
	}

	paidMap, err := h.repo.FindPaidInMonth(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("recurring expense: find paid for unmark")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	tx, paid := paidMap[id]
	if !paid {
		respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
		return
	}

	if err := h.txRepo.Delete(tx.ID, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
		log.Error().Err(err).Msg("recurring expense: delete payment transaction")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /recurring-expenses/generate-card-transactions?year=&month=
// Ensures all active card-linked recurring expenses have a transaction for the given month.
// Idempotent — safe to call multiple times.
func (h *RecurringExpenseHandler) GenerateCardTransactions(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	now := time.Now()
	year := now.Year()
	month := int(now.Month())

	if y := r.URL.Query().Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := r.URL.Query().Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			month = v
		}
	}

	if err := h.repo.EnsureCardTransactionsForMonth(accountID, year, month, userID, h.repo.DB()); err != nil {
		log.Error().Err(err).Msg("recurring expense: generate card transactions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
