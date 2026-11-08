package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
)

// ── get_transactions ──────────────────────────────────────────────────────────

type getTransactionsTool struct{ repo *repository.TransactionRepository }

func NewGetTransactionsTool(repo *repository.TransactionRepository) ReadTool {
	return &getTransactionsTool{repo: repo}
}

func (t *getTransactionsTool) Name() string { return "get_transactions" }

func (t *getTransactionsTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_transactions",
		Description: "Lista transações do usuário. Filtre por ano e mês para obter um mês específico.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"year":  map[string]any{"type": "integer", "description": "Ano (ex: 2026)"},
				"month": map[string]any{"type": "integer", "description": "Mês 1-12"},
			},
			"required": []string{"year", "month"},
		}),
	}
}

func (t *getTransactionsTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("get_transactions: parse input: %w", err)
	}
	txs, err := t.repo.ListByAccountMonth(accountID, p.Year, p.Month)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(txs)
	return string(b), nil
}

// ── get_spending_summary ──────────────────────────────────────────────────────

type getSpendingSummaryTool struct{ repo *repository.TransactionRepository }

func NewGetSpendingSummaryTool(repo *repository.TransactionRepository) ReadTool {
	return &getSpendingSummaryTool{repo: repo}
}

func (t *getSpendingSummaryTool) Name() string { return "get_spending_summary" }

func (t *getSpendingSummaryTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_spending_summary",
		Description: "Retorna total de receitas, despesas e gastos por categoria para um mês.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"year":  map[string]any{"type": "integer"},
				"month": map[string]any{"type": "integer"},
			},
			"required": []string{"year", "month"},
		}),
	}
}

func (t *getSpendingSummaryTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("get_spending_summary: parse input: %w", err)
	}
	summary, err := t.repo.SummaryByMonth(accountID, p.Year, p.Month)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(summary)
	return string(b), nil
}

// ── get_categories ────────────────────────────────────────────────────────────

type getCategoriesToolImpl struct{ repo *repository.TransactionRepository }

func NewGetCategoriesTool(repo *repository.TransactionRepository) ReadTool {
	return &getCategoriesToolImpl{repo: repo}
}

func (t *getCategoriesToolImpl) Name() string { return "get_categories" }

func (t *getCategoriesToolImpl) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_categories",
		Description: "Lista todas as categorias de transações que o usuário já usou.",
		InputSchema: mustMarshal(map[string]any{"type": "object", "properties": map[string]any{}}),
	}
}

func (t *getCategoriesToolImpl) Execute(_ context.Context, accountID uuid.UUID, _ json.RawMessage) (string, error) {
	txs, err := t.repo.ListByAccount(accountID)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	var cats []string
	for _, tx := range txs {
		if !seen[tx.Category] {
			seen[tx.Category] = true
			cats = append(cats, tx.Category)
		}
	}
	b, _ := json.Marshal(cats)
	return string(b), nil
}

// ── get_merchant_aliases ──────────────────────────────────────────────────────

type getMerchantAliasesTool struct{ repo *repository.MerchantAliasRepository }

func NewGetMerchantAliasesTool(repo *repository.MerchantAliasRepository) ReadTool {
	return &getMerchantAliasesTool{repo: repo}
}

func (t *getMerchantAliasesTool) Name() string { return "get_merchant_aliases" }

func (t *getMerchantAliasesTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_merchant_aliases",
		Description: "Lista os aliases de merchants cadastrados pelo usuário.",
		InputSchema: mustMarshal(map[string]any{"type": "object", "properties": map[string]any{}}),
	}
}

func (t *getMerchantAliasesTool) Execute(_ context.Context, accountID uuid.UUID, _ json.RawMessage) (string, error) {
	aliases, err := t.repo.List(accountID)
	if err != nil {
		return "", err
	}
	b, _ := json.Marshal(aliases)
	return string(b), nil
}

// ── get_goals ─────────────────────────────────────────────────────────────────

type getGoalsTool struct{ repo *repository.GoalRepository }

func NewGetGoalsTool(repo *repository.GoalRepository) ReadTool {
	return &getGoalsTool{repo: repo}
}

func (t *getGoalsTool) Name() string { return "get_goals" }

