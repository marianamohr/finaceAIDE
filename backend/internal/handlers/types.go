package handlers

import (
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/services/transactions"
)

// replaceBatchPayload is the PendingAction payload for replace_transaction_batch.
type replaceBatchPayload struct {
	Items []agent.ReplaceTransactionInput `json:"items"`
}

// installmentClarifyPayload is the PendingAction payload for clarify_installment_card.
// Drafts carries a single synthetic draft so the existing ClarifyPaymentMethodCard UI
// can render the credit-card picker; the remaining fields hold the installment purchase
// spec needed to generate the parcels once the user picks a card.
type installmentClarifyPayload struct {
	Drafts       []transactions.ClarifyDraft `json:"drafts"`
	TotalAmount  float64                     `json:"total_amount"`
	Installments int                         `json:"installments"`
	Category     string                      `json:"category"`
	Description  string                      `json:"description"`
	Merchant     string                      `json:"merchant"`
	Date         string                      `json:"date"`
}
