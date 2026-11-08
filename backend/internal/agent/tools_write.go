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
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/transactions"
)

// currencyToLocale maps ISO 4217 currency codes to display locale codes.
// Defaults to "BR" for unknown currencies.
func currencyToLocale(c string) string {
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

// userCurrency returns the user's base currency from context, defaulting to "BRL" on error.
func userCurrency(ctx context.Context, svc *currency.Service) string {
	userID := auth.UserIDFromContext(ctx)
	c, err := svc.GetUserBaseCurrency(ctx, userID.String())
	if err != nil || c == "" {
		return "BRL"
	}
	return c
}

// resolveCategoryForAmount looks up the category by slug and validates that it is
// compatible with the given amount sign. Returns the Category or an error with a
// human-readable message that the AI can use to self-correct.
func resolveCategoryForAmount(catRepo *repository.CategoryRepository, slug string, amount float64) (*models.Category, error) {
	cat, err := catRepo.FindBySlug(slug)
	if err != nil {
		return nil, fmt.Errorf("categoria desconhecida: %q — use apenas os slugs listados em CATEGORIAS VÁLIDAS", slug)
	}
	if !cat.IsCompatibleWith(amount) {
		sign := "positivo (receita)"
		if amount < 0 {
			sign = "negativo (despesa)"
		}
		return nil, fmt.Errorf("categoria %q (tipo: %q) incompatível com amount %s (%.2f) — use a categoria correta para o tipo de transação", slug, cat.Type, sign, amount)
	}
	return cat, nil
}

// ── create_transaction ────────────────────────────────────────────────────────

type createTransactionTool struct {
	repo        *repository.TransactionRepository
	catRepo     *repository.CategoryRepository
	currencySvc *currency.Service
	pmRepo      *repository.PaymentMethodRepository
	paRepo      *repository.PendingActionRepository
	validator   *transactions.Validator
}

func NewCreateTransactionTool(
	repo *repository.TransactionRepository,
	catRepo *repository.CategoryRepository,
	currencySvc *currency.Service,
	pmRepo *repository.PaymentMethodRepository,
	paRepo *repository.PendingActionRepository,
	validator *transactions.Validator,
) WriteTool {
	return &createTransactionTool{
		repo:        repo,
		catRepo:     catRepo,
		currencySvc: currencySvc,
		pmRepo:      pmRepo,
		paRepo:      paRepo,
		validator:   validator,
	}
}

func (t *createTransactionTool) Name() string { return "create_transaction" }

func (t *createTransactionTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "create_transaction",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os detalhes e aguarde aprovação explícita antes de usar. " +
			"Cria uma transação financeira manual a partir de informações fornecidas pelo usuário no chat.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amount":            map[string]any{"type": "number", "description": "Valor negativo para despesa, positivo para receita"},
				"category":          map[string]any{"type": "string"},
				"description":       map[string]any{"type": "string"},
				"date":              map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD)"},
				"merchant":          map[string]any{"type": "string"},
				"payment_mode_slug": map[string]any{"type": "string", "description": "Forma de pagamento: credito, debito, pix, vr, boleto. Omitir se não informado pelo usuário."},
			},
			"required": []string{"amount", "category", "date"},
		}),
	}
}

