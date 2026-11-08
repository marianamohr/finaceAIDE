package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/installments"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/rs/zerolog/log"
)

type AgentHandler struct {
	registry        *agent.Registry
	pendingRepo     *repository.PendingActionRepository
	insightRepo     *repository.InsightRepository
	txRepo          *repository.TransactionRepository
	pmRepo          *repository.PaymentMethodRepository
	catRepo         *repository.CategoryRepository
	installmentRepo *repository.InstallmentPurchaseRepository
	chatRepo        *repository.ChatRepository
	validator       *transactions.Validator
}

func NewAgentHandler(
	registry *agent.Registry,
	pendingRepo *repository.PendingActionRepository,
	insightRepo *repository.InsightRepository,
	txRepo *repository.TransactionRepository,
	pmRepo *repository.PaymentMethodRepository,
	catRepo *repository.CategoryRepository,
	installmentRepo *repository.InstallmentPurchaseRepository,
	chatRepo *repository.ChatRepository,
	validator *transactions.Validator,
) *AgentHandler {
	return &AgentHandler{
		registry:        registry,
		pendingRepo:     pendingRepo,
		insightRepo:     insightRepo,
		txRepo:          txRepo,
		pmRepo:          pmRepo,
		catRepo:         catRepo,
		installmentRepo: installmentRepo,
		chatRepo:        chatRepo,
		validator:       validator,
	}
}

// savedTx is the minimal view of a persisted transaction needed to describe it
// in the chat-history confirmation message.
type savedTx struct {
	Description string
	Category    string
	Amount      float64
}

// savedTransactionsContent renders a past-tense assistant line listing the
// transactions just persisted. Empty input yields an empty string.
func savedTransactionsContent(txs []savedTx) string {
	if len(txs) == 0 {
		return ""
	}
	lines := make([]string, 0, len(txs)+1)
	lines = append(lines, "✓ Já registrei no banco (não registrar de novo):")
	for _, tx := range txs {
		label := tx.Description
		if label == "" {
			label = tx.Category
		}
		lines = append(lines, fmt.Sprintf("  • R$ %.2f — %s (%s)", math.Abs(tx.Amount), label, tx.Category))
	}
	return strings.Join(lines, "\n")
}

// recordConfirmationInHistory persists a past-tense assistant turn recording what
// was just saved, so the chat history the model re-reads on later turns reflects
// the database. Without this, confirmed purchases (whose only persisted ack is a
// bland "Ok.") get re-proposed by the model. Failures are non-fatal: the
// transactions are already persisted, this only keeps the history consistent.
func (h *AgentHandler) recordConfirmationInHistory(userID, accountID uuid.UUID, content string) {
	if content == "" || h.chatRepo == nil {
		return
	}
	msg := &models.ChatMessage{
		UserID:    userID,
		AccountID: accountID,
		Role:      "assistant",
		Content:   content,
	}
	if err := h.chatRepo.Save(msg); err != nil {
		log.Error().Err(err).Msg("agent: record confirmation in chat history")
	}
}

