package transactions

import "github.com/marianamohr/financas/internal/models"

// ClarifyPayload é o jsonb gravado em PendingAction.Payload quando
// ToolName == "clarify_payment_method". Uma PendingAction pode carregar vários
// drafts (uploads de OCR geram listas).
type ClarifyPayload struct {
	Drafts []ClarifyDraft `json:"drafts"`
}

// ClarifyDraft agrupa um rascunho de transação com a necessidade de clarificação
// que impediu sua persistência.
type ClarifyDraft struct {
	Tx    models.Transaction `json:"tx"`
	Needs ClarificationNeed  `json:"needs"`
}