func (t *createTransactionTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
		Merchant    string  `json:"merchant"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_transaction: parse preview: %w", err)
	}
	label := p.Description
	if label == "" {
		label = p.Merchant
	}
	if label == "" {
		label = p.Category
	}
	return fmt.Sprintf("Criar transação: R$ %.2f — %s (%s)", math.Abs(p.Amount), label, p.Category), nil
}

func (t *createTransactionTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Amount          float64 `json:"amount"`
		Category        string  `json:"category"`
		Description     string  `json:"description"`
		Date            string  `json:"date"`
		Merchant        string  `json:"merchant"`
		PaymentModeSlug string  `json:"payment_mode_slug"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_transaction: parse input: %w", err)
	}

	// Derive transaction type from amount sign; transfer is never created via chat.
	txType := "income"
	if p.Amount < 0 {
		txType = "expense"
	}

	cur := userCurrency(ctx, t.currencySvc)
	draft := &models.Transaction{
		AccountID:      accountID,
		UserID:         auth.UserIDFromContext(ctx),
		Amount:         p.Amount,
		Category:       p.Category,
		Description:    p.Description,
		Merchant:       p.Merchant,
		Date:           parseDateNoon(p.Date),
		Source:         "chat",
		Type:           txType,
		Currency:       cur,
		Locale:         currencyToLocale(cur),
		OriginalAmount: math.Abs(p.Amount),
	}

	hint := transactions.PaymentHint{
		Slug: p.PaymentModeSlug,
	}
	need, err := t.validator.ValidateAndEnrich(ctx, accountID, draft, hint)
	if err != nil {
		return "", fmt.Errorf("create_transaction: %w", err)
	}

	if need != nil {
		paID, perr := t.savePendingClarification(accountID, draft, need)
		if perr != nil {
			return "", fmt.Errorf("create_transaction: salvar pending de clarificação: %w", perr)
		}
		return fmt.Sprintf("Preciso confirmar a forma de pagamento (pending_action=%s)", paID), nil
	}

	if draft.PaymentMethodID != nil {
		if pm, perr := t.pmRepo.FindByID(*draft.PaymentMethodID, accountID); perr == nil && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			y, m := models.ComputeBillingMonth(draft.Date, *pm.ClosingDay)
			my := int(m)
			draft.BillingYear = &y
			draft.BillingMonth = &my
		}
	}

	if err := t.repo.Create(draft); err != nil {
		return "", fmt.Errorf("create_transaction: persistir: %w", err)
	}
	return fmt.Sprintf("Transação criada: R$ %.2f — %s", math.Abs(p.Amount), draft.Category), nil
}

// savePendingClarification persiste draft + ClarificationNeed em uma única
// PendingAction com ToolName="clarify_payment_method". /agent/confirm (Tarefa 10)
// resolve isso re-executando o validador com o PM escolhido.
func (t *createTransactionTool) savePendingClarification(accountID uuid.UUID, draft *models.Transaction, need *transactions.ClarificationNeed) (uuid.UUID, error) {
	payload, err := json.Marshal(transactions.ClarifyPayload{
		Drafts: []transactions.ClarifyDraft{{Tx: *draft, Needs: *need}},
	})
	if err != nil {
		return uuid.Nil, err
	}
	pa := &models.PendingAction{
		AccountID: accountID,
		ToolName:  "clarify_payment_method",
		Payload:   payload,
		Preview:   need.Prompt,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := t.paRepo.Create(pa); err != nil {
		return uuid.Nil, err
	}
	return pa.ID, nil
}

// ── update_transaction_category ───────────────────────────────────────────────

type updateTransactionCategoryTool struct {
	repo    *repository.TransactionRepository
	catRepo *repository.CategoryRepository
}

func NewUpdateTransactionCategoryTool(repo *repository.TransactionRepository, catRepo *repository.CategoryRepository) WriteTool {
	return &updateTransactionCategoryTool{repo: repo, catRepo: catRepo}
}

func (t *updateTransactionCategoryTool) Name() string { return "update_transaction_category" }

func (t *updateTransactionCategoryTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "update_transaction_category",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os itens afetados e aguarde aprovação explícita antes de usar. " +
			"Recategoriza uma ou várias transações.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transaction_ids": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"category": map[string]any{"type": "string"},
			},
			"required": []string{"transaction_ids", "category"},
		}),
	}
}

