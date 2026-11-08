package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/rs/zerolog/log"
)

// TransactionHandler handles CRUD operations for transactions.
// Every method extracts accountID from the JWT context — never from the request body.
type TransactionHandler struct {
	repo      *repository.TransactionRepository
	catRepo   *repository.CategoryRepository
	pmRepo    *repository.PaymentMethodRepository
	validator *transactions.Validator
}

func NewTransactionHandler(
	repo *repository.TransactionRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
	validator *transactions.Validator,
) *TransactionHandler {
	return &TransactionHandler{
		repo:      repo,
		catRepo:   catRepo,
		pmRepo:    pmRepo,
		validator: validator,
	}
}

// transactionRequest is the shape accepted by Create and Update.
type transactionRequest struct {
	Amount        float64    `json:"amount"`
	Category      string     `json:"category"`
	Merchant      string     `json:"merchant"`
	Description   string     `json:"description"`
	Date          string     `json:"date"` // ISO 8601: YYYY-MM-DD
	Source        string     `json:"source"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id"`
}

func (req *transactionRequest) validate() string {
	if req.Amount == 0 {
		return "amount is required and must be non-zero"
	}
	if req.Category == "" {
		return "category is required"
	}
	if req.Date == "" {
		return "date is required (YYYY-MM-DD)"
	}
	return ""
}

// List godoc
// GET /transactions?year=2026&month=4
// Returns transactions for the authenticated account. If year+month are provided,
// filters to that month only; otherwise returns all transactions ordered by date desc.
func (h *TransactionHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var txs []models.Transaction
	var err error

	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	if yearStr != "" && monthStr != "" {
		var year, month int
		if _, e1 := fmt.Sscanf(yearStr, "%d", &year); e1 != nil {
			respondError(w, http.StatusBadRequest, "invalid year")
			return
		}
		if _, e2 := fmt.Sscanf(monthStr, "%d", &month); e2 != nil || month < 1 || month > 12 {
			respondError(w, http.StatusBadRequest, "invalid month")
			return
		}
		txs, err = h.repo.ListByAccountMonth(accountID, year, month)
	} else {
		txs, err = h.repo.ListByAccount(accountID)
	}

	if err != nil {
		log.Error().Err(err).Msg("transaction: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, txs)
}

// Create godoc
// POST /transactions
// Body: { "amount", "category", "description", "date", "source", "payment_method_id" }
// Response 201: created transaction
// Response 422: validation error or needs_clarification (payment method not resolved)
func (h *TransactionHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	accountID := auth.AccountIDFromContext(r.Context())

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondError(w, http.StatusBadRequest, "date must be in YYYY-MM-DD format")
		return
	}

	source := req.Source
	if source == "" {
		source = "manual"
	}

	txType := "expense"
	if req.Amount > 0 {
		txType = "income"
	}

	tx := &models.Transaction{
		UserID:      userID,
		AccountID:   accountID,
		Amount:      req.Amount,
		Category:    req.Category,
		Merchant:    req.Merchant,
		Description: req.Description,
		Date:        date,
		Source:      source,
		Type:        txType,
	}

	hint := transactions.PaymentHint{}
	if req.PaymentMethodID != nil {
		hint.PaymentMethodID = req.PaymentMethodID
	}

	need, err := h.validator.ValidateAndEnrich(r.Context(), accountID, tx, hint)
	if err != nil {
		respondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if need != nil {
		respondJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"type":          "needs_clarification",
			"clarification": need,
		})
		return
	}

	if tx.PaymentMethodID != nil {
		if pm, err := h.pmRepo.FindByID(*tx.PaymentMethodID, accountID); err == nil && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			y, m := models.ComputeBillingMonth(date, *pm.ClosingDay)
			my := int(m)
			tx.BillingYear = &y
			tx.BillingMonth = &my
		}
	}

	if err := h.repo.Create(tx); err != nil {
		log.Error().Err(err).Msg("transaction: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusCreated, tx)
}

// Get godoc
// GET /transactions/{id}
// Returns a single transaction — only if it belongs to the authenticated account.
func (h *TransactionHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	tx, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "transaction not found")
			return
		}
		log.Error().Err(err).Msg("transaction: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, tx)
}

