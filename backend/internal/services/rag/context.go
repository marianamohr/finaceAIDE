package rag

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
)

const ragWindowDays = 30
const recentTransactionsLimit = 10

// Builder fetches financial data and assembles the system prompt for the AI.
type Builder struct {
	txRepo   *repository.TransactionRepository
	goalRepo *repository.GoalRepository
	catRepo  *repository.CategoryRepository
	pmRepo   *repository.PaymentMethodRepository
}

func NewBuilder(txRepo *repository.TransactionRepository, goalRepo *repository.GoalRepository, catRepo *repository.CategoryRepository, pmRepo *repository.PaymentMethodRepository) *Builder {
	return &Builder{txRepo: txRepo, goalRepo: goalRepo, catRepo: catRepo, pmRepo: pmRepo}
}

// Build returns a fully assembled system prompt with the account's financial context.
// clientDatetime is an ISO 8601 string sent by the client (e.g. "2026-05-01T14:32:00-03:00").
// Falls back to UTC server time if empty or unparseable.
// baseCurrency is the user's configured currency (e.g. "EUR", "BRL").
func (b *Builder) Build(accountID uuid.UUID, categoryContext string, clientDatetime string, baseCurrency string) (string, error) {
	summary, err := b.txRepo.SummaryLastDays(accountID, ragWindowDays)
	if err != nil {
		return "", fmt.Errorf("rag: summary: %w", err)
	}

	recent, err := b.txRepo.RecentByAccount(accountID, recentTransactionsLimit)
	if err != nil {
		return "", fmt.Errorf("rag: recent transactions: %w", err)
	}

	goals, err := b.goalRepo.List(accountID)
	if err != nil {
		// Non-fatal: continue without goals context
		goals = nil
	}

	now := ParseClientDatetime(clientDatetime)
	suggestions, err := b.goalRepo.SuggestedContributions(accountID, now.Year(), int(now.Month()))
	if err != nil {
		suggestions = nil
	}

	cats, err := b.catRepo.ListByMode(categoryContext)
	if err != nil {
		cats = nil
	}

	return buildPrompt(summary, recent, goals, suggestions, cats, categoryContext, clientDatetime, baseCurrency), nil
}

// ParseClientDatetime parses an ISO 8601 datetime string sent by the client.
// Returns the parsed time preserving the client's timezone offset, or UTC now as fallback.
func ParseClientDatetime(s string) time.Time {
	if s == "" {
		return time.Now().UTC()
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC()
	}
	return t
}