func (t *updateTransactionCategoryTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		TransactionIDs []string `json:"transaction_ids"`
		Category       string   `json:"category"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("update_transaction_category: parse preview: %w", err)
	}
	return fmt.Sprintf("Recategorizar %d transação(ões) para \"%s\"", len(p.TransactionIDs), p.Category), nil
}

func (t *updateTransactionCategoryTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		TransactionIDs []string `json:"transaction_ids"`
		Category       string   `json:"category"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("update_transaction_category: parse input: %w", err)
	}
	// Validate slug exists (amount-agnostic for recategorization — user intent overrides sign check)
	cat, err := t.catRepo.FindBySlug(p.Category)
	if err != nil {
		return "", fmt.Errorf("update_transaction_category: categoria desconhecida: %q", p.Category)
	}
	var updated int
	for _, idStr := range p.TransactionIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			continue
		}
		tx, err := t.repo.FindByID(id, accountID)
		if err != nil {
			continue
		}
		tx.Category = cat.Slug
		tx.CategoryID = &cat.ID
		if err := t.repo.Update(tx, accountID); err != nil {
			continue
		}
		updated++
	}
	return fmt.Sprintf("%d transação(ões) recategorizadas para \"%s\"", updated, cat.Slug), nil
}

// ── create_merchant_alias ─────────────────────────────────────────────────────

type createMerchantAliasTool struct {
	repo *repository.MerchantAliasRepository
}

func NewCreateMerchantAliasTool(repo *repository.MerchantAliasRepository) WriteTool {
	return &createMerchantAliasTool{repo: repo}
}

func (t *createMerchantAliasTool) Name() string { return "create_merchant_alias" }

func (t *createMerchantAliasTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "create_merchant_alias",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os itens afetados e aguarde aprovação explícita antes de usar. " +
			"Cria um alias para normalizar o nome de um merchant.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"raw_name":     map[string]any{"type": "string"},
				"display_name": map[string]any{"type": "string"},
			},
			"required": []string{"raw_name", "display_name"},
		}),
	}
}

func (t *createMerchantAliasTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		RawName     string `json:"raw_name"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_merchant_alias: parse preview: %w", err)
	}
	return fmt.Sprintf("Criar alias: \"%s\" → \"%s\"", p.RawName, p.DisplayName), nil
}

func (t *createMerchantAliasTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		RawName     string `json:"raw_name"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_merchant_alias: parse input: %w", err)
	}
	alias := &models.MerchantAlias{
		AccountID:   accountID,
		RawName:     p.RawName,
		DisplayName: p.DisplayName,
		IsCustom:    true,
	}
	if err := t.repo.Create(alias); err != nil {
		return "", err
	}
	return fmt.Sprintf("Alias criado: \"%s\" → \"%s\"", p.RawName, p.DisplayName), nil
}

// ── create_goal ───────────────────────────────────────────────────────────────

type createGoalTool struct{ repo *repository.GoalRepository }

func NewCreateGoalTool(repo *repository.GoalRepository) WriteTool {
	return &createGoalTool{repo: repo}
}

func (t *createGoalTool) Name() string { return "create_goal" }

func (t *createGoalTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "create_goal",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os detalhes e aguarde aprovação explícita antes de usar. " +
			"Cria uma reserva/caixinha financeira.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":                 map[string]any{"type": "string"},
				"target_amount":        map[string]any{"type": "number", "description": "Valor alvo da reserva"},
				"initial_amount":       map[string]any{"type": "number", "description": "Saldo inicial (ex: venda de bens). Padrão: 0"},
				"monthly_contribution": map[string]any{"type": "number", "description": "Aporte mensal fixo. 0 = variável"},
				"annual_rate":          map[string]any{"type": "number", "description": "Taxa de rendimento anual (ex: 0.045 = 4.5%)"},
				"allocation_pct":       map[string]any{"type": "number", "description": "Percentual do bucket 'futuro' destinado a esta reserva (ex: 0.15 = 15%)"},
				"goal_type":            map[string]any{"type": "string", "description": "\"emergency_global\" ou \"custom\" (padrão)"},
				"institution":          map[string]any{"type": "string", "description": "Onde o dinheiro está guardado (ex: \"Nubank\", \"Clear\", \"XP\")"},
				"account_type":         map[string]any{"type": "string", "description": "Tipo de conta/produto (ex: \"conta corrente\", \"CDB\", \"ações\", \"fundo\")"},
				"deadline":             map[string]any{"type": "string", "description": "ISO 8601 date (YYYY-MM-DD), opcional"},
			},
			"required": []string{"name", "target_amount"},
		}),
	}
}

