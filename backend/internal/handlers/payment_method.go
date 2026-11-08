package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

// paymentMethodStore is the repository interface used by PaymentMethodHandler.
// Keeping it unexported and defined here (not in the repository package) avoids
// circular imports and lets unit tests inject stubs without needing a real database.
type paymentMethodStore interface {
	ListActive(accountID uuid.UUID) ([]models.PaymentMethod, error)
	FindByID(id, accountID uuid.UUID) (*models.PaymentMethod, error)
	Create(m *models.PaymentMethod) error
	Update(m *models.PaymentMethod, accountID uuid.UUID) error
	SoftDelete(id, accountID uuid.UUID) error
	HardDelete(id, accountID uuid.UUID) error
	HasTransactions(id uuid.UUID) (bool, error)
	Balance(id uuid.UUID) (float64, error)
}

// paymentMethodResponse embeds PaymentMethod and adds the computed running balance.
type paymentMethodResponse struct {
	models.PaymentMethod
	Balance *float64 `json:"balance,omitempty"`
}

// invoiceTxStore is the minimal transaction repository interface for invoice pay/unpay.
type invoiceTxStore interface {
	PaymentMethodSummaryByMonth(accountID uuid.UUID, year, month int) ([]repository.PaymentMethodSummary, error)
	FindInvoicePayment(paymentMethodID uuid.UUID, billingYear, billingMonth int) (*models.Transaction, error)
	Create(tx *models.Transaction) error
	CreateBatch(txs ...*models.Transaction) error
	Delete(id, accountID uuid.UUID) error
}

type PaymentMethodHandler struct {
	repo    paymentMethodStore
	txRepo  invoiceTxStore
	catRepo *repository.CategoryRepository
}

// NewPaymentMethodHandler constructs a handler. The concrete
// *repository.PaymentMethodRepository satisfies paymentMethodStore,
// and *repository.TransactionRepository satisfies invoiceTxStore.
func NewPaymentMethodHandler(repo *repository.PaymentMethodRepository, txRepo *repository.TransactionRepository, catRepo *repository.CategoryRepository) *PaymentMethodHandler {
	return &PaymentMethodHandler{repo: repo, txRepo: txRepo, catRepo: catRepo}
}

type paymentMethodRequest struct {
	Name           string                   `json:"name"`
	Kind           models.PaymentMethodKind `json:"kind"`
	ClosingDay     *int                     `json:"closing_day"`
	DueDay         *int                     `json:"due_day"`
	CreditLimit    *float64                 `json:"credit_limit"`
	LastFourDigits *string                  `json:"last_four_digits"`
	Color          string                   `json:"color"`
}

func (req *paymentMethodRequest) validate() (string, bool) {
	if len(req.Name) == 0 {
		return "name is required", false
	}
	if len(req.Name) > 100 {
		return "name must be at most 100 characters", false
	}
	if req.Kind == models.KindCreditCard {
		if req.ClosingDay == nil {
			return "closing_day is required for credit_card", false
		}
		if *req.ClosingDay < 1 || *req.ClosingDay > 31 {
			return "closing_day must be between 1 and 31", false
		}
	} else if req.ClosingDay != nil {
		return "closing_day is only valid for credit_card", false
	}
	if req.LastFourDigits != nil {
		v := *req.LastFourDigits
		if len(v) != 4 {
			return "last_four_digits must be exactly 4 characters", false
		}
		for _, c := range v {
			if c < '0' || c > '9' {
				return "last_four_digits must contain only digits", false
			}
		}
	}
	return "", true
}