func buildPrompt(
	summary *repository.TransactionSummary,
	recent []models.Transaction,
	goals []models.Goal,
	suggestions []repository.SuggestedContribution,
	cats []models.Category,
	categoryContext string,
	clientDatetime string,
	baseCurrency string,
) string {
	var sb strings.Builder

	sb.WriteString("Você é um assistente financeiro pessoal chamado FinanceAI.\n")
	sb.WriteString("Seu objetivo é ajudar o usuário a entender e melhorar sua situação financeira\n")
	sb.WriteString("com base nos dados reais dele. Seja direto, prático e responda sempre em português.\n\n")
	now := ParseClientDatetime(clientDatetime)
	todayStr := now.Format("2006-01-02")
	sb.WriteString(fmt.Sprintf("⚠️ DATA DE HOJE (ABSOLUTA): %s (%s) — hora local do usuário: %s\n",
		todayStr, now.Weekday().String(), now.Format("15:04")))
	sb.WriteString("Esta é a data real do dispositivo do usuário. Use-a como referência exclusiva para \"hoje\", \"ontem\", \"amanhã\".\n\n")

	currencySymbol := currencySymbolFor(baseCurrency)
	sb.WriteString(fmt.Sprintf("== MOEDA BASE DO USUÁRIO: %s (%s) ==\n", baseCurrency, currencySymbol))
	sb.WriteString(fmt.Sprintf("Todos os valores estão em %s. Nunca converta para outra moeda. Nunca pergunte se deve converter.\n\n", baseCurrency))

	sb.WriteString(fmt.Sprintf("== CONTEXTO FINANCEIRO DO USUÁRIO (últimos %d dias) ==\n", ragWindowDays))
	sb.WriteString(fmt.Sprintf("Receita total: %s %.2f\n", currencySymbol, summary.Income))
	sb.WriteString(fmt.Sprintf("Despesa total: %s %.2f\n", currencySymbol, summary.Expenses))
	sb.WriteString(fmt.Sprintf("Saldo do período: %s %.2f\n", currencySymbol, summary.Balance))

	if len(summary.Categories) > 0 {
		sb.WriteString("Categorias com maior gasto: ")
		sb.WriteString(formatTopCategories(summary.Categories, currencySymbol))
		sb.WriteString("\n")
	}

	if len(recent) > 0 {
		sb.WriteString(fmt.Sprintf("Últimas %d transações (HISTÓRICO PASSADO — datas abaixo NÃO representam hoje):\n", len(recent)))
		for _, tx := range recent {
			sb.WriteString(fmt.Sprintf("  - id:%s | %s | %s | %s %.2f | %s\n",
				tx.ID.String(),
				tx.Date.Format("02/01/2006"),
				tx.Category,
				currencySymbol,
				tx.Amount,
				tx.Description,
			))
		}
	}

	if len(goals) > 0 {
		sb.WriteString("\n== RESERVAS / CAIXINHAS ==\n")
		for _, g := range goals {
			pct := 0.0
			if g.TargetAmount > 0 {
				pct = g.CurrentAmount / g.TargetAmount * 100
			}
			deadlineStr := ""
			if g.Deadline != nil {
				deadlineStr = fmt.Sprintf(" | prazo: %s", g.Deadline.Format("2006-01-02"))
			}
			sb.WriteString(fmt.Sprintf("  - id:%s | %s | saldo: %s %.2f / meta: %s %.2f (%.0f%%)%s | alocação: %.0f%% do futuro\n",
				g.ID.String(),
				g.Name,
				currencySymbol,
				g.CurrentAmount,
				currencySymbol,
				g.TargetAmount,
				pct,
				deadlineStr,
				g.AllocationPct*100,
			))
		}
	}

	if len(suggestions) > 0 {
		sb.WriteString("\n== APORTES SUGERIDOS ESTE MÊS ==\n")
		for _, s := range suggestions {
			sb.WriteString(fmt.Sprintf("  - %s: %s %.2f\n", s.GoalName, currencySymbol, s.SuggestedAmt))
		}
		sb.WriteString("(Use add_goal_transaction com source=\"manual\" para registrar aportes autorizados pelo usuário)\n")
	}

	sb.WriteString("\n== CATEGORIAS VÁLIDAS ==\n")
	sb.WriteString(buildCategoriesSection(cats))

	sb.WriteString("\n== INSTRUÇÕES ==\n")
	sb.WriteString("- Use os dados acima para responder com precisão — nunca invente números\n")
	sb.WriteString("- REGRA DE SINAL: despesas/gastos usam valor NEGATIVO; receitas/entradas usam valor POSITIVO\n")
	sb.WriteString(fmt.Sprintf("  Exemplos: gasto de %s50 → amount: -50 | salário de %s3000 → amount: 3000\n", currencySymbol, currencySymbol))
	sb.WriteString(fmt.Sprintf("- REGRA DE DATA (CRÍTICA): hoje é %s. Use SOMENTE esta data para \"hoje\"/\"ontem\"/\"amanhã\".\n", todayStr))
	sb.WriteString("  As datas listadas no histórico acima são de transações PASSADAS — nunca as use para inferir o dia de hoje.\n")
	sb.WriteString("  \"hoje\" → data de hoje exata. \"ontem\" → data de hoje menos 1 dia. \"anteontem\" → menos 2 dias.\n")
	sb.WriteString("- REGRA DE CATEGORIA: use APENAS os slugs listados em == CATEGORIAS VÁLIDAS == acima, exatamente como estão escritos (sem acentos, sem maiúsculas)\n")
	sb.WriteString("  Para RECEITAS (amount > 0): use slugs da seção RECEITAS (salario, vr, ppr, freelance, etc.) — NUNCA use slug de despesa para receitas\n")
	sb.WriteString("  Para DESPESAS (amount < 0): use slugs da seção DESPESAS — NUNCA use slug de receita para despesas\n")
	sb.WriteString("- Se o usuário descrever um gasto ou receita, chame IMEDIATAMENTE a tool create_transaction — não peça confirmação antes\n")
	sb.WriteString("  A tool já aciona um fluxo de confirmação para o usuário; não duplique isso com texto\n")
	sb.WriteString(fmt.Sprintf("  Após chamar a tool, confirme brevemente: \"Registrei %s 50,00 em alimentação hoje.\"\n", currencySymbol))
	sb.WriteString("- Se o usuário disser que uma transação JÁ REGISTRADA estava errada (valor, data, categoria):\n")
	sb.WriteString("  Chame replace_transaction IMEDIATAMENTE (uma chamada por transação afetada) — NÃO peça confirmação textual antes de chamar a tool.\n")
	sb.WriteString("  A tool já aciona o fluxo de confirmação; chamar a tool É a forma de pedir confirmação ao usuário.\n")
	sb.WriteString("  Se souber quais são as transações afetadas, chame a tool diretamente. Não diga 'confirma que devo fazer X?' — apenas faça.\n")
	sb.WriteString("  NÃO use create_transaction para corrigir transações existentes.\n")
	sb.WriteString("  transaction_id: use o campo 'id:...' do histórico acima.\n")

	sb.WriteString("- Se o usuário enviar uma foto de nota fiscal, os dados já foram extraídos — aguarde confirmação via botão\n")
	sb.WriteString("- Para análises, sempre cite os números reais do contexto\n")
	sb.WriteString("- Se não houver dados suficientes para responder, diga claramente\n")
	sb.WriteString("- Nunca pergunte dados que você já tem no contexto\n")
	sb.WriteString("- Compra ÚNICA PARCELADA no cartão de crédito (ex.: \"comprei uma TV de 3000 em 10x\"): chame create_installment_purchase (NÃO crie várias create_transaction). Se o usuário citou o cartão, passe card_name.\n")

	sb.WriteString("\n== FORMAS DE PAGAMENTO ==\n")
	sb.WriteString("Ao registrar uma DESPESA, envie payment_mode_slug em create_transaction usando um destes termos:\n")
	sb.WriteString("  - \"crédito\", \"cartão de crédito\" → credito\n")
	sb.WriteString("  - \"débito\", \"cartão de débito\", \"débito automático\" → debito\n")
	sb.WriteString("  - \"pix\" → pix\n")
	sb.WriteString("  - \"vr\", \"vale refeição\", \"vale alimentação\" → vr\n")
	sb.WriteString("  - \"boleto\" → boleto (será pago via débito em conta)\n")
	sb.WriteString("Você NÃO escolhe um cartão específico — o backend resolve o slug em uma forma de pagamento real do usuário.\n")
	sb.WriteString("Se o usuário não mencionou nenhuma, omita o campo; o backend vai perguntar.\n")
	sb.WriteString("Se ele citou um termo que não está nesta lista (ex.: \"no Nubank\" sem dizer crédito/débito), omita também — o backend vai perguntar com a lista de cartões do usuário.\n")
	sb.WriteString("Para receitas e transferências internas (type=transfer): omita payment_mode_slug.\n")

	return sb.String()
}