func (t *createGoalTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		Name         string  `json:"name"`
		TargetAmount float64 `json:"target_amount"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_goal: parse preview: %w", err)
	}
	return fmt.Sprintf("Criar meta \"%s\" com alvo de R$ %.2f", p.Name, p.TargetAmount), nil
}

func (t *createGoalTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Name                string  `json:"name"`
		TargetAmount        float64 `json:"target_amount"`
		InitialAmount       float64 `json:"initial_amount"`
		MonthlyContribution float64 `json:"monthly_contribution"`
		AnnualRate          float64 `json:"annual_rate"`
		AllocationPct       float64 `json:"allocation_pct"`
		GoalType            string  `json:"goal_type"`
		Institution         string  `json:"institution"`
		AccountType         string  `json:"account_type"`
		Deadline            string  `json:"deadline"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("create_goal: parse input: %w", err)
	}
	goalType := p.GoalType
	if goalType == "" {
		goalType = "custom"
	}
	goal := &models.Goal{
		AccountID:           accountID,
		Name:                p.Name,
		TargetAmount:        p.TargetAmount,
		InitialAmount:       p.InitialAmount,
		CurrentAmount:       p.InitialAmount,
		MonthlyContribution: p.MonthlyContribution,
		AnnualRate:          p.AnnualRate,
		AllocationPct:       p.AllocationPct,
		GoalType:            goalType,
		Institution:         p.Institution,
		AccountType:         p.AccountType,
	}
	if p.Deadline != "" {
		d, err := time.Parse("2006-01-02", p.Deadline)
		if err == nil {
			goal.Deadline = &d
		}
	}
	if err := t.repo.Create(goal); err != nil {
		return "", err
	}
	return fmt.Sprintf("Reserva \"%s\" criada com alvo de %.2f e saldo inicial de %.2f", goal.Name, goal.TargetAmount, goal.InitialAmount), nil
}

// ── update_goal_progress ──────────────────────────────────────────────────────

type updateGoalProgressTool struct{ repo *repository.GoalRepository }

func NewUpdateGoalProgressTool(repo *repository.GoalRepository) WriteTool {
	return &updateGoalProgressTool{repo: repo}
}

func (t *updateGoalProgressTool) Name() string { return "update_goal_progress" }

func (t *updateGoalProgressTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "update_goal_progress",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os detalhes e aguarde aprovação explícita antes de usar. " +
			"Atualiza o progresso ou os dados de uma meta existente.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"goal_id":        map[string]any{"type": "string"},
				"current_amount": map[string]any{"type": "number"},
			},
			"required": []string{"goal_id", "current_amount"},
		}),
	}
}

func (t *updateGoalProgressTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		GoalID        string  `json:"goal_id"`
		CurrentAmount float64 `json:"current_amount"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("update_goal_progress: parse preview: %w", err)
	}
	return fmt.Sprintf("Atualizar progresso da meta %s para R$ %.2f", p.GoalID, p.CurrentAmount), nil
}

func (t *updateGoalProgressTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		GoalID        string  `json:"goal_id"`
		CurrentAmount float64 `json:"current_amount"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("update_goal_progress: parse input: %w", err)
	}
	id, err := uuid.Parse(p.GoalID)
	if err != nil {
		return "", fmt.Errorf("update_goal_progress: invalid goal_id: %w", err)
	}
	goal := &models.Goal{ID: id, CurrentAmount: p.CurrentAmount}
	if err := t.repo.Update(goal, accountID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Progresso da meta atualizado para R$ %.2f", p.CurrentAmount), nil
}

// ── add_goal_transaction ──────────────────────────────────────────────────────

type addGoalTransactionTool struct {
	repo        *repository.GoalRepository
	txRepo      *repository.TransactionRepository
	currencySvc *currency.Service
}

func NewAddGoalTransactionTool(repo *repository.GoalRepository, txRepo *repository.TransactionRepository, currencySvc *currency.Service) WriteTool {
	return &addGoalTransactionTool{repo: repo, txRepo: txRepo, currencySvc: currencySvc}
}

func (t *addGoalTransactionTool) Name() string { return "add_goal_transaction" }

