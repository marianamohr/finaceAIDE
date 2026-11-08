package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/rag"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/rs/zerolog/log"
)

const chatHistoryLimit = 20
const maxToolIterations = 5
const pendingActionTTL = 30 * time.Minute

type ChatHandler struct {
	aiProvider  ai.Provider
	ragBuilder  *rag.Builder
	chatRepo    *repository.ChatRepository
	txRepo      *repository.TransactionRepository
	userRepo    *repository.UserRepository
	registry    *agent.Registry
	pendingRepo *repository.PendingActionRepository
	currencySvc *currency.Service
	pmRepo      *repository.PaymentMethodRepository
	validator   *transactions.Validator
}

func NewChatHandler(
	provider ai.Provider,
	ragBuilder *rag.Builder,
	chatRepo *repository.ChatRepository,
	txRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
	registry *agent.Registry,
	pendingRepo *repository.PendingActionRepository,
	currencySvc *currency.Service,
	pmRepo *repository.PaymentMethodRepository,
	validator *transactions.Validator,
) *ChatHandler {
	return &ChatHandler{
		aiProvider:  provider,
		ragBuilder:  ragBuilder,
		chatRepo:    chatRepo,
		txRepo:      txRepo,
		userRepo:    userRepo,
		registry:    registry,
		pendingRepo: pendingRepo,
		currencySvc: currencySvc,
		pmRepo:      pmRepo,
		validator:   validator,
	}
}

type chatRequest struct {
	Message        string `json:"message"`
	ClientDatetime string `json:"client_datetime"`
}

type pendingActionResponse struct {
	ID       string          `json:"id"`
	ToolName string          `json:"tool_name"`
	Preview  string          `json:"preview"`
	Payload  json.RawMessage `json:"payload,omitempty"`
}

type chatResponse struct {
	Reply         string                 `json:"reply"`
	PendingAction *pendingActionResponse `json:"pending_action,omitempty"`
}