func (t *getGoalsTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_goals",
		Description: "Lista as reservas/caixinhas financeiras do usuário, com saldo atual, meta, tipo, alocação e as últimas 3 movimentações de cada uma.",
		InputSchema: mustMarshal(map[string]any{"type": "object", "properties": map[string]any{}}),
	}
}

type goalWithTransactions struct {
	Goal             interface{}                  `json:"goal"`
	RecentMovements  []models.GoalTransaction     `json:"recent_movements"`
}

func (t *getGoalsTool) Execute(_ context.Context, accountID uuid.UUID, _ json.RawMessage) (string, error) {
	goals, err := t.repo.List(accountID)
	if err != nil {
		return "", err
	}

	result := make([]goalWithTransactions, 0, len(goals))
	for _, g := range goals {
		txs, err := t.repo.ListTransactions(g.ID, accountID)
		if err != nil {
			txs = []models.GoalTransaction{}
		}
		// Keep only last 3
		if len(txs) > 3 {
			txs = txs[:3]
		}
		result = append(result, goalWithTransactions{
			Goal:            g,
			RecentMovements: txs,
		})
	}

	b, _ := json.Marshal(result)
	return string(b), nil
}

// ── get_goal_suggestions ──────────────────────────────────────────────────────

type getGoalSuggestionsTool struct{ repo *repository.GoalRepository }

func NewGetGoalSuggestionsTool(repo *repository.GoalRepository) ReadTool {
	return &getGoalSuggestionsTool{repo: repo}
}

func (t *getGoalSuggestionsTool) Name() string { return "get_goal_suggestions" }

func (t *getGoalSuggestionsTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "get_goal_suggestions",
		Description: "Retorna os aportes sugeridos para cada reserva no mês especificado, calculados a partir das receitas do mês × percentual futuro × alocação de cada reserva.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"year":  map[string]any{"type": "integer", "description": "Ano. Padrão: ano atual."},
				"month": map[string]any{"type": "integer", "description": "Mês 1-12. Padrão: mês atual."},
			},
		}),
	}
}