// Update godoc
// PUT /transactions/{id}
// Updates a transaction — only if it belongs to the authenticated account.
func (h *TransactionHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	var req transactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg := req.validate(); msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		respondError(w, http.StatusBadRequest, "date must be in YYYY-MM-DD format")
		return
	}

	cat, err := h.catRepo.FindBySlug(req.Category)
	if err != nil {
		respondError(w, http.StatusBadRequest, "categoria inválida: "+req.Category)
		return
	}
	if !cat.IsCompatibleWith(req.Amount) {
		respondError(w, http.StatusBadRequest, "categoria incompatível com o sinal do valor")
		return
	}

	// Load the existing transaction and apply only the user-editable fields. The
	// repository persists with Select("*"), so building a fresh struct here would
	// wipe every system-managed column the edit form never sends —
	// recurring_expense_id (unlinking gastos fixos), type, installment_*, voucher,
	// image_url, created_at and the currency fields.
	existing, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "transaction not found")
			return
		}
		log.Error().Err(err).Msg("transaction: update find")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	existing.Amount = req.Amount
	existing.Category = cat.Slug
	existing.CategoryID = &cat.ID
	existing.Merchant = req.Merchant
	existing.Description = req.Description
	existing.Date = date
	existing.PaymentMethodID = req.PaymentMethodID
	if req.Source != "" {
		existing.Source = req.Source
	}

	// Billing month is derived solely from the payment method — recompute it from
	// scratch so switching to a non-card method (or none) clears it.
	existing.BillingYear = nil
	existing.BillingMonth = nil
	if req.PaymentMethodID != nil {
		if pm, err := h.pmRepo.FindByID(*req.PaymentMethodID, accountID); err == nil && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			y, m := models.ComputeBillingMonth(date, *pm.ClosingDay)
			my := int(m)
			existing.BillingYear = &y
			existing.BillingMonth = &my
		}
	}

	if err := h.repo.Update(existing, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "transaction not found")
			return
		}
		log.Error().Err(err).Msg("transaction: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// Delete godoc
// DELETE /transactions/{id}
// Deletes a transaction — only if it belongs to the authenticated account.
func (h *TransactionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid transaction id")
		return
	}

	if err := h.repo.Delete(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "transaction not found")
			return
		}
		log.Error().Err(err).Msg("transaction: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Summary godoc
// GET /transactions/summary?year=2026&month=4
// Returns income, expenses, balance and per-category totals.
// If year+month are provided, scopes to that month; otherwise uses current month.
func (h *TransactionHandler) Summary(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now().UTC()
	year := now.Year()
	month := int(now.Month())

	yearStr := r.URL.Query().Get("year")
	monthStr := r.URL.Query().Get("month")
	if yearStr != "" && monthStr != "" {
		if _, e1 := fmt.Sscanf(yearStr, "%d", &year); e1 != nil {
			respondError(w, http.StatusBadRequest, "invalid year")
			return
		}
		if _, e2 := fmt.Sscanf(monthStr, "%d", &month); e2 != nil || month < 1 || month > 12 {
			respondError(w, http.StatusBadRequest, "invalid month")
			return
		}
	}

	summary, err := h.repo.SummaryByMonth(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("transaction: summary")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"income":     summary.Income,
		"expenses":   summary.Expenses,
		"balance":    summary.Balance,
		"categories": summary.Categories,
		"month":      fmt.Sprintf("%d-%02d", year, month),
	})
}

// BalanceTransferPending godoc
// GET /transactions/balance-transfer/pending?year=2026&month=5
// Returns the previous month's leftover balance if positive and not yet transferred.
func (h *TransactionHandler) BalanceTransferPending(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	if y := r.URL.Query().Get("year"); y != "" {
		fmt.Sscanf(y, "%d", &year) //nolint:errcheck
	}
	if m := r.URL.Query().Get("month"); m != "" {
		fmt.Sscanf(m, "%d", &month) //nolint:errcheck
	}

	amount, err := h.repo.BalanceTransferPending(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("transaction: balance transfer pending")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"amount": amount, "pending": amount > 0})
}

