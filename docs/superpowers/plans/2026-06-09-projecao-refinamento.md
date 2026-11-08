# Refinamento da Projeção de Gastos — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir as listas planas da tela `/projecao` por acordeões agrupados — fixos por categoria, cartão e parcelas por nome do cartão — todos fechados por padrão.

**Architecture:** O backend passa a retornar `category_label` (slug da categoria) para fixos e `card_name` (nome do método de pagamento) para cartão e parcelas. O frontend implementa um componente `AccordionGroup` que agrupa os itens por essas chaves e os expande ao toque.

**Tech Stack:** Go/GORM (backend), React/TypeScript + Tailwind + shadcn/ui (frontend), Vitest + Testing Library (testes frontend), Go integration tests com PostgreSQL.

---

## Mapa de arquivos

| Arquivo | Ação |
|---|---|
| `backend/internal/repository/transaction.go` | Modificar: struct `ProjectionItem`, queries e assembly em `ExpenseProjectionDetailByMonths` |
| `backend/internal/repository/expense_projection_detail_integration_test.go` | Modificar: `projectionDetailDB` + novos sub-testes + helper `mkCategory` |
| `frontend/src/hooks/useAnalytics.ts` | Modificar: interface `ProjectionItem` |
| `frontend/src/pages/ProjecaoDetalhe.tsx` | Modificar: substituir `ItemList` por `AccordionGroup` |
| `frontend/src/pages/ProjecaoDetalhe.test.tsx` | Criar: testes do `AccordionGroup` |

---

## Task 1: Backend — campo `category_label` nos fixos

**Files:**
- Modify: `backend/internal/repository/transaction.go:748-752`
- Modify: `backend/internal/repository/expense_projection_detail_integration_test.go`

---

- [ ] **Step 1: Adicionar helper `mkCategory` e atualizar `projectionDetailDB` no arquivo de teste**

Abra `backend/internal/repository/expense_projection_detail_integration_test.go`.

Altere `projectionDetailDB` para incluir `models.Category`:

```go
func projectionDetailDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := projectionDB(t)
	require.NoError(t, db.AutoMigrate(&models.InstallmentPurchase{}, &models.Category{}))
	return db
}
```

Adicione o helper `mkCategory` logo abaixo da função `projectionDetailDB`:

```go
func mkCategory(t *testing.T, db *gorm.DB, slug string) *models.Category {
	t.Helper()
	// slug único para evitar conflito de uniqueIndex em testes paralelos
	uniqueSlug := slug + "-" + uuid.New().String()[:8]
	cat := &models.Category{
		ID:    uuid.New(),
		Slug:  uniqueSlug,
		Label: slug,
		Type:  "expense",
		Mode:  "standard",
	}
	require.NoError(t, db.Create(cat).Error)
	t.Cleanup(func() { db.Unscoped().Delete(cat) })
	return cat
}
```

---

- [ ] **Step 2: Escrever os testes que devem falhar**

No `TestExpenseProjectionDetailByMonths`, adicione dois sub-testes ao final (antes do fechamento de `}`):

```go
t.Run("fixed item has category_label from category slug", func(t *testing.T) {
	t.Parallel()
	accountID := uuid.New()
	cat := mkCategory(t, db, "casa")

	re := &models.RecurringExpense{
		ID:         uuid.New(),
		AccountID:  accountID,
		Name:       "Aluguel",
		Amount:     1500,
		CategoryID: &cat.ID,
		Active:     true,
	}
	require.NoError(t, db.Create(re).Error)
	t.Cleanup(func() { db.Unscoped().Delete(re) })

	got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
	require.NoError(t, err)
	require.Len(t, got[0].Items.Fixed, 1)
	assert.Equal(t, cat.Slug, got[0].Items.Fixed[0].CategoryLabel)
})

t.Run("fixed item without category defaults to outros", func(t *testing.T) {
	t.Parallel()
	accountID := uuid.New()
	mkRecurring(t, db, accountID, 100, nil, nil, nil) // sem categoria

	got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
	require.NoError(t, err)
	require.Len(t, got[0].Items.Fixed, 1)
	assert.Equal(t, "outros", got[0].Items.Fixed[0].CategoryLabel)
})
```