func (t *getGoalSuggestionsTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		Year  int `json:"year"`
		Month int `json:"month"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("get_goal_suggestions: parse input: %w", err)
	}
	now := time.Now().UTC()
	if p.Year == 0 {
		p.Year = now.Year()
	}
	if p.Month == 0 {
		p.Month = int(now.Month())
	}
	suggestions, err := t.repo.SuggestedContributions(accountID, p.Year, p.Month)
	if err != nil {
		return "", fmt.Errorf("get_goal_suggestions: %w", err)
	}
	b, _ := json.Marshal(suggestions)
	return string(b), nil
}

// ── compare_periods ───────────────────────────────────────────────────────────

type comparePeriodsTool struct{ repo *repository.TransactionRepository }

func NewComparePeriodsTool(repo *repository.TransactionRepository) ReadTool {
	return &comparePeriodsTool{repo: repo}
}

func (t *comparePeriodsTool) Name() string { return "compare_periods" }

func (t *comparePeriodsTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "compare_periods",
		Description: "Compara gastos por categoria entre dois meses.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"year_a":  map[string]any{"type": "integer"},
				"month_a": map[string]any{"type": "integer"},
				"year_b":  map[string]any{"type": "integer"},
				"month_b": map[string]any{"type": "integer"},
			},
			"required": []string{"year_a", "month_a", "year_b", "month_b"},
		}),
	}
}

func (t *comparePeriodsTool) Execute(_ context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p struct {
		YearA  int `json:"year_a"`
		MonthA int `json:"month_a"`
		YearB  int `json:"year_b"`
		MonthB int `json:"month_b"`
	}
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("compare_periods: parse input: %w", err)
	}
	a, err := t.repo.SummaryByMonth(accountID, p.YearA, p.MonthA)
	if err != nil {
		return "", err
	}
	b, err := t.repo.SummaryByMonth(accountID, p.YearB, p.MonthB)
	if err != nil {
		return "", err
	}

	type delta struct {
		Category string  `json:"category"`
		PeriodA  float64 `json:"period_a"`
		PeriodB  float64 `json:"period_b"`
		Delta    float64 `json:"delta"`
	}
	var deltas []delta
	all := map[string]bool{}
	for k := range a.Categories {
		all[k] = true
	}
	for k := range b.Categories {
		all[k] = true
	}
	for cat := range all {
		deltas = append(deltas, delta{
			Category: cat,
			PeriodA:  a.Categories[cat],
			PeriodB:  b.Categories[cat],
			Delta:    b.Categories[cat] - a.Categories[cat],
		})
	}
	out, _ := json.Marshal(map[string]any{"deltas": deltas, "total_a": a.Expenses, "total_b": b.Expenses})
	return string(out), nil
}

// ── project_balance ───────────────────────────────────────────────────────────

type projectBalanceTool struct{ repo *repository.TransactionRepository }

func NewProjectBalanceTool(repo *repository.TransactionRepository) ReadTool {
	return &projectBalanceTool{repo: repo}
}

func (t *projectBalanceTool) Name() string { return "project_balance" }

func (t *projectBalanceTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "project_balance",
		Description: "Projeta o saldo ao final do mês atual com base no ritmo de gastos até hoje.",
		InputSchema: mustMarshal(map[string]any{"type": "object", "properties": map[string]any{}}),
	}
}

func (t *projectBalanceTool) Execute(_ context.Context, accountID uuid.UUID, _ json.RawMessage) (string, error) {
	now := time.Now().UTC()
	summary, err := t.repo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		return "", err
	}
	daysInMonth := float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day())
	daysPassed := float64(now.Day())
	projectedExpenses := summary.Expenses * (daysInMonth / daysPassed)
	projectedBalance := summary.Income + projectedExpenses

	out, _ := json.Marshal(map[string]any{
		"current_balance":   math.Round(summary.Balance*100) / 100,
		"projected_balance": math.Round(projectedBalance*100) / 100,
		"days_passed":       int(daysPassed),
		"days_in_month":     int(daysInMonth),
	})
	return string(out), nil
}

// ── detect_anomalies ──────────────────────────────────────────────────────────

type detectAnomaliesTool struct{ repo *repository.TransactionRepository }

func NewDetectAnomaliesTool(repo *repository.TransactionRepository) ReadTool {
	return &detectAnomaliesTool{repo: repo}
}

func (t *detectAnomaliesTool) Name() string { return "detect_anomalies" }

func (t *detectAnomaliesTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name:        "detect_anomalies",
		Description: "Detecta categorias com gastos fora do padrão histórico (último mês vs. média dos 3 anteriores).",
		InputSchema: mustMarshal(map[string]any{"type": "object", "properties": map[string]any{}}),
	}
}

func (t *detectAnomaliesTool) Execute(_ context.Context, accountID uuid.UUID, _ json.RawMessage) (string, error) {
	now := time.Now().UTC()
	current, err := t.repo.SummaryByMonth(accountID, now.Year(), int(now.Month()))
	if err != nil {
		return "", err
	}

	categoryTotals := map[string][]float64{}
	for i := 1; i <= 3; i++ {
		prev := now.AddDate(0, -i, 0)
		s, err := t.repo.SummaryByMonth(accountID, prev.Year(), int(prev.Month()))
		if err != nil {
			continue
		}
		for cat, val := range s.Categories {
			categoryTotals[cat] = append(categoryTotals[cat], math.Abs(val))
		}
	}

	type anomaly struct {
		Category  string  `json:"category"`
		Current   float64 `json:"current"`
		AvgPast3  float64 `json:"avg_past_3"`
		PctChange float64 `json:"pct_change"`
	}
	var anomalies []anomaly
	for cat, curVal := range current.Categories {
		if curVal >= 0 {
			continue
		}
		past := categoryTotals[cat]
		if len(past) == 0 {
			continue
		}
		var sum float64
		for _, v := range past {
			sum += v
		}
		avg := sum / float64(len(past))
		if avg == 0 {
			continue
		}
		cur := math.Abs(curVal)
		pct := (cur - avg) / avg * 100
		if pct > 30 {
			anomalies = append(anomalies, anomaly{
				Category:  cat,
				Current:   math.Round(cur*100) / 100,
				AvgPast3:  math.Round(avg*100) / 100,
				PctChange: math.Round(pct*10) / 10,
			})
		}
	}
	out, _ := json.Marshal(anomalies)
	return string(out), nil
}

// mustMarshal marshals v to json.RawMessage, panicking on error (only called at init time with static data).
func mustMarshal(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
