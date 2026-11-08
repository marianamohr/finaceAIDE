package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
)

// stubStore is a configurable test double for the paymentMethodStore interface.
// Defined here (white-box test file) to access the unexported interface.
type stubStore struct {
	listActiveResult   []models.PaymentMethod
	listActiveErr      error
	findByIDResult     *models.PaymentMethod
	findByIDErr        error
	createErr          error
	updateErr          error
	softDeleteErr      error
	hardDeleteErr      error
	hasTransactions    bool
	hasTransactionsErr error
	balanceResult      float64
	balanceErr         error
}

func (s *stubStore) ListActive(_ uuid.UUID) ([]models.PaymentMethod, error) {
	return s.listActiveResult, s.listActiveErr
}
func (s *stubStore) FindByID(_, _ uuid.UUID) (*models.PaymentMethod, error) {
	return s.findByIDResult, s.findByIDErr
}
func (s *stubStore) Create(m *models.PaymentMethod) error              { return s.createErr }
func (s *stubStore) Update(_ *models.PaymentMethod, _ uuid.UUID) error { return s.updateErr }
func (s *stubStore) SoftDelete(_, _ uuid.UUID) error                   { return s.softDeleteErr }
func (s *stubStore) HardDelete(_, _ uuid.UUID) error                   { return s.hardDeleteErr }
func (s *stubStore) HasTransactions(_ uuid.UUID) (bool, error) {
	return s.hasTransactions, s.hasTransactionsErr
}
func (s *stubStore) Balance(_ uuid.UUID) (float64, error) {
	return s.balanceResult, s.balanceErr
}

func stubHandler(store *stubStore) (*PaymentMethodHandler, *chi.Mux) {
	h := &PaymentMethodHandler{repo: store}
	r := chi.NewRouter()
	r.Get("/payment-methods", h.List)
	r.Post("/payment-methods", h.Create)
	r.Put("/payment-methods/{id}", h.Update)
	r.Delete("/payment-methods/{id}", h.Delete)
	return h, r
}

// stubTxStore is a configurable test double for the invoiceTxStore interface.
type stubTxStore struct {
	summaryResult         []repository.PaymentMethodSummary
	summaryErr            error
	findInvoicePaymentTx  *models.Transaction
	findInvoicePaymentErr error
	createErr             error
	createBatchErr        error
	deleteErr             error
	createdBatch          []*models.Transaction
}

func (s *stubTxStore) PaymentMethodSummaryByMonth(_ uuid.UUID, _, _ int) ([]repository.PaymentMethodSummary, error) {
	return s.summaryResult, s.summaryErr
}
func (s *stubTxStore) FindInvoicePayment(_ uuid.UUID, _, _ int) (*models.Transaction, error) {
	return s.findInvoicePaymentTx, s.findInvoicePaymentErr
}
func (s *stubTxStore) Create(_ *models.Transaction) error { return s.createErr }
func (s *stubTxStore) CreateBatch(txs ...*models.Transaction) error {
	s.createdBatch = txs
	return s.createBatchErr
}
func (s *stubTxStore) Delete(_, _ uuid.UUID) error { return s.deleteErr }

func stubInvoiceHandler(pmStore *stubStore, txStore *stubTxStore) (*PaymentMethodHandler, *chi.Mux) {
	h := &PaymentMethodHandler{repo: pmStore, txRepo: txStore}
	r := chi.NewRouter()
	r.Post("/payment-methods/{id}/invoices/{year}/{month}/pay", h.PayInvoice)
	r.Delete("/payment-methods/{id}/invoices/{year}/{month}/pay", h.UnpayInvoice)
	return h, r
}