---

- [ ] **Step 3: Rodar os testes para confirmar que falham**

```bash
cd backend
go test -v -tags=integration -run "TestExpenseProjectionDetailByMonths/fixed_item_has_category_label" ./internal/repository/
```

Resultado esperado: **FAIL** — `CategoryLabel` não existe no struct.

---

- [ ] **Step 4: Adicionar `CategoryLabel` ao struct `ProjectionItem`**

Em `backend/internal/repository/transaction.go`, localize o struct `ProjectionItem` (em torno da linha 748) e adicione o campo:

```go
// ProjectionItem is a single named expense in a projection bucket.
type ProjectionItem struct {
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	InstallmentInfo string  `json:"installment_info,omitempty"`
	CategoryLabel   string  `json:"category_label,omitempty"`
	CardName        string  `json:"card_name,omitempty"`
}
```

---

- [ ] **Step 5: Atualizar a query de fixos para retornar a categoria**

Em `ExpenseProjectionDetailByMonths`, localize o bloco `// ---- fixed items ----`.

Substitua o struct `fixedRows` e a query:

```go
// ---- fixed items ----
var fixedRows []struct {
	Name          string
	Amount        float64
	CategoryLabel string
}
if err := r.db.Raw(`
	SELECT re.name, re.amount, COALESCE(c.slug, 'outros') AS category_label
	FROM recurring_expenses re
	LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
	LEFT JOIN categories c ON c.id = re.category_id
	WHERE re.account_id = ?
	  AND re.active = true
	  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
	  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
	  AND (pm.id IS NULL OR pm.kind <> 'credit_card')
	ORDER BY re.amount DESC
`, accountID, year, month, year, month).Scan(&fixedRows).Error; err != nil {
	return nil, fmt.Errorf("expense projection detail fixed items: %w", err)
}
```

---

- [ ] **Step 6: Atualizar o assembly dos fixos**

Localize o bloco `// ---- assemble items ----` e atualize o loop de `fixedItems`:

```go
fixedItems := make([]ProjectionItem, 0, len(fixedRows))
for _, row := range fixedRows {
	fixedItems = append(fixedItems, ProjectionItem{
		Name:          row.Name,
		Amount:        row.Amount,
		CategoryLabel: row.CategoryLabel,
	})
}
```

---

- [ ] **Step 7: Rodar os testes para confirmar que passam**

```bash
cd backend
go test -v -tags=integration -run "TestExpenseProjectionDetailByMonths" ./internal/repository/
```

Resultado esperado: todos os sub-testes **PASS**.

---

- [ ] **Step 8: Commit**

```bash
git add backend/internal/repository/transaction.go \
        backend/internal/repository/expense_projection_detail_integration_test.go
git commit -m "feat(projecao): adiciona category_label nos itens fixos da projeção"
```

---

## Task 2: Backend — campo `card_name` em cartão e parcelas

**Files:**
- Modify: `backend/internal/repository/transaction.go`
- Modify: `backend/internal/repository/expense_projection_detail_integration_test.go`

---

- [ ] **Step 1: Escrever os testes que devem falhar**

No `TestExpenseProjectionDetailByMonths`, adicione três sub-testes ao final:

```go
t.Run("installment item has card_name from payment method", func(t *testing.T) {
	t.Parallel()
	accountID := uuid.New()
	card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)

	ip := &models.InstallmentPurchase{
		ID:              uuid.New(),
		AccountID:       accountID,
		PaymentMethodID: card.ID,
		Description:     "Sofá",
		Merchant:        "Sofá",
		TotalAmount:     600,
		Installments:    6,
		PurchaseDate:    time.Now().UTC(),
	}
	require.NoError(t, db.Create(ip).Error)
	t.Cleanup(func() { db.Unscoped().Delete(ip) })

	num := 1
	tx := &models.Transaction{
		ID:                    uuid.New(),
		UserID:                uuid.New(),
		AccountID:             accountID,
		Amount:                -100,
		Category:              "outros",
		Date:                  time.Now().UTC(),
		Source:                "installment",
		Type:                  "regular",
		PaymentMethodID:       &card.ID,
		InstallmentPurchaseID: &ip.ID,
		InstallmentNumber:     &num,
		BillingYear:           &year,
		BillingMonth:          &month,
		Locale:                "BR",
		Currency:              "BRL",
		Merchant:              "Sofá",
	}
	require.NoError(t, db.Create(tx).Error)
	t.Cleanup(func() { db.Unscoped().Delete(tx) })

	got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
	require.NoError(t, err)
	require.Len(t, got[0].Items.Installments, 1)
	assert.Equal(t, card.Name, got[0].Items.Installments[0].CardName)
})

t.Run("materialized card item has card_name from payment method", func(t *testing.T) {
	t.Parallel()
	accountID := uuid.New()
	card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
	mkBillingTx(t, db, accountID, -90, &card.ID, "manual", "regular", year, month, nil)

	got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
	require.NoError(t, err)
	require.Len(t, got[0].Items.Card, 1)
	assert.Equal(t, card.Name, got[0].Items.Card[0].CardName)
})

t.Run("virtual card subscription has card_name from payment method", func(t *testing.T) {
	t.Parallel()
	accountID := uuid.New()
	card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
	mkRecurring(t, db, accountID, 45, &card.ID, nil, nil)

	got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
	require.NoError(t, err)
	require.Len(t, got[0].Items.Card, 1)
	assert.Equal(t, card.Name, got[0].Items.Card[0].CardName)
})
```

---

- [ ] **Step 2: Rodar os testes para confirmar que falham**

```bash
cd backend
go test -v -tags=integration -run "TestExpenseProjectionDetailByMonths/installment_item_has_card_name" ./internal/repository/
```

Resultado esperado: **FAIL** — `CardName` está vazio.

---

- [ ] **Step 3: Atualizar a query de parcelas (installRows)**

Localize o bloco `// ---- installment items ----` em `ExpenseProjectionDetailByMonths`.

Substitua o struct e a query:

```go
// ---- installment items ----
var installRows []struct {
	Name              string
	Amount            float64
	InstallmentNumber *int
	TotalInstallments *int
	CardName          string
}
if err := r.db.Raw(`
	SELECT
		COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
		-t.amount AS amount,
		t.installment_number,
		ip.installments AS total_installments,
		pm.name AS card_name
	FROM transactions t
	JOIN payment_methods pm ON pm.id = t.payment_method_id
	LEFT JOIN installment_purchases ip ON ip.id = t.installment_purchase_id
	WHERE t.account_id = ?
	  AND pm.kind = 'credit_card'
	  AND t.source = 'installment'
	  AND t.type != 'invoice_payment'
	  AND t.billing_year = ?
	  AND t.billing_month = ?
	ORDER BY t.amount ASC
`, accountID, cardYear, cardMonth).Scan(&installRows).Error; err != nil {
	return nil, fmt.Errorf("expense projection detail installment items: %w", err)
}
```

---

- [ ] **Step 4: Atualizar a query de cartão materializados (cardMatRows)**

Localize o bloco `// ---- card items: materialized ----`.

Substitua o struct e a query:

```go
// ---- card items: materialized ----
var cardMatRows []struct {
	Name     string
	Amount   float64
	CardName string
}
if err := r.db.Raw(`
	SELECT
		COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
		-t.amount AS amount,
		pm.name AS card_name
	FROM transactions t
	JOIN payment_methods pm ON pm.id = t.payment_method_id
	WHERE t.account_id = ?
	  AND pm.kind = 'credit_card'
	  AND t.source <> 'installment'
	  AND t.type != 'invoice_payment'
	  AND t.billing_year = ?
	  AND t.billing_month = ?
	ORDER BY t.amount ASC
`, accountID, cardYear, cardMonth).Scan(&cardMatRows).Error; err != nil {
	return nil, fmt.Errorf("expense projection detail card materialized items: %w", err)
}
```

---

- [ ] **Step 5: Atualizar a query de cartão virtual (cardVirtRows)**

Localize o bloco `// ---- card items: virtual (subscriptions not yet materialized) ----`.

Substitua o struct e a query:

```go
// ---- card items: virtual (subscriptions not yet materialized) ----
var cardVirtRows []struct {
	Name     string
	Amount   float64
	CardName string
}
if err := r.db.Raw(`
	SELECT re.name, re.amount, pm.name AS card_name
	FROM recurring_expenses re
	JOIN payment_methods pm ON pm.id = re.payment_method_id
	WHERE re.account_id = ?
	  AND re.active = true
	  AND pm.kind = 'credit_card'
	  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
	  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
	  AND NOT EXISTS (
		SELECT 1 FROM transactions tx
		WHERE tx.recurring_expense_id = re.id
		  AND tx.billing_year = ?
		  AND tx.billing_month = ?
	)
	ORDER BY re.amount DESC
`, accountID, cardYear, cardMonth, cardYear, cardMonth, cardYear, cardMonth).Scan(&cardVirtRows).Error; err != nil {
	return nil, fmt.Errorf("expense projection detail card virtual items: %w", err)
}
```

---

- [ ] **Step 6: Atualizar o assembly de parcelas e cartão**

Localize o bloco `// ---- assemble items ----` e atualize os três loops:

```go
installItems := make([]ProjectionItem, 0, len(installRows))
for _, row := range installRows {
	item := ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName}
	if row.InstallmentNumber != nil && row.TotalInstallments != nil {
		item.InstallmentInfo = fmt.Sprintf("%d/%d", *row.InstallmentNumber, *row.TotalInstallments)
	}
	installItems = append(installItems, item)
}

cardItems := make([]ProjectionItem, 0, len(cardMatRows)+len(cardVirtRows))
for _, row := range cardMatRows {
	cardItems = append(cardItems, ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName})
}
for _, row := range cardVirtRows {
	cardItems = append(cardItems, ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName})
}
```

---

- [ ] **Step 7: Rodar todos os testes de projeção**

```bash
cd backend
go test -v -tags=integration -run "TestExpenseProjectionDetailByMonths" ./internal/repository/
```

Resultado esperado: todos os sub-testes **PASS**.

---

- [ ] **Step 8: Commit**

```bash
git add backend/internal/repository/transaction.go \
        backend/internal/repository/expense_projection_detail_integration_test.go
git commit -m "feat(projecao): adiciona card_name nos itens de cartão e parcelas da projeção"
```

---

## Task 3: Frontend — AccordionGroup e integração na página

**Files:**
- Modify: `frontend/src/hooks/useAnalytics.ts`
- Modify: `frontend/src/pages/ProjecaoDetalhe.tsx`
- Create: `frontend/src/pages/ProjecaoDetalhe.test.tsx`

---

- [ ] **Step 1: Atualizar a interface `ProjectionItem` em `useAnalytics.ts`**

Abra `frontend/src/hooks/useAnalytics.ts`. Localize a interface `ProjectionItem` (em torno da linha 73) e substitua:

```ts
export interface ProjectionItem {
  name: string
  amount: number
  installment_info?: string
  category_label?: string
  card_name?: string
}
```

---

- [ ] **Step 2: Criar o arquivo de testes `ProjecaoDetalhe.test.tsx`**

Crie `frontend/src/pages/ProjecaoDetalhe.test.tsx` com o seguinte conteúdo:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"
import ProjecaoDetalhe from "./ProjecaoDetalhe"

vi.mock("react-router-dom", () => ({
  useNavigate: () => vi.fn(),
}))

vi.mock("@/components/BottomNav/BottomNav", () => ({
  BottomNav: () => <div data-testid="bottom-nav" />,
}))

vi.mock("@/components/PageHeader/PageHeader", () => ({
  PageHeader: ({ title }: { title: string }) => <div data-testid="page-header">{title}</div>,
}))

vi.mock("@/store/currency", () => ({
  useCurrencyStore: (sel: (s: { currency: string }) => unknown) => sel({ currency: "BRL" }),
  formatCurrency: (v: number) => `R$ ${v.toFixed(2)}`,
  maskCurrency: () => "R$ •••",
}))

vi.mock("@/store/privacy", () => ({
  usePrivacyStore: (sel: (s: { valuesHidden: boolean }) => unknown) => sel({ valuesHidden: false }),
}))

