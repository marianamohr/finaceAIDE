package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/installments"
)

type createInstallmentPurchaseTool struct {
	repo    *repository.InstallmentPurchaseRepository
	pmRepo  *repository.PaymentMethodRepository
	catRepo *repository.CategoryRepository
}

func NewCreateInstallmentPurchaseTool(
	repo *repository.InstallmentPurchaseRepository,
	pmRepo *repository.PaymentMethodRepository,
	catRepo *repository.CategoryRepository,
) WriteTool {
	return &createInstallmentPurchaseTool{repo: repo, pmRepo: pmRepo, catRepo: catRepo}
}

func (t *createInstallmentPurchaseTool) Name() string { return "create_installment_purchase" }

func (t *createInstallmentPurchaseTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "create_installment_purchase",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Registra uma compra ÚNICA PARCELADA no cartão de crédito " +
			"(ex.: \"comprei uma TV de 3000 em 10x\"). Gera N parcelas em faturas consecutivas. " +
			"Use SOMENTE para compras parceladas no crédito — não use para gasto fixo/assinatura nem compra à vista. " +
			"Se o usuário tiver mais de um cartão de crédito e não disser qual, informe card_name com o nome que ele citar.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"total_amount": map[string]any{"type": "number", "description": "Valor TOTAL da compra (positivo)"},
				"installments": map[string]any{"type": "integer", "description": "Número de parcelas (>= 2)"},
				"category":     map[string]any{"type": "string", "description": "slug da categoria"},
				"description":  map[string]any{"type": "string"},
				"merchant":     map[string]any{"type": "string"},
				"date":         map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD); data da compra"},
				"card_name":    map[string]any{"type": "string", "description": "Nome do cartão de crédito, se o usuário citou"},
			},
			"required": []string{"total_amount", "installments", "category", "date"},
		}),
	}
}

type installmentToolInput struct {
	TotalAmount  float64 `json:"total_amount"`
	Installments int     `json:"installments"`
	Category     string  `json:"category"`
	Description  string  `json:"description"`
	Merchant     string  `json:"merchant"`
	Date         string  `json:"date"`
	CardName     string  `json:"card_name"`
}

func (t *createInstallmentPurchaseTool) Preview(input json.RawMessage) (string, error) {
	var p installmentToolInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_installment_purchase: parse preview: %w", err)
	}
	label := p.Description
	if label == "" {
		label = p.Merchant
	}
	return fmt.Sprintf("Compra parcelada: R$ %.2f em %dx — %s", math.Abs(p.TotalAmount), p.Installments, label), nil
}

// resolveCreditCard escolhe um cartão de crédito ativo. Devolve (pm, "", nil) quando
// resolvido; (nil, msg, nil) quando precisa que o usuário escolha; (nil, "", err) em erro.
func (t *createInstallmentPurchaseTool) resolveCreditCard(accountID uuid.UUID, cardName string) (*models.PaymentMethod, string, error) {
	pms, err := t.pmRepo.ListActive(accountID)
	if err != nil {
		return nil, "", err
	}
	var cards []models.PaymentMethod
	for _, pm := range pms {
		if pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			cards = append(cards, pm)
		}
	}
	if len(cards) == 0 {
		return nil, "", fmt.Errorf("nenhum cartão de crédito ativo com dia de fechamento — cadastre um antes de parcelar")
	}
	if cardName != "" {
		var matches []models.PaymentMethod
		for _, c := range cards {
			if strings.Contains(strings.ToLower(c.Name), strings.ToLower(cardName)) {
				matches = append(matches, c)
			}
		}
		if len(matches) == 1 {
			return &matches[0], "", nil
		}
	}
	if len(cards) == 1 {
		return &cards[0], "", nil
	}
	names := make([]string, len(cards))
	for i, c := range cards {
		names[i] = c.Name
	}
	return nil, fmt.Sprintf("Em qual cartão foi a compra parcelada? Opções: %s", strings.Join(names, ", ")), nil
}

func (t *createInstallmentPurchaseTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p installmentToolInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_installment_purchase: parse input: %w", err)
	}
	if p.TotalAmount <= 0 {
		return "", fmt.Errorf("total_amount deve ser maior que zero")
	}
	if p.Installments < 2 {
		return "", fmt.Errorf("installments deve ser >= 2")
	}
	date, err := time.Parse("2006-01-02", p.Date)
	if err != nil {
		return "", fmt.Errorf("date deve estar no formato YYYY-MM-DD")
	}
	cat, err := t.catRepo.FindBySlug(p.Category)
	if err != nil {
		return "", fmt.Errorf("categoria desconhecida: %q", p.Category)
	}

	card, ask, err := t.resolveCreditCard(accountID, p.CardName)
	if err != nil {
		return "", err
	}
	if ask != "" {
		// Defense in depth: the chat layer resolves the card (or asks via a
		// clarify_installment_card picker) BEFORE confirmation, so Execute should
		// only run with an unambiguous card. If we still can't resolve one, fail
		// loudly instead of silently creating nothing.
		return "", fmt.Errorf("%s", ask)
	}

	desc := p.Description
	if desc == "" {
		desc = p.Merchant
	}
	purchase := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: card.ID,
		CategoryID:      &cat.ID,
		Description:     desc,
		Merchant:        p.Merchant,
		TotalAmount:     p.TotalAmount,
		Installments:    p.Installments,
		PurchaseDate:    date,
	}
	userID := auth.UserIDFromContext(ctx)
	txs := installments.GenerateTransactions(purchase, *card.ClosingDay, userID, cat.Slug)
	if err := t.repo.Create(purchase, txs); err != nil {
		return "", fmt.Errorf("create_installment_purchase: persistir: %w", err)
	}
	return fmt.Sprintf("Compra parcelada criada: R$ %.2f em %dx no %s (parcela ~R$ %.2f).",
		p.TotalAmount, p.Installments, card.Name, p.TotalAmount/float64(p.Installments)), nil
}
