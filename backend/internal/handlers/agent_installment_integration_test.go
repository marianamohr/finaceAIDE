//go:build integration

package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/handlers"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
)

// TestAgentConfirm_ClarifyInstallmentCard_CreatesParcels reproduces the bug where
// a chat installment purchase with multiple credit cards created nothing on confirm.
// The fix routes the ambiguous case through a clarify_installment_card pending action
// whose confirmation (with the chosen card) materializes the parent + N parcels.
func TestAgentConfirm_ClarifyInstallmentCard_CreatesParcels(t *testing.T) {
	db := newHandlerTestDB(t)
	if err := db.AutoMigrate(&models.PendingAction{}, &models.InstallmentPurchase{}, &models.Category{}, &models.ChatMessage{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	pendingRepo := repository.NewPendingActionRepository(db)
	insightRepo := repository.NewInsightRepository(db)
	txRepo := repository.NewTransactionRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	installmentRepo := repository.NewInstallmentPurchaseRepository(db)
	chatRepo := repository.NewChatRepository(db)
	validator := transactions.NewValidator(catRepo, pmRepo)
	registry := agent.NewRegistry(nil, nil)
	h := handlers.NewAgentHandler(registry, pendingRepo, insightRepo, txRepo, pmRepo, catRepo, installmentRepo, chatRepo, validator)

	mux := chi.NewRouter()
	mux.Post("/agent/confirm", h.Confirm)

	accountID := uuid.New()
	card := seedCreditCard(t, db, accountID, 26)
	seedCategory(t, db, "lazer")

	t.Cleanup(func() {
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.Transaction{})
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.InstallmentPurchase{})
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.PendingAction{})
		db.Unscoped().Where("account_id = ?", accountID).Delete(&models.ChatMessage{})
	})

	payload, err := json.Marshal(map[string]any{
		"total_amount": 1500.0,
		"installments": 10,
		"category":     "lazer",
		"description":  "TV parcelada",
		"merchant":     "",
		"date":         "2026-05-30",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	pa := &models.PendingAction{
		AccountID: accountID,
		ToolName:  "clarify_installment_card",
		Payload:   payload,
		Preview:   "Em qual cartão foi a compra parcelada em 10x?",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := pendingRepo.Create(pa); err != nil {
		t.Fatalf("create pending action: %v", err)
	}

	body := map[string]any{
		"pending_action_id": pa.ID.String(),
		"resolutions": []map[string]any{
			{"index": 0, "payment_method_id": card.ID.String()},
		},
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, authedRequest(http.MethodPost, "/agent/confirm", body, accountID))
	if w.Code != http.StatusOK {
		t.Fatalf("confirm: expected 200, got %d — body: %s", w.Code, w.Body.String())
	}

	purchases, err := installmentRepo.ListByAccount(accountID)
	if err != nil {
		t.Fatalf("list purchases: %v", err)
	}
	if len(purchases) != 1 {
		t.Fatalf("expected 1 installment purchase, got %d", len(purchases))
	}
	if purchases[0].PaymentMethodID != card.ID {
		t.Errorf("purchase card = %s, want %s", purchases[0].PaymentMethodID, card.ID)
	}

	txs, err := installmentRepo.ListTransactions(purchases[0].ID, accountID)
	if err != nil {
		t.Fatalf("list parcels: %v", err)
	}
	if len(txs) != 10 {
		t.Fatalf("expected 10 parcels, got %d", len(txs))
	}
	var sum float64
	for _, tx := range txs {
		if tx.Source != "installment" {
			t.Errorf("parcel source = %q, want installment", tx.Source)
		}
		sum += tx.Amount
	}
	if d := sum + 1500.0; d > 0.001 || d < -0.001 {
		t.Errorf("parcels sum = %.2f, want -1500.00", sum)
	}

	// Confirming must record a past-tense assistant turn in the chat history so the
	// model sees the purchase as already saved and does not re-propose it next turn.
	hist, err := chatRepo.HistoryByAccount(accountID, 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("expected 1 chat message after confirm, got %d", len(hist))
	}
	if hist[0].Role != "assistant" || !strings.Contains(hist[0].Content, "Já registrei a compra parcelada") {
		t.Errorf("unexpected confirmation message: role=%q content=%q", hist[0].Role, hist[0].Content)
	}
}