const mockData = [
  {
    year: 2026,
    month: 7,
    label: "Jul/26",
    fixed: 3180,
    installments: 300,
    card: 120,
    total: 3600,
    items: {
      fixed: [
        { name: "Aluguel", amount: 2800, category_label: "casa" },
        { name: "Condomínio", amount: 380, category_label: "casa" },
        { name: "Plano de saúde", amount: 320, category_label: "saude" },
      ],
      installments: [
        { name: "Notebook", amount: 300, installment_info: "3/10", card_name: "Nubank Ana" },
      ],
      card: [
        { name: "Claude", amount: 120, card_name: "Nubank Ana" },
      ],
    },
  },
]

vi.mock("@/hooks/useAnalytics", () => ({
  useExpenseProjectionDetail: () => ({ loading: false, error: false, data: mockData }),
}))

describe("ProjecaoDetalhe — AccordionGroup", () => {
  it("exibe header do grupo de fixos com label da categoria", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.getByText(/Casa/i)).toBeInTheDocument()
    expect(screen.getByText(/Saúde/i)).toBeInTheDocument()
  })

  it("itens de fixos ficam ocultos por padrão", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.queryByText("Aluguel")).not.toBeInTheDocument()
    expect(screen.queryByText("Plano de saúde")).not.toBeInTheDocument()
  })

  it("expande grupo de fixos ao clicar no header", () => {
    render(<ProjecaoDetalhe />)
    fireEvent.click(screen.getByRole("button", { name: /Casa/i }))
    expect(screen.getByText("Aluguel")).toBeInTheDocument()
    expect(screen.getByText("Condomínio")).toBeInTheDocument()
    expect(screen.queryByText("Plano de saúde")).not.toBeInTheDocument()
  })

  it("exibe header do cartão com nome do cartão", () => {
    render(<ProjecaoDetalhe />)
    // dois grupos "Nubank Ana": cartão e parcelas
    expect(screen.getAllByText(/Nubank Ana/i).length).toBeGreaterThanOrEqual(2)
  })

  it("itens de cartão ficam ocultos por padrão", () => {
    render(<ProjecaoDetalhe />)
    expect(screen.queryByText("Claude")).not.toBeInTheDocument()
    expect(screen.queryByText("Notebook")).not.toBeInTheDocument()
  })

  it("expande parcelas ao clicar e mostra installment_info", () => {
    render(<ProjecaoDetalhe />)
    // pega o primeiro botão com texto "Nubank Ana" (parcelas)
    const btns = screen.getAllByRole("button", { name: /Nubank Ana/i })
    fireEvent.click(btns[0])
    expect(screen.getByText("parcela 3/10")).toBeInTheDocument()
  })
})
```

---

- [ ] **Step 3: Rodar os testes para confirmar que falham**

```bash
cd frontend
npm run test -- ProjecaoDetalhe
```

Resultado esperado: **FAIL** — `AccordionGroup` não existe.

---

- [ ] **Step 4: Implementar o `AccordionGroup` em `ProjecaoDetalhe.tsx`**

Abra `frontend/src/pages/ProjecaoDetalhe.tsx`.

Remova o componente `ItemList` inteiro e substitua pelos novos helpers e pelo componente `AccordionGroup`. Coloque o código antes da linha `const COLOR_FIXED`:

```tsx
const CATEGORY_ICONS: Record<string, string> = {
  casa: "🏠",
  saude: "❤️",
  lazer: "🎮",
  impostos: "📋",
  transporte: "🚗",
  educacao: "📚",
  servicos: "⚙️",
  outros: "📦",
}

const CATEGORY_LABELS: Record<string, string> = {
  casa: "Casa",
  saude: "Saúde",
  lazer: "Lazer",
  impostos: "Impostos",
  transporte: "Transporte",
  educacao: "Educação",
  servicos: "Serviços",
  outros: "Outros",
}

type GroupByField = "category_label" | "card_name"

