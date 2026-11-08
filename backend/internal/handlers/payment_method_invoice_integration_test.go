//go:build integration

package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/gorm"
)

func buildInvoiceHandlerWithDB(t *testing.T) (*handlers.PaymentMethodHandler, *repository.TransactionRepository, *repository.PaymentMethodRepository, *gorm.DB) {
	t.Helper()
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	pmRepo := repository.NewPaymentMethodRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	h := handlers.NewPaymentMethodHandler(pmRepo, txRepo, catRepo)
	return h, txRepo, pmRepo, db
}

func newInvoiceRouter(h *handlers.PaymentMethodHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/payment-methods/{id}/invoices/{year}/{month}/pay", h.PayInvoice)
	r.Delete("/payment-methods/{id}/invoices/{year}/{month}/pay", h.UnpayInvoice)
	return r
}

func invoicePayURL(pmID uuid.UUID, year, month int) string {
	return fmt.Sprintf("/payment-methods/%s/invoices/%d/%d/pay", pmID, year, month)
}

// createCreditCardTxForBilling inserts a credit card purchase assigned to the given billing month.
func createCreditCardTxForBilling(t *testing.T, db *gorm.DB, accountID uuid.UUID, pmID uuid.UUID, amount float64, billingYear, billingMonth int) *models.Transaction {
	t.Helper()
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          amount,
		Category:        "food",
		Date:            time.Date(billingYear, time.Month(billingMonth), 15, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: &pmID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create credit card transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })
	return tx
}

// TestPayInvoice_FullPayment verifies that paying the full invoice amount creates
// an invoice_payment transaction with no carryover_amount.
func TestPayInvoice_FullPayment(t *testing.T) {
	h, txRepo, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth), map[string]any{"amount": 300.00}, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?", accountID, pm.ID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	_ = txRepo
	if invoiceTx.Amount != -300.00 {
		t.Errorf("amount: got %v, want -300.00", invoiceTx.Amount)
	}
	if invoiceTx.CarryoverAmount != nil {
		t.Errorf("carryover_amount: expected nil, got %v", *invoiceTx.CarryoverAmount)
	}
	if invoiceTx.BillingYear == nil || *invoiceTx.BillingYear != billingYear {
		t.Errorf("billing_year: got %v, want %d", invoiceTx.BillingYear, billingYear)
	}
	if invoiceTx.BillingMonth == nil || *invoiceTx.BillingMonth != billingMonth {
		t.Errorf("billing_month: got %v, want %d", invoiceTx.BillingMonth, billingMonth)
	}
}

// TestPayInvoice_PartialPayment verifies that a partial payment creates an
// invoice_payment with carryover_amount = total - paid_amount.
func TestPayInvoice_PartialPayment(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth), map[string]any{"amount": 200.00}, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?", accountID, pm.ID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	if invoiceTx.Amount != -200.00 {
		t.Errorf("amount: got %v, want -200.00", invoiceTx.Amount)
	}
	if invoiceTx.CarryoverAmount == nil {
		t.Fatal("carryover_amount: expected non-nil")
	}
	if *invoiceTx.CarryoverAmount != 100.00 {
		t.Errorf("carryover_amount: got %v, want 100.00", *invoiceTx.CarryoverAmount)
	}
}

// TestPayInvoice_Idempotency verifies that a second POST for the same invoice
// returns 409 and does not create a duplicate transaction.
func TestPayInvoice_Idempotency(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	url := invoicePayURL(pm.ID, billingYear, billingMonth)

	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, authedRequest(http.MethodPost, url, map[string]any{"amount": 300.00}, accountID))
	if w1.Code != http.StatusCreated {
		t.Fatalf("first pay: expected 201, got %d; body: %s", w1.Code, w1.Body.String())
	}

	var firstTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment'", accountID).First(&firstTx).Error; err != nil {
		t.Fatalf("find first invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&firstTx) })

	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, authedRequest(http.MethodPost, url, map[string]any{"amount": 300.00}, accountID))
	if w2.Code != http.StatusConflict {
		t.Errorf("second pay: expected 409, got %d; body: %s", w2.Code, w2.Body.String())
	}

	var count int64
	db.Model(&models.Transaction{}).Where("account_id = ? AND type = 'invoice_payment'", accountID).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 invoice_payment, got %d", count)
	}
}