// Confirm handles POST /agent/confirm
// Body: { "pending_action_id": "uuid" }
// Response 200: { "tool_name": "...", "result": "..." }
func (h *AgentHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context()) // needed for batch transaction UserID

	var req struct {
		PendingActionID string `json:"pending_action_id"`
		Resolutions     []struct {
			Index           int       `json:"index"`
			PaymentMethodID uuid.UUID `json:"payment_method_id"`
		} `json:"resolutions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PendingActionID == "" {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	paID, err := uuid.Parse(req.PendingActionID)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid pending_action_id")
		return
	}

	pa, err := h.pendingRepo.ClaimAndExecute(paID, accountID)
	if err != nil {
		log.Error().Err(err).Msg("agent: claim pending action")
		respondError(w, http.StatusBadRequest, "invalid or expired action")
		return
	}

	if pa.ToolName == "replace_transaction_batch" {
		var batch replaceBatchPayload
		if err := json.Unmarshal(pa.Payload, &batch); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to parse replace batch")
			return
		}
		items := make([]repository.ReplaceBatchItem, 0, len(batch.Items))
		for _, inp := range batch.Items {
			txID, err := uuid.Parse(inp.TransactionID)
			if err != nil {
				respondError(w, http.StatusBadRequest, "invalid transaction_id: "+inp.TransactionID)
				return
			}
			newTxs := make([]*models.Transaction, 0, len(inp.Replacements))
			for _, r := range inp.Replacements {
				newTxs = append(newTxs, &models.Transaction{
					AccountID:   accountID,
					UserID:      userID,
					Amount:      r.Amount,
					Category:    r.Category,
					Description: r.Description,
					Merchant:    r.Merchant,
					Date:        parseDateNoon(r.Date),
					Source:      "chat",
				})
			}
			items = append(items, repository.ReplaceBatchItem{OriginalID: txID, NewTxs: newTxs})
		}
		if err := h.txRepo.ReplaceBatch(accountID, items); err != nil {
			log.Error().Err(err).Msg("agent: replace batch")
			respondError(w, http.StatusInternalServerError, "failed to execute replace batch")
			return
		}
		h.recordConfirmationInHistory(userID, accountID,
			fmt.Sprintf("✓ Já corrigi %d transação(ões) no banco (não corrigir de novo).", len(items)))
		respondJSON(w, http.StatusOK, map[string]string{
			"tool_name": pa.ToolName,
			"result":    fmt.Sprintf("%d transação(ões) substituída(s)", len(items)),
		})
		return
	}

	// Special case: batch transaction creation from upload OCR flow.
	if pa.ToolName == "create_transaction_batch" {
		var txs []models.Transaction
		if err := json.Unmarshal(pa.Payload, &txs); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to parse transactions")
			return
		}
		type createdTx struct {
			ID          string  `json:"id"`
			Description string  `json:"description"`
			Category    string  `json:"category"`
			Amount      float64 `json:"amount"`
		}
		var created []createdTx
		for i := range txs {
			txs[i].AccountID = accountID
			txs[i].UserID = userID
			if err := h.txRepo.Create(&txs[i]); err != nil {
				log.Error().Err(err).Msg("agent: confirm batch transactions")
			} else {
				created = append(created, createdTx{
					ID:          txs[i].ID.String(),
					Description: txs[i].Description,
					Category:    txs[i].Category,
					Amount:      txs[i].Amount,
				})
			}
		}
		saved := make([]savedTx, 0, len(created))
		for _, c := range created {
			saved = append(saved, savedTx{Description: c.Description, Category: c.Category, Amount: c.Amount})
		}
		h.recordConfirmationInHistory(userID, accountID, savedTransactionsContent(saved))
		respondJSON(w, http.StatusOK, map[string]any{
			"tool_name":    pa.ToolName,
			"result":       fmt.Sprintf("%d transação(ões) salva(s)", len(created)),
			"transactions": created,
		})
		return
	}

	if pa.ToolName == "clarify_payment_method" {
		var payload transactions.ClarifyPayload
		if err := json.Unmarshal(pa.Payload, &payload); err != nil {
			respondError(w, http.StatusInternalServerError, "payload de pending action corrompido")
			return
		}
		if len(req.Resolutions) != len(payload.Drafts) {
			respondError(w, http.StatusBadRequest, "é preciso resolver todos os drafts pendentes")
			return
		}

		created := make([]models.Transaction, 0, len(payload.Drafts))
		for _, res := range req.Resolutions {
			if res.Index < 0 || res.Index >= len(payload.Drafts) {
				respondError(w, http.StatusBadRequest, "índice de resolução fora do range")
				return
			}
			draft := payload.Drafts[res.Index].Tx
			pmID := res.PaymentMethodID
			hint := transactions.PaymentHint{PaymentMethodID: &pmID}
			need, verr := h.validator.ValidateAndEnrich(r.Context(), accountID, &draft, hint)
			if verr != nil {
				respondError(w, http.StatusUnprocessableEntity, verr.Error())
				return
			}
			if need != nil {
				respondError(w, http.StatusUnprocessableEntity, "resolução ainda incompleta")
				return
			}
			// Defensively ensure UserID is set (original draft may have been created in a
			// different request context, e.g. upload handler).
			if draft.UserID == uuid.Nil {
				draft.UserID = userID
			}
			if draft.PaymentMethodID != nil {
				if pm, perr := h.pmRepo.FindByID(*draft.PaymentMethodID, accountID); perr == nil && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
					y, m := models.ComputeBillingMonth(draft.Date, *pm.ClosingDay)
					my := int(m)
					draft.BillingYear = &y
					draft.BillingMonth = &my
				}
			}
			if err := h.txRepo.Create(&draft); err != nil {
				log.Error().Err(err).Msg("agent: confirm clarify_payment_method: persist tx")
				respondError(w, http.StatusInternalServerError, "internal error")
				return
			}
			created = append(created, draft)
		}

		saved := make([]savedTx, 0, len(created))
		for _, tx := range created {
			saved = append(saved, savedTx{Description: tx.Description, Category: tx.Category, Amount: tx.Amount})
		}
		h.recordConfirmationInHistory(userID, accountID, savedTransactionsContent(saved))
		respondJSON(w, http.StatusOK, map[string]any{
			"tool_name":    pa.ToolName,
			"transactions": created,
		})
		return
	}

	if pa.ToolName == "clarify_installment_card" {
		var payload installmentClarifyPayload
		if err := json.Unmarshal(pa.Payload, &payload); err != nil {
			respondError(w, http.StatusInternalServerError, "payload de pending action corrompido")
			return
		}
		if len(req.Resolutions) < 1 {
			respondError(w, http.StatusBadRequest, "é preciso escolher um cartão")
			return
		}
		pm, err := h.pmRepo.FindByID(req.Resolutions[0].PaymentMethodID, accountID)
		if err != nil || pm.Kind != models.KindCreditCard || pm.ClosingDay == nil {
			respondError(w, http.StatusUnprocessableEntity, "cartão de crédito inválido")
			return
		}
		cat, err := h.catRepo.FindBySlug(payload.Category)
		if err != nil {
			respondError(w, http.StatusBadRequest, "categoria inválida: "+payload.Category)
			return
		}
		desc := payload.Description
		if desc == "" {
			desc = payload.Merchant
		}
		purchase := &models.InstallmentPurchase{
			AccountID:       accountID,
			PaymentMethodID: pm.ID,
			CategoryID:      &cat.ID,
			Description:     desc,
			Merchant:        payload.Merchant,
			TotalAmount:     payload.TotalAmount,
			Installments:    payload.Installments,
			PurchaseDate:    parseDateNoon(payload.Date),
		}
		txs := installments.GenerateTransactions(purchase, *pm.ClosingDay, userID, cat.Slug)
		if err := h.installmentRepo.Create(purchase, txs); err != nil {
			log.Error().Err(err).Msg("agent: confirm clarify_installment_card: create")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		type createdTx struct {
			ID          string  `json:"id"`
			Description string  `json:"description"`
			Category    string  `json:"category"`
			Amount      float64 `json:"amount"`
		}
		created := make([]createdTx, len(txs))
		for i, tx := range txs {
			created[i] = createdTx{ID: tx.ID.String(), Description: tx.Description, Category: tx.Category, Amount: tx.Amount}
		}
		h.recordConfirmationInHistory(userID, accountID, fmt.Sprintf(
			"✓ Já registrei a compra parcelada no banco (não registrar de novo): R$ %.2f em %dx — %s (%s)",
			payload.TotalAmount, payload.Installments, desc, payload.Category,
		))
		respondJSON(w, http.StatusOK, map[string]any{
			"tool_name":    pa.ToolName,
			"transactions": created,
		})
		return
	}

	result, err := h.registry.ExecuteWrite(r.Context(), accountID, pa.ToolName, pa.Payload)
	if err != nil {
		log.Error().Err(err).Str("tool", pa.ToolName).Msg("agent: execute write tool")
		respondError(w, http.StatusInternalServerError, "failed to execute action")
		return
	}

	if result != "" {
		h.recordConfirmationInHistory(userID, accountID, "✓ "+result)
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"tool_name": pa.ToolName,
		"result":    result,
	})
}

// ListInsights handles GET /agent/insights?unread=true
// Response 200: []AgentInsight
func (h *AgentHandler) ListInsights(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	unreadOnly := r.URL.Query().Get("unread") == "true"

	insights, err := h.insightRepo.List(accountID, unreadOnly)
	if err != nil {
		log.Error().Err(err).Msg("agent: list insights")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, insights)
}

// MarkInsightRead handles PATCH /agent/insights/{id}/read
func (h *AgentHandler) MarkInsightRead(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid insight id")
		return
	}

	if err := h.insightRepo.MarkRead(id, accountID); err != nil {
		log.Error().Err(err).Msg("agent: mark insight read")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