function AccordionGroup({
  items,
  groupBy,
  color,
}: {
  items: ProjectionItem[]
  groupBy: GroupByField
  color: string
}) {
  const fmt = useFmt()
  const isCard = groupBy === "card_name"

  const groups = items.reduce<Record<string, ProjectionItem[]>>((acc, item) => {
    const key = (item[groupBy] ?? "outros") || "outros"
    acc[key] = [...(acc[key] ?? []), item]
    return acc
  }, {})

  const [open, setOpen] = useState<Record<string, boolean>>({})
  const toggle = (key: string) =>
    setOpen((prev) => ({ ...prev, [key]: !prev[key] }))

  if (Object.keys(groups).length === 0) return null

  return (
    <div className="space-y-2">
      {Object.entries(groups).map(([key, groupItems]) => {
        const subtotal = groupItems.reduce((s, i) => s + i.amount, 0)
        const expanded = open[key] ?? false
        const icon = isCard ? "💳" : (CATEGORY_ICONS[key] ?? "📦")
        const label = isCard ? key : (CATEGORY_LABELS[key] ?? key)

        return (
          <div key={key} className="rounded-lg overflow-hidden border border-border">
            <button
              className="w-full flex items-center justify-between px-4 py-3 bg-card hover:bg-muted/50 transition-colors"
              onClick={() => toggle(key)}
            >
              <span className="text-sm font-medium text-foreground">
                {icon} {label}
              </span>
              <div className="flex items-center gap-2">
                <span className="text-sm font-bold" style={{ color }}>
                  {fmt(subtotal)}
                </span>
                <span className="text-xs text-muted-foreground">
                  {expanded ? "▼" : "▶"}
                </span>
              </div>
            </button>
            {expanded && (
              <div>
                {groupItems.map((item, i) => (
                  <div
                    key={i}
                    className="flex items-center justify-between px-4 py-2 pl-8 bg-card border-t border-border"
                  >
                    <div>
                      <p className="text-sm text-foreground">{item.name}</p>
                      {item.installment_info && (
                        <p className="text-xs text-muted-foreground">
                          parcela {item.installment_info}
                        </p>
                      )}
                    </div>
                    <span className="text-sm font-semibold" style={{ color }}>
                      {fmt(item.amount)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}
```

Adicione `useState` ao import de react no topo do arquivo:

```tsx
import { useState } from "react"
```

---

- [ ] **Step 5: Substituir os usos de `ItemList` por `AccordionGroup` na página**

Ainda em `ProjecaoDetalhe.tsx`, localize os três blocos de `<ItemList>` dentro do `TabsContent` e substitua-os:

```tsx
{/* Fixed items */}
{m.items.fixed.length > 0 && (
  <div className="space-y-2">
    <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_FIXED }}>
      Fixos
    </p>
    <AccordionGroup items={m.items.fixed} groupBy="category_label" color={COLOR_FIXED} />
  </div>
)}

{/* Installment items */}
{m.items.installments.length > 0 && (
  <div className="space-y-2">
    <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_INSTALLMENTS }}>
      Parcelas
    </p>
    <AccordionGroup items={m.items.installments} groupBy="card_name" color={COLOR_INSTALLMENTS} />
  </div>
)}

{/* Card items */}
{m.items.card.length > 0 && (
  <div className="space-y-2">
    <p className="text-xs font-bold uppercase tracking-wide" style={{ color: COLOR_CARD }}>
      Cartão
    </p>
    <AccordionGroup items={m.items.card} groupBy="card_name" color={COLOR_CARD} />
  </div>
)}
```

---

- [ ] **Step 6: Rodar os testes para confirmar que passam**

```bash
cd frontend
npm run test -- ProjecaoDetalhe
```

Resultado esperado: todos os testes **PASS**.

---

- [ ] **Step 7: Commit**

```bash
git add frontend/src/hooks/useAnalytics.ts \
        frontend/src/pages/ProjecaoDetalhe.tsx \
        frontend/src/pages/ProjecaoDetalhe.test.tsx
git commit -m "feat(projecao): substitui listas planas por acordeões agrupados por categoria e cartão"
```

---

## Verificação final

Após os três tasks:

1. Acesse a aplicação em `http://localhost:5173`
2. Navegue para Gestão → Projeção
3. Confirme que os fixos aparecem agrupados por categoria (ex: 🏠 Casa) com o subtotal
4. Confirme que os acordeões iniciam fechados
5. Toque em um grupo e confirme que os itens aparecem
6. Confirme que cartão e parcelas mostram o nome do cartão (ex: 💳 Nubank Ana)