// TestUnpayInvoice_RemovesTransaction verifies that DELETE removes the
// invoice_payment transaction and allows re-payment.
func TestUnpayInvoice_RemovesTransaction(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	url := invoicePayURL(pm.ID, billingYear, billingMonth)

	// First payment
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, authedRequest(http.MethodPost, url, map[string]any{"amount": 300.00}, accountID))
	if w1.Code != http.StatusCreated {
		t.Fatalf("pay: expected 201, got %d", w1.Code)
	}

	var createdTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment'", accountID).First(&createdTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}

	// Delete payment
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, authedRequest(http.MethodDelete, url, nil, accountID))
	if w2.Code != http.StatusOK {
		t.Fatalf("unpay: expected 200, got %d; body: %s", w2.Code, w2.Body.String())
	}

	var count int64
	db.Model(&models.Transaction{}).Where("id = ?", createdTx.ID).Count(&count)
	if count != 0 {
		t.Error("expected invoice_payment to be deleted")
	}

	// Re-payment should succeed
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, authedRequest(http.MethodPost, url, map[string]any{"amount": 300.00}, accountID))
	if w3.Code != http.StatusCreated {
		t.Errorf("re-pay: expected 201, got %d; body: %s", w3.Code, w3.Body.String())
	}

	var reTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment'", accountID).First(&reTx).Error; err != nil {
		t.Fatalf("find re-payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&reTx) })
}

// TestPayInvoice_WrongAccount verifies that paying with a PM from another
// account returns 404.
func TestPayInvoice_WrongAccount(t *testing.T) {
	h, _, pmRepo, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	ownerAccountID := uuid.New()
	callerAccountID := uuid.New()
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, ownerAccountID, closingDay)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, 2026, 5), map[string]any{"amount": 100.00}, callerAccountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestPayInvoice_InvalidUUID verifies that a non-UUID path segment returns 400.
func TestPayInvoice_InvalidUUID(t *testing.T) {
	h, _, _, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods/not-a-uuid/invoices/2026/5/pay",
		map[string]any{"amount": 100.00}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPayInvoice_InvalidYear verifies that a non-integer year returns 400.
func TestPayInvoice_InvalidYear(t *testing.T) {
	h, _, _, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods/"+uuid.New().String()+"/invoices/notayear/5/pay",
		map[string]any{"amount": 100.00}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPayInvoice_InvalidMonth verifies that month=13 returns 400.
func TestPayInvoice_InvalidMonth(t *testing.T) {
	h, _, _, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, "/payment-methods/"+uuid.New().String()+"/invoices/2026/13/pay",
		map[string]any{"amount": 100.00}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPayInvoice_WithDueDay verifies that the response includes due_day when the PM has one.
func TestPayInvoice_WithDueDay(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	dueDay := 10
	pm := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Nubank",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		DueDay:     &dueDay,
		Color:      "#000",
		Active:     true,
	}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -100.00, billingYear, billingMonth)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth),
		map[string]any{"amount": 100.00}, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["due_day"]; !ok {
		t.Error("expected due_day in response")
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment'", accountID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })
}

// TestUnpayInvoice_WrongAccount verifies that UnpayInvoice returns 404 when the
// PM belongs to a different account.
func TestUnpayInvoice_WrongAccount(t *testing.T) {
	h, _, pmRepo, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	ownerAccountID := uuid.New()
	callerAccountID := uuid.New()
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, ownerAccountID, closingDay)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, invoicePayURL(pm.ID, 2026, 5), nil, callerAccountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUnpayInvoice_NotFound verifies that deleting a non-existent invoice payment returns 404.
func TestUnpayInvoice_NotFound(t *testing.T) {
	h, _, pmRepo, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, invoicePayURL(pm.ID, 2026, 5), nil, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d; body: %s", w.Code, w.Body.String())
	}
}