func (t *addGoalTransactionTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "add_goal_transaction",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os detalhes e aguarde aprovação explícita antes de usar. " +
			"Registra um movimento em uma reserva: aporte, rendimento ou saque. " +
			"Aportes (source=manual/chat) criam automaticamente despesa na conta principal. " +
			"Saques (source=withdrawal) criam automaticamente entrada + despesa na conta principal.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"goal_id":          map[string]any{"type": "string", "description": "UUID da reserva"},
				"amount":           map[string]any{"type": "number", "description": "Positivo para aporte/rendimento, negativo para saque"},
				"description":      map[string]any{"type": "string"},
				"date":             map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD)"},
				"source":           map[string]any{"type": "string", "description": "\"manual\" | \"initial\" | \"yield\" | \"withdrawal\""},
				"expense_category": map[string]any{"type": "string", "description": "Categoria da despesa gerada no saque (obrigatório se source=withdrawal)"},
			},
			"required": []string{"goal_id", "amount", "source"},
		}),
	}
}

func (t *addGoalTransactionTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		GoalID      string  `json:"goal_id"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		Source      string  `json:"source"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("add_goal_transaction: parse preview: %w", err)
	}
	action := "Aporte"
	switch p.Source {
	case "withdrawal":
		action = "Saque"
	case "yield":
		action = "Rendimento"
	case "initial":
		action = "Saldo inicial"
	}
	shortID := p.GoalID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	label := p.Description
	if label == "" {
		label = p.Source
	}
	return fmt.Sprintf("%s de %.2f na reserva %s… (%s)", action, math.Abs(p.Amount), shortID, label), nil
}