// List handles GET /payment-methods.
// For voucher and cash methods, includes the computed running balance.
func (h *PaymentMethodHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	methods, err := h.repo.ListActive(accountID)
	if err != nil {
		log.Error().Err(err).Msg("payment_method: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	resp := make([]paymentMethodResponse, len(methods))
	for i, pm := range methods {
		resp[i] = paymentMethodResponse{PaymentMethod: pm}
		if pm.Kind == models.KindVoucher || pm.Kind == models.KindCash {
			bal, err := h.repo.Balance(pm.ID)
			if err != nil {
				log.Error().Err(err).Msg("payment_method: list balance")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			resp[i].Balance = &bal
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

// Create handles POST /payment-methods.
func (h *PaymentMethodHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	var req paymentMethodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg, ok := req.validate(); !ok {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	m := &models.PaymentMethod{
		AccountID:      accountID,
		Name:           req.Name,
		Kind:           req.Kind,
		ClosingDay:     req.ClosingDay,
		DueDay:         req.DueDay,
		CreditLimit:    req.CreditLimit,
		LastFourDigits: req.LastFourDigits,
		Color:          req.Color,
	}
	if err := h.repo.Create(m); err != nil {
		log.Error().Err(err).Msg("payment_method: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusCreated, m)
}

// Update handles PUT /payment-methods/{id}.
// Rejects kind changes when the method has associated transactions.
func (h *PaymentMethodHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payment method id")
		return
	}

	var req paymentMethodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if msg, ok := req.validate(); !ok {
		respondError(w, http.StatusBadRequest, msg)
		return
	}

	existing, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "payment method not found")
			return
		}
		log.Error().Err(err).Msg("payment_method: update find")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if existing.Kind != req.Kind {
		hasTxs, err := h.repo.HasTransactions(id)
		if err != nil {
			log.Error().Err(err).Msg("payment_method: update check transactions")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		if hasTxs {
			respondError(w, http.StatusUnprocessableEntity, "cannot change kind: payment method has associated transactions")
			return
		}
	}

	existing.Name = req.Name
	existing.Kind = req.Kind
	existing.ClosingDay = req.ClosingDay
	existing.DueDay = req.DueDay
	existing.CreditLimit = req.CreditLimit
	existing.Color = req.Color
	existing.LastFourDigits = req.LastFourDigits

	if err := h.repo.Update(existing, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "payment method not found")
			return
		}
		log.Error().Err(err).Msg("payment_method: update")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

// PayInvoice handles POST /payment-methods/{id}/invoices/{year}/{month}/pay.
// Creates an invoice_payment transaction for the given billing month.
// If debit_payment_method_id is provided, also creates a regular transaction on that method
// with category "pagamento_fatura" to track the real cash outflow.
// Returns 409 if already paid; supports partial payment with carryover tracking.
func (h *PaymentMethodHandler) PayInvoice(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	pmID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payment method id")
		return
	}

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil || month < 1 || month > 12 {
		respondError(w, http.StatusBadRequest, "invalid month")
		return
	}

	var req struct {
		Amount               float64    `json:"amount"`
		DebitPaymentMethodID *uuid.UUID `json:"debit_payment_method_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Amount <= 0 {
		respondError(w, http.StatusBadRequest, "amount must be positive")
		return
	}

	pm, err := h.repo.FindByID(pmID, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "payment method not found")
			return
		}
		log.Error().Err(err).Msg("pay_invoice: find payment method")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	_, err = h.txRepo.FindInvoicePayment(pmID, year, month)
	if err == nil {
		respondError(w, http.StatusConflict, "invoice already paid for this month")
		return
	}
	if !errors.Is(err, repository.ErrNotFound) {
		log.Error().Err(err).Msg("pay_invoice: idempotency check")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var debitPM *models.PaymentMethod
	if req.DebitPaymentMethodID != nil {
		debitPM, err = h.repo.FindByID(*req.DebitPaymentMethodID, accountID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "debit payment method not found")
				return
			}
			log.Error().Err(err).Msg("pay_invoice: find debit payment method")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	summaries, err := h.txRepo.PaymentMethodSummaryByMonth(accountID, year, month)
	if err != nil {
		log.Error().Err(err).Msg("pay_invoice: summary")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var invoiceTotal float64
	for _, s := range summaries {
		if s.PaymentMethodID != nil && *s.PaymentMethodID == pmID {
			invoiceTotal = math.Abs(s.Total)
			break
		}
	}

	roundedAmount := math.Round(req.Amount*100) / 100
	roundedTotal := math.Round(invoiceTotal*100) / 100

	billingYear := year
	billingMonth := month
	now := time.Now().UTC()

	invoicePayTx := &models.Transaction{
		UserID:          userID,
		AccountID:       accountID,
		Amount:          -roundedAmount,
		Category:        "",
		Date:            now,
		Source:          "manual",
		Type:            "invoice_payment",
		PaymentMethodID: &pmID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
		Locale:          "BR",
		Currency:        "BRL",
	}

	var debitTx *models.Transaction
	if debitPM != nil {
		debitID := debitPM.ID
		debitTx = &models.Transaction{
			UserID:    userID,
			AccountID: accountID,
			Amount:    -roundedAmount,
			Category:  "pagamento_fatura",
			Date:      now,
			Source:    "manual",
			Type:      "regular",
			Locale:    "BR",
			Currency:  "BRL",
		}
		if debitPM.Kind == models.KindCreditCard {
			if debitPM.ClosingDay != nil {
				dbYear, dbMonth := models.ComputeBillingMonth(now, *debitPM.ClosingDay)
				dbMonthInt := int(dbMonth)
				debitTx.PaymentMethodID = &debitID
				debitTx.BillingYear = &dbYear
				debitTx.BillingMonth = &dbMonthInt
			} else {
				debitTx.PaymentMethodID = &debitID
			}
		} else {
			debitTx.PaymentMethodID = &debitID
		}
		if h.catRepo != nil {
			if cat, err := h.catRepo.FindBySlug("pagamento_fatura"); err == nil {
				debitTx.CategoryID = &cat.ID
			}
		}
	}

	if roundedAmount > roundedTotal {
		excess := math.Round((roundedAmount-roundedTotal)*100) / 100
		adjustTx := &models.Transaction{
			UserID:          userID,
			AccountID:       accountID,
			Amount:          -excess,
			Category:        "pagamento_fatura",
			Description:     "ajuste de fatura",
			Date:            now,
			Source:          "manual",
			Type:            "regular",
			PaymentMethodID: &pmID,
			BillingYear:     &billingYear,
			BillingMonth:    &billingMonth,
			Locale:          "BR",
			Currency:        "BRL",
		}
		if h.catRepo != nil {
			if cat, err := h.catRepo.FindBySlug("pagamento_fatura"); err == nil {
				adjustTx.CategoryID = &cat.ID
			}
		}
		batch := []*models.Transaction{adjustTx, invoicePayTx}
		if debitTx != nil {
			batch = append(batch, debitTx)
		}
		if err := h.txRepo.CreateBatch(batch...); err != nil {
			log.Error().Err(err).Msg("pay_invoice: create adjustment + invoice_payment")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else {
		diff := math.Round((roundedTotal-roundedAmount)*100) / 100
		if diff > 0 {
			invoicePayTx.CarryoverAmount = &diff
		}
		if debitTx != nil {
			if err := h.txRepo.CreateBatch(invoicePayTx, debitTx); err != nil {
				log.Error().Err(err).Msg("pay_invoice: create invoice_payment + debit")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
		} else {
			if err := h.txRepo.Create(invoicePayTx); err != nil {
				log.Error().Err(err).Msg("pay_invoice: create transaction")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
		}
	}

	resp := map[string]any{
		"transaction": invoicePayTx,
	}
	if pm.DueDay != nil {
		resp["due_day"] = *pm.DueDay
	}

	respondJSON(w, http.StatusCreated, resp)
}

// UnpayInvoice handles DELETE /payment-methods/{id}/invoices/{year}/{month}/pay.
// Removes the invoice_payment transaction for the given billing month.
func (h *PaymentMethodHandler) UnpayInvoice(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	pmID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payment method id")
		return
	}

	year, err := strconv.Atoi(chi.URLParam(r, "year"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid year")
		return
	}

	month, err := strconv.Atoi(chi.URLParam(r, "month"))
	if err != nil || month < 1 || month > 12 {
		respondError(w, http.StatusBadRequest, "invalid month")
		return
	}

	if _, err := h.repo.FindByID(pmID, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "payment method not found")
			return
		}
		log.Error().Err(err).Msg("unpay_invoice: find payment method")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	existing, err := h.txRepo.FindInvoicePayment(pmID, year, month)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "invoice payment not found")
			return
		}
		log.Error().Err(err).Msg("unpay_invoice: find invoice payment")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if err := h.txRepo.Delete(existing.ID, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "invoice payment not found")
			return
		}
		log.Error().Err(err).Msg("unpay_invoice: delete transaction")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Delete handles DELETE /payment-methods/{id}.
// Soft-deletes (active=false) when transactions exist; hard-deletes otherwise.
func (h *PaymentMethodHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid payment method id")
		return
	}

	hasTxs, err := h.repo.HasTransactions(id)
	if err != nil {
		log.Error().Err(err).Msg("payment_method: delete check transactions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if hasTxs {
		if err := h.repo.SoftDelete(id, accountID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "payment method not found")
				return
			}
			log.Error().Err(err).Msg("payment_method: soft delete")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else {
		if err := h.repo.HardDelete(id, accountID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				respondError(w, http.StatusNotFound, "payment method not found")
				return
			}
			log.Error().Err(err).Msg("payment_method: hard delete")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{"deleted": true})
}