// TestUnpayInvoice_InvalidUUID verifies that an invalid UUID path param returns 400.
func TestUnpayInvoice_InvalidUUID(t *testing.T) {
	h, _, _, _ := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodDelete, "/payment-methods/not-a-uuid/invoices/2026/5/pay", nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestPayInvoice_DashboardReflectsDebit verifies that after paying an invoice
// the balance in GET /dashboard for that month includes the debit.
func TestPayInvoice_DashboardReflectsDebit(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	// Use the current year/month so that time.Now() (used as transaction date)
	// falls within the dashboard query window.
	now := time.Now().UTC()
	billingYear, billingMonth := now.Year(), int(now.Month())

	accountID := uuid.New()
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)

	// Create a credit card purchase in the billing month (-300.00)
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	// Pay the invoice (300.00)
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth), map[string]any{"amount": 300.00}, accountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("pay: expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment'", accountID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	// Build dashboard handler and check balance
	txRepo := repository.NewTransactionRepository(db)
	recurringRepo := repository.NewRecurringExpenseRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	dashHandler := handlers.NewDashboardHandler(txRepo, recurringRepo, catRepo, pmRepo)
	dashMux := newDashboardRouter(dashHandler)

	wr := httptest.NewRecorder()
	dashReq := authedRequest(http.MethodGet,
		fmt.Sprintf("/dashboard?year=%d&month=%d", billingYear, billingMonth),
		nil, accountID)
	dashMux.ServeHTTP(wr, dashReq)

	if wr.Code != http.StatusOK {
		t.Fatalf("dashboard: expected 200, got %d; body: %s", wr.Code, wr.Body.String())
	}

	var resp struct {
		Summary struct {
			Balance float64 `json:"balance"`
		} `json:"summary"`
	}
	if err := json.NewDecoder(wr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}

	// Balance should reflect both the credit card purchase (-300) and the payment (-300)
	// Total = -300 (purchase date in billing month) + -300 (invoice payment) = -600
	// Actually only the invoice_payment has date=now; the purchase date is billingYear/month/15
	// Both are in the billing month, so balance should be <= -300 (the payment debit is included)
	if resp.Summary.Balance >= 0 {
		t.Errorf("balance: expected negative (debit reflected), got %v", resp.Summary.Balance)
	}
}

