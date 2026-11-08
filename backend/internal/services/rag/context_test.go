package rag

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
)

func emptySummary() *repository.TransactionSummary {
	return &repository.TransactionSummary{Categories: map[string]float64{}}
}

func TestBuildPrompt_IncludesPaymentModeInstructions(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(emptySummary(), nil, nil, nil, nil, "", "", "BRL")

	for _, want := range []string{
		"payment_mode_slug",
		"crédito",
		"débito",
		"pix",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildPrompt_IncludesCurrencyInfo(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(emptySummary(), nil, nil, nil, nil, "", "", "BRL")

	if !strings.Contains(prompt, "MOEDA BASE") {
		t.Error("prompt should contain currency base info")
	}
	if !strings.Contains(prompt, "BRL") {
		t.Error("prompt should contain the BRL currency code")
	}
}

func TestBuildPrompt_IncludesFinancialContext(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(emptySummary(), nil, nil, nil, nil, "", "", "BRL")

	for _, want := range []string{
		"CONTEXTO FINANCEIRO",
		"Receita total",
		"Despesa total",
		"Saldo do período",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildPrompt_IncludesTransactionRules(t *testing.T) {
	t.Parallel()

	prompt := buildPrompt(emptySummary(), nil, nil, nil, nil, "", "", "BRL")

	for _, want := range []string{
		"REGRA DE SINAL",
		"NEGATIVO",
		"POSITIVO",
		"REGRA DE DATA",
		"REGRA DE CATEGORIA",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildPrompt_WithTransactions_IncludesHistory(t *testing.T) {
	t.Parallel()

	txs := []models.Transaction{
		{
			ID:          uuid.New(),
			Date:        time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
			Category:    "alimentacao",
			Amount:      -50.00,
			Description: "Almoço",
		},
	}

	prompt := buildPrompt(emptySummary(), txs, nil, nil, nil, "", "", "BRL")

	if !strings.Contains(prompt, "alimentacao") {
		t.Error("prompt should contain transaction category")
	}
	if !strings.Contains(prompt, "50.00") {
		t.Error("prompt should contain transaction amount")
	}
}

func TestBuildPrompt_WithCategories(t *testing.T) {
	t.Parallel()

	cats := []models.Category{
		{Type: "income", Slug: "salario"},
		{Type: "expense", Slug: "alimentacao"},
	}

	prompt := buildPrompt(emptySummary(), nil, nil, nil, cats, "", "", "BRL")

	for _, want := range []string{"salario", "alimentacao", "RECEITAS", "DESPESAS"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestParseClientDatetime_Valid(t *testing.T) {
	t.Parallel()

	got := ParseClientDatetime("2026-05-01T14:32:00-03:00")
	want := time.Date(2026, 5, 1, 14, 32, 0, 0, got.Location())

	if !got.Equal(want) {
		t.Errorf("ParseClientDatetime returned %v, want %v", got, want)
	}
}

func TestParseClientDatetime_Empty(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	got := ParseClientDatetime("")
	after := time.Now().UTC()

	if got.Before(before) || got.After(after) {
		t.Errorf("ParseClientDatetime(\"\") should return approximately now, got %v", got)
	}
}

func TestParseClientDatetime_Invalid(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	got := ParseClientDatetime("not-a-date")
	after := time.Now().UTC()

	if got.Before(before) || got.After(after) {
		t.Errorf("ParseClientDatetime with invalid input should return approximately now, got %v", got)
	}
}

func TestCurrencySymbolFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		currency string
		want     string
	}{
		{"BRL", "R$"},
		{"EUR", "€"},
		{"USD", "$"},
		{"GBP", "£"},
		{"XXX", "R$"},
	}

	for _, tt := range tests {
		t.Run(tt.currency, func(t *testing.T) {
			t.Parallel()
			got := currencySymbolFor(tt.currency)
			if got != tt.want {
				t.Errorf("currencySymbolFor(%q) = %q, want %q", tt.currency, got, tt.want)
			}
		})
	}
}

func TestFormatTopCategories_Empty(t *testing.T) {
	t.Parallel()

	got := formatTopCategories(map[string]float64{}, "R$")

	if got != "" {
		t.Errorf("expected empty string for empty categories, got %q", got)
	}
}

func TestFormatTopCategories_TopThree(t *testing.T) {
	t.Parallel()

	cats := map[string]float64{
		"alimentacao": -500,
		"transporte":  -200,
		"lazer":       -300,
		"saude":       -100,
	}
	got := formatTopCategories(cats, "R$")

	if !strings.Contains(got, "alimentacao") {
		t.Error("top categories should contain 'alimentacao' (highest spend)")
	}
	if !strings.Contains(got, "lazer") {
		t.Error("top categories should contain 'lazer'")
	}
}

func TestNewBuilder_ReturnsNonNil(t *testing.T) {
	t.Parallel()

	b := NewBuilder(nil, nil, nil, nil)

	if b == nil {
		t.Error("NewBuilder should return a non-nil Builder")
	}
}

func TestBuildPrompt_WithRecentTransactions(t *testing.T) {
	t.Parallel()

	recent := []models.Transaction{
		{
			ID:          uuid.New(),
			Amount:      -150.00,
			Category:    "alimentacao",
			Description: "Mercado",
			Date:        time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		},
	}

	prompt := buildPrompt(emptySummary(), recent, nil, nil, nil, "", "", "BRL")

	if !strings.Contains(prompt, "Mercado") {
		t.Error("prompt should contain recent transaction description")
	}
}

func TestBuildPrompt_WithGoals(t *testing.T) {
	t.Parallel()

	deadline := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	goals := []models.Goal{
		{
			ID:            uuid.New(),
			Name:          "Viagem Europa",
			CurrentAmount: 1000,
			TargetAmount:  5000,
			AllocationPct: 0.1,
			Deadline:      &deadline,
		},
	}
	suggestions := []repository.SuggestedContribution{
		{GoalName: "Viagem Europa", SuggestedAmt: 300},
	}

	prompt := buildPrompt(emptySummary(), nil, goals, suggestions, nil, "", "", "BRL")

	if !strings.Contains(prompt, "Viagem Europa") {
		t.Error("prompt should contain goal name")
	}
	if !strings.Contains(prompt, "APORTES SUGERIDOS") {
		t.Error("prompt should contain suggested contributions section")
	}
}

func TestBuildPrompt_WithSummaryCategories(t *testing.T) {
	t.Parallel()

	summary := &repository.TransactionSummary{
		Income:   3000,
		Expenses: -500,
		Balance:  2500,
		Categories: map[string]float64{
			"alimentacao": -300,
			"transporte":  -200,
		},
	}

	prompt := buildPrompt(summary, nil, nil, nil, nil, "", "", "BRL")

	if !strings.Contains(prompt, "alimentacao") {
		t.Error("prompt should contain top spending category")
	}
}

func TestBuildCategoriesSection(t *testing.T) {
	t.Parallel()

	cats := []models.Category{
		{Type: "income", Slug: "salario", Examples: "salário, pagamento"},
		{Type: "expense", Slug: "alimentacao", Examples: "mercado, restaurante"},
		{Type: "income", Slug: "freelance"},
	}
	got := buildCategoriesSection(cats)

	for _, want := range []string{"salario", "salário, pagamento", "alimentacao", "mercado, restaurante", "freelance"} {
		if !strings.Contains(got, want) {
			t.Errorf("categories section should contain %q", want)
		}
	}
}