func (t *addGoalTransactionTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		GoalID          string  `json:"goal_id"`
		Amount          float64 `json:"amount"`
		Description     string  `json:"description"`
		Date            string  `json:"date"`
		Source          string  `json:"source"`
		ExpenseCategory string  `json:"expense_category"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("add_goal_transaction: parse input: %w", err)
	}
	goalID, err := uuid.Parse(p.GoalID)
	if err != nil {
		return "", fmt.Errorf("add_goal_transaction: invalid goal_id: %w", err)
	}

	txDate := parseDateNoon(p.Date)
	cur := userCurrency(ctx, t.currencySvc)
	loc := currencyToLocale(cur)

	gtx := &models.GoalTransaction{
		GoalID:      goalID,
		AccountID:   accountID,
		Amount:      p.Amount,
		Description: p.Description,
		Date:        txDate,
		Source:      p.Source,
	}

	if p.Source == "withdrawal" {
		if p.Amount >= 0 {
			return "", fmt.Errorf("add_goal_transaction: withdrawal amount must be negative")
		}
		expCat := p.ExpenseCategory
		if expCat == "" {
			expCat = models.CategoryOutros
		}
		absAmt := -p.Amount
		userID := auth.UserIDFromContext(ctx)

		// Fetch goal name for description
		goals, err := t.repo.List(accountID)
		if err != nil {
			return "", fmt.Errorf("add_goal_transaction: fetch goals: %w", err)
		}
		goalName := p.GoalID[:8]
		for _, g := range goals {
			if g.ID == goalID {
				goalName = g.Name
				break
			}
		}

		entryDesc := fmt.Sprintf("Saque reserva %s", goalName)
		if p.Description != "" {
			entryDesc = fmt.Sprintf("Saque reserva %s — %s", goalName, p.Description)
		}
		entry := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         absAmt,
			Category:       models.CategoryReserva,
			Description:    entryDesc,
			Date:           txDate,
			Source:         "reserve",
			Type:           "transfer",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: absAmt,
		}
		expense := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         -absAmt,
			Category:       expCat,
			Description:    entryDesc,
			Date:           txDate,
			Source:         "reserve",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: absAmt,
		}
		if err := t.repo.WithdrawAndRecord(gtx, entry, expense); err != nil {
			return "", fmt.Errorf("add_goal_transaction: withdrawal: %w", err)
		}
		return fmt.Sprintf("Saque de %.2f da reserva \"%s\" registrado. Entrada + despesa (%s) criadas na conta principal.", absAmt, goalName, expCat), nil
	}

	if p.Source == "manual" || p.Source == "chat" {
		if p.Amount <= 0 {
			return "", fmt.Errorf("add_goal_transaction: deposit amount must be positive")
		}
		expCat := p.ExpenseCategory
		if expCat == "" {
			expCat = models.CategoryReserva
		}
		userID := auth.UserIDFromContext(ctx)

		goals, err := t.repo.List(accountID)
		if err != nil {
			return "", fmt.Errorf("add_goal_transaction: fetch goals: %w", err)
		}
		goalName := p.GoalID[:8]
		for _, g := range goals {
			if g.ID == goalID {
				goalName = g.Name
				break
			}
		}

		expenseDesc := fmt.Sprintf("Aporte reserva %s", goalName)
		if p.Description != "" {
			expenseDesc = fmt.Sprintf("Aporte reserva %s — %s", goalName, p.Description)
		}
		expense := &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         -p.Amount,
			Category:       expCat,
			Description:    expenseDesc,
			Date:           txDate,
			Source:         "reserve",
			Type:           "transfer",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: p.Amount,
		}
		if err := t.repo.DepositAndRecord(gtx, expense); err != nil {
			return "", fmt.Errorf("add_goal_transaction: deposit: %w", err)
		}
		return fmt.Sprintf("Aporte de %.2f na reserva \"%s\" registrado. Despesa (%s) criada na conta principal.", p.Amount, goalName, expCat), nil
	}

	if err := t.repo.AddTransaction(gtx); err != nil {
		return "", fmt.Errorf("add_goal_transaction: %w", err)
	}
	return fmt.Sprintf("Movimento de %.2f registrado na reserva (source: %s)", p.Amount, p.Source), nil
}

// ── add_transaction_note ──────────────────────────────────────────────────────

type addTransactionNoteTool struct {
	repo *repository.TransactionRepository
}

func NewAddTransactionNoteTool(repo *repository.TransactionRepository) WriteTool {
	return &addTransactionNoteTool{repo: repo}
}

func (t *addTransactionNoteTool) Name() string { return "add_transaction_note" }

func (t *addTransactionNoteTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "add_transaction_note",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. Apresente os detalhes e aguarde aprovação explícita antes de usar. " +
			"Adiciona uma nota descritiva a uma transação.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transaction_id": map[string]any{"type": "string"},
				"note":           map[string]any{"type": "string"},
			},
			"required": []string{"transaction_id", "note"},
		}),
	}
}

func (t *addTransactionNoteTool) Preview(input json.RawMessage) (string, error) {
	var p struct {
		TransactionID string `json:"transaction_id"`
		Note          string `json:"note"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("add_transaction_note: parse preview: %w", err)
	}
	return fmt.Sprintf("Adicionar nota à transação %s: \"%s\"", p.TransactionID, p.Note), nil
}