func stubRequest(method, target string, body any, accountID uuid.UUID) *http.Request {
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	r := httptest.NewRequest(method, target, bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithValues(r.Context(), uuid.New(), accountID, "user", "active")
	return r.WithContext(ctx)
}

func TestPaymentMethodHandler_List_StoreError(t *testing.T) {
	_, mux := stubHandler(&stubStore{listActiveErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodGet, "/payment-methods", nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Create_StoreError(t *testing.T) {
	_, mux := stubHandler(&stubStore{createErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	body := map[string]any{"name": "PIX", "kind": "pix", "color": ""}
	r := stubRequest(http.MethodPost, "/payment-methods", body, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Update_FindByIDError(t *testing.T) {
	_, mux := stubHandler(&stubStore{findByIDErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	body := map[string]any{"name": "Test", "kind": "pix", "color": ""}
	r := stubRequest(http.MethodPut, "/payment-methods/"+uuid.New().String(), body, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Update_HasTransactionsError(t *testing.T) {
	closingDay := 3
	existing := &models.PaymentMethod{
		ID:         uuid.New(),
		AccountID:  uuid.New(),
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
	}
	_, mux := stubHandler(&stubStore{
		findByIDResult:     existing,
		hasTransactionsErr: errors.New("db failure"),
	})
	w := httptest.NewRecorder()
	// debit has no closing_day — kind change triggers HasTransactions check
	body := map[string]any{"name": "Débito", "kind": "debit", "color": ""}
	r := stubRequest(http.MethodPut, "/payment-methods/"+existing.ID.String(), body, existing.AccountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Update_UpdateError(t *testing.T) {
	closingDay := 3
	existing := &models.PaymentMethod{
		ID:         uuid.New(),
		AccountID:  uuid.New(),
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
	}
	_, mux := stubHandler(&stubStore{
		findByIDResult: existing,
		updateErr:      errors.New("db failure"),
	})
	w := httptest.NewRecorder()
	// same kind (credit_card), valid payload
	body := map[string]any{"name": "Updated", "kind": "credit_card", "closing_day": 5, "color": ""}
	r := stubRequest(http.MethodPut, "/payment-methods/"+existing.ID.String(), body, existing.AccountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Update_UpdateErrNotFound(t *testing.T) {
	closingDay := 3
	existing := &models.PaymentMethod{
		ID:         uuid.New(),
		AccountID:  uuid.New(),
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
	}
	_, mux := stubHandler(&stubStore{
		findByIDResult: existing,
		updateErr:      repository.ErrNotFound,
	})
	w := httptest.NewRecorder()
	body := map[string]any{"name": "Updated", "kind": "credit_card", "closing_day": 5, "color": ""}
	r := stubRequest(http.MethodPut, "/payment-methods/"+existing.ID.String(), body, existing.AccountID)
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Delete_HasTransactionsError(t *testing.T) {
	_, mux := stubHandler(&stubStore{hasTransactionsErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String(), nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Delete_SoftDeleteErrNotFound(t *testing.T) {
	_, mux := stubHandler(&stubStore{hasTransactions: true, softDeleteErr: repository.ErrNotFound})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String(), nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Delete_SoftDeleteError(t *testing.T) {
	_, mux := stubHandler(&stubStore{hasTransactions: true, softDeleteErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String(), nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Delete_HardDeleteError(t *testing.T) {
	_, mux := stubHandler(&stubStore{hardDeleteErr: errors.New("db failure")})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String(), nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPaymentMethodHandler_Update_Validation(t *testing.T) {
	closingDay := 3
	existing := &models.PaymentMethod{
		ID:         uuid.New(),
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
	}
	_, mux := stubHandler(&stubStore{findByIDResult: existing})
	w := httptest.NewRecorder()
	// credit_card without closing_day — validation error in Update
	body := map[string]any{"name": "Test", "kind": "credit_card", "color": ""}
	r := stubRequest(http.MethodPut, "/payment-methods/"+existing.ID.String(), body, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Invoice error-path tests (stub, no DB needed) ---

func TestPayInvoice_FindPMStoreError(t *testing.T) {
	pmStore := &stubStore{findByIDErr: errors.New("db failure")}
	txStore := &stubTxStore{findInvoicePaymentErr: repository.ErrNotFound}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+uuid.New().String()+"/invoices/2026/5/pay",
		map[string]any{"amount": 100.0}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPayInvoice_IdempotencyCheckStoreError(t *testing.T) {
	pmID := uuid.New()
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID, Kind: models.KindCreditCard}}
	txStore := &stubTxStore{findInvoicePaymentErr: errors.New("db failure")}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		map[string]any{"amount": 100.0}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPayInvoice_SummaryStoreError(t *testing.T) {
	pmID := uuid.New()
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID, Kind: models.KindCreditCard}}
	txStore := &stubTxStore{
		findInvoicePaymentErr: repository.ErrNotFound,
		summaryErr:            errors.New("db failure"),
	}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		map[string]any{"amount": 50.0}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestPayInvoice_CreateStoreError(t *testing.T) {
	pmID := uuid.New()
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID, Kind: models.KindCreditCard}}
	txStore := &stubTxStore{
		findInvoicePaymentErr: repository.ErrNotFound,
		summaryResult:         []repository.PaymentMethodSummary{{PaymentMethodID: &pmID, Total: -200.0}},
		createErr:             errors.New("db failure"),
	}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		map[string]any{"amount": 200.0}, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUnpayInvoice_FindPMStoreError(t *testing.T) {
	pmStore := &stubStore{findByIDErr: errors.New("db failure")}
	_, mux := stubInvoiceHandler(pmStore, &stubTxStore{})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String()+"/invoices/2026/5/pay",
		nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUnpayInvoice_FindInvoicePaymentStoreError(t *testing.T) {
	pmID := uuid.New()
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID}}
	txStore := &stubTxStore{findInvoicePaymentErr: errors.New("db failure")}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUnpayInvoice_DeleteStoreError(t *testing.T) {
	pmID := uuid.New()
	invoiceTx := &models.Transaction{ID: uuid.New()}
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID}}
	txStore := &stubTxStore{findInvoicePaymentTx: invoiceTx, deleteErr: errors.New("db failure")}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestUnpayInvoice_PMNotFound(t *testing.T) {
	pmStore := &stubStore{findByIDErr: repository.ErrNotFound}
	_, mux := stubInvoiceHandler(pmStore, &stubTxStore{})
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+uuid.New().String()+"/invoices/2026/5/pay",
		nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUnpayInvoice_InvoicePaymentNotFound(t *testing.T) {
	pmID := uuid.New()
	pmStore := &stubStore{findByIDResult: &models.PaymentMethod{ID: pmID}}
	txStore := &stubTxStore{findInvoicePaymentErr: repository.ErrNotFound}
	_, mux := stubInvoiceHandler(pmStore, txStore)
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodDelete, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay",
		nil, uuid.New())
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// TestPayInvoice_NegativeAmount verifies that a non-positive amount returns 400
// before any repository call, so both stores can be nil.
func TestPayInvoice_NegativeAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
	}{
		{"negative", -100.0},
		{"zero", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mux := stubInvoiceHandler(nil, nil)
			pmID := uuid.New()
			body := map[string]any{"amount": tt.amount}
			w := httptest.NewRecorder()
			r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay", body, uuid.New())
			mux.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d, want 400", w.Code)
			}
		})
	}
}

// TestPayInvoice_Overpayment verifies that paying more than the invoice total
// succeeds: CreateBatch is called with an adjustment tx and the invoice_payment.
func TestPayInvoice_Overpayment(t *testing.T) {
	pmID := uuid.New()
	accountID := uuid.New()
	pm := &models.PaymentMethod{
		ID:        pmID,
		AccountID: accountID,
		Kind:      models.KindCreditCard,
		Name:      "Nubank",
		Color:     "#000",
	}
	txStore := &stubTxStore{
		summaryResult: []repository.PaymentMethodSummary{
			{PaymentMethodID: &pmID, Total: -50.00},
		},
		findInvoicePaymentErr: repository.ErrNotFound,
	}
	pmStore := &stubStore{findByIDResult: pm}

	_, mux := stubInvoiceHandler(pmStore, txStore)
	body := map[string]any{"amount": 100.00}
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body: %s", w.Code, w.Body.String())
	}
	if len(txStore.createdBatch) != 2 {
		t.Fatalf("expected 2 transactions in batch, got %d", len(txStore.createdBatch))
	}
	adjustTx := txStore.createdBatch[0]
	invoicePayTx := txStore.createdBatch[1]
	if adjustTx.Amount != -50.00 {
		t.Errorf("adjustment amount: got %v, want -50.00", adjustTx.Amount)
	}
	if adjustTx.Category != "pagamento_fatura" {
		t.Errorf("adjustment category: got %q, want \"pagamento_fatura\"", adjustTx.Category)
	}
	if invoicePayTx.Amount != -100.00 {
		t.Errorf("invoice_payment amount: got %v, want -100.00", invoicePayTx.Amount)
	}
}

// TestPayInvoice_WithDebitMethod_NoDebit verifies that omitting debit_payment_method_id
// falls through to the existing Create path (no CreateBatch called).
func TestPayInvoice_WithDebitMethod_NoDebit(t *testing.T) {
	pmID := uuid.New()
	accountID := uuid.New()
	pm := &models.PaymentMethod{ID: pmID, AccountID: accountID, Kind: models.KindCreditCard, Name: "Nubank", Color: "#000"}
	txStore := &stubTxStore{
		summaryResult:         []repository.PaymentMethodSummary{{PaymentMethodID: &pmID, Total: -100.00}},
		findInvoicePaymentErr: repository.ErrNotFound,
	}
	pmStore := &stubStore{findByIDResult: pm}
	_, mux := stubInvoiceHandler(pmStore, txStore)

	body := map[string]any{"amount": 100.00} // no debit_payment_method_id
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body: %s", w.Code, w.Body.String())
	}
	if txStore.createdBatch != nil {
		t.Errorf("expected no batch (debit absent), but CreateBatch was called")
	}
}