// ConfirmBalanceTransfer godoc
// POST /transactions/balance-transfer
// Creates a balance_transfer income transaction for the given month.
func (h *TransactionHandler) ConfirmBalanceTransfer(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := auth.AccountIDFromContext(ctx)
	userID := auth.UserIDFromContext(ctx)

	var req struct {
		Amount float64 `json:"amount"`
		Year   int     `json:"year"`
		Month  int     `json:"month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Amount <= 0 || req.Year == 0 || req.Month == 0 {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	prevMonth := time.Date(req.Year, time.Month(req.Month-1), 1, 0, 0, 0, 0, time.UTC)
	prevLabel := prevMonth.Format("January 2006")

	tx := &models.Transaction{
		AccountID:   accountID,
		UserID:      userID,
		Amount:      req.Amount,
		Category:    models.CategoryOutrasEntradas,
		Description: fmt.Sprintf("Saldo transferido de %s", prevLabel),
		Date:        time.Date(req.Year, time.Month(req.Month), 1, 12, 0, 0, 0, time.UTC),
		Source:      "balance_transfer",
	}
	if err := h.repo.Create(tx); err != nil {
		log.Error().Err(err).Msg("transaction: confirm balance transfer")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, tx)
}

// ListCategories godoc
// GET /categories
// Returns all categories with slugs, labels, types and modes.
func (h *TransactionHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := h.catRepo.ListAll()
	if err != nil {
		log.Error().Err(err).Msg("categories: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, cats)
}

// PaymentMethodAnalytics godoc
// GET /transactions/payment-methods-analytics?months=6
// Returns per-payment-method spending breakdown for the last N months.
func (h *TransactionHandler) PaymentMethodAnalytics(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		fmt.Sscanf(m, "%d", &months) //nolint:errcheck
	}
	if months < 1 || months > 24 {
		months = 6
	}

	data, err := h.repo.PaymentMethodAnalyticsByMonths(accountID, months)
	if err != nil {
		log.Error().Err(err).Msg("transaction: payment method analytics")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, data)
}

// ExpenseProjection godoc
// GET /transactions/expense-projection?months=6
// Returns projected spending for the next N months, broken down into
// fixed expenses, installments and credit-card buckets.
func (h *TransactionHandler) ExpenseProjection(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		fmt.Sscanf(m, "%d", &months) //nolint:errcheck
	}
	if months < 1 || months > 12 {
		months = 6
	}

	data, err := h.repo.ExpenseProjectionByMonths(accountID, months)
	if err != nil {
		log.Error().Err(err).Msg("transaction: expense projection")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"months": data})
}

// ExpenseProjectionDetail godoc
// GET /transactions/expense-projection/detail?months=6
// Returns projected spending for the next N months with itemized lists per bucket.
func (h *TransactionHandler) ExpenseProjectionDetail(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	months := 6
	if m := r.URL.Query().Get("months"); m != "" {
		fmt.Sscanf(m, "%d", &months) //nolint:errcheck
	}
	if months < 1 || months > 12 {
		months = 6
	}

	data, err := h.repo.ExpenseProjectionDetailByMonths(accountID, months)
	if err != nil {
		log.Error().Err(err).Msg("transaction: expense projection detail")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"months": data})
}

// parseUUID extracts a UUID path parameter by name.
func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, param))
}

// VoucherDepositPending godoc
// GET /transactions/voucher-deposit/pending?year=2026&month=5
// Returns vouchers without a confirmed deposit in the given month.
func (h *TransactionHandler) VoucherDepositPending(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	if y := r.URL.Query().Get("year"); y != "" {
		fmt.Sscanf(y, "%d", &year) //nolint:errcheck
	}
	if m := r.URL.Query().Get("month"); m != "" {
		fmt.Sscanf(m, "%d", &month) //nolint:errcheck
	}

	pending, err := h.repo.VoucherDepositsPending(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("transaction: voucher deposit pending")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, pending)
}

// ConfirmVoucherDeposit godoc
// POST /transactions/voucher-deposit
// Creates a voucher_deposit transaction for the given voucher and month.
func (h *TransactionHandler) ConfirmVoucherDeposit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := auth.AccountIDFromContext(ctx)
	userID := auth.UserIDFromContext(ctx)

	var req struct {
		PaymentMethodID uuid.UUID `json:"payment_method_id"`
		Amount          float64   `json:"amount"`
		Year            int       `json:"year"`
		Month           int       `json:"month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.Amount <= 0 || req.Year == 0 || req.Month == 0 || req.PaymentMethodID == uuid.Nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	monthLabel := time.Date(req.Year, time.Month(req.Month), 1, 0, 0, 0, 0, time.UTC).
		Format("January 2006")

	tx := &models.Transaction{
		AccountID:              accountID,
		UserID:                 userID,
		Amount:                 req.Amount,
		Category:               "vr",
		Description:            fmt.Sprintf("Depósito VR — %s", monthLabel),
		Date:                   time.Date(req.Year, time.Month(req.Month), 1, 12, 0, 0, 0, time.UTC),
		Source:                 "voucher_deposit",
		Type:                   "regular",
		VoucherPaymentMethodID: &req.PaymentMethodID,
	}
	if err := h.repo.Create(tx); err != nil {
		log.Error().Err(err).Msg("transaction: confirm voucher deposit")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, tx)
}