func (t *addTransactionNoteTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		TransactionID string `json:"transaction_id"`
		Note          string `json:"note"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("add_transaction_note: parse input: %w", err)
	}
	id, err := uuid.Parse(p.TransactionID)
	if err != nil {
		return "", fmt.Errorf("add_transaction_note: invalid transaction_id: %w", err)
	}
	tx, err := t.repo.FindByID(id, accountID)
	if err != nil {
		return "", fmt.Errorf("add_transaction_note: find transaction: %w", err)
	}
	tx.Description = p.Note
	if err := t.repo.Update(tx, accountID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Nota adicionada à transação %s", p.TransactionID), nil
}

// ── replace_transaction ───────────────────────────────────────────────────────

type replaceTransactionTool struct {
	repo        *repository.TransactionRepository
	catRepo     *repository.CategoryRepository
	currencySvc *currency.Service
}

func NewReplaceTransactionTool(repo *repository.TransactionRepository, catRepo *repository.CategoryRepository, currencySvc *currency.Service) WriteTool {
	return &replaceTransactionTool{repo: repo, catRepo: catRepo, currencySvc: currencySvc}
}

func (t *replaceTransactionTool) Name() string { return "replace_transaction" }

func (t *replaceTransactionTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "replace_transaction",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. " +
			"Substitui uma transação existente por uma ou mais novas. " +
			"Use para: corrigir valor, data ou categoria de uma transação existente. " +
			"Se uma nota gerou múltiplas transações, chame esta tool uma vez por transação — todas serão agrupadas em uma única confirmação.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transaction_id": map[string]any{
					"type":        "string",
					"description": "UUID da transação a ser substituída (campo 'id' do histórico)",
				},
				"replacements": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "Novas transações que substituem a original",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"amount":      map[string]any{"type": "number", "description": "Negativo para despesa, positivo para receita"},
							"category":    map[string]any{"type": "string"},
							"description": map[string]any{"type": "string"},
							"date":        map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD)"},
							"merchant":    map[string]any{"type": "string"},
						},
						"required": []string{"amount", "category", "date"},
					},
				},
			},
			"required": []string{"transaction_id", "replacements"},
		}),
	}
}

// ReplaceTransactionInput is exported so chat.go can unmarshal it for batching.
type ReplaceTransactionInput struct {
	TransactionID string `json:"transaction_id"`
	Replacements  []struct {
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
		Date        string  `json:"date"`
		Merchant    string  `json:"merchant"`
	} `json:"replacements"`
}

func (t *replaceTransactionTool) Preview(input json.RawMessage) (string, error) {
	var p ReplaceTransactionInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("replace_transaction: parse preview: %w", err)
	}
	return PreviewReplaceInput(p), nil
}

func PreviewReplaceInput(p ReplaceTransactionInput) string {
	shortID := p.TransactionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	lines := []string{fmt.Sprintf("Substituir %s… por %d nova(s):", shortID, len(p.Replacements))}
	for _, r := range p.Replacements {
		label := r.Description
		if label == "" {
			label = r.Merchant
		}
		if label == "" {
			label = r.Category
		}
		lines = append(lines, fmt.Sprintf("  • R$ %.2f — %s (%s) em %s", math.Abs(r.Amount), label, r.Category, r.Date))
	}
	return strings.Join(lines, "\n")
}

func (t *replaceTransactionTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p ReplaceTransactionInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("replace_transaction: parse input: %w", err)
	}
	txID, err := uuid.Parse(p.TransactionID)
	if err != nil {
		return "", fmt.Errorf("replace_transaction: invalid transaction_id: %w", err)
	}
	userID := auth.UserIDFromContext(ctx)
	cur := userCurrency(ctx, t.currencySvc)
	loc := currencyToLocale(cur)
	newTxs := make([]*models.Transaction, 0, len(p.Replacements))
	for _, r := range p.Replacements {
		cat, err := resolveCategoryForAmount(t.catRepo, r.Category, r.Amount)
		if err != nil {
			return "", fmt.Errorf("replace_transaction: %w", err)
		}
		newTxs = append(newTxs, &models.Transaction{
			AccountID:      accountID,
			UserID:         userID,
			Amount:         r.Amount,
			Category:       cat.Slug,
			CategoryID:     &cat.ID,
			Description:    r.Description,
			Merchant:       r.Merchant,
			Date:           parseDateNoon(r.Date),
			Source:         "chat",
			Currency:       cur,
			Locale:         loc,
			OriginalAmount: math.Abs(r.Amount),
		})
	}
	err = t.repo.ReplaceBatch(accountID, []repository.ReplaceBatchItem{{OriginalID: txID, NewTxs: newTxs}})
	if err != nil {
		return "", fmt.Errorf("replace_transaction: %w", err)
	}
	return fmt.Sprintf("Transação substituída por %d nova(s)", len(newTxs)), nil
}

// parseDateNoon parses a YYYY-MM-DD string and returns noon UTC on that date.
// Using noon avoids off-by-one display bugs when clients render in UTC-N timezones.
// Falls back to today at noon UTC if the string is missing or unparseable.
func parseDateNoon(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		now := time.Now()
		return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
	}
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.UTC)
}