// buildCategoriesSection builds the CATEGORIAS VÁLIDAS prompt section from DB categories.
// Income and expense categories are separated into two groups with usage instructions.
func buildCategoriesSection(cats []models.Category) string {
	var sb strings.Builder
	sb.WriteString("RECEITAS (amount positivo) — use EXATAMENTE estes slugs:\n")
	for _, c := range cats {
		if c.Type == "income" {
			if c.Examples != "" {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", c.Slug, c.Examples))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s\n", c.Slug))
			}
		}
	}
	sb.WriteString("\nDESPESAS (amount negativo) — use EXATAMENTE estes slugs:\n")
	for _, c := range cats {
		if c.Type == "expense" {
			if c.Examples != "" {
				sb.WriteString(fmt.Sprintf("  - %s: %s\n", c.Slug, c.Examples))
			} else {
				sb.WriteString(fmt.Sprintf("  - %s\n", c.Slug))
			}
		}
	}
	return sb.String()
}

func currencySymbolFor(currency string) string {
	switch currency {
	case "EUR":
		return "€"
	case "USD":
		return "$"
	case "GBP":
		return "£"
	default:
		return "R$"
	}
}

// formatTopCategories returns the top-3 spending categories as a string.
func formatTopCategories(cats map[string]float64, symbol string) string {
	type kv struct {
		k string
		v float64
	}
	var sorted []kv
	for k, v := range cats {
		if v < 0 { // expenses are negative
			sorted = append(sorted, kv{k, v})
		}
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].v < sorted[j].v // most negative first
	})

	limit := 3
	if len(sorted) < limit {
		limit = len(sorted)
	}

	parts := make([]string, limit)
	for i := 0; i < limit; i++ {
		parts[i] = fmt.Sprintf("%s (%s %.2f)", sorted[i].k, symbol, sorted[i].v)
	}
	return strings.Join(parts, ", ")
}