func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	accountID := auth.AccountIDFromContext(r.Context())

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		respondError(w, http.StatusBadRequest, "message is required")
		return
	}

	userMsg := &models.ChatMessage{
		UserID:    userID,
		AccountID: accountID,
		Role:      "user",
		Content:   req.Message,
	}
	if err := h.chatRepo.Save(userMsg); err != nil {
		log.Error().Err(err).Msg("chat: save user message")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		log.Error().Err(err).Msg("chat: fetch user")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	provider := h.aiProvider
	if p, ok := h.aiProvider.(*ai.AnthropicProvider); ok && user.UserAPIKey != "" {
		provider = p.WithUserKey(user.UserAPIKey)
	}

	baseCurrency := "BRL"
	if c, err := h.currencySvc.GetUserBaseCurrency(r.Context(), userID.String()); err == nil && c != "" {
		baseCurrency = c
	}

	systemPrompt, err := h.ragBuilder.Build(accountID, user.CategoryContext, req.ClientDatetime, baseCurrency)
	if err != nil {
		log.Error().Err(err).Msg("chat: build rag context")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	history, err := h.chatRepo.HistoryByAccount(accountID, chatHistoryLimit)
	if err != nil {
		log.Error().Err(err).Msg("chat: fetch history")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	messages := buildHistoryMessages(history)

	finalReply, pendingAction, err := h.runAgentLoop(r.Context(), accountID, systemPrompt, messages, provider)
	if err != nil {
		log.Error().Err(err).Msg("chat: agent loop")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// When a pending action exists, save a neutral acknowledgement so the history
	// doesn't contain "Vou registrar X e Y..." which would cause the model to
	// re-emit those tool calls on the next turn.
	savedReply := finalReply
	if pendingAction != nil {
		savedReply = "Ok."
	}
	if savedReply != "" {
		assistantMsg := &models.ChatMessage{
			UserID:    userID,
			AccountID: accountID,
			Role:      "assistant",
			Content:   savedReply,
		}
		if err := h.chatRepo.Save(assistantMsg); err != nil {
			log.Error().Err(err).Msg("chat: save assistant message")
		}
	}

	resp := chatResponse{Reply: finalReply}
	if pendingAction != nil {
		resp.PendingAction = pendingAction
	}
	respondJSON(w, http.StatusOK, resp)
}

func (h *ChatHandler) runAgentLoop(
	ctx context.Context,
	accountID uuid.UUID,
	systemPrompt string,
	messages []ai.Message,
	provider ai.Provider,
) (string, *pendingActionResponse, error) {
	var finalContent string

	for i := 0; i < maxToolIterations; i++ {
		resp, err := provider.ChatWithTools(ctx, ai.ChatWithToolsRequest{
			Model:        ai.SelectModel(ai.TaskFinancialChat),
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        h.registry.Schemas(),
			MaxTokens:    1024,
		})
		if err != nil {
			return "", nil, fmt.Errorf("agent loop: call ai: %w", err)
		}

		finalContent = resp.Content

		if resp.StopReason == "end_turn" || len(resp.ToolUses) == 0 {
			break
		}

		// Build the assistant turn with proper tool_use content blocks.
		assistantBlocks := buildAssistantBlocks(resp.Content, resp.ToolUses)
		assistantBlocksJSON, err := json.Marshal(assistantBlocks)
		if err != nil {
			return "", nil, fmt.Errorf("agent loop: marshal assistant blocks: %w", err)
		}
		messages = append(messages, ai.Message{
			Role:          "assistant",
			ContentBlocks: assistantBlocksJSON,
		})

		// Process each tool call.
		// Collect all create_transaction and replace_transaction calls to batch them into a single PendingAction.
		var (
			toolResults       []map[string]any
			batchTxInputs     []createTransactionInput
			replaceInputs     []agent.ReplaceTransactionInput
			installmentBlocks []ai.ToolUseBlock
			otherWriteBlocks  []ai.ToolUseBlock
		)
		for _, block := range resp.ToolUses {
			if block.Name == "create_transaction" {
				var input createTransactionInput
				if err := json.Unmarshal(block.Input, &input); err == nil {
					batchTxInputs = append(batchTxInputs, input)
					continue
				}
			}
			if block.Name == "replace_transaction" {
				var input agent.ReplaceTransactionInput
				if err := json.Unmarshal(block.Input, &input); err == nil {
					replaceInputs = append(replaceInputs, input)
					continue
				}
			}
			if block.Name == "create_installment_purchase" {
				installmentBlocks = append(installmentBlocks, block)
				continue
			}
			if h.registry.IsWrite(block.Name) {
				otherWriteBlocks = append(otherWriteBlocks, block)
				continue
			}
			result, err := h.registry.ExecuteRead(ctx, accountID, block)
			if err != nil {
				log.Error().Err(err).Str("tool", block.Name).Msg("chat: execute read tool")
				result = fmt.Sprintf("Erro ao executar %s", block.Name)
			}
			toolResults = append(toolResults, map[string]any{
				"type":        "tool_result",
				"tool_use_id": block.ID,
				"content":     result,
			})
		}

		// Route every create_transaction through the validator BEFORE creating the
		// confirmation PendingAction. Resolved drafts go into a create_transaction_batch
		// (fully populated, no further validation on confirm). Drafts that need PM
		// clarification go into a clarify_payment_method PA so the UI can render chips.
		if len(batchTxInputs) > 0 {
			resolvedDrafts := make([]models.Transaction, 0, len(batchTxInputs))
			clarifyDrafts := make([]transactions.ClarifyDraft, 0)
			for _, inp := range batchTxInputs {
				draft := h.buildChatDraft(ctx, accountID, inp)
				hint := transactions.PaymentHint{Slug: inp.PaymentModeSlug}
				need, verr := h.validator.ValidateAndEnrich(ctx, accountID, draft, hint)
				if verr != nil {
					return "", nil, fmt.Errorf("agent loop: validate draft: %w", verr)
				}
				if need != nil {
					clarifyDrafts = append(clarifyDrafts, transactions.ClarifyDraft{Tx: *draft, Needs: *need})
					continue
				}
				if draft.PaymentMethodID != nil {
					if pm, perr := h.pmRepo.FindByID(*draft.PaymentMethodID, accountID); perr == nil && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
						y, m := models.ComputeBillingMonth(draft.Date, *pm.ClosingDay)
						my := int(m)
						draft.BillingYear = &y
						draft.BillingMonth = &my
					}
				}
				resolvedDrafts = append(resolvedDrafts, *draft)
			}

			if len(clarifyDrafts) > 0 {
				payload, err := json.Marshal(transactions.ClarifyPayload{Drafts: clarifyDrafts})
				if err != nil {
					return "", nil, fmt.Errorf("agent loop: marshal clarify payload: %w", err)
				}
				preview := clarifyDrafts[0].Needs.Prompt
				if len(clarifyDrafts) > 1 {
					preview = fmt.Sprintf("%d transação(ões) precisam de forma de pagamento", len(clarifyDrafts))
				}
				pa := &models.PendingAction{
					AccountID: accountID,
					ToolName:  "clarify_payment_method",
					Payload:   payload,
					Preview:   preview,
					ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
				}
				if err := h.pendingRepo.Create(pa); err != nil {
					return "", nil, fmt.Errorf("agent loop: save clarify pending: %w", err)
				}
				if len(resolvedDrafts) > 0 {
					log.Warn().Int("dropped", len(resolvedDrafts)).Msg("chat: mixed clarify+resolved drafts; resolved drafts not persisted this turn")
				}
				return finalContent, &pendingActionResponse{
					ID:       pa.ID.String(),
					ToolName: pa.ToolName,
					Preview:  pa.Preview,
					Payload:  payload,
				}, nil
			}

			payload, err := json.Marshal(resolvedDrafts)
			if err != nil {
				return "", nil, fmt.Errorf("agent loop: marshal batch: %w", err)
			}
			preview := buildBatchPreview(resolvedDrafts)
			pa := &models.PendingAction{
				AccountID: accountID,
				ToolName:  "create_transaction_batch",
				Payload:   payload,
				Preview:   preview,
				ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
			}
			if err := h.pendingRepo.Create(pa); err != nil {
				return "", nil, fmt.Errorf("agent loop: save pending action batch: %w", err)
			}
			return finalContent, &pendingActionResponse{
				ID:       pa.ID.String(),
				ToolName: pa.ToolName,
				Preview:  pa.Preview,
			}, nil
		}

		// Batch all replace_transaction calls into one PendingAction.
		if len(replaceInputs) > 0 {
			previewLines := []string{fmt.Sprintf("Substituir %d transação(ões):", len(replaceInputs))}
			for _, inp := range replaceInputs {
				previewLines = append(previewLines, agent.PreviewReplaceInput(inp))
			}
			preview := strings.Join(previewLines, "\n")
			payload, err := json.Marshal(replaceBatchPayload{Items: replaceInputs})
			if err != nil {
				return "", nil, fmt.Errorf("agent loop: marshal replace batch: %w", err)
			}
			pa := &models.PendingAction{
				AccountID: accountID,
				ToolName:  "replace_transaction_batch",
				Payload:   payload,
				Preview:   preview,
				ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
			}
			if err := h.pendingRepo.Create(pa); err != nil {
				return "", nil, fmt.Errorf("agent loop: save replace batch: %w", err)
			}
			return finalContent, &pendingActionResponse{
				ID:       pa.ID.String(),
				ToolName: pa.ToolName,
				Preview:  pa.Preview,
			}, nil
		}

		// Route create_installment_purchase through credit-card resolution BEFORE
		// creating the confirmation PendingAction (mirrors create_transaction). When the
		// card is unambiguous, fall through to the generic write-tool confirm. When the
		// account has multiple credit cards and the user didn't pick one, surface a
		// clarify_installment_card PA so the UI renders a card picker.
		if len(installmentBlocks) > 0 {
			block := installmentBlocks[0]
			var in struct {
				TotalAmount  float64 `json:"total_amount"`
				Installments int     `json:"installments"`
				Category     string  `json:"category"`
				Description  string  `json:"description"`
				Merchant     string  `json:"merchant"`
				Date         string  `json:"date"`
				CardName     string  `json:"card_name"`
			}
			if err := json.Unmarshal(block.Input, &in); err != nil {
				return "", nil, fmt.Errorf("agent loop: parse installment input: %w", err)
			}

			pms, err := h.pmRepo.ListActive(accountID)
			if err != nil {
				return "", nil, fmt.Errorf("agent loop: list payment methods: %w", err)
			}
			var cards []models.PaymentMethod
			for _, pm := range pms {
				if pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
					cards = append(cards, pm)
				}
			}

			var resolved *models.PaymentMethod
			if in.CardName != "" {
				var matches []models.PaymentMethod
				for i := range cards {
					if strings.Contains(strings.ToLower(cards[i].Name), strings.ToLower(in.CardName)) {
						matches = append(matches, cards[i])
					}
				}
				if len(matches) == 1 {
					resolved = &matches[0]
				}
			}
			if resolved == nil && len(cards) == 1 {
				resolved = &cards[0]
			}

			if resolved != nil || len(cards) == 0 {
				// Unambiguous (or no card at all — let Execute surface the error):
				// hand off to the generic write-tool confirmation flow below.
				otherWriteBlocks = append(otherWriteBlocks, block)
			} else {
				options := make([]transactions.PaymentMethodOption, len(cards))
				for i, c := range cards {
					options[i] = transactions.PaymentMethodOption{ID: c.ID, Label: c.Name, Kind: string(c.Kind)}
				}
				desc := in.Description
				if desc == "" {
					desc = in.Merchant
				}
				draft := transactions.ClarifyDraft{
					Tx: models.Transaction{
						Description: desc,
						Amount:      -in.TotalAmount,
						Category:    in.Category,
						Date:        parseDateNoon(in.Date),
					},
					Needs: transactions.ClarificationNeed{
						Field:   "payment_method",
						Kind:    transactions.ClarificationNotMentioned,
						Prompt:  fmt.Sprintf("Em qual cartão foi a compra parcelada em %dx?", in.Installments),
						Options: options,
					},
				}
				payload, err := json.Marshal(installmentClarifyPayload{
					Drafts:       []transactions.ClarifyDraft{draft},
					TotalAmount:  in.TotalAmount,
					Installments: in.Installments,
					Category:     in.Category,
					Description:  in.Description,
					Merchant:     in.Merchant,
					Date:         in.Date,
				})
				if err != nil {
					return "", nil, fmt.Errorf("agent loop: marshal installment clarify: %w", err)
				}
				pa := &models.PendingAction{
					AccountID: accountID,
					ToolName:  "clarify_installment_card",
					Payload:   payload,
					Preview:   draft.Needs.Prompt,
					ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
				}
				if err := h.pendingRepo.Create(pa); err != nil {
					return "", nil, fmt.Errorf("agent loop: save installment clarify pending: %w", err)
				}
				return finalContent, &pendingActionResponse{
					ID:       pa.ID.String(),
					ToolName: pa.ToolName,
					Preview:  pa.Preview,
					Payload:  payload,
				}, nil
			}
		}

		if len(otherWriteBlocks) > 0 {
			block := otherWriteBlocks[0]
			preview, err := h.registry.PreviewWrite(block.Name, block.Input)
			if err != nil {
				log.Error().Err(err).Str("tool", block.Name).Msg("chat: preview write tool")
				preview = fmt.Sprintf("Executar %s", block.Name)
			}
			pa := &models.PendingAction{
				AccountID: accountID,
				ToolName:  block.Name,
				Payload:   block.Input,
				Preview:   preview,
				ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
			}
			if err := h.pendingRepo.Create(pa); err != nil {
				return "", nil, fmt.Errorf("agent loop: save pending action: %w", err)
			}
			return finalContent, &pendingActionResponse{
				ID:       pa.ID.String(),
				ToolName: pa.ToolName,
				Preview:  pa.Preview,
			}, nil
		}

		// Feed tool results back as a user turn with proper content blocks.
		if len(toolResults) > 0 {
			toolResultsJSON, err := json.Marshal(toolResults)
			if err != nil {
				return "", nil, fmt.Errorf("agent loop: marshal tool results: %w", err)
			}
			messages = append(messages, ai.Message{
				Role:          "user",
				ContentBlocks: toolResultsJSON,
			})
		}
	}

	return finalContent, nil, nil
}

type createTransactionInput struct {
	Amount          float64 `json:"amount"`
	Category        string  `json:"category"`
	Description     string  `json:"description"`
	Date            string  `json:"date"`
	Merchant        string  `json:"merchant"`
	PaymentModeSlug string  `json:"payment_mode_slug"`
}

// chatUserCurrency lookups the user's base currency, defaulting to "BRL".
func chatUserCurrency(ctx context.Context, svc *currency.Service) string {
	userID := auth.UserIDFromContext(ctx)
	c, err := svc.GetUserBaseCurrency(ctx, userID.String())
	if err != nil || c == "" {
		return "BRL"
	}
	return c
}

func chatLocaleFor(c string) string {
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

func deriveTypeFromAmount(amount float64) string {
	if amount > 0 {
		return "income"
	}
	return "expense"
}

// buildChatDraft mounts a fully-populated Transaction draft from a chat input,
// matching the shape the validator expects (Type, Source, Category slug, etc.).
func (h *ChatHandler) buildChatDraft(ctx context.Context, accountID uuid.UUID, inp createTransactionInput) *models.Transaction {
	cur := chatUserCurrency(ctx, h.currencySvc)
	return &models.Transaction{
		AccountID:      accountID,
		UserID:         auth.UserIDFromContext(ctx),
		Amount:         inp.Amount,
		Category:       inp.Category,
		Description:    inp.Description,
		Merchant:       inp.Merchant,
		Date:           parseDateNoon(inp.Date),
		Type:           deriveTypeFromAmount(inp.Amount),
		Source:         "chat",
		Currency:       cur,
		Locale:         chatLocaleFor(cur),
		OriginalAmount: math.Abs(inp.Amount),
	}
}

func buildBatchPreview(txs []models.Transaction) string {
	lines := make([]string, 0, len(txs)+1)
	lines = append(lines, fmt.Sprintf("Criar %d transação(ões):", len(txs)))
	for _, tx := range txs {
		label := tx.Description
		if label == "" {
			label = tx.Merchant
		}
		if label == "" {
			label = tx.Category
		}
		lines = append(lines, fmt.Sprintf("  • R$ %.2f — %s (%s)", math.Abs(tx.Amount), label, tx.Category))
	}
	return strings.Join(lines, "\n")
}

// buildHistoryMessages maps persisted chat history to the model's message array.
// It enforces two Anthropic API invariants the raw history can violate:
//   - the first message must come from the user (the bounded history window may
//     start on an assistant turn, and /agent/confirm appends assistant-only turns);
//   - consecutive turns with the same role are coalesced into one, so an "Ok."
//     acknowledgement followed by a "✓ registrei..." confirmation stays a single turn.
func buildHistoryMessages(history []models.ChatMessage) []ai.Message {
	messages := make([]ai.Message, 0, len(history))
	for _, m := range history {
		if len(messages) == 0 && m.Role != "user" {
			continue
		}
		if n := len(messages); n > 0 && messages[n-1].Role == m.Role {
			messages[n-1].Content = strings.TrimSpace(messages[n-1].Content + "\n" + m.Content)
			continue
		}
		messages = append(messages, ai.Message{Role: m.Role, Content: m.Content})
	}
	return messages
}

// buildAssistantBlocks constructs the Anthropic content block array for an assistant turn
// that includes both text and tool_use blocks.
func buildAssistantBlocks(text string, toolUses []ai.ToolUseBlock) []map[string]any {
	var blocks []map[string]any
	if strings.TrimSpace(text) != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	for _, tu := range toolUses {
		block := map[string]any{
			"type":  "tool_use",
			"id":    tu.ID,
			"name":  tu.Name,
			"input": tu.Input,
		}
		blocks = append(blocks, block)
	}
	return blocks
}