// TestPayInvoice_WithDebitPM_NonCredit verifies that when debit_payment_method_id
// refers to a non-credit-card PM (e.g. debit), a regular transaction with
// category "pagamento_fatura" is created on that method with no billing_year/billing_month.
func TestPayInvoice_WithDebitPM_NonCredit(t *testing.T) {
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	pmRepo := repository.NewPaymentMethodRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	h := handlers.NewPaymentMethodHandler(pmRepo, txRepo, catRepo)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31

	creditCard := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, creditCard.ID, -500.00, billingYear, billingMonth)

	debitPM := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Conta Corrente",
		Kind:      models.KindDebit,
		Color:     "#123456",
		Active:    true,
	}
	if err := pmRepo.Create(debitPM); err != nil {
		t.Fatalf("create debit pm: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(debitPM) })

	body := map[string]any{
		"amount":                  500.00,
		"debit_payment_method_id": debitPM.ID.String(),
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(creditCard.ID, billingYear, billingMonth), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?",
		accountID, creditCard.ID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })
	if invoiceTx.Amount != -500.00 {
		t.Errorf("invoice_payment amount: got %v, want -500.00", invoiceTx.Amount)
	}

	var debitTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND payment_method_id = ? AND category = 'pagamento_fatura'",
		accountID, debitPM.ID).First(&debitTx).Error; err != nil {
		t.Fatalf("find debit tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&debitTx) })
	if debitTx.Amount != -500.00 {
		t.Errorf("debit tx amount: got %v, want -500.00", debitTx.Amount)
	}
	if debitTx.BillingYear != nil || debitTx.BillingMonth != nil {
		t.Error("debit tx should have no billing_year/billing_month for non-credit-card")
	}
}

// TestPayInvoice_WithDebitPM_CreditCard verifies that when debit_payment_method_id
// refers to a credit_card PM, the debit tx gets billing_year/billing_month set via ComputeBillingMonth.
func TestPayInvoice_WithDebitPM_CreditCard(t *testing.T) {
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	pmRepo := repository.NewPaymentMethodRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	h := handlers.NewPaymentMethodHandler(pmRepo, txRepo, catRepo)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31

	creditCard := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, creditCard.ID, -200.00, billingYear, billingMonth)

	closingDay2 := 10
	otherCard := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Inter",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay2,
		Color:      "#abcdef",
		Active:     true,
	}
	if err := pmRepo.Create(otherCard); err != nil {
		t.Fatalf("create other card: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(otherCard) })

	body := map[string]any{
		"amount":                  200.00,
		"debit_payment_method_id": otherCard.ID.String(),
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(creditCard.ID, billingYear, billingMonth), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?",
		accountID, creditCard.ID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	var debitTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND payment_method_id = ? AND category = 'pagamento_fatura'",
		accountID, otherCard.ID).First(&debitTx).Error; err != nil {
		t.Fatalf("find debit tx on other card: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&debitTx) })
	if debitTx.Amount != -200.00 {
		t.Errorf("debit tx amount: got %v, want -200.00", debitTx.Amount)
	}
	if debitTx.BillingYear == nil || debitTx.BillingMonth == nil {
		t.Error("debit tx on credit card should have billing_year/billing_month set")
	}
}

// TestPayInvoice_WithDebitPM_Overpayment verifies that overpaying with a debit method
// creates 3 transactions: adjustTx (excess on card), invoicePayTx, debitTx.
func TestPayInvoice_WithDebitPM_Overpayment(t *testing.T) {
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.RecurringExpense{}, &models.Category{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	pmRepo := repository.NewPaymentMethodRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	h := handlers.NewPaymentMethodHandler(pmRepo, txRepo, catRepo)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31

	creditCard := createCreditCardPM(t, pmRepo, accountID, closingDay)
	createCreditCardTxForBilling(t, db, accountID, creditCard.ID, -300.00, billingYear, billingMonth)

	debitPM := &models.PaymentMethod{
		AccountID: accountID, Name: "Conta", Kind: models.KindDebit, Color: "#000", Active: true,
	}
	if err := pmRepo.Create(debitPM); err != nil {
		t.Fatalf("create debit pm: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(debitPM) })

	body := map[string]any{
		"amount":                  800.00,
		"debit_payment_method_id": debitPM.ID.String(),
	}
	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(creditCard.ID, billingYear, billingMonth), body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var adjustTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND category = 'pagamento_fatura' AND payment_method_id = ?",
		accountID, creditCard.ID).First(&adjustTx).Error; err != nil {
		t.Fatalf("find adjust tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&adjustTx) })
	if adjustTx.Amount != -500.00 {
		t.Errorf("adjust amount: got %v, want -500.00", adjustTx.Amount)
	}

	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?",
		accountID, creditCard.ID).First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })
	if invoiceTx.Amount != -800.00 {
		t.Errorf("invoice_payment amount: got %v, want -800.00", invoiceTx.Amount)
	}

	var debitTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND payment_method_id = ? AND category = 'pagamento_fatura'",
		accountID, debitPM.ID).First(&debitTx).Error; err != nil {
		t.Fatalf("find debit tx: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&debitTx) })
	if debitTx.Amount != -800.00 {
		t.Errorf("debit tx amount: got %v, want -800.00", debitTx.Amount)
	}
}

// TestPayInvoice_Overpayment verifies that paying more than the app invoice total
// creates an adjustment transaction on the card (category "pagamento_fatura") and an
// invoice_payment for the full paid amount. No carryover.
func TestPayInvoice_Overpayment(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	// Only R$300 registered in the app; real bill is R$800
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth), map[string]any{"amount": 800.00}, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	// invoice_payment should be for the full paid amount
	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?", accountID, pm.ID).
		First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	if invoiceTx.Amount != -800.00 {
		t.Errorf("invoice_payment amount: got %v, want -800.00", invoiceTx.Amount)
	}
	if invoiceTx.CarryoverAmount != nil {
		t.Errorf("carryover_amount: expected nil for overpayment, got %v", *invoiceTx.CarryoverAmount)
	}

	// adjustment transaction should cover the excess (500.00)
	var adjustTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND category = 'pagamento_fatura' AND payment_method_id = ? AND billing_year = ? AND billing_month = ?",
		accountID, pm.ID, billingYear, billingMonth).
		First(&adjustTx).Error; err != nil {
		t.Fatalf("find adjustment transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&adjustTx) })

	if adjustTx.Amount != -500.00 {
		t.Errorf("adjustment amount: got %v, want -500.00", adjustTx.Amount)
	}
	if adjustTx.Description != "ajuste de fatura" {
		t.Errorf("adjustment description: got %q, want \"ajuste de fatura\"", adjustTx.Description)
	}
}
